# Cowork 数据模型文档

## 1. 数据库概览

### 1.1 数据库选择

| 环境 | 数据库 | 理由 |
|------|--------|------|
| 开发/单机 | SQLite | 零配置、单文件、易备份 |
| 生产/多机 | PostgreSQL | 高并发、主从复制、扩展性好 |

### 1.2 ORM

使用 **GORM** 作为 ORM，支持：
- 自动迁移
- 模型关联
- 钩子函数
- 多数据库支持

---

## 2. 核心数据模型

### 2.1 任务 (Task)

```go
// Task 表示一个任务
type Task struct {
    // 基本信息
    ID          string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
    Type        string     `gorm:"type:varchar(50);index" json:"type"`
    Description string     `gorm:"type:text" json:"description"`
    
    // 状态
    Status      TaskStatus `gorm:"type:varchar(20);index" json:"status"`
    Progress    int        `gorm:"default:0" json:"progress"`          // 0-100
    
    // 优先级
    Priority    Priority   `gorm:"type:varchar(10);default:'medium'" json:"priority"`
    
    // 分配
    WorkerID    *string    `gorm:"type:varchar(64);index" json:"worker_id"`
    Worker      *Worker    `gorm:"foreignKey:WorkerID" json:"worker,omitempty"`
    
    // 标签匹配
    RequiredTags pq.StringArray `gorm:"type:text[]" json:"required_tags"`
    PreferredModel string       `gorm:"type:varchar(50)" json:"preferred_model"`
    
    // 输入输出
    Input       JSON       `gorm:"type:jsonb" json:"input"`
    Output      JSON       `gorm:"type:jsonb" json:"output"`
    Error       *string    `gorm:"type:text" json:"error"`
    
    // 配置
    Config      JSON       `gorm:"type:jsonb" json:"config"`
    
    // 工作目录
    WorkDir     string     `gorm:"type:varchar(255)" json:"work_dir"`
    
    // 时间
    CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
    StartedAt   *time.Time `json:"started_at"`
    CompletedAt *time.Time `json:"completed_at"`
    
    // 关联
    Logs        []TaskLog  `gorm:"foreignKey:TaskID" json:"logs,omitempty"`
    Files       []TaskFile `gorm:"foreignKey:TaskID" json:"files,omitempty"`
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

// JSON 类型用于存储 JSON 数据
type JSON map[string]interface{}

// 实现 GORM 的 Scanner/Valuer 接口
func (j JSON) Value() (driver.Value, error) {
    return json.Marshal(j)
}

func (j *JSON) Scan(value interface{}) error {
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }
    return json.Unmarshal(bytes, j)
}
```

**SQL (SQLite)**:
```sql
CREATE TABLE tasks (
    id VARCHAR(64) PRIMARY KEY,
    type VARCHAR(50),
    description TEXT,
    status VARCHAR(20),
    progress INTEGER DEFAULT 0,
    priority VARCHAR(10) DEFAULT 'medium',
    worker_id VARCHAR(64),
    required_tags TEXT,        -- JSON array
    preferred_model VARCHAR(50),
    input TEXT,                -- JSON
    output TEXT,               -- JSON
    error TEXT,
    config TEXT,               -- JSON
    work_dir VARCHAR(255),
    created_at DATETIME,
    updated_at DATETIME,
    started_at DATETIME,
    completed_at DATETIME
);

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_type ON tasks(type);
CREATE INDEX idx_tasks_worker_id ON tasks(worker_id);
CREATE INDEX idx_tasks_created_at ON tasks(created_at);
```

### 2.2 任务日志 (TaskLog)

```go
// TaskLog 任务执行日志
type TaskLog struct {
    ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
    TaskID    string    `gorm:"type:varchar(64);index;not null" json:"task_id"`
    Task      *Task     `gorm:"foreignKey:TaskID" json:"-"`
    
    Time      time.Time `gorm:"autoCreateTime;index" json:"time"`
    Level     string    `gorm:"type:varchar(10)" json:"level"`    // debug, info, warn, error
    Message   string    `gorm:"type:text" json:"message"`
    
    // 可选的额外数据
    Metadata  JSON      `gorm:"type:jsonb" json:"metadata"`
}
```

