package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/shared/models"
)

// ToolScheduler 工具执行调度器
type ToolScheduler struct {
	registry      *tools.Registry
	toolExecStore store.ToolExecutionStore
	taskStore     store.TaskStore
	taskCreator   TaskCreator

	// 执行队列
	pendingQueue chan *models.ToolExecution
	activeTasks  map[string]*ToolExecutionTask
	mu           sync.RWMutex

	// 配置
	maxConcurrent int
}

// TaskCreator 任务创建器接口
type TaskCreator interface {
	CreateTaskForToolExecution(execution *models.ToolExecution) (*models.Task, error)
}

// ToolExecutionTask 工具执行任务
type ToolExecutionTask struct {
	Execution *models.ToolExecution
	Task      *models.Task
	Cancel    context.CancelFunc
	Done      chan struct{}
}

// NewToolScheduler 创建工具执行调度器
func NewToolScheduler(
	registry *tools.Registry,
	toolExecStore store.ToolExecutionStore,
	taskStore store.TaskStore,
	taskCreator TaskCreator,
	maxConcurrent int,
) *ToolScheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	return &ToolScheduler{
		registry:      registry,
		toolExecStore: toolExecStore,
		taskStore:     taskStore,
		taskCreator:   taskCreator,
		pendingQueue:  make(chan *models.ToolExecution, 100),
		activeTasks:   make(map[string]*ToolExecutionTask),
		maxConcurrent: maxConcurrent,
	}
}

// Start 启动调度器
func (s *ToolScheduler) Start(ctx context.Context) {
	go s.processQueue(ctx)
}

// processQueue 处理执行队列
func (s *ToolScheduler) processQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case execution := <-s.pendingQueue:
			go s.executeTool(ctx, execution)
		}
	}
}

// Schedule 调度工具执行
func (s *ToolScheduler) Schedule(execution *models.ToolExecution) error {
	// 获取工具定义
	toolDef, err := s.registry.Get(execution.ToolName)
	if err != nil {
		return fmt.Errorf("tool not found: %s", execution.ToolName)
	}

	// 更新状态为 pending
	execution.Status = string(models.ToolExecutionStatusPending)
	if err := s.toolExecStore.Create(execution); err != nil {
		return fmt.Errorf("failed to create execution record: %w", err)
	}

	// 本地工具直接执行
	if toolDef.ExecuteMode == models.ToolExecuteModeLocal {
		return s.executeLocalTool(execution)
	}

	// 远程工具加入队列
	select {
	case s.pendingQueue <- execution:
		return nil
	default:
		return fmt.Errorf("execution queue is full")
	}
}

// executeTool 执行工具
func (s *ToolScheduler) executeTool(ctx context.Context, execution *models.ToolExecution) {
	// 检查并发限制
	s.mu.Lock()
	if len(s.activeTasks) >= s.maxConcurrent {
		s.mu.Unlock()
		// 重新加入队列
		s.pendingQueue <- execution
		return
	}

	// 创建执行任务
	taskCtx, cancel := context.WithCancel(ctx)
	execTask := &ToolExecutionTask{
		Execution: execution,
		Cancel:    cancel,
		Done:      make(chan struct{}),
	}
	s.activeTasks[execution.ToolCallID] = execTask
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.activeTasks, execution.ToolCallID)
		s.mu.Unlock()
		close(execTask.Done)
	}()

	// 更新状态为运行中
	if err := s.toolExecStore.UpdateStatus(execution.ID, string(models.ToolExecutionStatusRunning), "", false); err != nil {
		log.Printf("Failed to update execution status: %v", err)
		return
	}

	// 创建远程执行任务
	task, err := s.createRemoteTask(execution)
	if err != nil {
		s.toolExecStore.UpdateStatus(execution.ID, string(models.ToolExecutionStatusFailed), err.Error(), true)
		return
	}

	execTask.Task = task

	// 等待任务完成
	s.waitForTaskCompletion(taskCtx, execTask)
}

