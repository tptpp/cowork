package agent

import (
	"encoding/json"

	"github.com/tp/cowork/internal/shared/models"
)

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
	Index        int          `json:"index"`
	Message      ChatMessage  `json:"message"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason"`
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
	Type    string         `json:"type"`
	Index   int            `json:"index,omitempty"`
	Delta   *ContentDelta  `json:"delta,omitempty"`
	Message *ChatMessage   `json:"message,omitempty"`
	Content []ContentBlock `json:"content,omitempty"`
}

// ContentDelta 内容增量
type ContentDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
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

// BuildToolResultMessage 构建工具结果消息 (helper function for Agent)
func BuildToolResultMessage(toolCallID, toolName, result string) ChatMessage {
	return ChatMessage{
		Role:       "tool",
		ToolCallID: toolCallID,
		Content:    result,
		Name:       toolName,
	}
}