**SQL**:
```sql
CREATE TABLE task_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id VARCHAR(64) NOT NULL,
    time DATETIME,
    level VARCHAR(10),
    message TEXT,
    metadata TEXT,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX idx_task_logs_task_id ON task_logs(task_id);
CREATE INDEX idx_task_logs_time ON task_logs(time);
```

### 2.3 任务文件 (TaskFile)

```go
// TaskFile 任务输出文件
type TaskFile struct {
    ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
    TaskID    string    `gorm:"type:varchar(64);index;not null" json:"task_id"`
    Task      *Task     `gorm:"foreignKey:TaskID" json:"-"`
    
    Name      string    `gorm:"type:varchar(255);not null" json:"name"`
    Path      string    `gorm:"type:varchar(512)" json:"path"`      // 文件系统路径
    Size      int64     `json:"size"`                              // 字节数
    MimeType  string    `gorm:"type:varchar(100)" json:"mime_type"`
    
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
```

### 2.4 Worker

```go
// Worker 工作节点
type Worker struct {
    // 基本信息
    ID        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
    Name      string    `gorm:"type:varchar(100);uniqueIndex" json:"name"`
    
    // 能力标签
    Tags      pq.StringArray `gorm:"type:text[]" json:"tags"`
    Model     string         `gorm:"type:varchar(50)" json:"model"`
    
    // 状态
    Status    WorkerStatus `gorm:"type:varchar(20);index" json:"status"`
    
    // 并发
    MaxConcurrent int       `gorm:"default:1" json:"max_concurrent"`
    
    // 能力
    Capabilities JSON       `gorm:"type:jsonb" json:"capabilities"`
    // 示例: { "docker": true, "gpu": false, "work_dir": "/tmp/cowork" }
    
    // 元数据
    Metadata     JSON       `gorm:"type:jsonb" json:"metadata"`
    // 示例: { "hostname": "laptop-01", "os": "linux", "version": "1.0.0" }
    
    // 统计
    CompletedTasks int      `gorm:"default:0" json:"completed_tasks"`
    FailedTasks    int      `gorm:"default:0" json:"failed_tasks"`
    
    // 时间
    CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
    LastSeen   time.Time  `gorm:"index" json:"last_seen"`
    
    // 关联
    CurrentTasks []Task   `gorm:"foreignKey:WorkerID" json:"current_tasks,omitempty"`
}

// WorkerStatus Worker 状态
type WorkerStatus string

const (
    WorkerStatusIdle    WorkerStatus = "idle"
    WorkerStatusBusy    WorkerStatus = "busy"
    WorkerStatusOffline WorkerStatus = "offline"
    WorkerStatusError   WorkerStatus = "error"
)
```

**SQL**:
```sql
CREATE TABLE workers (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(100) UNIQUE,
    tags TEXT,                  -- JSON array
    model VARCHAR(50),
    status VARCHAR(20),
    max_concurrent INTEGER DEFAULT 1,
    capabilities TEXT,          -- JSON
    metadata TEXT,              -- JSON
    completed_tasks INTEGER DEFAULT 0,
    failed_tasks INTEGER DEFAULT 0,
    created_at DATETIME,
    last_seen DATETIME
);

CREATE INDEX idx_workers_status ON workers(status);
CREATE INDEX idx_workers_last_seen ON workers(last_seen);
```

### 2.5 Agent 会话 (AgentSession)

```go
// AgentSession Agent 对话会话
type AgentSession struct {
    ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
    
    // 模型配置
    Model       string    `gorm:"type:varchar(50)" json:"model"`
    SystemPrompt string   `gorm:"type:text" json:"system_prompt"`
    
    // 上下文
    Context     JSON      `gorm:"type:jsonb" json:"context"`
    
    // 关联任务
    TaskID      *string   `gorm:"type:varchar(64);index" json:"task_id"`
    Task        *Task     `gorm:"foreignKey:TaskID" json:"task,omitempty"`
    
    // 配置
    Config      JSON      `gorm:"type:jsonb" json:"config"`
    
    // 时间
    CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
    
    // 消息
    Messages    []AgentMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}
```

### 2.6 Agent 消息 (AgentMessage)

```go
// AgentMessage 对话消息
type AgentMessage struct {
    ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
    SessionID  string    `gorm:"type:varchar(64);index;not null" json:"session_id"`
    Session    *AgentSession `gorm:"foreignKey:SessionID" json:"-"`
    
    Role       string    `gorm:"type:varchar(20);not null" json:"role"`  // user, assistant, system
    Content    string    `gorm:"type:text;not null" json:"content"`
    
    // Token 统计
    Tokens     int       `json:"tokens"`
    
    CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}
```

