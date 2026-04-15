package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/shared/models"
)

// Agent 统一的 Agent 结构
type Agent struct {
	ID            string
	Template      *models.AgentTemplate
	Messages      []ChatMessage
	WorkspacePath string

	// 依赖组件
	llmClient     *LLMClient
	registry      *tools.Registry
	toolExecStore store.ToolExecutionStore
	taskStore     store.TaskStore
	sessionStore  store.AgentSessionStore

	// 配置
	maxToolRounds int
}

// AgentConfig Agent 配置
type AgentConfig struct {
	MaxToolRounds int
	Timeout       time.Duration
}

// NewAgent 创建 Agent
func NewAgent(
	id string,
	template *models.AgentTemplate,
	llmClient *LLMClient,
	registry *tools.Registry,
	toolExecStore store.ToolExecutionStore,
	taskStore store.TaskStore,
	sessionStore store.AgentSessionStore,
	config AgentConfig,
) *Agent {
	if config.MaxToolRounds <= 0 {
		config.MaxToolRounds = 10
	}

	return &Agent{
		ID:            id,
		Template:      template,
		Messages:      []ChatMessage{},
		WorkspacePath: fmt.Sprintf("workspace/%s/", id),
		llmClient:     llmClient,
		registry:      registry,
		toolExecStore: toolExecStore,
		taskStore:     taskStore,
		sessionStore:  sessionStore,
		maxToolRounds: config.MaxToolRounds,
	}
}

// ProcessMessage 处理消息 - 核心：调用模型 + 执行工具
func (a *Agent) ProcessMessage(
	ctx context.Context,
	userMessage string,
	cfg ModelConfig,
	onToken func(string),
) (*ProcessResult, error) {
	// 保存用户消息
	if a.sessionStore != nil {
		if _, err := a.sessionStore.AddMessage(a.ID, "user", userMessage); err != nil {
			log.Printf("Failed to save user message: %v", err)
		}
	}

	// 添加到对话历史
	a.Messages = append(a.Messages, ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	// 处理多轮对话（可能包含工具调用）
	return a.processWithToolCalls(ctx, cfg, onToken)
}

// processWithToolCalls 处理带 Tool Calling 的对话
func (a *Agent) processWithToolCalls(
	ctx context.Context,
	cfg ModelConfig,
	onToken func(string),
) (*ProcessResult, error) {
	result := &ProcessResult{TotalRounds: 0}
	var toolResults []ToolResultInfo

	// 获取系统提示词
	systemPrompt := ""
	if a.Template != nil {
		systemPrompt = a.Template.BasePrompt
	}

	// 获取允许的工具列表
	toolNames := a.getAllowedTools()

	for round := 0; round < a.maxToolRounds; round++ {
		result.TotalRounds = round + 1

		var toolCalls []models.ToolCall
		var response string
		var finishReason string

		// 根据模型类型调用不同的 API
		switch cfg.Type {
		case "openai", "glm":
			req, err := a.llmClient.BuildOpenAIRequest(cfg.Model, a.Messages, systemPrompt, toolNames)
			if err != nil {
				return nil, err
			}

			var resp *ChatResponse
			if onToken != nil {
				resp, err = a.llmClient.StreamOpenAI(cfg, req, onToken)
			} else {
				resp, err = a.llmClient.CallOpenAI(cfg, req)
			}
			if err != nil {
				return nil, err
			}

			if len(resp.Choices) > 0 {
				response = fmt.Sprintf("%v", resp.Choices[0].Message.Content)
				toolCalls = resp.Choices[0].Message.ToolCalls
				finishReason = resp.Choices[0].FinishReason
			}

		case "anthropic":
			req, err := a.llmClient.BuildAnthropicRequest(cfg.Model, a.Messages, systemPrompt, toolNames)
			if err != nil {
				return nil, err
			}

			var resp *AnthropicResponse
			if onToken != nil {
				resp, err = a.llmClient.StreamAnthropic(cfg, req, onToken)
			} else {
				resp, err = a.llmClient.CallAnthropic(cfg, req)
			}
			if err != nil {
				return nil, err
			}

			// 提取文本内容
			for _, block := range resp.Content {
				if block.Type == "text" {
					response += block.Text
				}
			}
			toolCalls = a.llmClient.ParseToolCallsFromAnthropic(resp)
			finishReason = resp.StopReason

		default:
			return nil, fmt.Errorf("unsupported model type: %s", cfg.Type)
		}

		// 如果没有 tool calls，保存响应并返回
		if len(toolCalls) == 0 {
			if a.sessionStore != nil {
				if _, err := a.sessionStore.AddMessage(a.ID, "assistant", response); err != nil {
					log.Printf("Failed to save assistant message: %v", err)
				}
			}

			result.Response = response
			result.FinishReason = finishReason
			result.ToolResults = toolResults
			return result, nil
		}

		// 有 tool calls，保存带 tool_calls 的助手消息
		result.ToolCalls = toolCalls

		// 添加助手消息到对话历史
		a.Messages = append(a.Messages, ChatMessage{
			Role:      "assistant",
			Content:   response,
			ToolCalls: toolCalls,
		})

		// 执行所有 tool calls
		var execResults []ToolResultInfo
		for _, tc := range toolCalls {
			execResult, err := a.executeToolCall(tc)
			if err != nil {
				log.Printf("Failed to execute tool %s: %v", tc.Function.Name, err)
				execResult = ToolResultInfo{
					ToolCallID: tc.ID,
					ToolName:   tc.Function.Name,
					Result:     err.Error(),
					IsError:    true,
				}
			}
			execResults = append(execResults, execResult)

			// 添加 tool result 到对话历史
			a.Messages = append(a.Messages, ChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    execResult.Result,
				Name:       tc.Function.Name,
			})
		}

		toolResults = append(toolResults, execResults...)
	}

	// 达到最大轮数
	return nil, fmt.Errorf("max tool call rounds reached (%d)", a.maxToolRounds)
}