// executeLocalTool 执行本地工具
func (s *ToolScheduler) executeLocalTool(execution *models.ToolExecution) error {
	// 更新状态为运行中
	if err := s.toolExecStore.UpdateStatus(execution.ID, string(models.ToolExecutionStatusRunning), "", false); err != nil {
		return err
	}

	var result string
	var isError bool

	// 解析参数
	var args map[string]interface{}
	if execution.Arguments != nil {
		args = execution.Arguments
	}

	// 根据工具名称执行
	switch execution.ToolName {
	case "create_task":
		result, isError = s.handleCreateTask(args)
	case "query_task":
		result, isError = s.handleQueryTask(args)
	case "request_approval":
		result = "Approval requested. Waiting for user confirmation."
		isError = false
	default:
		result = fmt.Sprintf("Unknown local tool: %s", execution.ToolName)
		isError = true
	}

	// 更新结果
	status := string(models.ToolExecutionStatusCompleted)
	if isError {
		status = string(models.ToolExecutionStatusFailed)
	}

	return s.toolExecStore.UpdateStatus(execution.ID, status, result, isError)
}

// handleCreateTask 处理 create_task 工具
func (s *ToolScheduler) handleCreateTask(args map[string]interface{}) (string, bool) {
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

	if priority, ok := args["priority"].(string); ok {
		task.Priority = models.Priority(priority)
	}

	if input, ok := args["input"].(map[string]interface{}); ok {
		task.Input = input
	}

	if tags, ok := args["required_tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				task.RequiredTags = append(task.RequiredTags, s)
			}
		}
	}

	if err := s.taskStore.Create(task); err != nil {
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
func (s *ToolScheduler) handleQueryTask(args map[string]interface{}) (string, bool) {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return "Missing required field: task_id", true
	}

	task, err := s.taskStore.Get(taskID)
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

// createRemoteTask 创建远程执行任务
func (s *ToolScheduler) createRemoteTask(execution *models.ToolExecution) (*models.Task, error) {
	if s.taskCreator == nil {
		return nil, fmt.Errorf("task creator not configured")
	}

	return s.taskCreator.CreateTaskForToolExecution(execution)
}

// waitForTaskCompletion 等待任务完成
func (s *ToolScheduler) waitForTaskCompletion(ctx context.Context, execTask *ToolExecutionTask) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 检查任务状态
			if execTask.Task != nil {
				task, err := s.taskStore.Get(execTask.Task.ID)
				if err != nil {
					continue
				}

				if task.Status == models.TaskStatusCompleted ||
					task.Status == models.TaskStatusFailed ||
					task.Status == models.TaskStatusCancelled {
					// 任务完成，更新执行记录
					result := ""
					isError := task.Status == models.TaskStatusFailed

					if task.Output != nil {
						if outputStr, ok := task.Output["stdout"].(string); ok {
							result = outputStr
						}
						if errStr, ok := task.Output["stderr"].(string); ok && errStr != "" {
							result += "\n" + errStr
						}
					}

					if task.Error != nil {
						result = *task.Error
					}

					status := string(models.ToolExecutionStatusCompleted)
					if isError {
						status = string(models.ToolExecutionStatusFailed)
					}

					s.toolExecStore.UpdateStatus(execTask.Execution.ID, status, result, isError)
					return
				}
			}
		}
	}
}

// Cancel 取消工具执行
func (s *ToolScheduler) Cancel(toolCallID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	execTask, exists := s.activeTasks[toolCallID]
	if !exists {
		return fmt.Errorf("execution not found: %s", toolCallID)
	}

	// 取消任务
	execTask.Cancel()

	// 更新执行状态
	return s.toolExecStore.UpdateStatus(
		execTask.Execution.ID,
		string(models.ToolExecutionStatusFailed),
		"Execution cancelled",
		true,
	)
}

// GetStatus 获取工具执行状态
func (s *ToolScheduler) GetStatus(toolCallID string) (*models.ToolExecution, error) {
	return s.toolExecStore.GetByToolCallID(toolCallID)
}

// ListPendingExecutions 列出待执行的工具调用
func (s *ToolScheduler) ListPendingExecutions(conversationID string) ([]models.ToolExecution, error) {
	executions, err := s.toolExecStore.ListByConversation(conversationID)
	if err != nil {
		return nil, err
	}

	var pending []models.ToolExecution
	for _, exec := range executions {
		if exec.Status == string(models.ToolExecutionStatusPending) ||
			exec.Status == string(models.ToolExecutionStatusRunning) {
			pending = append(pending, exec)
		}
	}

	return pending, nil
}