**SQL**:
```sql
CREATE TABLE agent_sessions (
    id VARCHAR(64) PRIMARY KEY,
    model VARCHAR(50),
    system_prompt TEXT,
    context TEXT,              -- JSON
    task_id VARCHAR(64),
    config TEXT,               -- JSON
    created_at DATETIME,
    updated_at DATETIME
);

CREATE TABLE agent_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id VARCHAR(64) NOT NULL,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    tokens INTEGER,
    created_at DATETIME,
    FOREIGN KEY (session_id) REFERENCES agent_sessions(id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_messages_session_id ON agent_messages(session_id);
```

### 2.7 通知 (Notification)

```go
// Notification 系统通知
type Notification struct {
    ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
    
    Type      string    `gorm:"type:varchar(50);index" json:"type"`
    // task_complete, task_failed, worker_offline, worker_online, error
    
    Title     string    `gorm:"type:varchar(255)" json:"title"`
    Message   string    `gorm:"type:text" json:"message"`
    
    // 关联数据
    Data      JSON      `gorm:"type:jsonb" json:"data"`
    // 示例: { "task_id": "task-101", "worker_id": "worker-1" }
    
    Read      bool      `gorm:"default:false;index" json:"read"`
    
    CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}
```

**SQL**:
```sql
CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type VARCHAR(50),
    title VARCHAR(255),
    message TEXT,
    data TEXT,                 -- JSON
    read BOOLEAN DEFAULT 0,
    created_at DATETIME
);

CREATE INDEX idx_notifications_type ON notifications(type);
CREATE INDEX idx_notifications_read ON notifications(read);
CREATE INDEX idx_notifications_created_at ON notifications(created_at);
```

### 2.8 用户布局 (UserLayout)

```go
// UserLayout 用户 Dashboard 布局
type UserLayout struct {
    ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID    string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"user_id"`
    
    // Widget 配置
    Widgets   JSON      `gorm:"type:jsonb" json:"widgets"`
    // 示例:
    // [
    //   {
    //     "id": "widget-1",
    //     "type": "task-queue",
    //     "layout": { "x": 0, "y": 0, "w": 4, "h": 3 },
    //     "settings": { "max_tasks": 10 }
    //   }
    // ]
    
    Version   int       `gorm:"default:1" json:"version"`
    
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
```

**SQL**:
```sql
CREATE TABLE user_layouts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id VARCHAR(64) UNIQUE NOT NULL,
    widgets TEXT,              -- JSON array
    version INTEGER DEFAULT 1,
    created_at DATETIME,
    updated_at DATETIME
);
```

---

## 3. 索引设计

### 3.1 主要索引

```sql
-- 任务查询优化
CREATE INDEX idx_tasks_status_created ON tasks(status, created_at DESC);
CREATE INDEX idx_tasks_type_status ON tasks(type, status);

-- Worker 查询优化
CREATE INDEX idx_workers_status_tags ON workers(status);  -- tags 用 JSON 查询

-- 日志查询优化
CREATE INDEX idx_task_logs_task_time ON task_logs(task_id, time DESC);

-- 通知查询优化
CREATE INDEX idx_notifications_read_created ON notifications(read, created_at DESC);
```

### 3.2 全文搜索（可选）

如果需要搜索任务描述：
```sql
-- SQLite FTS5
CREATE VIRTUAL TABLE tasks_fts USING fts5(
    description,
    content='tasks',
    content_rowid='rowid'
);

-- 触发器保持同步
CREATE TRIGGER tasks_ai AFTER INSERT ON tasks BEGIN
    INSERT INTO tasks_fts(rowid, description) 
    VALUES (new.rowid, new.description);
END;
```

---

## 4. 数据迁移

### 4.1 GORM 自动迁移

```go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &Task{},
        &TaskLog{},
        &TaskFile{},
        &Worker{},
        &AgentSession{},
        &AgentMessage{},
        &Notification{},
        &UserLayout{},
    )
}
```

### 4.2 版本迁移

```go
// 使用 golang-migrate 或类似工具
// migrations/001_initial_schema.up.sql
// migrations/001_initial_schema.down.sql

