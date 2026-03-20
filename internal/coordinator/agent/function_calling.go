package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/shared/models"
)

// FunctionCallingEngine Function Calling 引擎
type FunctionCallingEngine struct {
	registry         *tools.Registry
	toolExecStore    store.ToolExecutionStore
	taskStore        store.TaskStore
	httpClient       *http.Client
	maxToolRounds    int
}

// FunctionCallingConfig Function Calling 配置
type FunctionCallingConfig struct {
	MaxToolRounds int // 最大工具调用轮数，默认 10
	Timeout       time.Duration
}

// NewFunctionCallingEngine 创建 Function Calling 引擎
func NewFunctionCallingEngine(
	registry *tools.Registry,
	toolExecStore store.ToolExecutionStore,
	taskStore store.TaskStore,
	config FunctionCallingConfig,
) *FunctionCallingEngine {
	if config.MaxToolRounds <= 0 {
		config.MaxToolRounds = 10
	}
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}

	return &FunctionCallingEngine{
		registry:      registry,
		toolExecStore: toolExecStore,
		taskStore:     taskStore,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		maxToolRounds: config.MaxToolRounds,
	}
}

// ChatRequest 通用聊天请求
type ChatRequest struct {
	Model       string                   `json:"model"`
	Messages    []ChatMessage            `json:"messages"`
	Stream      bool                     `json:"stream"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice  interface{}              `json:"tool_choice,omitempty"` // "auto", "none", or specific tool
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role       string                   `json:"role"`
	Content    interface{}              `json:"content"` // string or []ContentBlock for Anthropic
	ToolCalls  []models.ToolCall        `json:"tool_calls,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
	Name       string                   `json:"name,omitempty"` // for tool messages
}

