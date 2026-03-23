package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// Message represents a chat message
type Message struct {
	Role    string     `json:"role"`
	Content string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID       string     `json:"id"`
	Type     string     `json:"type"`
	Function Function  `json:"function"`
}

// Function represents a function call
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool represents a tool definition (OpenAI compatible)
type Tool struct {
	Type     string      `json:"type"`
	Function ToolDefFunc `json:"function"`
}

// ToolDefFunc represents the function part of a tool
type ToolDefFunc struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a choice in the response
type Choice struct {
	Index        int          `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
	Delta        *Delta       `json:"delta,omitempty"`
}

// ResponseMessage represents a message in the response
type ResponseMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Delta represents a streaming delta
type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streaming chunk
type StreamChunk struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice represents a choice in a streaming chunk
type ChunkChoice struct {
	Index        int    `json:"index"`
	Delta        Delta  `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

// ToolExecutor defines the interface for executing tools
type ToolExecutor interface {
	ExecuteToolCall(name string, arguments string) (string, error)
}

// AIClient represents an AI API client
type AIClient struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

// NewAIClient creates a new AI client
func NewAIClient(baseURL, model, apiKey string) *AIClient {
	// Ensure baseURL doesn't end with /
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &AIClient{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 0, // No timeout for streaming
		},
	}
}

// Chat sends a chat completion request (non-streaming)
func (c *AIClient) Chat(systemPrompt string, messages []Message, tools []Tool, maxTokens int, temperature float64) (*ChatResponse, error) {
	// Build request messages with system prompt
	reqMessages := make([]Message, 0, len(messages)+1)
	if systemPrompt != "" {
		reqMessages = append(reqMessages, Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	reqMessages = append(reqMessages, messages...)

	req := ChatRequest{
		Model:       c.model,
		Messages:    reqMessages,
		Tools:       tools,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatStream sends a chat completion request with streaming
func (c *AIClient) ChatStream(systemPrompt string, messages []Message, tools []Tool, maxTokens int, temperature float64, onChunk func(string, []ToolCall, string)) error {
	// Build request messages with system prompt
	reqMessages := make([]Message, 0, len(messages)+1)
	if systemPrompt != "" {
		reqMessages = append(reqMessages, Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	reqMessages = append(reqMessages, messages...)

	req := ChatRequest{
		Model:       c.model,
		Messages:    reqMessages,
		Tools:       tools,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse SSE stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip comments
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse data line
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for end of stream
			if data == "[DONE]" {
				break
			}

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				slog.Warn("Failed to parse chunk", "error", err, "data", data)
				continue
			}

			// Extract content and tool calls from chunk
			for _, choice := range chunk.Choices {
				var content string
				var toolCalls []ToolCall
				var finishReason string

				if choice.Delta.Content != "" {
					content = choice.Delta.Content
				}
				if len(choice.Delta.ToolCalls) > 0 {
					toolCalls = choice.Delta.ToolCalls
				}
				if choice.FinishReason != "" {
					finishReason = choice.FinishReason
				}

				onChunk(content, toolCalls, finishReason)
			}
		}
	}

	return scanner.Err()
}

// ChatWithTools sends a chat request and handles tool calls automatically
// Returns the final response content and any tool calls made
func (c *AIClient) ChatWithTools(systemPrompt string, messages []Message, tools []Tool, maxTokens int, temperature float64, executor ToolExecutor, maxIterations int) (string, []ToolCall, error) {
	currentMessages := make([]Message, len(messages))
	copy(currentMessages, messages)

	var allToolCalls []ToolCall
	iteration := 0

	for {
		if iteration >= maxIterations {
			return "", allToolCalls, fmt.Errorf("max iterations (%d) reached", maxIterations)
		}
		iteration++

		// Make request
		resp, err := c.Chat(systemPrompt, currentMessages, tools, maxTokens, temperature)
		if err != nil {
			return "", allToolCalls, err
		}

		if len(resp.Choices) == 0 {
			return "", allToolCalls, fmt.Errorf("no choices in response")
		}

		choice := resp.Choices[0]
		assistantMessage := choice.Message

		// Check for tool calls
		if len(assistantMessage.ToolCalls) == 0 {
			// No tool calls, return the content
			return assistantMessage.Content, allToolCalls, nil
		}

		// Add assistant message with tool calls
		currentMessages = append(currentMessages, Message{
			Role:      "assistant",
			Content:   assistantMessage.Content,
			ToolCalls: assistantMessage.ToolCalls,
		})

		// Execute each tool call
		for _, toolCall := range assistantMessage.ToolCalls {
			slog.Info("Executing tool call", "name", toolCall.Function.Name, "id", toolCall.ID)
			allToolCalls = append(allToolCalls, toolCall)

			var result string
			var isError bool

			if executor != nil {
				output, err := executor.ExecuteToolCall(toolCall.Function.Name, toolCall.Function.Arguments)
				if err != nil {
					result = fmt.Sprintf("Error: %v", err)
					isError = true
				} else {
					result = output
				}
			} else {
				result = "Error: no tool executor configured"
				isError = true
			}

			// Add tool result message
			toolResultContent := result
			if isError {
				toolResultContent = fmt.Sprintf("ERROR: %s", result)
			}

			currentMessages = append(currentMessages, Message{
				Role:       "tool",
				Content:    toolResultContent,
				ToolCallID: toolCall.ID,
			})

			slog.Info("Tool call completed", "name", toolCall.Function.Name, "is_error", isError)
		}
	}
}

// GetModel returns the model name
func (c *AIClient) GetModel() string {
	return c.model
}

// GetBaseURL returns the base URL
func (c *AIClient) GetBaseURL() string {
	return c.baseURL
}