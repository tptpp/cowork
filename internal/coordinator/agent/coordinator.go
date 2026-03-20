package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/shared/models"
)

// ConversationCoordinator 对话协调器 - 处理多轮 Tool Calling
type ConversationCoordinator struct {
	engine         *FunctionCallingEngine
	scheduler      *ToolScheduler
	sessionStore   store.AgentSessionStore
	toolExecStore  store.ToolExecutionStore
	registry       *tools.Registry

	// Phase 5: 任务拆解相关
	decomposer     *TaskDecomposer
	depManager     *DependencyManager
	progressTracker *ProgressTracker
	groupStore     store.TaskGroupStore
	depStore       store.TaskDependencyStore
}

// CoordinatorConfig 协调器配置
type CoordinatorConfig struct {
	MaxToolRounds int
}

// NewConversationCoordinator 创建对话协调器
func NewConversationCoordinator(
	engine *FunctionCallingEngine,
	scheduler *ToolScheduler,
	sessionStore store.AgentSessionStore,
	toolExecStore store.ToolExecutionStore,
	registry *tools.Registry,
) *ConversationCoordinator {
	return &ConversationCoordinator{
		engine:        engine,
		scheduler:     scheduler,
		sessionStore:  sessionStore,
		toolExecStore: toolExecStore,
		registry:      registry,
	}
}

// NewConversationCoordinatorWithDecomposer 创建带任务拆解功能的协调器
func NewConversationCoordinatorWithDecomposer(
	engine *FunctionCallingEngine,
	scheduler *ToolScheduler,
	sessionStore store.AgentSessionStore,
	toolExecStore store.ToolExecutionStore,
	registry *tools.Registry,
	taskStore store.TaskStore,
	groupStore store.TaskGroupStore,
	depStore store.TaskDependencyStore,
	config CoordinatorConfig,
) *ConversationCoordinator {
	// 创建依赖管理器
	depManager := NewDependencyManager(depStore, taskStore)

	// 创建进度追踪器
	progressTracker := NewProgressTracker(groupStore, taskStore, depManager, ProgressTrackerConfig{})

	// 创建任务拆解器
	decomposer := NewTaskDecomposer(engine, taskStore, groupStore, depStore, DecomposerConfig{
		MaxSubTasks: 10,
	})

	return &ConversationCoordinator{
		engine:          engine,
		scheduler:       scheduler,
		sessionStore:    sessionStore,
		toolExecStore:   toolExecStore,
		registry:        registry,
		decomposer:      decomposer,
		depManager:      depManager,
		progressTracker: progressTracker,
		groupStore:      groupStore,
		depStore:        depStore,
	}
}

