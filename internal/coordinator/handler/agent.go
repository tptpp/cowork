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
	"github.com/tp/cowork/internal/coordinator/agent"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/shared/errors"
	"github.com/tp/cowork/internal/shared/models"
)

// AgentHandler Agent API 处理器
type AgentHandler struct {
	store         store.AgentSessionStore
	toolExecStore store.ToolExecutionStore
	taskStore     store.TaskStore
	fileStore     store.TaskFileStore
	registry      *tools.Registry
	coordinator   *agent.ConversationCoordinator
	modelRouter   *ModelRouter // 模型路由
}

// NewAgentHandler 创建 Agent 处理器
func NewAgentHandler(
	store store.AgentSessionStore,
	toolExecStore store.ToolExecutionStore,
	taskStore store.TaskStore,
	fileStore store.TaskFileStore,
	registry *tools.Registry,
) *AgentHandler {
	return &AgentHandler{
		store:         store,
		toolExecStore: toolExecStore,
		taskStore:     taskStore,
		fileStore:     fileStore,
		registry:      registry,
		modelRouter:   NewModelRouter(), // 默认从环境变量加载
	}
}

// SetModelRouter 设置模型路由
func (h *AgentHandler) SetModelRouter(router *ModelRouter) {
	h.modelRouter = router
}

