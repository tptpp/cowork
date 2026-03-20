package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/scheduler"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/tools"
	"github.com/tp/cowork/internal/coordinator/ws"
	"github.com/tp/cowork/internal/shared/errors"
	"github.com/tp/cowork/internal/shared/models"
)

// Handler API 处理器
type Handler struct {
	taskStore        store.TaskStore
	workerStore      store.WorkerStore
	taskLogStore     store.TaskLogStore
	notificationStore store.NotificationStore
	userLayoutStore  store.UserLayoutStore
	agentStore       store.AgentSessionStore
	agentHandler     *AgentHandler
	toolHandler      *ToolHandler
	hub              *ws.Hub
	scheduler        *scheduler.Scheduler
	startTime        time.Time
}

// NewHandler 创建处理器
func NewHandler(
	taskStore store.TaskStore,
	workerStore store.WorkerStore,
	taskLogStore store.TaskLogStore,
	notificationStore store.NotificationStore,
	userLayoutStore store.UserLayoutStore,
	agentStore store.AgentSessionStore,
	hub *ws.Hub,
	sched *scheduler.Scheduler,
	toolRegistry *tools.Registry,
) *Handler {
	return &Handler{
		taskStore:        taskStore,
		workerStore:      workerStore,
		taskLogStore:     taskLogStore,
		notificationStore: notificationStore,
		userLayoutStore:  userLayoutStore,
		agentStore:       agentStore,
		agentHandler:     NewAgentHandler(agentStore),
		toolHandler:      NewToolHandler(toolRegistry),
		hub:              hub,
		scheduler:        sched,
		startTime:        time.Now(),
	}
}

// Response 通用响应
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo 错误信息
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Pagination 分页信息
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// success 成功响应
func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// successWithPagination 带分页的成功响应
func successWithPagination(c *gin.Context, data interface{}, pagination Pagination) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// fail 失败响应
func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	})
}

// failWithError 使用 AppError 失败响应
func failWithError(c *gin.Context, err *errors.AppError) {
	c.JSON(err.Code.HTTPStatus(), Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    string(err.Code),
			Message: err.Message,
		},
	})
}

// SystemStats 系统统计信息
type SystemStats struct {
	Tasks   TaskStats   `json:"tasks"`
	Workers WorkerStats `json:"workers"`
	System  SystemInfo  `json:"system"`
}