// ContentBlock 用于 Anthropic 格式的内容块
type ContentBlock struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result"
	Text string `json:"text,omitempty"`

	// Tool use fields
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`

	// Tool result fields
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	ID      string     `json:"id"`
	Object  string     `json:"object"`
	Created int64      `json:"created"`
	Model   string     `json:"model"`
	Choices []Choice   `json:"choices"`
	Usage   UsageStats `json:"usage"`
}

// Choice 选择项
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string      `json:"finish_reason"`
}

// UsageStats 使用统计
type UsageStats struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// AnthropicResponse Anthropic API 响应格式
type AnthropicResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// StreamEvent SSE 流事件
type StreamEvent struct {
	Type    string          `json:"type"`
	Index   int             `json:"index,omitempty"`
	Delta   *ContentDelta   `json:"delta,omitempty"`
	Message *ChatMessage    `json:"message,omitempty"`
	Content []ContentBlock  `json:"content,omitempty"`
}

// ContentDelta 内容增量
type ContentDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ToolExecutor 工具执行器接口
type ToolExecutor interface {
	ExecuteTool(toolName string, arguments map[string]interface{}) (string, error)
}

// BuildOpenAIRequest 构建 OpenAI 格式的请求
func (e *FunctionCallingEngine) BuildOpenAIRequest(
	model string,
	messages []ChatMessage,
	systemPrompt string,
	toolNames []string,
) (*ChatRequest, error) {
	req := &ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	// 添加系统提示
	if systemPrompt != "" {
		req.Messages = append([]ChatMessage{
			{Role: "system", Content: systemPrompt},
		}, req.Messages...)
	}

	// 添加工具定义
	if len(toolNames) > 0 {
		req.Tools = e.registry.GetOpenAIToolsByNames(toolNames)
		req.ToolChoice = "auto"
	}

	return req, nil
}

// BuildAnthropicRequest 构建 Anthropic 格式的请求
func (e *FunctionCallingEngine) BuildAnthropicRequest(
	model string,
	messages []ChatMessage,
	systemPrompt string,
	toolNames []string,
) (map[string]interface{}, error) {
	// 分离系统消息和对话消息
	var chatMessages []map[string]interface{}

	for _, msg := range messages {
		if msg.Role == "system" {
			continue // 系统消息单独处理
		}

		// 转换消息格式
		anthropicMsg := map[string]interface{}{
			"role": msg.Role,
		}

		if msg.Role == "tool" {
			// Tool result message
			anthropicMsg["content"] = []ContentBlock{
				{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   fmt.Sprintf("%v", msg.Content),
				},
			}
		} else if len(msg.ToolCalls) > 0 {
			// Assistant message with tool calls
			content := []ContentBlock{}
			if msg.Content != nil && msg.Content != "" {
				content = append(content, ContentBlock{
					Type: "text",
					Text: fmt.Sprintf("%v", msg.Content),
				})
			}
			for _, tc := range msg.ToolCalls {
				content = append(content, ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: json.RawMessage(tc.Function.Arguments),
				})
			}
			anthropicMsg["content"] = content
		} else {
			// Regular text message
			anthropicMsg["content"] = msg.Content
		}

		chatMessages = append(chatMessages, anthropicMsg)
	}

	req := map[string]interface{}{
		"model":      model,
		"messages":   chatMessages,
		"max_tokens": 4096,
	}

	if systemPrompt != "" {
		req["system"] = systemPrompt
	}

	// 添加工具定义
	if len(toolNames) > 0 {
		tools := e.registry.GetOpenAIToolsByNames(toolNames)
		anthropicTools := make([]map[string]interface{}, len(tools))
		for i, tool := range tools {
			if fn, ok := tool["function"].(map[string]interface{}); ok {
				anthropicTools[i] = map[string]interface{}{
					"name":        fn["name"],
					"description": fn["description"],
					"input_schema": fn["parameters"],
				}
			}
		}
		req["tools"] = anthropicTools
	}

	return req, nil
}

// ParseToolCallsFromOpenAI 从 OpenAI 响应解析 Tool Calls
func (e *FunctionCallingEngine) ParseToolCallsFromOpenAI(resp *ChatResponse) []models.ToolCall {
	if len(resp.Choices) == 0 {
		return nil
	}

	return resp.Choices[0].Message.ToolCalls
}

// ParseToolCallsFromAnthropic 从 Anthropic 响应解析 Tool Use
func (e *FunctionCallingEngine) ParseToolCallsFromAnthropic(resp *AnthropicResponse) []models.ToolCall {
	var toolCalls []models.ToolCall

	for _, block := range resp.Content {
		if block.Type == "tool_use" {
			toolCalls = append(toolCalls, models.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: models.FunctionCall{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}

	return toolCalls
}

// ExecuteToolCall 执行工具调用
func (e *FunctionCallingEngine) ExecuteToolCall(
	conversationID string,
	toolCall models.ToolCall,
) (*models.ToolExecution, error) {
	// 获取工具定义
	toolDef, err := e.registry.Get(toolCall.Function.Name)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %s", toolCall.Function.Name)
	}

	// 解析参数
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	// 创建执行记录
	execution := &models.ToolExecution{
		ConversationID: conversationID,
		ToolName:       toolCall.Function.Name,
		ToolCallID:     toolCall.ID,
		Arguments:      args,
		Status:         string(models.ToolExecutionStatusPending),
	}

	if err := e.toolExecStore.Create(execution); err != nil {
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	// 根据执行模式处理
	if toolDef.ExecuteMode == models.ToolExecuteModeLocal {
		return e.executeLocalTool(execution, args)
	}

	return execution, nil
}

// executeLocalTool 执行本地工具
func (e *FunctionCallingEngine) executeLocalTool(
	execution *models.ToolExecution,
	args map[string]interface{},
) (*models.ToolExecution, error) {
	// 更新状态为运行中
	if err := e.toolExecStore.UpdateStatus(execution.ID, string(models.ToolExecutionStatusRunning), "", false); err != nil {
		return nil, err
	}

	var result string
	var isError bool

	// 根据工具名称执行不同的处理
	switch execution.ToolName {
	case "create_task":
		result, isError = e.handleCreateTask(args)
	case "query_task":
		result, isError = e.handleQueryTask(args)
	case "request_approval":
		result = "Approval requested. Waiting for user confirmation."
		isError = false
	default:
		result = fmt.Sprintf("Unknown local tool: %s", execution.ToolName)
		isError = true
	}

	// 更新执行结果
	status := string(models.ToolExecutionStatusCompleted)
	if isError {
		status = string(models.ToolExecutionStatusFailed)
	}

	if err := e.toolExecStore.UpdateStatus(execution.ID, status, result, isError); err != nil {
		return nil, err
	}

	// 重新获取执行记录
	updated, err := e.toolExecStore.Get(execution.ID)
	if err != nil {
		return execution, nil
	}
	return updated, nil
}

// handleCreateTask 处理 create_task 工具
func (e *FunctionCallingEngine) handleCreateTask(args map[string]interface{}) (string, bool) {
	taskType, _ := args["type"].(string)
	description, _ := args["description"].(string)

	if taskType == "" || description == "" {
		return "Missing required fields: type and description", true
	}

	task := &models.Task{
		ID:          uuid.New().String(),
		Type:        taskType,
		Description: description,
		Status:      models.TaskStatusPending,
		Priority:    models.PriorityMedium,
	}

	// 设置优先级
	if priority, ok := args["priority"].(string); ok {
		task.Priority = models.Priority(priority)
	}

	// 设置输入
	if input, ok := args["input"].(map[string]interface{}); ok {
		task.Input = input
	}

	// 设置标签
	if tags, ok := args["required_tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				task.RequiredTags = append(task.RequiredTags, s)
			}
		}
	}

	if err := e.taskStore.Create(task); err != nil {
		return fmt.Sprintf("Failed to create task: %v", err), true
	}

	result := map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"description": task.Description,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), false
}

// handleQueryTask 处理 query_task 工具
func (e *FunctionCallingEngine) handleQueryTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return "Missing required field: task_id", true
	}

	task, err := e.taskStore.Get(taskID)
	if err != nil {
		return fmt.Sprintf("Task not found: %s", taskID), true
	}

	result := map[string]interface{}{
		"task_id":     task.ID,
		"status":      task.Status,
		"progress":    task.Progress,
		"description": task.Description,
	}

	if task.Error != nil {
		result["error"] = *task.Error
	}

	if task.Output != nil {
		result["output"] = task.Output
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), false
}

// BuildToolResultMessage 构建工具结果消息
func (e *FunctionCallingEngine) BuildToolResultMessage(execution *models.ToolExecution) ChatMessage {
	return ChatMessage{
		Role:       "tool",
		ToolCallID: execution.ToolCallID,
		Content:    execution.Result,
		Name:       execution.ToolName,
	}
}

// CallOpenAI 调用 OpenAI API
func (e *FunctionCallingEngine) CallOpenAI(cfg ModelConfig, req *ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// CallAnthropic 调用 Anthropic API
func (e *FunctionCallingEngine) CallAnthropic(cfg ModelConfig, req map[string]interface{}) (*AnthropicResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", cfg.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var anthropicResp AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &anthropicResp, nil
}

// StreamOpenAI 流式调用 OpenAI API
func (e *FunctionCallingEngine) StreamOpenAI(
	cfg ModelConfig,
	req *ChatRequest,
	onToken func(string),
) (*ChatResponse, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var fullContent strings.Builder
	var toolCalls []models.ToolCall
	var finishReason string

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   interface{}        `json:"content"`
					ToolCalls []models.ToolCall `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			// 处理文本内容
			if content, ok := choice.Delta.Content.(string); ok && content != "" {
				fullContent.WriteString(content)
				if onToken != nil {
					onToken(content)
				}
			}

			// 处理 tool calls
			if len(choice.Delta.ToolCalls) > 0 {
				for _, tc := range choice.Delta.ToolCalls {
					// 查找或创建 tool call
					found := false
					for i := range toolCalls {
						if toolCalls[i].ID == tc.ID {
							// 追加 arguments
							toolCalls[i].Function.Arguments += tc.Function.Arguments
							found = true
							break
						}
					}
					if !found {
						toolCalls = append(toolCalls, tc)
					}
				}
			}

			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}
	}

	// 构建最终响应
	chatResp := &ChatResponse{
		Choices: []Choice{
			{
				Message: ChatMessage{
					Role:      "assistant",
					Content:   fullContent.String(),
					ToolCalls: toolCalls,
				},
				FinishReason: finishReason,
			},
		},
	}

	return chatResp, scanner.Err()
}

