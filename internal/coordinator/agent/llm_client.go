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

	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/shared/models"
)

// LLMClient provides unified API calling for LLM providers
type LLMClient struct {
	httpClient *http.Client
	registry   *tools.Registry
}

// NewLLMClient creates a new LLM client
func NewLLMClient(registry *tools.Registry, timeout time.Duration) *LLMClient {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &LLMClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		registry: registry,
	}
}

// BuildOpenAIRequest builds an OpenAI format request
func (c *LLMClient) BuildOpenAIRequest(
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

	// Add system prompt
	if systemPrompt != "" {
		req.Messages = append([]ChatMessage{
			{Role: "system", Content: systemPrompt},
		}, req.Messages...)
	}

	// Add tool definitions
	if len(toolNames) > 0 {
		req.Tools = c.registry.GetOpenAIToolsByNames(toolNames)
		req.ToolChoice = "auto"
	}

	return req, nil
}

// BuildAnthropicRequest builds an Anthropic format request
func (c *LLMClient) BuildAnthropicRequest(
	model string,
	messages []ChatMessage,
	systemPrompt string,
	toolNames []string,
) (map[string]interface{}, error) {
	// Separate system messages and chat messages
	var chatMessages []map[string]interface{}

	for _, msg := range messages {
		if msg.Role == "system" {
			continue // System messages are handled separately
		}

		// Convert message format
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

	// Add tool definitions
	if len(toolNames) > 0 {
		tools := c.registry.GetOpenAIToolsByNames(toolNames)
		anthropicTools := make([]map[string]interface{}, len(tools))
		for i, tool := range tools {
			if fn, ok := tool["function"].(map[string]interface{}); ok {
				anthropicTools[i] = map[string]interface{}{
					"name":         fn["name"],
					"description":  fn["description"],
					"input_schema": fn["parameters"],
				}
			}
		}
		req["tools"] = anthropicTools
	}

	return req, nil
}

// CallOpenAI calls OpenAI API
func (c *LLMClient) CallOpenAI(cfg ModelConfig, req *ChatRequest) (*ChatResponse, error) {
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

	resp, err := c.httpClient.Do(httpReq)
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

// CallAnthropic calls Anthropic API
func (c *LLMClient) CallAnthropic(cfg ModelConfig, req map[string]interface{}) (*AnthropicResponse, error) {
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

	resp, err := c.httpClient.Do(httpReq)
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

// StreamOpenAI streams OpenAI API calls
func (c *LLMClient) StreamOpenAI(
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

	resp, err := c.httpClient.Do(httpReq)
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
					Content   interface{}       `json:"content"`
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

			// Handle text content
			if content, ok := choice.Delta.Content.(string); ok && content != "" {
				fullContent.WriteString(content)
				if onToken != nil {
					onToken(content)
				}
			}

			// Handle tool calls
			if len(choice.Delta.ToolCalls) > 0 {
				for _, tc := range choice.Delta.ToolCalls {
					// Find or create tool call
					found := false
					for i := range toolCalls {
						if toolCalls[i].ID == tc.ID {
							// Append arguments
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

	// Build final response
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

// StreamAnthropic streams Anthropic API calls
func (c *LLMClient) StreamAnthropic(
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

	resp, err := c.httpClient.Do(httpReq)
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
			Type         string             `json:"type"`
			Index        int                `json:"index"`
			Delta        json.RawMessage    `json:"delta"`
			Message      *AnthropicResponse `json:"message,omitempty"`
			ContentBlock *ContentBlock      `json:"content_block,omitempty"`
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
				Type        string `json:"type"`
				Text        string `json:"text"`
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
				// Append JSON input
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
			// Message start, can get message info
		}
	}

	// If there's text content, add to content
	if textContent.Len() > 0 {
		// Check if text block already exists
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

	// Build final response
	anthropicResp := &AnthropicResponse{
		Content:    content,
		StopReason: stopReason,
	}

	return anthropicResp, scanner.Err()
}

// ParseToolCallsFromAnthropic parses tool use from Anthropic response
func (c *LLMClient) ParseToolCallsFromAnthropic(resp *AnthropicResponse) []models.ToolCall {
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

// ParseToolCallsFromOpenAI parses tool calls from OpenAI response
func (c *LLMClient) ParseToolCallsFromOpenAI(resp *ChatResponse) []models.ToolCall {
	if len(resp.Choices) == 0 {
		return nil
	}

	return resp.Choices[0].Message.ToolCalls
}

