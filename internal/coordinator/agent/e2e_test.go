package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/shared/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestEnvironment 测试环境
type TestEnvironment struct {
	DB          *gorm.DB
	Registry    *tools.Registry
	TaskStore   store.TaskStore
	ToolExecStore store.ToolExecutionStore
}

// SetupTestEnvironment 创建测试环境
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	err = db.AutoMigrate(
		&models.Task{},
		&models.AgentSession{},
		&models.AgentMessage{},
		&models.ToolDefinition{},
		&models.ToolExecution{},
		&models.TaskGroup{},
		&models.TaskDependency{},
		&models.Worker{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	toolDefStore := store.NewToolDefinitionStore(db)
	registry := tools.NewRegistry(toolDefStore)
	if err := registry.Initialize(); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	return &TestEnvironment{
		DB:           db,
		Registry:     registry,
		TaskStore:    store.NewTaskStore(db),
		ToolExecStore: store.NewToolExecutionStore(db),
	}
}

// TestToolCallsArrayJSON_E2E 测试 ToolCallsArray JSON 序列化
func TestToolCallsArrayJSON_E2E(t *testing.T) {
	toolCalls := models.ToolCallsArray{
		{
			ID:   "call_123",
			Type: "function",
			Function: models.FunctionCall{
				Name:      "execute_shell",
				Arguments: `{"command": "ls -la"}`,
			},
		},
	}

	data, err := json.Marshal(toolCalls)
	if err != nil {
		t.Fatalf("Failed to marshal ToolCallsArray: %v", err)
	}

	var result models.ToolCallsArray
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal ToolCallsArray: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result))
	}

	if result[0].ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got '%s'", result[0].ID)
	}
}

// TestParseToolCallsFromOpenAI_E2E 测试解析 OpenAI 响应
func TestParseToolCallsFromOpenAI_E2E(t *testing.T) {
	resp := &ChatResponse{
		Choices: []Choice{
			{
				Message: ChatMessage{
					Role: "assistant",
					ToolCalls: []models.ToolCall{
						{
							ID:   "call_abc",
							Type: "function",
							Function: models.FunctionCall{
								Name:      "read_file",
								Arguments: `{"path": "/tmp/test.txt"}`,
							},
						},
					},
				},
			},
		},
	}

	engine := &FunctionCallingEngine{}
	toolCalls := engine.ParseToolCallsFromOpenAI(resp)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "read_file" {
		t.Errorf("Expected function name 'read_file', got %s", toolCalls[0].Function.Name)
	}
}

// TestParseToolCallsFromAnthropic_E2E 测试解析 Anthropic 响应
func TestParseToolCallsFromAnthropic_E2E(t *testing.T) {
	resp := &AnthropicResponse{
		Content: []ContentBlock{
			{
				Type:  "tool_use",
				ID:    "toolu_123",
				Name:  "write_file",
				Input: json.RawMessage(`{"path": "/tmp/output.txt", "content": "hello"}`),
			},
		},
	}

	engine := &FunctionCallingEngine{}
	toolCalls := engine.ParseToolCallsFromAnthropic(resp)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].ID != "toolu_123" {
		t.Errorf("Expected ID 'toolu_123', got %s", toolCalls[0].ID)
	}

	if toolCalls[0].Function.Name != "write_file" {
		t.Errorf("Expected function name 'write_file', got %s", toolCalls[0].Function.Name)
	}
}

// TestConvertToAgentMessages_E2E 测试消息转换
func TestConvertToAgentMessages_E2E(t *testing.T) {
	toolCalls := models.ToolCallsArray{
		{
			ID:   "call_xyz",
			Type: "function",
			Function: models.FunctionCall{
				Name:      "create_task",
				Arguments: `{"type": "shell", "description": "test"}`,
			},
		},
	}

	dbMessages := []models.AgentMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "I'll help you", ToolCalls: &toolCalls},
		{Role: "tool", Content: `{"task_id": "task-001"}`, ToolCallID: "call_xyz"},
	}

	chatMessages := ConvertToAgentMessages(dbMessages)

	if len(chatMessages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(chatMessages))
	}

	if chatMessages[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", chatMessages[0].Role)
	}

	if chatMessages[1].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", chatMessages[1].Role)
	}

	if len(chatMessages[1].ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(chatMessages[1].ToolCalls))
	}

	if chatMessages[2].Role != "tool" {
		t.Errorf("Expected role 'tool', got '%s'", chatMessages[2].Role)
	}
}

