package store

import (
	"errors"
	"fmt"
	"time"

	"github.com/tp/cowork/internal/shared/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 数据库配置
type Config struct {
	Path string // SQLite 数据库文件路径
}

// Store 数据库存储
type Store struct {
	db *gorm.DB
}

// New 创建新的数据库存储
func New(cfg Config) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// 自动迁移
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Store{db: db}, nil
}

// autoMigrate 自动迁移数据库表
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Task{},
		&models.TaskLog{},
		&models.TaskFile{},
		&models.Worker{},
		&models.AgentSession{},
		&models.AgentMessage{},
		&models.Notification{},
		&models.UserLayout{},
		&models.ToolDefinition{},
		&models.ToolExecution{},
		&models.TaskGroup{},
		&models.TaskDependency{},
	)
}

// DB 获取底层数据库连接
func (s *Store) DB() *gorm.DB {
	return s.db
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// TaskStore 任务存储接口
type TaskStore interface {
	Create(task *models.Task) error
	Get(id string) (*models.Task, error)
	List(opts ListOptions) ([]models.Task, int64, error)
	Update(task *models.Task) error
	Delete(id string) error
	GetByStatus(status models.TaskStatus) ([]models.Task, error)
	CountByStatus() (map[models.TaskStatus]int64, error)
}

// taskStore 任务存储实现
type taskStore struct {
	db *gorm.DB
}

// NewTaskStore 创建任务存储
func NewTaskStore(db *gorm.DB) TaskStore {
	return &taskStore{db: db}
}

// Create 创建任务
func (s *taskStore) Create(task *models.Task) error {
	return s.db.Create(task).Error
}

// Get 获取任务
func (s *taskStore) Get(id string) (*models.Task, error) {
	var task models.Task
	err := s.db.Preload("Worker").Preload("Logs").Preload("Files").First(&task, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ListOptions 列表查询选项
type ListOptions struct {
	Page     int
	PageSize int
	Status   string
	Type     string
	WorkerID string
	Sort     string
}

// List 获取任务列表
func (s *taskStore) List(opts ListOptions) ([]models.Task, int64, error) {
	var tasks []models.Task
	var total int64

	query := s.db.Model(&models.Task{})

	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.Type != "" {
		query = query.Where("type = ?", opts.Type)
	}
	if opts.WorkerID != "" {
		query = query.Where("worker_id = ?", opts.WorkerID)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	order := "created_at DESC"
	if opts.Sort != "" {
		order = opts.Sort
	}
	query = query.Order(order)

	// 分页
	if opts.Page > 0 && opts.PageSize > 0 {
		offset := (opts.Page - 1) * opts.PageSize
		query = query.Offset(offset).Limit(opts.PageSize)
	}

	err := query.Preload("Worker").Find(&tasks).Error
	return tasks, total, err
}

// Update 更新任务
func (s *taskStore) Update(task *models.Task) error {
	return s.db.Save(task).Error
}

// Delete 删除任务
func (s *taskStore) Delete(id string) error {
	return s.db.Delete(&models.Task{}, "id = ?", id).Error
}

// GetByStatus 按状态获取任务
func (s *taskStore) GetByStatus(status models.TaskStatus) ([]models.Task, error) {
	var tasks []models.Task
	err := s.db.Where("status = ?", status).Find(&tasks).Error
	return tasks, err
}

// CountByStatus 按状态统计任务数量（单次查询优化）
func (s *taskStore) CountByStatus() (map[models.TaskStatus]int64, error) {
	type StatusCount struct {
		Status models.TaskStatus
		Count  int64
	}

	var results []StatusCount
	err := s.db.Model(&models.Task{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	counts := make(map[models.TaskStatus]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}

	return counts, nil
}

// WorkerStore Worker 存储接口
type WorkerStore interface {
	Create(worker *models.Worker) error
	Get(id string) (*models.Worker, error)
	GetByName(name string) (*models.Worker, error)
	List() ([]models.Worker, error)
	Update(worker *models.Worker) error
	Delete(id string) error
	UpdateHeartbeat(id string, status models.WorkerStatus) error
	CountByStatus() (map[models.WorkerStatus]int64, error)
}

// workerStore Worker 存储实现
type workerStore struct {
	db *gorm.DB
}

// NewWorkerStore 创建 Worker 存储
func NewWorkerStore(db *gorm.DB) WorkerStore {
	return &workerStore{db: db}
}

// Create 创建 Worker
func (s *workerStore) Create(worker *models.Worker) error {
	return s.db.Create(worker).Error
}

// Get 获取 Worker
func (s *workerStore) Get(id string) (*models.Worker, error) {
	var worker models.Worker
	err := s.db.First(&worker, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &worker, nil
}

// GetByName 按名称获取 Worker
func (s *workerStore) GetByName(name string) (*models.Worker, error) {
	var worker models.Worker
	err := s.db.First(&worker, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &worker, nil
}

// List 获取 Worker 列表
func (s *workerStore) List() ([]models.Worker, error) {
	var workers []models.Worker
	err := s.db.Find(&workers).Error
	return workers, err
}

// Update 更新 Worker
func (s *workerStore) Update(worker *models.Worker) error {
	return s.db.Save(worker).Error
}

// Delete 删除 Worker
func (s *workerStore) Delete(id string) error {
	return s.db.Delete(&models.Worker{}, "id = ?", id).Error
}

// UpdateHeartbeat 更新心跳时间
func (s *workerStore) UpdateHeartbeat(id string, status models.WorkerStatus) error {
	return s.db.Model(&models.Worker{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":    status,
		"last_seen": gorm.Expr("datetime('now')"),
	}).Error
}

// CountByStatus 按状态统计 Worker 数量（单次查询优化）
func (s *workerStore) CountByStatus() (map[models.WorkerStatus]int64, error) {
	type StatusCount struct {
		Status models.WorkerStatus
		Count  int64
	}

	var results []StatusCount
	err := s.db.Model(&models.Worker{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	counts := make(map[models.WorkerStatus]int64)
	for _, r := range results {
		counts[r.Status] = r.Count
	}

	return counts, nil
}

// TaskLogStore 任务日志存储接口
type TaskLogStore interface {
	Create(log *models.TaskLog) error
	List(taskID string, limit, offset int) ([]models.TaskLog, error)
}

// taskLogStore 任务日志存储实现
type taskLogStore struct {
	db *gorm.DB
}

// NewTaskLogStore 创建任务日志存储
func NewTaskLogStore(db *gorm.DB) TaskLogStore {
	return &taskLogStore{db: db}
}

// Create 创建日志
func (s *taskLogStore) Create(log *models.TaskLog) error {
	return s.db.Create(log).Error
}

// List 获取日志列表
func (s *taskLogStore) List(taskID string, limit, offset int) ([]models.TaskLog, error) {
	var logs []models.TaskLog
	query := s.db.Where("task_id = ?", taskID).Order("time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&logs).Error
	return logs, err
}

// NotificationStore 通知存储接口
type NotificationStore interface {
	Create(notification *models.Notification) error
	List(unreadOnly bool, limit int) ([]models.Notification, error)
	MarkRead(ids []uint) error
	MarkAllRead() error
}

// notificationStore 通知存储实现
type notificationStore struct {
	db *gorm.DB
}

// NewNotificationStore 创建通知存储
func NewNotificationStore(db *gorm.DB) NotificationStore {
	return &notificationStore{db: db}
}

// Create 创建通知
func (s *notificationStore) Create(notification *models.Notification) error {
	return s.db.Create(notification).Error
}

// List 获取通知列表
func (s *notificationStore) List(unreadOnly bool, limit int) ([]models.Notification, error) {
	var notifications []models.Notification
	query := s.db.Order("created_at DESC")
	if unreadOnly {
		query = query.Where("read = ?", false)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&notifications).Error
	return notifications, err
}

// MarkRead 标记已读
func (s *notificationStore) MarkRead(ids []uint) error {
	return s.db.Model(&models.Notification{}).Where("id IN ?", ids).Update("read", true).Error
}

// MarkAllRead 标记全部已读
func (s *notificationStore) MarkAllRead() error {
	return s.db.Model(&models.Notification{}).Where("read = ?", false).Update("read", true).Error
}

// UserLayoutStore 用户布局存储接口
type UserLayoutStore interface {
	Get(userID string) (*models.UserLayout, error)
	Save(userID string, widgets models.JSON) error
}

// userLayoutStore 用户布局存储实现
type userLayoutStore struct {
	db *gorm.DB
}

// NewUserLayoutStore 创建用户布局存储
func NewUserLayoutStore(db *gorm.DB) UserLayoutStore {
	return &userLayoutStore{db: db}
}

// Get 获取用户布局
func (s *userLayoutStore) Get(userID string) (*models.UserLayout, error) {
	var layout models.UserLayout
	err := s.db.Where("user_id = ?", userID).First(&layout).Error
	if err != nil {
		return nil, err
	}
	return &layout, nil
}

// Save 保存用户布局
func (s *userLayoutStore) Save(userID string, widgets models.JSON) error {
	var layout models.UserLayout
	err := s.db.Where("user_id = ?", userID).First(&layout).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新记录
			layout = models.UserLayout{
				UserID:  userID,
				Widgets: widgets,
			}
			return s.db.Create(&layout).Error
		}
		return err
	}
	// 更新现有记录
	layout.Widgets = widgets
	return s.db.Save(&layout).Error
}

// AgentSessionStore Agent 会话存储接口
type AgentSessionStore interface {
	Create(session *models.AgentSession) error
	Get(id string) (*models.AgentSession, error)
	List() ([]models.AgentSession, error)
	Delete(id string) error
	AddMessage(sessionID string, role, content string) (*models.AgentMessage, error)
	AddMessageWithToolCalls(msg *models.AgentMessage) error
	GetMessages(sessionID string, limit int) ([]models.AgentMessage, error)
}

// agentSessionStore Agent 会话存储实现
type agentSessionStore struct {
	db *gorm.DB
}

// NewAgentSessionStore 创建 Agent 会话存储
func NewAgentSessionStore(db *gorm.DB) AgentSessionStore {
	return &agentSessionStore{db: db}
}

// Create 创建会话
func (s *agentSessionStore) Create(session *models.AgentSession) error {
	return s.db.Create(session).Error
}

// Get 获取会话
func (s *agentSessionStore) Get(id string) (*models.AgentSession, error) {
	var session models.AgentSession
	err := s.db.First(&session, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// List 获取会话列表
func (s *agentSessionStore) List() ([]models.AgentSession, error) {
	var sessions []models.AgentSession
	err := s.db.Order("updated_at DESC").Find(&sessions).Error
	return sessions, err
}

// Delete 删除会话
func (s *agentSessionStore) Delete(id string) error {
	// 先删除关联的消息
	if err := s.db.Delete(&models.AgentMessage{}, "session_id = ?", id).Error; err != nil {
		return err
	}
	return s.db.Delete(&models.AgentSession{}, "id = ?", id).Error
}

// AddMessage 添加消息
func (s *agentSessionStore) AddMessage(sessionID string, role, content string) (*models.AgentMessage, error) {
	msg := &models.AgentMessage{
		SessionID: sessionID,
		Role:      role,
		Content:   content,
	}
	if err := s.db.Create(msg).Error; err != nil {
		return nil, err
	}
	// 更新会话的 updated_at
	s.db.Model(&models.AgentSession{}).Where("id = ?", sessionID).Update("updated_at", gorm.Expr("datetime('now')"))
	return msg, nil
}

// AddMessageWithToolCalls 添加带 ToolCalls 的消息
func (s *agentSessionStore) AddMessageWithToolCalls(msg *models.AgentMessage) error {
	if err := s.db.Create(msg).Error; err != nil {
		return err
	}
	// 更新会话的 updated_at
	return s.db.Model(&models.AgentSession{}).Where("id = ?", msg.SessionID).Update("updated_at", gorm.Expr("datetime('now')")).Error
}

// GetMessages 获取消息列表
func (s *agentSessionStore) GetMessages(sessionID string, limit int) ([]models.AgentMessage, error) {
	var messages []models.AgentMessage
	query := s.db.Where("session_id = ?", sessionID).Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&messages).Error
	return messages, err
}

// ToolDefinitionStore 工具定义存储接口
type ToolDefinitionStore interface {
	Create(tool *models.ToolDefinition) error
	Get(id string) (*models.ToolDefinition, error)
	GetByName(name string) (*models.ToolDefinition, error)
	List(opts ToolListOptions) ([]models.ToolDefinition, int64, error)
	Update(tool *models.ToolDefinition) error
	Delete(id string) error
	GetByCategory(category models.ToolCategory) ([]models.ToolDefinition, error)
	GetEnabled() ([]models.ToolDefinition, error)
	GetBuiltin() ([]models.ToolDefinition, error)
}

// ToolListOptions 工具列表查询选项
type ToolListOptions struct {
	Page     int
	PageSize int
	Category string
	Enabled  *bool
}

// toolDefinitionStore 工具定义存储实现
type toolDefinitionStore struct {
	db *gorm.DB
}

// NewToolDefinitionStore 创建工具定义存储
func NewToolDefinitionStore(db *gorm.DB) ToolDefinitionStore {
	return &toolDefinitionStore{db: db}
}

// Create 创建工具定义
func (s *toolDefinitionStore) Create(tool *models.ToolDefinition) error {
	return s.db.Create(tool).Error
}

// Get 获取工具定义
func (s *toolDefinitionStore) Get(id string) (*models.ToolDefinition, error) {
	var tool models.ToolDefinition
	err := s.db.First(&tool, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

// GetByName 按名称获取工具定义
func (s *toolDefinitionStore) GetByName(name string) (*models.ToolDefinition, error) {
	var tool models.ToolDefinition
	err := s.db.First(&tool, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

// List 获取工具定义列表
func (s *toolDefinitionStore) List(opts ToolListOptions) ([]models.ToolDefinition, int64, error) {
	var tools []models.ToolDefinition
	var total int64

	query := s.db.Model(&models.ToolDefinition{})

	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Enabled != nil {
		query = query.Where("is_enabled = ?", *opts.Enabled)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	query = query.Order("name ASC")

	// 分页
	if opts.Page > 0 && opts.PageSize > 0 {
		offset := (opts.Page - 1) * opts.PageSize
		query = query.Offset(offset).Limit(opts.PageSize)
	}

	err := query.Find(&tools).Error
	return tools, total, err
}

// Update 更新工具定义
func (s *toolDefinitionStore) Update(tool *models.ToolDefinition) error {
	return s.db.Save(tool).Error
}

// Delete 删除工具定义
func (s *toolDefinitionStore) Delete(id string) error {
	return s.db.Delete(&models.ToolDefinition{}, "id = ?", id).Error
}

// GetByCategory 按类别获取工具定义
func (s *toolDefinitionStore) GetByCategory(category models.ToolCategory) ([]models.ToolDefinition, error) {
	var tools []models.ToolDefinition
	err := s.db.Where("category = ? AND is_enabled = ?", category, true).Find(&tools).Error
	return tools, err
}

// GetEnabled 获取所有启用的工具定义
func (s *toolDefinitionStore) GetEnabled() ([]models.ToolDefinition, error) {
	var tools []models.ToolDefinition
	err := s.db.Where("is_enabled = ?", true).Order("name ASC").Find(&tools).Error
	return tools, err
}

// GetBuiltin 获取所有内置工具定义
func (s *toolDefinitionStore) GetBuiltin() ([]models.ToolDefinition, error) {
	var tools []models.ToolDefinition
	err := s.db.Where("is_builtin = ?", true).Order("name ASC").Find(&tools).Error
	return tools, err
}

// ToolExecutionStore 工具执行存储接口
type ToolExecutionStore interface {
	Create(execution *models.ToolExecution) error
	Get(id uint) (*models.ToolExecution, error)
	GetByToolCallID(toolCallID string) (*models.ToolExecution, error)
	ListByConversation(conversationID string) ([]models.ToolExecution, error)
	ListByMessage(messageID uint) ([]models.ToolExecution, error)
	Update(execution *models.ToolExecution) error
	UpdateStatus(id uint, status string, result string, isError bool) error
}

// toolExecutionStore 工具执行存储实现
type toolExecutionStore struct {
	db *gorm.DB
}

// NewToolExecutionStore 创建工具执行存储
func NewToolExecutionStore(db *gorm.DB) ToolExecutionStore {
	return &toolExecutionStore{db: db}
}

// Create 创建工具执行记录
func (s *toolExecutionStore) Create(execution *models.ToolExecution) error {
	return s.db.Create(execution).Error
}

// Get 获取工具执行记录
func (s *toolExecutionStore) Get(id uint) (*models.ToolExecution, error) {
	var execution models.ToolExecution
	err := s.db.First(&execution, id).Error
	if err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetByToolCallID 根据工具调用 ID 获取执行记录
func (s *toolExecutionStore) GetByToolCallID(toolCallID string) (*models.ToolExecution, error) {
	var execution models.ToolExecution
	err := s.db.Where("tool_call_id = ?", toolCallID).First(&execution).Error
	if err != nil {
		return nil, err
	}
	return &execution, nil
}

// ListByConversation 获取对话的所有工具执行记录
func (s *toolExecutionStore) ListByConversation(conversationID string) ([]models.ToolExecution, error) {
	var executions []models.ToolExecution
	err := s.db.Where("conversation_id = ?", conversationID).Order("created_at ASC").Find(&executions).Error
	return executions, err
}

// ListByMessage 获取消息的所有工具执行记录
func (s *toolExecutionStore) ListByMessage(messageID uint) ([]models.ToolExecution, error) {
	var executions []models.ToolExecution
	err := s.db.Where("message_id = ?", messageID).Order("created_at ASC").Find(&executions).Error
	return executions, err
}

// Update 更新工具执行记录
func (s *toolExecutionStore) Update(execution *models.ToolExecution) error {
	return s.db.Save(execution).Error
}

// UpdateStatus 更新工具执行状态
func (s *toolExecutionStore) UpdateStatus(id uint, status string, result string, isError bool) error {
	updates := map[string]interface{}{
		"status":   status,
		"result":   result,
		"is_error": isError,
	}

	if status == string(models.ToolExecutionStatusRunning) {
		now := time.Now()
		updates["started_at"] = &now
	} else if status == string(models.ToolExecutionStatusCompleted) ||
		status == string(models.ToolExecutionStatusFailed) {
		now := time.Now()
		updates["completed_at"] = &now
	}

	return s.db.Model(&models.ToolExecution{}).Where("id = ?", id).Updates(updates).Error
}

// ========== Phase 5: 任务组和依赖存储 ==========

// TaskGroupStore 任务组存储接口
type TaskGroupStore interface {
	Create(group *models.TaskGroup) error
	Get(id string) (*models.TaskGroup, error)
	GetByConversation(conversationID string) ([]models.TaskGroup, error)
	Update(group *models.TaskGroup) error
	Delete(id string) error
	List(opts ListOptions) ([]models.TaskGroup, int64, error)
}

// taskGroupStore 任务组存储实现
type taskGroupStore struct {
	db *gorm.DB
}

// NewTaskGroupStore 创建任务组存储
func NewTaskGroupStore(db *gorm.DB) TaskGroupStore {
	return &taskGroupStore{db: db}
}

// Create 创建任务组
func (s *taskGroupStore) Create(group *models.TaskGroup) error {
	return s.db.Create(group).Error
}

// Get 获取任务组
func (s *taskGroupStore) Get(id string) (*models.TaskGroup, error) {
	var group models.TaskGroup
	err := s.db.Preload("Tasks").First(&group, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// GetByConversation 按会话 ID 获取任务组
func (s *taskGroupStore) GetByConversation(conversationID string) ([]models.TaskGroup, error) {
	var groups []models.TaskGroup
	err := s.db.Where("conversation_id = ?", conversationID).Order("created_at DESC").Find(&groups).Error
	return groups, err
}

// Update 更新任务组
func (s *taskGroupStore) Update(group *models.TaskGroup) error {
	return s.db.Save(group).Error
}

// Delete 删除任务组
func (s *taskGroupStore) Delete(id string) error {
	return s.db.Delete(&models.TaskGroup{}, "id = ?", id).Error
}

// List 获取任务组列表
func (s *taskGroupStore) List(opts ListOptions) ([]models.TaskGroup, int64, error) {
	var groups []models.TaskGroup
	var total int64

	query := s.db.Model(&models.TaskGroup{})

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	order := "created_at DESC"
	if opts.Sort != "" {
		order = opts.Sort
	}
	query = query.Order(order)

	// 分页
	if opts.Page > 0 && opts.PageSize > 0 {
		offset := (opts.Page - 1) * opts.PageSize
		query = query.Offset(offset).Limit(opts.PageSize)
	}

	err := query.Find(&groups).Error
	return groups, total, err
}

// TaskDependencyStore 任务依赖存储接口
type TaskDependencyStore interface {
	Create(dep *models.TaskDependency) error
	Get(id uint) (*models.TaskDependency, error)
	GetByTaskID(taskID string) ([]models.TaskDependency, error)
	GetDependents(taskID string) ([]models.TaskDependency, error) // 获取依赖此任务的其他任务
	MarkSatisfied(id uint) error
	Delete(id uint) error
	GetGroupTasks(groupID string) ([]models.Task, error)
}

// taskDependencyStore 任务依赖存储实现
type taskDependencyStore struct {
	db *gorm.DB
}

// NewTaskDependencyStore 创建任务依赖存储
func NewTaskDependencyStore(db *gorm.DB) TaskDependencyStore {
	return &taskDependencyStore{db: db}
}

// Create 创建依赖关系
func (s *taskDependencyStore) Create(dep *models.TaskDependency) error {
	return s.db.Create(dep).Error
}

// Get 获取依赖关系
func (s *taskDependencyStore) Get(id uint) (*models.TaskDependency, error) {
	var dep models.TaskDependency
	err := s.db.First(&dep, id).Error
	if err != nil {
		return nil, err
	}
	return &dep, nil
}

// GetByTaskID 获取任务的所有依赖
func (s *taskDependencyStore) GetByTaskID(taskID string) ([]models.TaskDependency, error) {
	var deps []models.TaskDependency
	err := s.db.Where("task_id = ?", taskID).Find(&deps).Error
	return deps, err
}

// GetDependents 获取依赖此任务的其他任务
func (s *taskDependencyStore) GetDependents(taskID string) ([]models.TaskDependency, error) {
	var deps []models.TaskDependency
	err := s.db.Where("depends_on_task_id = ?", taskID).Find(&deps).Error
	return deps, err
}

// MarkSatisfied 标记依赖已满足
func (s *taskDependencyStore) MarkSatisfied(id uint) error {
	return s.db.Model(&models.TaskDependency{}).Where("id = ?", id).Update("is_satisfied", true).Error
}

// Delete 删除依赖关系
func (s *taskDependencyStore) Delete(id uint) error {
	return s.db.Delete(&models.TaskDependency{}, id).Error
}

// GetGroupTasks 获取任务组中的所有任务
func (s *taskDependencyStore) GetGroupTasks(groupID string) ([]models.Task, error) {
	var tasks []models.Task
	err := s.db.Where("group_id = ?", groupID).Order("created_at ASC").Find(&tasks).Error
	return tasks, err
}

// TaskFileStore 任务文件存储接口
type TaskFileStore interface {
	Create(file *models.TaskFile) error
	Get(id uint) (*models.TaskFile, error)
	ListByTask(taskID string) ([]models.TaskFile, error)
	Delete(id uint) error
}

// taskFileStore 任务文件存储实现
type taskFileStore struct {
	db *gorm.DB
}

// NewTaskFileStore 创建任务文件存储
func NewTaskFileStore(db *gorm.DB) TaskFileStore {
	return &taskFileStore{db: db}
}

// Create 创建文件记录
func (s *taskFileStore) Create(file *models.TaskFile) error {
	return s.db.Create(file).Error
}

// Get 获取文件记录
func (s *taskFileStore) Get(id uint) (*models.TaskFile, error) {
	var file models.TaskFile
	err := s.db.First(&file, id).Error
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// ListByTask 获取任务的所有文件
func (s *taskFileStore) ListByTask(taskID string) ([]models.TaskFile, error) {
	var files []models.TaskFile
	err := s.db.Where("task_id = ?", taskID).Order("created_at DESC").Find(&files).Error
	return files, err
}

// Delete 删除文件记录
func (s *taskFileStore) Delete(id uint) error {
	return s.db.Delete(&models.TaskFile{}, id).Error
}