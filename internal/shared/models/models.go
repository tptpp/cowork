package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// JSON 类型用于存储 JSON 数据
type JSON map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// StringArray 用于存储字符串数组
type StringArray []string

// Value 实现 driver.Valuer 接口
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

// Scan 实现 sql.Scanner 接口
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, a)
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Priority 优先级
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// Task 表示一个任务
type Task struct {
	// 基本信息
	ID          string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Type        string     `gorm:"type:varchar(50);index" json:"type"`
	Description string     `gorm:"type:text" json:"description"`

	// 状态
	Status      TaskStatus `gorm:"type:varchar(20);index" json:"status"`
	Progress    int        `gorm:"default:0" json:"progress"` // 0-100

	// 优先级
	Priority    Priority   `gorm:"type:varchar(10);default:'medium'" json:"priority"`

	// 分配
	WorkerID    *string    `gorm:"type:varchar(64);index" json:"worker_id"`
	Worker      *Worker    `gorm:"foreignKey:WorkerID" json:"worker,omitempty"`

	// 标签匹配
	RequiredTags   StringArray `gorm:"type:text" json:"required_tags"`
	PreferredModel string      `gorm:"type:varchar(50)" json:"preferred_model"`

	// 输入输出
	Input  JSON    `gorm:"type:text" json:"input"`
	Output JSON    `gorm:"type:text" json:"output"`
	Error  *string `gorm:"type:text" json:"error"`

	// 配置
	Config JSON `gorm:"type:text" json:"config"`

	// 工作目录
	WorkDir string `gorm:"type:varchar(255)" json:"work_dir"`

	// 时间
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// 关联
	Logs  []TaskLog  `gorm:"foreignKey:TaskID" json:"logs,omitempty"`
	Files []TaskFile `gorm:"foreignKey:TaskID" json:"files,omitempty"`
}

// TableName 指定表名
func (Task) TableName() string {
	return "tasks"
}

// TaskLog 任务执行日志
type TaskLog struct {
	ID      uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID  string    `gorm:"type:varchar(64);index;not null" json:"task_id"`
	Task    *Task     `gorm:"foreignKey:TaskID" json:"-"`

	Time    time.Time `gorm:"autoCreateTime;index" json:"time"`
	Level   string    `gorm:"type:varchar(10)" json:"level"` // debug, info, warn, error
	Message string    `gorm:"type:text" json:"message"`

	// 可选的额外数据
	Metadata JSON `gorm:"type:text" json:"metadata"`
}

// TableName 指定表名
func (TaskLog) TableName() string {
	return "task_logs"
}

// TaskFile 任务输出文件
type TaskFile struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID    string    `gorm:"type:varchar(64);index;not null" json:"task_id"`
	Task      *Task     `gorm:"foreignKey:TaskID" json:"-"`

	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Path      string    `gorm:"type:varchar(512)" json:"path"` // 文件系统路径
	Size      int64     `json:"size"`                          // 字节数
	MimeType  string    `gorm:"type:varchar(100)" json:"mime_type"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (TaskFile) TableName() string {
	return "task_files"
}

// WorkerStatus Worker 状态
type WorkerStatus string

const (
	WorkerStatusIdle    WorkerStatus = "idle"
	WorkerStatusBusy    WorkerStatus = "busy"
	WorkerStatusOffline WorkerStatus = "offline"
	WorkerStatusError   WorkerStatus = "error"
)

// Worker 工作节点
type Worker struct {
	// 基本信息
	ID   string `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Name string `gorm:"type:varchar(100);uniqueIndex" json:"name"`

	// 能力标签
	Tags  StringArray `gorm:"type:text" json:"tags"`
	Model string      `gorm:"type:varchar(50)" json:"model"`

	// 状态
	Status WorkerStatus `gorm:"type:varchar(20);index" json:"status"`

	// 并发
	MaxConcurrent int `gorm:"default:1" json:"max_concurrent"`

	// 能力
	Capabilities JSON `gorm:"type:text" json:"capabilities"`
	// 示例: { "docker": true, "gpu": false, "work_dir": "/tmp/cowork" }

	// 元数据
	Metadata JSON `gorm:"type:text" json:"metadata"`
	// 示例: { "hostname": "laptop-01", "os": "linux", "version": "1.0.0" }

	// 统计
	CompletedTasks int `gorm:"default:0" json:"completed_tasks"`
	FailedTasks    int `gorm:"default:0" json:"failed_tasks"`

	// 时间
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	LastSeen  time.Time `gorm:"index" json:"last_seen"`

	// 关联
	CurrentTasks []Task `gorm:"foreignKey:WorkerID" json:"current_tasks,omitempty"`
}

// TableName 指定表名
func (Worker) TableName() string {
	return "workers"
}

// AgentSession Agent 对话会话
type AgentSession struct {
	ID string `gorm:"primaryKey;type:varchar(64)" json:"id"`

	// 模型配置
	Model        string `gorm:"type:varchar(50)" json:"model"`
	SystemPrompt string `gorm:"type:text" json:"system_prompt"`

	// 上下文
	Context JSON `gorm:"type:text" json:"context"`

	// 关联任务
	TaskID *string `gorm:"type:varchar(64);index" json:"task_id"`
	Task   *Task   `gorm:"foreignKey:TaskID" json:"task,omitempty"`

	// 配置
	Config JSON `gorm:"type:text" json:"config"`

	// 时间
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// 消息
	Messages []AgentMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// TableName 指定表名
func (AgentSession) TableName() string {
	return "agent_sessions"
}

// AgentMessage 对话消息
type AgentMessage struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID string `gorm:"type:varchar(64);index;not null" json:"session_id"`
	Session   *AgentSession `gorm:"foreignKey:SessionID" json:"-"`

	Role     string    `gorm:"type:varchar(20);not null" json:"role"` // user, assistant, system
	Content  string    `gorm:"type:text;not null" json:"content"`

	// Token 统计
	Tokens int `json:"tokens"`

	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName 指定表名
func (AgentMessage) TableName() string {
	return "agent_messages"
}

// Notification 系统通知
type Notification struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`

	Type string `gorm:"type:varchar(50);index" json:"type"`
	// task_complete, task_failed, worker_offline, worker_online, error

	Title   string `gorm:"type:varchar(255)" json:"title"`
	Message string `gorm:"type:text" json:"message"`

	// 关联数据
	Data JSON `gorm:"type:text" json:"data"`
	// 示例: { "task_id": "task-101", "worker_id": "worker-1" }

	Read bool `gorm:"default:false;index" json:"read"`

	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName 指定表名
func (Notification) TableName() string {
	return "notifications"
}

// UserLayout 用户 Dashboard 布局
type UserLayout struct {
	ID     uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID string `gorm:"type:varchar(64);uniqueIndex;not null" json:"user_id"`

	// Widget 配置
	Widgets JSON `gorm:"type:text" json:"widgets"`
	// 示例:
	// [
	//   {
	//     "id": "widget-1",
	//     "type": "task-queue",
	//     "layout": { "x": 0, "y": 0, "w": 4, "h": 3 },
	//     "settings": { "max_tasks": 10 }
	//   }
	// ]

	Version int `gorm:"default:1" json:"version"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (UserLayout) TableName() string {
	return "user_layouts"
}