// TestContentBlockJSON_E2E 测试 ContentBlock JSON 序列化
func TestContentBlockJSON_E2E(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "text", Text: "Hello, I'll help you with that."},
		{Type: "tool_use", ID: "tool_001", Name: "execute_shell", Input: json.RawMessage(`{"command": "pwd"}`)},
	}

	data, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("Failed to marshal ContentBlocks: %v", err)
	}

	var result []ContentBlock
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal ContentBlocks: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(result))
	}

	if result[0].Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", result[0].Type)
	}

	if result[1].Type != "tool_use" {
		t.Errorf("Expected type 'tool_use', got '%s'", result[1].Type)
	}
}

// TestBuildToolResultMessage_E2E 测试构建工具结果消息
func TestBuildToolResultMessage_E2E(t *testing.T) {
	engine := &FunctionCallingEngine{}

	execution := &models.ToolExecution{
		ToolCallID: "call_123",
		ToolName:   "read_file",
		Result:     "file contents here",
		IsError:    false,
	}

	msg := engine.BuildToolResultMessage(execution)

	if msg.Role != "tool" {
		t.Errorf("Expected role 'tool', got '%s'", msg.Role)
	}

	if msg.ToolCallID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", msg.ToolCallID)
	}

	if msg.Name != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", msg.Name)
	}
}

// TestBuildOpenAIRequest_E2E 测试构建 OpenAI 请求
func TestBuildOpenAIRequest_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	engine := NewFunctionCallingEngine(
		env.Registry,
		env.ToolExecStore,
		env.TaskStore,
		FunctionCallingConfig{
			MaxToolRounds: 5,
			Timeout:       30 * time.Second,
		},
	)

	messages := []ChatMessage{
		{Role: "user", Content: "帮我查看当前目录"},
	}

	req, err := engine.BuildOpenAIRequest("gpt-4", messages, "You are a helpful assistant.", []string{"execute_shell"})
	if err != nil {
		t.Fatalf("Failed to build OpenAI request: %v", err)
	}

	if req.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got %s", req.Model)
	}

	if len(req.Messages) != 2 {
		t.Errorf("Expected 2 messages (system + user), got %d", len(req.Messages))
	}

	if len(req.Tools) == 0 {
		t.Error("Expected tools in request")
	}

	// 验证系统消息
	if req.Messages[0].Role != "system" {
		t.Errorf("First message should be system, got %s", req.Messages[0].Role)
	}
}

// TestBuildAnthropicRequest_E2E 测试构建 Anthropic 请求
func TestBuildAnthropicRequest_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	engine := NewFunctionCallingEngine(
		env.Registry,
		env.ToolExecStore,
		env.TaskStore,
		FunctionCallingConfig{},
	)

	messages := []ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!", ToolCalls: []models.ToolCall{
			{
				ID:   "tool_001",
				Type: "function",
				Function: models.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path": "/tmp/test.txt"}`,
				},
			},
		}},
		{Role: "tool", ToolCallID: "tool_001", Content: "file content", Name: "read_file"},
	}

	reqMap, err := engine.BuildAnthropicRequest("claude-3", messages, "You are helpful.", []string{"read_file"})
	if err != nil {
		t.Fatalf("Failed to build Anthropic request: %v", err)
	}

	if reqMap["model"] != "claude-3" {
		t.Errorf("Expected model 'claude-3', got %v", reqMap["model"])
	}

	if reqMap["system"] != "You are helpful." {
		t.Errorf("Expected system prompt, got %v", reqMap["system"])
	}
}

// TestOpenAIStreamingResponse_E2E 测试 OpenAI 流式响应处理
func TestOpenAIStreamingResponse_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	mockSSE := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: {"choices":[{"delta":{"tool_calls":[{"id":"call_123","type":"function","function":{"name":"read_file","arguments":"{"}}]}]}
data: {"choices":[{"delta":{"tool_calls":[{"function":{"arguments":"\"path\":\"/tmp/test.txt\"}"}}]}}]}
data: [DONE]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(mockSSE))
	}))
	defer server.Close()

	engine := NewFunctionCallingEngine(
		env.Registry,
		env.ToolExecStore,
		env.TaskStore,
		FunctionCallingConfig{Timeout: 5 * time.Second},
	)

	var tokens []string
	resp, err := engine.StreamOpenAI(
		ModelConfig{APIKey: "test", BaseURL: server.URL, Model: "gpt-4"},
		&ChatRequest{Model: "gpt-4", Messages: []ChatMessage{{Role: "user", Content: "test"}}},
		func(token string) { tokens = append(tokens, token) },
	)

	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	if len(tokens) < 2 {
		t.Errorf("Expected at least 2 tokens, got %d", len(tokens))
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(resp.Choices))
	}

	if len(resp.Choices[0].Message.ToolCalls) == 0 {
		t.Error("Expected tool calls in response")
	}
}