// SetCoordinator 设置协调器
func (h *AgentHandler) SetCoordinator(coordinator *agent.ConversationCoordinator) {
	h.coordinator = coordinator
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

// LoadFromConfig 从外部配置加载模型配置
func (r *ModelRouter) LoadFromConfig(modelType, baseURL, model, apiKey string) {
	if baseURL != "" && model != "" && apiKey != "" {
		r.configs[modelType] = ModelConfig{
			Type:    modelType,
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		}
		// 同时设置为默认模型（如果还没有默认）
		if _, ok := r.configs["default"]; !ok {
			r.configs["default"] = r.configs[modelType]
		}
	}
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
	CoordinatorTemplateID string          `json:"coordinator_template_id"` // 可选，默认 coordinator-template
	WorkerTemplateIDs     models.StringArray `json:"worker_template_ids"`     // 可选，空数组=自动
	Context               models.JSON    `json:"context"`
	TaskID                *string        `json:"task_id"`
}

// CreateAgentSession 创建 Agent 会话
func (h *AgentHandler) CreateAgentSession(c *gin.Context) {
	var req CreateAgentSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 设置默认 Coordinator 模板
	if req.CoordinatorTemplateID == "" {
		req.CoordinatorTemplateID = "coordinator-template"
	}

	session := &models.AgentSession{
		ID:                    uuid.New().String(),
		CoordinatorTemplateID: req.CoordinatorTemplateID,
		WorkerTemplateIDs:     req.WorkerTemplateIDs,
		Context:               req.Context,
		TaskID:                req.TaskID,
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
	if _, err := h.store.Get(sessionID); err != nil {
		failWithError(c, errors.SessionNotFound(sessionID))
		return
	}

	var req SendAgentMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 保存用户消息
	if _, err := h.store.AddMessage(sessionID, "user", req.Content); err != nil {
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
	// TODO: Load model and systemPrompt from CoordinatorTemplate (Task 8)
	fullResponse, err := h.streamChat(c, h.modelRouter, "default", messages, "")
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

// ============ Function Calling API ============

// SendAgentMessageWithToolsRequest 发送带工具支持的消息请求
type SendAgentMessageWithToolsRequest struct {
	Content          string   `json:"content" binding:"required"`
	Files            []uint   `json:"files,omitempty"`             // 文件 ID 列表
	Tools            []string `json:"tools,omitempty"`             // 指定可用工具
	AutoExecuteTools bool     `json:"auto_execute_tools,omitempty"` // 是否自动执行工具
}

// SendAgentMessageWithTools 发送消息并支持 Function Calling (SSE 流式响应)
func (h *AgentHandler) SendAgentMessageWithTools(c *gin.Context) {
	sessionID := c.Param("id")

	// 检查会话是否存在
	session, err := h.store.Get(sessionID)
	if err != nil {
		failWithError(c, errors.SessionNotFound(sessionID))
		return
	}

	var req SendAgentMessageWithToolsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 处理文件上下文
	content := req.Content
	if len(req.Files) > 0 && h.fileStore != nil {
		fileContext, err := h.buildFileContext(req.Files)
		if err != nil {
			failWithError(c, errors.WrapInternal("Failed to read file context", err))
			return
		}
		if fileContext != "" {
			content = fmt.Sprintf("%s\n\n[Attached Files]\n%s", req.Content, fileContext)
		}
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 获取模型配置
	// TODO: Load model config from CoordinatorTemplate (Task 8)
	cfg, ok := h.modelRouter.GetConfig("default")
	if !ok {
		// 没有配置，使用模拟响应
		h.mockStreamResponseWithTools(c, "default", req.Content)
		return
	}

	// 如果没有协调器，使用简单模式
	if h.coordinator == nil {
		h.streamWithoutFunctionCalling(c, session, cfg, content)
		return
	}

	// 获取工具列表：如果用户没有指定，使用所有可用工具
	toolNames := req.Tools
	if len(toolNames) == 0 {
		toolNames = h.registry.GetToolNames()
	}

	// 使用协调器处理 (支持 Function Calling + 任务拆解)
	onToken := func(token string) {
		c.SSEvent("message", StreamResponse{Type: "token", Content: token})
		c.Writer.Flush()
	}

	result, taskGroup, err := h.coordinator.ProcessWithDecomposition(
		c.Request.Context(),
		sessionID,
		content,
		agent.ModelConfig{
			Type:    cfg.Type,
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		},
		toolNames,
		onToken,
	)

	if err != nil {
		c.SSEvent("message", StreamResponse{Type: "error", Content: err.Error()})
		c.Writer.Flush()
		return
	}

		// 如果任务被拆解，发送任务组信息
		if taskGroup != nil {
			c.SSEvent("message", map[string]interface{}{
				"type":       "task_decomposed",
				"task_group": taskGroup,
				"total_tasks": taskGroup.TotalTasks,
			})
			c.Writer.Flush()
		}
	// 发送工具调用信息
	if len(result.ToolCalls) > 0 {
		c.SSEvent("message", map[string]interface{}{
			"type":       "tool_calls",
			"tool_calls": result.ToolCalls,
		})
		c.Writer.Flush()
	}

	// 发送完成事件
	c.SSEvent("message", StreamResponse{
		Type:    "done",
		Content: result.Response,
	})
	c.Writer.Flush()
}

// streamWithoutFunctionCalling 不使用 Function Calling 的流式响应
func (h *AgentHandler) streamWithoutFunctionCalling(c *gin.Context, session *models.AgentSession, cfg ModelConfig, content string) {
	// 保存用户消息
	_, err := h.store.AddMessage(session.ID, "user", content)
	if err != nil {
		c.SSEvent("message", StreamResponse{Type: "error", Content: "Failed to save message"})
		c.Writer.Flush()
		return
	}

	// 获取历史消息
	messages, err := h.store.GetMessages(session.ID, 50)
	if err != nil {
		c.SSEvent("message", StreamResponse{Type: "error", Content: "Failed to get history"})
		c.Writer.Flush()
		return
	}

	// 调用模型
	// TODO: Load model and systemPrompt from CoordinatorTemplate (Task 8)
	fullResponse, err := h.streamChat(c, h.modelRouter, "default", messages, "")
	if err != nil {
		c.SSEvent("message", StreamResponse{Type: "error", Content: err.Error()})
		c.Writer.Flush()
		return
	}

	// 保存助手消息
	_, err = h.store.AddMessage(session.ID, "assistant", fullResponse)
	if err != nil {
		c.SSEvent("message", StreamResponse{Type: "error", Content: "Failed to save response"})
		c.Writer.Flush()
		return
	}

	// 发送完成事件
	c.SSEvent("message", StreamResponse{Type: "done", Content: fullResponse})
	c.Writer.Flush()
}

// mockStreamResponseWithTools 模拟带工具的流式响应
func (h *AgentHandler) mockStreamResponseWithTools(c *gin.Context, model string, content string) {
	mockResponse := fmt.Sprintf("This is a simulated AI response from model '%s'. To enable real AI responses with Function Calling, configure the appropriate API keys:\n\n"+
		"- OPENAI_API_KEY for OpenAI models\n"+
		"- ANTHROPIC_API_KEY for Claude models\n"+
		"- GLM_API_KEY for GLM models\n\n"+
		"Your message: %s\n\nFunction Calling support is ready.", model, content)

	// 逐字符发送
	for _, char := range mockResponse {
		select {
		case <-c.Done():
			return
		default:
		}

		c.SSEvent("message", StreamResponse{Type: "token", Content: string(char)})
		c.Writer.Flush()
		time.Sleep(10 * time.Millisecond)
	}

	c.SSEvent("message", StreamResponse{Type: "done", Content: mockResponse})
	c.Writer.Flush()
}

// GetAgentToolExecutions 获取会话的工具执行记录
func (h *AgentHandler) GetAgentToolExecutions(c *gin.Context) {
	sessionID := c.Param("id")

	executions, err := h.toolExecStore.ListByConversation(sessionID)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get tool executions", err))
		return
	}

	success(c, executions)
}

// GetToolExecution 获取单个工具执行记录
func (h *AgentHandler) GetToolExecution(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		failWithError(c, errors.InvalidRequest("Invalid execution ID"))
		return
	}

	execution, err := h.toolExecStore.Get(uint(id))
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get tool execution", err))
		return
	}

	success(c, execution)
}

// GetAvailableTools 获取可用工具列表
func (h *AgentHandler) GetAvailableTools(c *gin.Context) {
	tools := h.registry.List()

	// 转换为 OpenAI 格式
	result := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		result[i] = tool.ToOpenAITool()
	}

	success(c, result)
}

// GetToolDefinition 获取工具定义
func (h *AgentHandler) GetToolDefinition(c *gin.Context) {
	name := c.Param("name")

	tool, err := h.registry.Get(name)
	if err != nil {
		failWithError(c, errors.WrapInternal("Tool not found", err))
		return
	}

	success(c, tool)
}

// ExecuteToolCallRequest 执行工具调用请求
type ExecuteToolCallRequest struct {
	ToolCallID string `json:"tool_call_id" binding:"required"`
	Approved   bool   `json:"approved"`
}

// ExecuteToolCall 执行工具调用 (Human-in-loop 批准/拒绝)
func (h *AgentHandler) ExecuteToolCall(c *gin.Context) {
	sessionID := c.Param("id")

	// 检查会话是否存在
	_, err := h.store.Get(sessionID)
	if err != nil {
		failWithError(c, errors.SessionNotFound(sessionID))
		return
	}

	var req ExecuteToolCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 获取工具执行记录
	execution, err := h.toolExecStore.GetByToolCallID(req.ToolCallID)
	if err != nil {
		failWithError(c, errors.WrapInternal("Tool execution not found", err))
		return
	}

	if !req.Approved {
		// 用户拒绝执行
		execution.Status = string(models.ToolExecutionStatusRejected)
		h.toolExecStore.Update(execution)

		success(c, gin.H{
			"status":  "rejected",
			"message": "Tool execution rejected by user",
		})
		return
	}

	// 用户批准执行 - 更新状态并触发执行
	execution.Status = string(models.ToolExecutionStatusRunning)
	h.toolExecStore.Update(execution)

	// 执行工具 (如果有协调器)
	if h.coordinator != nil {
		// 通过协调器执行工具
		result, err := h.coordinator.ExecuteToolDirectly(c.Request.Context(), execution)
		if err != nil {
			execution.Status = string(models.ToolExecutionStatusFailed)
			execution.Result = err.Error()
			execution.IsError = true
			h.toolExecStore.Update(execution)

			failWithError(c, errors.WrapInternal("Tool execution failed", err))
			return
		}

		execution.Status = string(models.ToolExecutionStatusCompleted)
		execution.Result = result
		h.toolExecStore.Update(execution)

		success(c, gin.H{
			"status":  "completed",
			"result":  result,
			"message": "Tool executed successfully",
		})
		return
	}

	// 没有协调器，返回模拟结果
	mockResult := fmt.Sprintf("Tool '%s' executed successfully (mock mode)", execution.ToolName)
	execution.Status = string(models.ToolExecutionStatusCompleted)
	execution.Result = mockResult
	h.toolExecStore.Update(execution)

	success(c, gin.H{
		"status":  "completed",
		"result":  mockResult,
		"message": "Tool executed successfully (mock mode - no coordinator)",
	})
}

// buildFileContext 构建文件上下文
func (h *AgentHandler) buildFileContext(fileIDs []uint) (string, error) {
	var contextBuilder strings.Builder

	for _, fileID := range fileIDs {
		file, err := h.fileStore.Get(fileID)
		if err != nil {
			continue // 跳过不存在的文件
		}

		// 读取文件内容
		content, err := os.ReadFile(file.Path)
		if err != nil {
			continue // 跳过无法读取的文件
		}

		// 限制文件内容长度
		maxContentLength := 10000 // 10KB per file
		contentStr := string(content)
		if len(contentStr) > maxContentLength {
			contentStr = contentStr[:maxContentLength] + "\n... (truncated)"
		}

		contextBuilder.WriteString(fmt.Sprintf("=== %s ===\n", file.Name))
		contextBuilder.WriteString(contentStr)
		contextBuilder.WriteString("\n\n")
	}

	return contextBuilder.String(), nil
}