// ProcessMessage 处理消息 (支持多轮 Tool Calling)
func (c *ConversationCoordinator) ProcessMessage(
	ctx context.Context,
	sessionID string,
	userMessage string,
	cfg ModelConfig,
	toolNames []string,
	onToken func(string),
) (*ProcessResult, error) {
	// 保存用户消息
	if _, err := c.sessionStore.AddMessage(sessionID, "user", userMessage); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	// 获取历史消息
	messages, err := c.sessionStore.GetMessages(sessionID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// 获取会话信息
	session, err := c.sessionStore.Get(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// 转换消息格式
	chatMessages := ConvertToAgentMessages(messages)

	// 处理多轮对话
	return c.processWithToolCalls(ctx, sessionID, cfg, session.SystemPrompt, chatMessages, toolNames, onToken)
}

// ProcessResult 处理结果
type ProcessResult struct {
	Response      string
	ToolCalls     []models.ToolCall
	ToolResults   []ToolResultInfo
	TotalRounds   int
	FinishReason  string
}

// ToolResultInfo 工具结果信息
type ToolResultInfo struct {
	ToolCallID string
	ToolName   string
	Result     string
	IsError    bool
}

// processWithToolCalls 处理带 Tool Calling 的对话
func (c *ConversationCoordinator) processWithToolCalls(
	ctx context.Context,
	sessionID string,
	cfg ModelConfig,
	systemPrompt string,
	messages []ChatMessage,
	toolNames []string,
	onToken func(string),
) (*ProcessResult, error) {
	result := &ProcessResult{
		TotalRounds: 0,
	}

	var toolResults []ToolResultInfo

	for round := 0; round < c.engine.maxToolRounds; round++ {
		result.TotalRounds = round + 1

		var toolCalls []models.ToolCall
		var response string
		var finishReason string

		// 根据模型类型调用不同的 API
		switch cfg.Type {
		case "openai", "glm":
			resp, err := c.callOpenAIWithRetry(ctx, cfg, systemPrompt, messages, toolNames, onToken)
			if err != nil {
				return nil, err
			}
			response = fmt.Sprintf("%v", resp.Choices[0].Message.Content)
			toolCalls = resp.Choices[0].Message.ToolCalls
			finishReason = resp.Choices[0].FinishReason

		case "anthropic":
			resp, err := c.callAnthropicWithRetry(ctx, cfg, systemPrompt, messages, toolNames, onToken)
			if err != nil {
				return nil, err
			}
			// 提取文本内容
			for _, block := range resp.Content {
				if block.Type == "text" {
					response += block.Text
				}
			}
			toolCalls = c.engine.ParseToolCallsFromAnthropic(resp)
			finishReason = resp.StopReason

		default:
			return nil, fmt.Errorf("unsupported model type: %s", cfg.Type)
		}

		// 如果没有 tool calls，保存响应并返回
		if len(toolCalls) == 0 {
			// 保存助手消息
			if _, err := c.sessionStore.AddMessage(sessionID, "assistant", response); err != nil {
				log.Printf("Failed to save assistant message: %v", err)
			}

			result.Response = response
			result.FinishReason = finishReason
			result.ToolResults = toolResults
			return result, nil
		}

		// 有 tool calls，保存带 tool_calls 的助手消息
		toolCallsArray := models.ToolCallsArray(toolCalls)
		assistantMsg := &models.AgentMessage{
			SessionID: sessionID,
			Role:      "assistant",
			Content:   response,
			ToolCalls: &toolCallsArray,
		}

		// 直接创建消息（绕过 AddMessage 的简化接口）
		if err := c.saveMessage(assistantMsg); err != nil {
			log.Printf("Failed to save assistant message with tool calls: %v", err)
		}

		result.ToolCalls = toolCalls

		// 执行所有 tool calls
		var execResults []ToolResultInfo
		for _, tc := range toolCalls {
			execResult, err := c.executeToolCall(sessionID, tc)
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

			// 保存 tool result 消息
			if _, err := c.sessionStore.AddMessage(sessionID, "tool", execResult.Result); err != nil {
				log.Printf("Failed to save tool result message: %v", err)
			}
		}

		toolResults = append(toolResults, execResults...)

		// 构建下一轮消息
		// 添加助手消息（带 tool calls）
		messages = append(messages, ChatMessage{
			Role:      "assistant",
			Content:   response,
			ToolCalls: toolCalls,
		})

		// 添加 tool result 消息
		for _, tr := range execResults {
			messages = append(messages, ChatMessage{
				Role:       "tool",
				ToolCallID: tr.ToolCallID,
				Content:    tr.Result,
				Name:       tr.ToolName,
			})
		}

		// 继续下一轮...
	}

	// 达到最大轮数
	return nil, fmt.Errorf("max tool call rounds reached (%d)", c.engine.maxToolRounds)
}

// callOpenAIWithRetry 调用 OpenAI API (带重试)
func (c *ConversationCoordinator) callOpenAIWithRetry(
	ctx context.Context,
	cfg ModelConfig,
	systemPrompt string,
	messages []ChatMessage,
	toolNames []string,
	onToken func(string),
) (*ChatResponse, error) {
	req, err := c.engine.BuildOpenAIRequest(cfg.Model, messages, systemPrompt, toolNames)
	if err != nil {
		return nil, err
	}

	if onToken != nil {
		return c.engine.StreamOpenAI(cfg, req, onToken)
	}

	return c.engine.CallOpenAI(cfg, req)
}

// callAnthropicWithRetry 调用 Anthropic API (带重试)
func (c *ConversationCoordinator) callAnthropicWithRetry(
	ctx context.Context,
	cfg ModelConfig,
	systemPrompt string,
	messages []ChatMessage,
	toolNames []string,
	onToken func(string),
) (*AnthropicResponse, error) {
	req, err := c.engine.BuildAnthropicRequest(cfg.Model, messages, systemPrompt, toolNames)
	if err != nil {
		return nil, err
	}

	if onToken != nil {
		return c.engine.StreamAnthropic(cfg, req, onToken)
	}

	return c.engine.CallAnthropic(cfg, req)
}

// executeToolCall 执行工具调用
func (c *ConversationCoordinator) executeToolCall(
	sessionID string,
	toolCall models.ToolCall,
) (ToolResultInfo, error) {
	// 获取工具定义
	toolDef, err := c.registry.Get(toolCall.Function.Name)
	if err != nil {
		return ToolResultInfo{}, fmt.Errorf("tool not found: %s", toolCall.Function.Name)
	}

	// 解析参数
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return ToolResultInfo{}, fmt.Errorf("failed to parse arguments: %w", err)
	}

	// 创建执行记录
	execution := &models.ToolExecution{
		ConversationID: sessionID,
		ToolName:       toolCall.Function.Name,
		ToolCallID:     toolCall.ID,
		Arguments:      args,
		Status:         string(models.ToolExecutionStatusPending),
	}

	// 根据执行模式处理
	if toolDef.ExecuteMode == models.ToolExecuteModeLocal {
		// 本地工具直接执行
		execResult, err := c.engine.ExecuteToolCall(sessionID, toolCall)
		if err != nil {
			return ToolResultInfo{
				ToolCallID: toolCall.ID,
				ToolName:   toolCall.Function.Name,
				Result:     err.Error(),
				IsError:    true,
			}, nil
		}

		return ToolResultInfo{
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.Function.Name,
			Result:     execResult.Result,
			IsError:    execResult.IsError,
		}, nil
	}

	// 远程工具 - 目前返回等待消息
	// Phase 3 将实现完整的远程执行
	if err := c.toolExecStore.Create(execution); err != nil {
		return ToolResultInfo{}, fmt.Errorf("failed to create execution record: %w", err)
	}

	// 标记为等待执行
	return ToolResultInfo{
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Function.Name,
		Result:     fmt.Sprintf("Tool execution pending (ID: %d). Remote tools require worker execution.", execution.ID),
		IsError:    false,
	}, nil
}

// saveMessage 保存消息
func (c *ConversationCoordinator) saveMessage(msg *models.AgentMessage) error {
	return c.sessionStore.AddMessageWithToolCalls(msg)
}

// GetPendingToolCalls 获取待处理的工具调用
func (c *ConversationCoordinator) GetPendingToolCalls(sessionID string) ([]models.ToolExecution, error) {
	return c.scheduler.ListPendingExecutions(sessionID)
}

// ContinueWithToolResults 继续对话 (使用工具结果)
func (c *ConversationCoordinator) ContinueWithToolResults(
	ctx context.Context,
	sessionID string,
	cfg ModelConfig,
	toolResults []ToolResultInfo,
	toolNames []string,
	onToken func(string),
) (*ProcessResult, error) {
	// 获取历史消息
	messages, err := c.sessionStore.GetMessages(sessionID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// 获取会话信息
	session, err := c.sessionStore.Get(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// 转换消息格式
	chatMessages := ConvertToAgentMessages(messages)

	// 添加工具结果消息
	for _, tr := range toolResults {
		chatMessages = append(chatMessages, ChatMessage{
			Role:       "tool",
			ToolCallID: tr.ToolCallID,
			Content:    tr.Result,
			Name:       tr.ToolName,
		})
	}

	// 继续处理
	return c.processWithToolCalls(ctx, sessionID, cfg, session.SystemPrompt, chatMessages, toolNames, onToken)
}

// ========== Phase 5: 任务拆解能力 ==========

// DecomposeTask 拆解复杂任务
func (c *ConversationCoordinator) DecomposeTask(
	ctx context.Context,
	goal string,
	conversationID string,
	modelCfg ModelConfig,
) (*models.TaskGroup, error) {
	if c.decomposer == nil {
		return nil, fmt.Errorf("task decomposer not initialized")
	}

	return c.decomposer.DecomposeTask(ctx, goal, conversationID, modelCfg)
}

// ShouldDecompose 判断是否需要拆解任务
func (c *ConversationCoordinator) ShouldDecompose(goal string) bool {
	if c.decomposer == nil {
		return false
	}
	return c.decomposer.ShouldDecompose(goal)
}

// GetTaskProgress 获取任务组进度
func (c *ConversationCoordinator) GetTaskProgress(groupID string) (*models.TaskProgressReport, error) {
	if c.progressTracker == nil {
		return nil, fmt.Errorf("progress tracker not initialized")
	}
	return c.progressTracker.GetProgressReport(groupID)
}

// GetReadyTasks 获取可以开始执行的任务
func (c *ConversationCoordinator) GetReadyTasks(groupID string) ([]string, error) {
	if c.depManager == nil {
		return nil, fmt.Errorf("dependency manager not initialized")
	}
	return c.depManager.GetReadyTasks(groupID)
}

// GetTaskExecutionOrder 获取建议的任务执行顺序
func (c *ConversationCoordinator) GetTaskExecutionOrder(groupID string) ([]string, error) {
	if c.progressTracker == nil {
		return nil, fmt.Errorf("progress tracker not initialized")
	}
	return c.progressTracker.GetTaskExecutionOrder(groupID)
}

// OnTaskCompleted 任务完成回调
func (c *ConversationCoordinator) OnTaskCompleted(ctx context.Context, taskID string) error {
	if c.depManager == nil || c.progressTracker == nil {
		return nil
	}

	// 获取任务
	task, err := c.scheduler.taskStore.Get(taskID)
	if err != nil {
		return err
	}

	// 更新依赖
	if err := c.depManager.UpdateDependencies(taskID); err != nil {
		log.Printf("Failed to update dependencies: %v", err)
	}

	// 更新进度
	if task.GroupID != nil {
		if err := c.progressTracker.UpdateProgress(ctx, *task.GroupID); err != nil {
			log.Printf("Failed to update progress: %v", err)
		}
	}

	return nil
}

// GetTaskGroup 获取任务组详情
func (c *ConversationCoordinator) GetTaskGroup(groupID string) (*models.TaskGroup, error) {
	if c.groupStore == nil {
		return nil, fmt.Errorf("group store not initialized")
	}
	return c.groupStore.Get(groupID)
}

// GetBlockedTasks 获取被阻塞的任务
func (c *ConversationCoordinator) GetBlockedTasks(groupID string) (map[string][]string, error) {
	if c.depManager == nil {
		return nil, fmt.Errorf("dependency manager not initialized")
	}
	return c.depManager.GetBlockedTasks(groupID)
}

// GetExecutionLayers 获取执行层级（用于并行执行）
func (c *ConversationCoordinator) GetExecutionLayers(groupID string) ([][]string, error) {
	if c.depManager == nil {
		return nil, fmt.Errorf("dependency manager not initialized")
	}
	return c.depManager.GetExecutionLayers(groupID)
}

// ProcessWithDecomposition 处理消息（支持自动拆解）
func (c *ConversationCoordinator) ProcessWithDecomposition(
	ctx context.Context,
	sessionID string,
	userMessage string,
	cfg ModelConfig,
	toolNames []string,
	onToken func(string),
) (*ProcessResult, *models.TaskGroup, error) {
	// 检查是否需要拆解
	if c.ShouldDecompose(userMessage) {
		// 执行拆解
		group, err := c.DecomposeTask(ctx, userMessage, sessionID, cfg)
		if err != nil {
			log.Printf("Failed to decompose task: %v", err)
			// 拆解失败，继续正常处理
		} else {
			// 返回拆解结果
			result := &ProcessResult{
				Response:     fmt.Sprintf("已将任务拆解为 %d 个子任务。任务组 ID: %s", group.TotalTasks, group.ID),
				FinishReason: "decomposed",
			}
			return result, group, nil
		}
	}

	// 正常处理
	result, err := c.ProcessMessage(ctx, sessionID, userMessage, cfg, toolNames, onToken)
	return result, nil, err
}