// TaskStats 任务统计
type TaskStats struct {
	Total     int64 `json:"total"`
	Pending   int64 `json:"pending"`
	Running   int64 `json:"running"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

// WorkerStats Worker 统计
type WorkerStats struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	Uptime    string `json:"uptime"`
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
}

// GetSystemStats 获取系统统计
func (h *Handler) GetSystemStats(c *gin.Context) {
	// 使用单次查询获取任务统计（优化）
	taskCounts, err := h.taskStore.CountByStatus()
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get task stats", err))
		return
	}

	pendingCount := taskCounts[models.TaskStatusPending]
	runningCount := taskCounts[models.TaskStatusRunning]
	completedCount := taskCounts[models.TaskStatusCompleted]
	failedCount := taskCounts[models.TaskStatusFailed]
	cancelledCount := taskCounts[models.TaskStatusCancelled]

	// 使用单次查询获取 Worker 统计（优化）
	workerCounts, err := h.workerStore.CountByStatus()
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get worker stats", err))
		return
	}

	var online, offline int64
	for status, count := range workerCounts {
		if status == models.WorkerStatusIdle || status == models.WorkerStatusBusy {
			online += count
		} else {
			offline += count
		}
	}

	// 计算总 Worker 数
	var totalWorkers int64
	for _, count := range workerCounts {
		totalWorkers += count
	}

	// 计算运行时间
	uptime := time.Since(h.startTime).Round(time.Second)

	stats := SystemStats{
		Tasks: TaskStats{
			Total:     pendingCount + runningCount + completedCount + failedCount + cancelledCount,
			Pending:   pendingCount,
			Running:   runningCount,
			Completed: completedCount,
			Failed:    failedCount,
		},
		Workers: WorkerStats{
			Total:   int(totalWorkers),
			Online:  int(online),
			Offline: int(offline),
		},
		System: SystemInfo{
			Uptime:    uptime.String(),
			Version:   "1.0.0",
			GoVersion: "go1.21",
		},
	}

	success(c, stats)
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Type            string              `json:"type" binding:"required"`
	Description     string              `json:"description"`
	Priority        models.Priority     `json:"priority"`
	RequiredTags    []string            `json:"required_tags"`
	PreferredModel  string              `json:"preferred_model"`
	Input           models.JSON         `json:"input"`
	Config          models.JSON         `json:"config"`
}

// CreateTask 创建任务
func (h *Handler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 设置默认优先级
	priority := req.Priority
	if priority == "" {
		priority = models.PriorityMedium
	}

	task := &models.Task{
		ID:             uuid.New().String(),
		Type:           req.Type,
		Description:    req.Description,
		Status:         models.TaskStatusPending,
		Priority:       priority,
		RequiredTags:   req.RequiredTags,
		PreferredModel: req.PreferredModel,
		Input:          req.Input,
		Config:         req.Config,
	}

	if err := h.taskStore.Create(task); err != nil {
		failWithError(c, errors.WrapInternal("Failed to create task", err))
		return
	}

	success(c, task)
}

// GetTasks 获取任务列表
func (h *Handler) GetTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize > 100 {
		pageSize = 100
	}

	opts := store.ListOptions{
		Page:     page,
		PageSize: pageSize,
		Status:   c.Query("status"),
		Type:     c.Query("type"),
		WorkerID: c.Query("worker_id"),
		Sort:     c.Query("sort"),
	}

	tasks, total, err := h.taskStore.List(opts)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get tasks", err))
		return
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	successWithPagination(c, tasks, Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetTask 获取任务详情
func (h *Handler) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.taskStore.Get(id)
	if err != nil {
		failWithError(c, errors.TaskNotFound(id))
		return
	}
	success(c, task)
}

// CancelTask 取消任务 (DELETE /tasks/:id)
func (h *Handler) CancelTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.taskStore.Get(id)
	if err != nil {
		failWithError(c, errors.TaskNotFound(id))
		return
	}

	// 检查任务是否可以被取消
	if task.Status == models.TaskStatusCompleted {
		failWithError(c, errors.New(errors.CodeTaskInvalidState, "Cannot cancel a completed task"))
		return
	}
	if task.Status == models.TaskStatusCancelled {
		failWithError(c, errors.New(errors.CodeTaskInvalidState, "Task is already cancelled"))
		return
	}

	// 更新任务状态
	now := time.Now()
	previousStatus := task.Status
	task.Status = models.TaskStatusCancelled
	task.CompletedAt = &now

	if err := h.taskStore.Update(task); err != nil {
		failWithError(c, errors.WrapInternal("Failed to cancel task", err))
		return
	}

	// 广播取消事件
	h.broadcastTaskUpdate(task, "cancelled")

	// 如果任务正在运行，通知调度器释放资源
	if previousStatus == models.TaskStatusRunning && task.WorkerID != nil {
		h.hub.BroadcastToChannel("worker:"+*task.WorkerID, []byte(`{"type":"task_cancelled","task_id":"`+id+`"}`))
	}

	success(c, gin.H{"id": id, "status": task.Status, "previous_status": previousStatus})
}

// GetTaskLogs 获取任务日志
func (h *Handler) GetTaskLogs(c *gin.Context) {
	id := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("lines", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, err := h.taskLogStore.List(id, limit, offset)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get logs", err))
		return
	}

	success(c, gin.H{
		"lines":    logs,
		"total":    len(logs),
		"has_more": len(logs) == limit,
	})
}

// RegisterWorkerRequest 注册 Worker 请求
type RegisterWorkerRequest struct {
	Name           string        `json:"name" binding:"required"`
	Tags           []string      `json:"tags" binding:"required"`
	Model          string        `json:"model"`
	MaxConcurrent  int           `json:"max_concurrent"`
	Capabilities   models.JSON   `json:"capabilities"`
	Metadata       models.JSON   `json:"metadata"`
}

// RegisterWorker 注册 Worker
func (h *Handler) RegisterWorker(c *gin.Context) {
	var req RegisterWorkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 检查名称是否已存在
	if existing, _ := h.workerStore.GetByName(req.Name); existing != nil {
		// 已存在，更新状态
		existing.Status = models.WorkerStatusIdle
		existing.LastSeen = time.Now()
		if err := h.workerStore.Update(existing); err != nil {
			failWithError(c, errors.WrapInternal("Failed to update worker", err))
			return
		}
		success(c, gin.H{
			"id":                existing.ID,
			"name":              existing.Name,
			"status":            existing.Status,
			"heartbeat_interval": 5,
			"created_at":        existing.CreatedAt,
		})
		return
	}

	// 创建新 Worker
	worker := &models.Worker{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Tags:          req.Tags,
		Model:         req.Model,
		Status:        models.WorkerStatusIdle,
		MaxConcurrent: req.MaxConcurrent,
		Capabilities:  req.Capabilities,
		Metadata:      req.Metadata,
		LastSeen:      time.Now(),
	}

	if worker.MaxConcurrent == 0 {
		worker.MaxConcurrent = 1
	}

	if err := h.workerStore.Create(worker); err != nil {
		failWithError(c, errors.WrapInternal("Failed to create worker", err))
		return
	}

	success(c, gin.H{
		"id":                 worker.ID,
		"name":               worker.Name,
		"status":             worker.Status,
		"heartbeat_interval": 5,
		"created_at":         worker.CreatedAt,
	})
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	Status        models.WorkerStatus `json:"status"`
	CurrentTasks  []string            `json:"current_tasks"`
	Progress      map[string]int      `json:"progress"`
	Resources     models.JSON         `json:"resources"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	AssignedTasks  []string `json:"assigned_tasks"`
	CancelledTasks []string `json:"cancelled_tasks"`
	Commands       []string `json:"commands"`
}