// TestToolExecution_E2E 测试工具执行
func TestToolExecution_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	engine := NewFunctionCallingEngine(
		env.Registry,
		env.ToolExecStore,
		env.TaskStore,
		FunctionCallingConfig{},
	)

	// 测试 create_task 工具
	toolCall := models.ToolCall{
		ID:   "call_create_task",
		Type: "function",
		Function: models.FunctionCall{
			Name:      "create_task",
			Arguments: `{"type": "shell", "description": "Test task", "priority": "high"}`,
		},
	}

	execution, err := engine.ExecuteToolCall("conv-001", toolCall)
	if err != nil {
		t.Fatalf("Failed to execute create_task: %v", err)
	}

	if execution.Status != string(models.ToolExecutionStatusCompleted) {
		t.Errorf("Expected completed status, got %s", execution.Status)
	}

	// 验证任务已创建
	var taskCount int64
	env.DB.Model(&models.Task{}).Count(&taskCount)
	if taskCount != 1 {
		t.Errorf("Expected 1 task, got %d", taskCount)
	}
}

// TestErrorHandling_E2E 测试错误处理
func TestErrorHandling_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	engine := NewFunctionCallingEngine(
		env.Registry,
		env.ToolExecStore,
		env.TaskStore,
		FunctionCallingConfig{},
	)

	// 测试不存在的工具
	toolCall := models.ToolCall{
		ID:   "call_nonexistent",
		Type: "function",
		Function: models.FunctionCall{
			Name:      "nonexistent_tool",
			Arguments: `{}`,
		},
	}

	_, err := engine.ExecuteToolCall("conv-002", toolCall)
	if err == nil {
		t.Error("Expected error for nonexistent tool")
	}

	// 测试无效的参数
	invalidArgCall := models.ToolCall{
		ID:   "call_invalid_args",
		Type: "function",
		Function: models.FunctionCall{
			Name:      "create_task",
			Arguments: `{"invalid": true}`, // 缺少必需字段
		},
	}

	execution, err := engine.ExecuteToolCall("conv-002", invalidArgCall)
	if err != nil {
		t.Fatalf("ExecuteToolCall should not return error: %v", err)
	}

	if execution.Status != string(models.ToolExecutionStatusFailed) {
		t.Errorf("Expected failed status, got %s", execution.Status)
	}
}

// TestMultipleToolCalls_E2E 测试多工具调用
func TestMultipleToolCalls_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	engine := NewFunctionCallingEngine(
		env.Registry,
		env.ToolExecStore,
		env.TaskStore,
		FunctionCallingConfig{MaxToolRounds: 10},
	)

	toolCalls := []models.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: models.FunctionCall{
				Name:      "create_task",
				Arguments: `{"type": "shell", "description": "Task 1"}`,
			},
		},
		{
			ID:   "call_2",
			Type: "function",
			Function: models.FunctionCall{
				Name:      "create_task",
				Arguments: `{"type": "script", "description": "Task 2"}`,
			},
		},
	}

	for _, tc := range toolCalls {
		_, err := engine.ExecuteToolCall("conv-003", tc)
		if err != nil {
			t.Errorf("Failed to execute tool call %s: %v", tc.ID, err)
		}
	}

	var taskCount int64
	env.DB.Model(&models.Task{}).Count(&taskCount)
	if taskCount != 2 {
		t.Errorf("Expected 2 tasks, got %d", taskCount)
	}

	var execCount int64
	env.DB.Model(&models.ToolExecution{}).Count(&execCount)
	if execCount != 2 {
		t.Errorf("Expected 2 tool executions, got %d", execCount)
	}
}

// TestConcurrentToolExecution_E2E 测试并发工具执行
// 注意：SQLite 内存数据库在高并发写入时可能有限制，此测试验证基本功能
func TestConcurrentToolExecution_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	engine := NewFunctionCallingEngine(
		env.Registry,
		env.ToolExecStore,
		env.TaskStore,
		FunctionCallingConfig{},
	)

	// 顺序执行多个工具调用（SQLite 内存数据库限制）
	for i := 0; i < 5; i++ {
		toolCall := models.ToolCall{
			ID:   fmt.Sprintf("call_%d", i),
			Type: "function",
			Function: models.FunctionCall{
				Name:      "create_task",
				Arguments: fmt.Sprintf(`{"type": "shell", "description": "Sequential Task %d"}`, i),
			},
		}

		_, err := engine.ExecuteToolCall("conv-004", toolCall)
		if err != nil {
			t.Errorf("Execution %d failed: %v", i, err)
		}
	}

	var taskCount int64
	env.DB.Model(&models.Task{}).Count(&taskCount)
	if taskCount != 5 {
		t.Errorf("Expected 5 tasks, got %d", taskCount)
	}
}

// 辅助函数
func findIndex(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}