// getAllowedTools 获取允许的工具列表
func (a *Agent) getAllowedTools() []string {
	if a.Template == nil {
		// 无模板时返回所有工具
		return a.registry.GetToolNames()
	}

	// 如果有白名单，只返回白名单中的工具
	if len(a.Template.AllowedTools) > 0 {
		return []string(a.Template.AllowedTools)
	}

	// 否则返回所有工具（排除黑名单）
	allTools := a.registry.GetToolNames()
	restricted := []string(a.Template.RestrictedTools)

	var allowed []string
	for _, tool := range allTools {
		isRestricted := false
		for _, r := range restricted {
			if tool == r {
				isRestricted = true
				break
			}
		}
		if !isRestricted {
			allowed = append(allowed, tool)
		}
	}

	return allowed
}

// executeToolCall 执行工具调用
func (a *Agent) executeToolCall(toolCall models.ToolCall) (ToolResultInfo, error) {
	// 获取工具定义
	toolDef, err := a.registry.Get(toolCall.Function.Name)
	if err != nil {
		return ToolResultInfo{}, fmt.Errorf("tool not found: %s", toolCall.Function.Name)
	}

	// 检查工具权限
	if err := a.checkToolPermission(toolCall.Function.Name); err != nil {
		return ToolResultInfo{
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.Function.Name,
			Result:     err.Error(),
			IsError:    true,
		}, nil
	}

	// 解析参数
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return ToolResultInfo{}, fmt.Errorf("failed to parse arguments: %w", err)
	}

	// 创建执行记录
	execution := &models.ToolExecution{
		ConversationID: a.ID,
		ToolName:       toolCall.Function.Name,
		ToolCallID:     toolCall.ID,
		Arguments:      args,
		Status:         string(models.ToolExecutionStatusPending),
	}

	// 根据执行模式处理
	if toolDef.ExecuteMode == models.ToolExecuteModeLocal {
		return a.executeLocalTool(execution, args)
	}

	// 远程工具 - 返回待执行状态（需要 Worker）
	return ToolResultInfo{
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Function.Name,
		Result:     "Remote tool execution pending. Requires worker.",
		IsError:    false,
	}, nil
}

