package agent

import (
	"context"
	"fmt"

	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/coordinator/ws"
	"github.com/tp/cowork/internal/shared/models"
)

// Coordinator Coordinator Agent - 使用统一的 Agent 结构
type Coordinator struct {
	agent *Agent

	// Coordinator 特有的能力
	templateStore store.AgentTemplateStore
	hub           *ws.Hub // WebSocket 用于推送
}

// NewCoordinator 创建 Coordinator
func NewCoordinator(
	llmClient *LLMClient,
	registry *tools.Registry,
	toolExecStore store.ToolExecutionStore,
	taskStore store.TaskStore,
	sessionStore store.AgentSessionStore,
	templateStore store.AgentTemplateStore,
	hub *ws.Hub,
	config AgentConfig,
) *Coordinator {
	// 加载 coordinator 模板
	template, err := templateStore.Get("coordinator-template")
	if err != nil || template == nil {
		// 使用默认模板
		template = &models.AgentTemplate{
			ID:          "coordinator-template",
			Name:        "Coordinator",
			Description: "Task coordination, scheduling, monitoring",
			BasePrompt:  "You are a coordinator agent. Your role is to decompose tasks, monitor progress, and coordinate between agents. You can create tasks, query status, and communicate with other agents.",
			AllowedTools: models.StringArray{"create_task", "cancel_task", "query_tasks", "query_nodes", "send_message", "report_to_user"},
			IsSystem:    true,
		}
	}

	agent := NewAgent(
		"coordinator",
		template,
		llmClient,
		registry,
		toolExecStore,
		taskStore,
		sessionStore,
		config,
	)

	return &Coordinator{
		agent:         agent,
		templateStore: templateStore,
		hub:           hub,
	}
}

// ProcessMessage 处理消息（委托给 Agent）
func (c *Coordinator) ProcessMessage(
	ctx context.Context,
	userMessage string,
	cfg ModelConfig,
	onToken func(string),
) (*ProcessResult, error) {
	return c.agent.ProcessMessage(ctx, userMessage, cfg, onToken)
}

// ExecuteToolDirectly 直接执行工具 (用于 Human-in-loop 批准后执行)
func (c *Coordinator) ExecuteToolDirectly(
	ctx context.Context,
	execution *models.ToolExecution,
) (string, error) {
	// 获取工具定义
	toolDef, err := c.agent.registry.Get(execution.ToolName)
	if err != nil {
		return "", fmt.Errorf("tool not found: %s", execution.ToolName)
	}

	// 根据执行模式处理
	if toolDef.ExecuteMode == models.ToolExecuteModeLocal {
		// 本地工具直接执行（使用 Agent 的工具执行能力）
		var args map[string]interface{}
		if execution.Arguments != nil {
			args = execution.Arguments
		}

		result, err := c.agent.executeLocalTool(execution, args)
		if err != nil {
			return "", err
		}
		return result.Result, nil
	}

	// 远程工具 - 返回提示信息
	return fmt.Sprintf("Remote tool '%s' queued for execution. Requires worker.", execution.ToolName), nil
}

// ProcessResult 处理结果 (保持兼容)
type ProcessResult struct {
	Response      string
	ToolCalls     []models.ToolCall
	ToolResults   []ToolResultInfo
	TotalRounds   int
	FinishReason  string
}

// ToolResultInfo 工具结果信息 (保持兼容)
type ToolResultInfo struct {
	ToolCallID string
	ToolName   string
	Result     string
	IsError    bool
}