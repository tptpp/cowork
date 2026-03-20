package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/errors"
	"github.com/tp/cowork/internal/shared/models"
)

// AgentHandler Agent API 处理器
type AgentHandler struct {
	store store.AgentSessionStore
}

// NewAgentHandler 创建 Agent 处理器
func NewAgentHandler(store store.AgentSessionStore) *AgentHandler {
	return &AgentHandler{store: store}
}

// ModelConfig 模型配置
type ModelConfig struct {
	Type    string            `json:"type"`     // openai, anthropic, glm
	APIKey  string            `json:"api_key"`
	BaseURL string            `json:"base_url"`
	Model   string            `json:"model"`
	Headers map[string]string `json:"headers"`
}

// ModelRouter 多模型路由
type ModelRouter struct {
	configs map[string]ModelConfig
}

// NewModelRouter 创建模型路由
func NewModelRouter() *ModelRouter {
	router := &ModelRouter{
		configs: make(map[string]ModelConfig),
	}

	// 加载环境变量配置
	router.loadFromEnv()
	return router
}

// loadFromEnv 从环境变量加载模型配置
func (r *ModelRouter) loadFromEnv() {
	// OpenAI 配置
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		r.configs["openai"] = ModelConfig{
			Type:    "openai",
			APIKey:  apiKey,
			BaseURL: getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
			Model:   getEnvOrDefault("OPENAI_MODEL", "gpt-4"),
		}
	}

	// Anthropic 配置
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		r.configs["anthropic"] = ModelConfig{
			Type:    "anthropic",
			APIKey:  apiKey,
			BaseURL: getEnvOrDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com/v1"),
			Model:   getEnvOrDefault("ANTHROPIC_MODEL", "claude-3-sonnet-20240229"),
		}
	}

	// GLM 配置
	if apiKey := os.Getenv("GLM_API_KEY"); apiKey != "" {
		r.configs["glm"] = ModelConfig{
			Type:    "glm",
			APIKey:  apiKey,
			BaseURL: getEnvOrDefault("GLM_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
			Model:   getEnvOrDefault("GLM_MODEL", "glm-4"),
		}
	}

	// Default 配置
	if _, ok := r.configs["default"]; !ok {
		// 如果没有配置 default，使用第一个可用的模型
		for name, cfg := range r.configs {
			r.configs["default"] = cfg
			fmt.Printf("Using %s as default model\n", name)
			break
		}
	}
}

// GetConfig 获取模型配置
func (r *ModelRouter) GetConfig(model string) (ModelConfig, bool) {
	cfg, ok := r.configs[model]
	return cfg, ok
}