// checkToolPermission 检查工具权限
func (a *Agent) checkToolPermission(toolName string) error {
	if a.Template == nil {
		return nil
	}

	// 检查黑名单
	for _, t := range a.Template.RestrictedTools {
		if t == toolName {
			return fmt.Errorf("tool '%s' is restricted for template '%s'", toolName, a.Template.ID)
		}
	}

	// 检查白名单
	if len(a.Template.AllowedTools) > 0 {
		allowed := false
		for _, t := range a.Template.AllowedTools {
			if t == toolName {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("tool '%s' not in allowed list for template '%s'", toolName, a.Template.ID)
		}
	}

	return nil
}

// executeLocalTool 执行本地工具
func (a *Agent) executeLocalTool(
	execution *models.ToolExecution,
	args map[string]interface{},
) (ToolResultInfo, error) {
	// 更新状态为运行中
	if a.toolExecStore != nil {
		if err := a.toolExecStore.Create(execution); err != nil {
			return ToolResultInfo{}, fmt.Errorf("failed to create execution record: %w", err)
		}
		if err := a.toolExecStore.UpdateStatus(execution.ID, string(models.ToolExecutionStatusRunning), "", false); err != nil {
			return ToolResultInfo{}, err
		}
	}

	var result string
	var isError bool

	// 根据工具名称执行
	switch execution.ToolName {
	case "create_task":
		result, isError = a.toolCreateTask(args)
	case "query_task":
		result, isError = a.toolQueryTask(args)
	case "read_file":
		result, isError = a.toolReadFile(args)
	default:
		result = fmt.Sprintf("Unknown local tool: %s", execution.ToolName)
		isError = true
	}

	// 更新执行记录
	if a.toolExecStore != nil {
		status := string(models.ToolExecutionStatusCompleted)
		if isError {
			status = string(models.ToolExecutionStatusFailed)
		}
		a.toolExecStore.UpdateStatus(execution.ID, status, result, isError)
	}

	return ToolResultInfo{
		ToolCallID: execution.ToolCallID,
		ToolName:   execution.ToolName,
		Result:     result,
		IsError:    isError,
	}, nil
}

// toolCreateTask create_task 工具实现
func (a *Agent) toolCreateTask(args map[string]interface{}) (string, bool) {
	title, _ := args["title"].(string)
	description, _ := args["description"].(string)
	templateID, _ := args["template_id"].(string)

	if title == "" {
		return "Missing required parameter: title", true
	}

	task := &models.Task{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Status:      models.TaskStatusPending,
		Priority:    models.PriorityMedium,
		TemplateID:  templateID,
	}

	if a.taskStore != nil {
		if err := a.taskStore.Create(task); err != nil {
			return fmt.Sprintf("Failed to create task: %v", err), true
		}
	}

	return fmt.Sprintf("Task created: %s (ID: %s)", title, task.ID), false
}

// toolQueryTask query_task 工具实现
func (a *Agent) toolQueryTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)

	if taskID == "" {
		// 查询所有任务
		if a.taskStore != nil {
			tasks, _, err := a.taskStore.List(store.ListOptions{PageSize: 100})
			if err != nil {
				return fmt.Sprintf("Failed to list tasks: %v", err), true
			}

			result, _ := json.Marshal(tasks)
			return string(result), false
		}
		return "No task store available", true
	}

	// 查询特定任务
	if a.taskStore != nil {
		task, err := a.taskStore.Get(taskID)
		if err != nil {
			return fmt.Sprintf("Task not found: %s", taskID), true
		}

		result, _ := json.Marshal(task)
		return string(result), false
	}

	return "No task store available", true
}

// toolReadFile read_file 工具实现（本地查询）
func (a *Agent) toolReadFile(args map[string]interface{}) (string, bool) {
	path, _ := args["path"].(string)

	if path == "" {
		return "Missing required parameter: path", true
	}

	// 如果路径是 workspace 内的相对路径，转换为绝对路径
	if !strings.HasPrefix(path, "/") {
		path = a.WorkspacePath + path
	}

	// 读取文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Failed to read file: %v", err), true
	}

	return string(content), false
}