// WorkerHeartbeat Worker 心跳
func (h *Handler) WorkerHeartbeat(c *gin.Context) {
	id := c.Param("id")

	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 获取 Worker
	worker, err := h.workerStore.Get(id)
	if err != nil {
		failWithError(c, errors.WorkerNotFound(id))
		return
	}

	// 更新状态
	worker.Status = req.Status
	worker.LastSeen = time.Now()
	if err := h.workerStore.Update(worker); err != nil {
		failWithError(c, errors.WrapInternal("Failed to update worker", err))
		return
	}

	// 广播 Worker 状态更新
	h.hub.BroadcastToChannel("workers", []byte(`{"type":"worker_update","payload":{"id":"`+id+`","status":"`+string(req.Status)+`"}}`))

	success(c, HeartbeatResponse{
		AssignedTasks:  []string{},
		CancelledTasks: []string{},
		Commands:       []string{},
	})
}

// GetWorkers 获取 Worker 列表
func (h *Handler) GetWorkers(c *gin.Context) {
	workers, err := h.workerStore.List()
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get workers", err))
		return
	}

	// 过滤状态
	status := c.Query("status")
	if status != "" {
		var filtered []models.Worker
		for _, w := range workers {
			if string(w.Status) == status {
				filtered = append(filtered, w)
			}
		}
		workers = filtered
	}

	success(c, workers)
}

// GetWorker 获取 Worker 详情
func (h *Handler) GetWorker(c *gin.Context) {
	id := c.Param("id")
	worker, err := h.workerStore.Get(id)
	if err != nil {
		failWithError(c, errors.WorkerNotFound(id))
		return
	}
	success(c, worker)
}

// UnregisterWorker 注销 Worker
func (h *Handler) UnregisterWorker(c *gin.Context) {
	id := c.Param("id")
	if err := h.workerStore.Delete(id); err != nil {
		failWithError(c, errors.WorkerNotFound(id))
		return
	}
	success(c, gin.H{"id": id})
}

// UpdateTaskStatusRequest 更新任务状态请求
type UpdateTaskStatusRequest struct {
	Status   models.TaskStatus `json:"status"`
	Progress int               `json:"progress"`
	Output   models.JSON       `json:"output"`
	Error    string            `json:"error"`
}

