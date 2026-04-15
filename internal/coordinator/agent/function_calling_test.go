package agent

import (
	"encoding/json"
	"testing"

	"github.com/tp/cowork/internal/shared/models"
)

func TestToolCallsArrayJSON(t *testing.T) {
	// 测试 ToolCallsArray 的 JSON 序列化和反序列化
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

	// 序列化
	data, err := json.Marshal(toolCalls)
	if err != nil {
		t.Fatalf("Failed to marshal ToolCallsArray: %v", err)
	}

	// 反序列化
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

	if result[0].Function.Name != "execute_shell" {
		t.Errorf("Expected function name 'execute_shell', got '%s'", result[0].Function.Name)
	}
}

func TestParseToolCallsFromOpenAI(t *testing.T) {
	// 测试从 OpenAI 响应解析 Tool Calls
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

	// 使用 LLMClient 的方法
	client := NewLLMClient(nil, 0)
	toolCalls := client.ParseToolCallsFromOpenAI(resp)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].ID != "call_abc" {
		t.Errorf("Expected ID 'call_abc', got '%s'", toolCalls[0].ID)
	}

	if toolCalls[0].Function.Name != "read_file" {
		t.Errorf("Expected function name 'read_file', got '%s'", toolCalls[0].Function.Name)
	}
}

func TestParseToolCallsFromAnthropic(t *testing.T) {
	// 测试从 Anthropic 响应解析 Tool Use
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

	client := NewLLMClient(nil, 0)
	toolCalls := client.ParseToolCallsFromAnthropic(resp)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].ID != "toolu_123" {
		t.Errorf("Expected ID 'toolu_123', got '%s'", toolCalls[0].ID)
	}

	if toolCalls[0].Function.Name != "write_file" {
		t.Errorf("Expected function name 'write_file', got '%s'", toolCalls[0].Function.Name)
	}
}

func TestConvertToAgentMessages(t *testing.T) {
	// 测试数据库消息转换为聊天消息
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
		{
			Role:    "user",
			Content: "Hello",
		},
		{
			Role:      "assistant",
			Content:   "I'll help you",
			ToolCalls: &toolCalls,
		},
		{
			Role:       "tool",
			Content:    `{"task_id": "task-001"}`,
			ToolCallID: "call_xyz",
		},
	}

	chatMessages := ConvertToAgentMessages(dbMessages)

	if len(chatMessages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(chatMessages))
	}

	// 检查用户消息
	if chatMessages[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", chatMessages[0].Role)
	}

	// 检查助手消息（带 ToolCalls）
	if chatMessages[1].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", chatMessages[1].Role)
	}
	if len(chatMessages[1].ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(chatMessages[1].ToolCalls))
	}

	// 检查工具结果消息
	if chatMessages[2].Role != "tool" {
		t.Errorf("Expected role 'tool', got '%s'", chatMessages[2].Role)
	}
	if chatMessages[2].ToolCallID != "call_xyz" {
		t.Errorf("Expected tool call ID 'call_xyz', got '%s'", chatMessages[2].ToolCallID)
	}
}

func TestContentBlockJSON(t *testing.T) {
	// 测试 ContentBlock 的 JSON 序列化
	blocks := []ContentBlock{
		{
			Type: "text",
			Text: "Hello, I'll help you with that.",
		},
		{
			Type:  "tool_use",
			ID:    "tool_001",
			Name:  "execute_shell",
			Input: json.RawMessage(`{"command": "pwd"}`),
		},
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

func TestBuildToolResultMessage(t *testing.T) {
	msg := BuildToolResultMessage("call_123", "read_file", "file contents here")

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

func TestBuildOpenAIRequest(t *testing.T) {
	// Simple test without registry dependency
	client := NewLLMClient(nil, 0)

	messages := []ChatMessage{
		{Role: "user", Content: "帮我查看当前目录"},
	}

	req, err := client.BuildOpenAIRequest("gpt-4", messages, "You are a helpful assistant.", nil)
	if err != nil {
		t.Fatalf("Failed to build OpenAI request: %v", err)
	}

	if req.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got %s", req.Model)
	}

	if len(req.Messages) != 2 {
		t.Errorf("Expected 2 messages (system + user), got %d", len(req.Messages))
	}

	// 验证系统消息
	if req.Messages[0].Role != "system" {
		t.Errorf("First message should be system, got %s", req.Messages[0].Role)
	}

	// Verify tool_choice is not set when no tools
	if req.ToolChoice != nil {
		t.Errorf("ToolChoice should be nil when no tools specified")
	}
}

func TestBuildAnthropicRequest(t *testing.T) {
	client := NewLLMClient(nil, 0)

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

	reqMap, err := client.BuildAnthropicRequest("claude-3", messages, "You are helpful.", nil)
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