// ChatRequest 通用聊天请求
type ChatRequest struct {
	Model    string          `json:"model"`
	Messages []ChatMessage   `json:"messages"`
	Stream   bool            `json:"stream"`
	Config   json.RawMessage `json:"config,omitempty"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamResponse 流式响应
type StreamResponse struct {
	Type    string `json:"type"`    // token, done, error
	Content string `json:"content"`
}

// getEnvOrDefault 获取环境变量或默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// CreateAgentSessionRequest 创建 Agent 会话请求
type CreateAgentSessionRequest struct {
	Model        string      `json:"model"`
	SystemPrompt string      `json:"system_prompt"`
	Context      models.JSON `json:"context"`
	TaskID       *string     `json:"task_id"`
	Config       models.JSON `json:"config"`
}

// CreateAgentSession 创建 Agent 会话
func (h *AgentHandler) CreateAgentSession(c *gin.Context) {
	var req CreateAgentSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 设置默认模型
	if req.Model == "" {
		req.Model = "default"
	}

	session := &models.AgentSession{
		ID:           uuid.New().String(),
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Context:      req.Context,
		TaskID:       req.TaskID,
		Config:       req.Config,
	}

	if err := h.store.Create(session); err != nil {
		failWithError(c, errors.WrapInternal("Failed to create session", err))
		return
	}

	success(c, session)
}

// GetAgentSession 获取 Agent 会话
func (h *AgentHandler) GetAgentSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.store.Get(id)
	if err != nil {
		failWithError(c, errors.SessionNotFound(id))
		return
	}

	// 获取消息
	messages, err := h.store.GetMessages(id, 100)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get messages", err))
		return
	}

	success(c, gin.H{
		"session":  session,
		"messages": messages,
	})
}

// GetAgentSessions 获取 Agent 会话列表
func (h *AgentHandler) GetAgentSessions(c *gin.Context) {
	sessions, err := h.store.List()
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get sessions", err))
		return
	}

	success(c, sessions)
}

// DeleteAgentSession 删除 Agent 会话
func (h *AgentHandler) DeleteAgentSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.Delete(id); err != nil {
		failWithError(c, errors.SessionNotFound(id))
		return
	}

	success(c, gin.H{"id": id})
}

// SendAgentMessageRequest 发送消息请求
type SendAgentMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// SendAgentMessage 发送消息并获取流式响应
func (h *AgentHandler) SendAgentMessage(c *gin.Context) {
	sessionID := c.Param("id")

	// 检查会话是否存在
	session, err := h.store.Get(sessionID)
	if err != nil {
		failWithError(c, errors.SessionNotFound(sessionID))
		return
	}

	var req SendAgentMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 保存用户消息
	_, err = h.store.AddMessage(sessionID, "user", req.Content)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to save message", err))
		return
	}

	// 获取历史消息
	messages, err := h.store.GetMessages(sessionID, 50)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get history", err))
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 调用模型路由进行流式响应
	router := NewModelRouter()
	fullResponse, err := h.streamChat(c, router, session.Model, messages, session.SystemPrompt)
	if err != nil {
		c.SSEvent("message", StreamResponse{Type: "error", Content: err.Error()})
		c.Writer.Flush()
		return
	}

	// 保存助手消息
	_, err = h.store.AddMessage(sessionID, "assistant", fullResponse)
	if err != nil {
		c.SSEvent("message", StreamResponse{Type: "error", Content: "Failed to save response"})
		c.Writer.Flush()
		return
	}

	// 发送完成事件
	c.SSEvent("message", StreamResponse{Type: "done", Content: fullResponse})
	c.Writer.Flush()
}

// streamChat 执行流式聊天
func (h *AgentHandler) streamChat(c *gin.Context, router *ModelRouter, model string, messages []models.AgentMessage, systemPrompt string) (string, error) {
	cfg, ok := router.GetConfig(model)
	if !ok {
		// 使用默认模型或模拟响应
		return h.mockStreamResponse(c, model)
	}

	// 构建消息列表
	chatMessages := make([]ChatMessage, 0, len(messages)+1)

	// 添加系统提示
	if systemPrompt != "" {
		chatMessages = append(chatMessages, ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// 添加历史消息
	for _, msg := range messages {
		chatMessages = append(chatMessages, ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 根据模型类型调用不同的 API
	switch cfg.Type {
	case "openai":
		return h.streamOpenAI(c, cfg, chatMessages)
	case "anthropic":
		return h.streamAnthropic(c, cfg, chatMessages)
	case "glm":
		return h.streamGLM(c, cfg, chatMessages)
	default:
		return h.mockStreamResponse(c, model)
	}
}

// streamOpenAI 调用 OpenAI API 进行流式响应
func (h *AgentHandler) streamOpenAI(c *gin.Context, cfg ModelConfig, messages []ChatMessage) (string, error) {
	reqBody := map[string]interface{}{
		"model":    cfg.Model,
		"messages": messages,
		"stream":   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var fullContent strings.Builder
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
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)

			// 发送 SSE 事件
			c.SSEvent("message", StreamResponse{Type: "token", Content: content})
			c.Writer.Flush()
		}
	}

	return fullContent.String(), scanner.Err()
}

// streamAnthropic 调用 Anthropic API 进行流式响应
func (h *AgentHandler) streamAnthropic(c *gin.Context, cfg ModelConfig, messages []ChatMessage) (string, error) {
	// 分离系统消息和对话消息
	var systemPrompt string
	var chatMessages []ChatMessage

	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		} else {
			chatMessages = append(chatMessages, msg)
		}
	}

	reqBody := map[string]interface{}{
		"model":      cfg.Model,
		"messages":   chatMessages,
		"max_tokens": 4096,
		"stream":     true,
	}

	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
			content := event.Delta.Text
			fullContent.WriteString(content)

			// 发送 SSE 事件
			c.SSEvent("message", StreamResponse{Type: "token", Content: content})
			c.Writer.Flush()
		}
	}

	return fullContent.String(), scanner.Err()
}

// streamGLM 调用 GLM API 进行流式响应
func (h *AgentHandler) streamGLM(c *gin.Context, cfg ModelConfig, messages []ChatMessage) (string, error) {
	reqBody := map[string]interface{}{
		"model":    cfg.Model,
		"messages": messages,
		"stream":   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var fullContent strings.Builder
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
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)

			// 发送 SSE 事件
			c.SSEvent("message", StreamResponse{Type: "token", Content: content})
			c.Writer.Flush()
		}
	}

	return fullContent.String(), scanner.Err()
}

// mockStreamResponse 模拟流式响应（当没有配置 API 时使用）
func (h *AgentHandler) mockStreamResponse(c *gin.Context, model string) (string, error) {
	mockResponse := fmt.Sprintf("This is a simulated AI response from model '%s'. To enable real AI responses, configure the appropriate API keys in environment variables:\n\n"+
		"- OPENAI_API_KEY for OpenAI models\n"+
		"- ANTHROPIC_API_KEY for Claude models\n"+
		"- GLM_API_KEY for GLM models\n\n"+
		"Your message has been received and stored.", model)

	// 逐字符发送
	for _, char := range mockResponse {
		// 检查客户端是否断开
		select {
		case <-c.Done():
			return mockResponse, nil
		default:
		}

		// 发送 SSE 事件
		c.SSEvent("message", StreamResponse{Type: "token", Content: string(char)})
		c.Writer.Flush()
		time.Sleep(10 * time.Millisecond) // 模拟打字效果
	}

	return mockResponse, nil
}

// GetAgentMessages 获取会话消息列表
func (h *AgentHandler) GetAgentMessages(c *gin.Context) {
	sessionID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	messages, err := h.store.GetMessages(sessionID, limit)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get messages", err))
		return
	}

	success(c, messages)
}