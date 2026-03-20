package store

import (
	"errors"
	"fmt"

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
		Logger: logger.Default.LogMode(logger.Info),
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