// UpdateTaskStatus 更新任务状态 (Worker 调用)
func (h *Handler) UpdateTaskStatus(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTaskStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	task, err := h.taskStore.Get(id)
	if err != nil {
		failWithError(c, errors.TaskNotFound(id))
		return
	}

	// 更新状态
	task.Status = req.Status
	if req.Progress > 0 {
		task.Progress = req.Progress
	}
	if req.Output != nil {
		task.Output = req.Output
	}
	if req.Error != "" {
		task.Error = &req.Error
	}

	// 设置时间
	now := time.Now()
	if req.Status == models.TaskStatusCompleted || req.Status == models.TaskStatusFailed {
		task.CompletedAt = &now
	}

	if err := h.taskStore.Update(task); err != nil {
		failWithError(c, errors.WrapInternal("Failed to update task", err))
		return
	}

	// 广播任务状态更新
	h.broadcastTaskUpdate(task, string(req.Status))

	success(c, task)
}

// CreateTaskLogRequest 创建任务日志请求
type CreateTaskLogRequest struct {
	Level    string      `json:"level"`
	Message  string      `json:"message"`
	Metadata models.JSON `json:"metadata"`
}

// CreateTaskLog 创建任务日志 (Worker 调用)
func (h *Handler) CreateTaskLog(c *gin.Context) {
	id := c.Param("id")

	var req CreateTaskLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	// 验证任务存在
	_, err := h.taskStore.Get(id)
	if err != nil {
		failWithError(c, errors.TaskNotFound(id))
		return
	}

	// 创建日志
	taskLog := &models.TaskLog{
		TaskID:   id,
		Level:    req.Level,
		Message:  req.Message,
		Metadata: req.Metadata,
	}

	if taskLog.Level == "" {
		taskLog.Level = "info"
	}

	if err := h.taskLogStore.Create(taskLog); err != nil {
		failWithError(c, errors.WrapInternal("Failed to create log", err))
		return
	}

	// 广播日志
	h.broadcastTaskLog(id, taskLog)

	success(c, taskLog)
}

// broadcastTaskUpdate 广播任务状态更新
func (h *Handler) broadcastTaskUpdate(task *models.Task, event string) {
	update := map[string]interface{}{
		"type":      "task_update",
		"event":     event,
		"task_id":   task.ID,
		"status":    task.Status,
		"progress":  task.Progress,
		"worker_id": task.WorkerID,
		"timestamp": time.Now().Unix(),
	}

	data, _ := json.Marshal(update)
	h.hub.BroadcastToChannel("tasks", data)
	h.hub.BroadcastToChannel("task:"+task.ID, data)

	// 创建通知
	switch event {
	case "completed":
		h.CreateNotification("task_complete", "任务完成",
			"任务 "+task.ID+" 已成功完成",
			models.JSON{"task_id": task.ID, "type": task.Type})
	case "failed":
		errMsg := ""
		if task.Error != nil {
			errMsg = *task.Error
		}
		h.CreateNotification("task_failed", "任务失败",
			"任务 "+task.ID+" 执行失败: "+errMsg,
			models.JSON{"task_id": task.ID, "error": errMsg})
	}
}

// broadcastTaskLog 广播任务日志
func (h *Handler) broadcastTaskLog(taskID string, log *models.TaskLog) {
	update := map[string]interface{}{
		"type":      "task_log",
		"task_id":   taskID,
		"level":     log.Level,
		"message":   log.Message,
		"timestamp": log.Time.Unix(),
	}

	data, _ := json.Marshal(update)
	h.hub.BroadcastToChannel("task:"+taskID, data)
}

// DefaultUserID 默认用户 ID（目前没有用户系统）
const DefaultUserID = "default"

// GetLayout 获取用户布局
func (h *Handler) GetLayout(c *gin.Context) {
	layout, err := h.userLayoutStore.Get(DefaultUserID)
	if err != nil {
		// 返回空布局
		success(c, gin.H{
			"widgets": []interface{}{},
		})
		return
	}

	success(c, gin.H{
		"widgets": layout.Widgets,
	})
}

// SaveLayoutRequest 保存布局请求
type SaveLayoutRequest struct {
	Widgets models.JSON `json:"widgets"`
}

// SaveLayout 保存用户布局
func (h *Handler) SaveLayout(c *gin.Context) {
	var req SaveLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	if err := h.userLayoutStore.Save(DefaultUserID, req.Widgets); err != nil {
		failWithError(c, errors.WrapInternal("Failed to save layout", err))
		return
	}

	success(c, gin.H{
		"success": true,
	})
}

// Agent API - 委托给 AgentHandler