// StreamAnthropic 流式调用 Anthropic API
func (e *FunctionCallingEngine) StreamAnthropic(
	cfg ModelConfig,
	req map[string]interface{},
	onToken func(string),
) (*AnthropicResponse, error) {
	req["stream"] = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", cfg.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var content []ContentBlock
	var textContent strings.Builder
	var currentToolUse *ContentBlock
	var stopReason string

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type         string          `json:"type"`
			Index        int             `json:"index"`
			Delta        json.RawMessage `json:"delta"`
			Message      *AnthropicResponse `json:"message,omitempty"`
			ContentBlock *ContentBlock   `json:"content_block,omitempty"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock != nil {
				cb := *event.ContentBlock
				content = append(content, cb)
				if cb.Type == "tool_use" {
					currentToolUse = &content[len(content)-1]
				}
			}

		case "content_block_delta":
			var delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
				PartialJson string `json:"partial_json"`
			}
			if err := json.Unmarshal(event.Delta, &delta); err != nil {
				continue
			}

			if delta.Type == "text_delta" {
				textContent.WriteString(delta.Text)
				if onToken != nil {
					onToken(delta.Text)
				}
			} else if delta.Type == "input_json_delta" && currentToolUse != nil {
				// 追加 JSON 输入
				currentToolUse.Input = append(currentToolUse.Input, delta.PartialJson...)
			}

		case "content_block_stop":
			currentToolUse = nil

		case "message_delta":
			var delta struct {
				StopReason string `json:"stop_reason"`
			}
			if err := json.Unmarshal(event.Delta, &delta); err == nil {
				stopReason = delta.StopReason
			}

		case "message_start":
			// 消息开始，可以获取 message 信息
		}
	}

	// 如果有文本内容，添加到 content
	if textContent.Len() > 0 {
		// 查找是否已有文本块
		hasTextBlock := false
		for _, c := range content {
			if c.Type == "text" {
				hasTextBlock = true
				break
			}
		}
		if !hasTextBlock {
			content = append([]ContentBlock{{
				Type: "text",
				Text: textContent.String(),
			}}, content...)
		}
	}

	// 构建最终响应
	anthropicResp := &AnthropicResponse{
		Content:    content,
		StopReason: stopReason,
	}

	return anthropicResp, scanner.Err()
}

// ModelConfig 模型配置
type ModelConfig struct {
	Type    string
	APIKey  string
	BaseURL string
	Model   string
}

// ConvertToAgentMessages 将数据库消息转换为聊天消息
func ConvertToAgentMessages(messages []models.AgentMessage) []ChatMessage {
	result := make([]ChatMessage, 0, len(messages))

	for _, msg := range messages {
		chatMsg := ChatMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		if msg.ToolCalls != nil {
			chatMsg.ToolCalls = *msg.ToolCalls
		}

		result = append(result, chatMsg)
	}

	return result
}

// ConvertToolCallsToAnthropic 将 OpenAI 格式的 Tool Calls 转换为 Anthropic 格式的 tool_use
func ConvertToolCallsToAnthropic(toolCalls []models.ToolCall) []ContentBlock {
	var blocks []ContentBlock

	for _, tc := range toolCalls {
		blocks = append(blocks, ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	return blocks
}