// 或使用 GORM 的 Migrator
type Migration struct {
    ID      string    `gorm:"primaryKey"`
    Applied bool      `gorm:"default:false"`
    AppliedAt time.Time
}
```

---

## 5. 数据关系图

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Data Model ERD                             │
│                                                                      │
│   ┌─────────────┐       ┌─────────────┐       ┌─────────────────┐  │
│   │   Worker    │       │    Task     │       │   TaskLog       │  │
│   │─────────────│       │─────────────│       │─────────────────│  │
│   │ id          │◄──────│ worker_id   │       │ id              │  │
│   │ name        │  1:N  │ id          │───────│ task_id         │  │
│   │ tags        │       │ type        │   1:N │ time            │  │
│   │ status      │       │ status      │       │ level           │  │
│   │ model       │       │ progress    │       │ message         │  │
│   └─────────────┘       │ input       │       └─────────────────┘  │
│                         │ output      │                            │
│                         │ work_dir    │       ┌─────────────────┐  │
│                         └──────┬──────┘       │   TaskFile      │  │
│                                │              │─────────────────│  │
│                                │              │ id              │  │
│                                └──────────────│ task_id         │  │
│                                    1:N        │ name            │  │
│                                               │ path            │  │
│                                               │ size            │  │
│                                               └─────────────────┘  │
│                                                                      │
│   ┌─────────────────┐       ┌─────────────────┐                    │
│   │  AgentSession   │       │  AgentMessage   │                    │
│   │─────────────────│       │─────────────────│                    │
│   │ id              │◄──────│ session_id      │                    │
│   │ model           │  1:N  │ id              │                    │
│   │ system_prompt   │       │ role            │                    │
│   │ context         │       │ content         │                    │
│   │ task_id         │───┐   │ tokens          │                    │
│   └─────────────────┘   │   └─────────────────┘                    │
│           │             │                                          │
│           │             │                                          │
│           └─────────────┼───────────────────Task (可选关联)        │
│                         │                                          │
│   ┌─────────────────┐   │                                          │
│   │  Notification   │   │                                          │
│   │─────────────────│   │                                          │
│   │ id              │   │                                          │
│   │ type            │   │                                          │
│   │ title           │   │                                          │
│   │ data            │───┘ (可选关联 task_id)                       │
│   └─────────────────┘                                              │
│                                                                      │
│   ┌─────────────────┐                                              │
│   │   UserLayout    │                                              │
│   │─────────────────│                                              │
│   │ id              │                                              │
│   │ user_id         │                                              │
│   │ widgets         │ (JSON)                                       │
│   └─────────────────┘                                              │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 6. 缓存策略

### 6.1 缓存数据

| 数据类型 | 缓存策略 | TTL |
|----------|----------|-----|
| Worker 状态 | 内存缓存 | 心跳更新时刷新 |
| 任务列表 | 内存缓存 | 5 秒 |
| 用户布局 | 内存缓存 | 修改时刷新 |
| 系统统计 | 内存缓存 | 10 秒 |

### 6.2 缓存实现

```go
import "github.com/dgraph-io/ristretto"

type Cache struct {
    r *ristretto.Cache
}

func NewCache() (*Cache, error) {
    r, err := ristretto.NewCache(&ristretto.Config{
        NumCounters: 1e7,     // 10M 计数器
        MaxCost:     1 << 30, // 1GB 最大成本
        BufferItems: 64,      // 缓冲区大小
    })
    return &Cache{r: r}, err
}

// 使用示例
func (s *TaskService) GetTask(id string) (*Task, error) {
    // 尝试从缓存获取
    if v, ok := s.cache.Get("task:" + id); ok {
        return v.(*Task), nil
    }
    
    // 从数据库获取
    task, err := s.repo.GetTask(id)
    if err != nil {
        return nil, err
    }
    
    // 写入缓存
    s.cache.Set("task:"+id, task, 1)
    return task, nil
}
```

---

## 7. 数据备份

### 7.1 SQLite 备份

```bash
# 在线备份
sqlite3 cowork.db ".backup 'cowork_backup.db'"

# 定时备份脚本
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
sqlite3 cowork.db ".backup 'backups/cowork_$DATE.db'"
```

### 7.2 PostgreSQL 备份

```bash
# 完整备份
pg_dump cowork > cowork_backup.sql

# 定时备份
pg_dump -Fc cowork > cowork_backup.dump
```