// CreateAgentSession 创建 Agent 会话
func (h *Handler) CreateAgentSession(c *gin.Context) {
	h.agentHandler.CreateAgentSession(c)
}

// GetAgentSession 获取 Agent 会话
func (h *Handler) GetAgentSession(c *gin.Context) {
	h.agentHandler.GetAgentSession(c)
}

// GetAgentSessions 获取 Agent 会话列表
func (h *Handler) GetAgentSessions(c *gin.Context) {
	h.agentHandler.GetAgentSessions(c)
}

// DeleteAgentSession 删除 Agent 会话
func (h *Handler) DeleteAgentSession(c *gin.Context) {
	h.agentHandler.DeleteAgentSession(c)
}

// SendAgentMessage 发送消息并获取流式响应
func (h *Handler) SendAgentMessage(c *gin.Context) {
	h.agentHandler.SendAgentMessage(c)
}

// GetAgentMessages 获取会话消息列表
func (h *Handler) GetAgentMessages(c *gin.Context) {
	h.agentHandler.GetAgentMessages(c)
}

// Notification API

// GetNotifications 获取通知列表
func (h *Handler) GetNotifications(c *gin.Context) {
	unreadOnly := c.Query("unread") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	notifications, err := h.notificationStore.List(unreadOnly, limit)
	if err != nil {
		failWithError(c, errors.WrapInternal("Failed to get notifications", err))
		return
	}

	success(c, notifications)
}

// MarkNotificationsRead 标记通知已读
func (h *Handler) MarkNotificationsRead(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failWithError(c, errors.InvalidRequest(err.Error()))
		return
	}

	if len(req.IDs) == 0 {
		failWithError(c, errors.InvalidRequest("No notification IDs provided"))
		return
	}

	if err := h.notificationStore.MarkRead(req.IDs); err != nil {
		failWithError(c, errors.WrapInternal("Failed to mark notifications as read", err))
		return
	}

	success(c, gin.H{"success": true})
}

// MarkAllNotificationsRead 标记所有通知已读
func (h *Handler) MarkAllNotificationsRead(c *gin.Context) {
	if err := h.notificationStore.MarkAllRead(); err != nil {
		failWithError(c, errors.WrapInternal("Failed to mark all notifications as read", err))
		return
	}

	success(c, gin.H{"success": true})
}

// CreateNotification 创建并发送通知
func (h *Handler) CreateNotification(notifType, title, message string, data models.JSON) {
	notification := &models.Notification{
		Type:    notifType,
		Title:   title,
		Message: message,
		Data:    data,
		Read:    false,
	}

	if err := h.notificationStore.Create(notification); err != nil {
		return
	}

	// 广播通知到前端
	h.broadcastNotification(notification)
}

// broadcastNotification 广播通知
func (h *Handler) broadcastNotification(notification *models.Notification) {
	update := map[string]interface{}{
		"type":        "notification",
		"id":          notification.ID,
		"notif_type":  notification.Type,
		"title":       notification.Title,
		"message":     notification.Message,
		"data":        notification.Data,
		"read":        notification.Read,
		"created_at":  notification.CreatedAt.Unix(),
	}

	data, _ := json.Marshal(update)
	h.hub.BroadcastToChannel("notifications", data)
	// 也广播到所有客户端
	h.hub.Broadcast(data)
}

// Tool API - 委托给 ToolHandler

// GetTools 获取工具列表
func (h *Handler) GetTools(c *gin.Context) {
	h.toolHandler.GetTools(c)
}

// GetTool 获取工具详情
func (h *Handler) GetTool(c *gin.Context) {
	h.toolHandler.GetTool(c)
}

// CreateTool 创建自定义工具
func (h *Handler) CreateTool(c *gin.Context) {
	h.toolHandler.CreateTool(c)
}

// UpdateTool 更新工具
func (h *Handler) UpdateTool(c *gin.Context) {
	h.toolHandler.UpdateTool(c)
}

// DeleteTool 删除工具
func (h *Handler) DeleteTool(c *gin.Context) {
	h.toolHandler.DeleteTool(c)
}

// EnableTool 启用工具
func (h *Handler) EnableTool(c *gin.Context) {
	h.toolHandler.EnableTool(c)
}

// DisableTool 禁用工具
func (h *Handler) DisableTool(c *gin.Context) {
	h.toolHandler.DisableTool(c)
}