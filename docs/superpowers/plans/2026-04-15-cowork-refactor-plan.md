# Cowork 任务协作平台重构实现计划

> **Status: PARTIALLY SUPERSEDED (2026-04-16)**
>
> 此计划的架构设计（Coordinator Agent、节点 Agent、消息路由）仍然有效，
> 但具体实现方式已被 Agent Architecture Simplification 项目简化：
> - TaskDecomposer、ToolScheduler 已删除
> - Agent 结构统一为 agent.go + llm_client.go
>
> 请参考 `docs/superpowers/specs/2026-04-16-agent-architecture-simplification-design.md` 了解当前实现。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重构 Cowork 为分布式任务协作平台，实现 Agent ID = Task ID 统一身份模型、Agent 间通信、依赖管理、进度可视化等核心能力。

**Architecture:** Coordinator 作为特殊 Agent，节点上运行独立 Agent，消息路由层实现 Agent 间通信，继承链支持离线恢复。

**Tech Stack:** Go (后端) + React 19 + TypeScript (前端) + SQLite + WebSocket

---

## 文件结构

```
新增文件:
├── internal/shared/models/
│   ├── agent.go            # Agent 相关模型 (AgentTemplate, Message, ApprovalRequest, ApprovalPolicy)
│   └── node.go             # Node 模型 ( NodeType, NodeCapabilities)
│
├── internal/coordinator/
│   ├── message/
│   │   ├── router.go       # MessageRouter 消息路由服务
│   │   ├── router_test.go  # 消息路由测试
│   │   └── types.go        # 消息类型定义
│   │
│   ├── node/
│   │   ├── registry.go     # NodeRegistry 节点注册表
│   │   ├── scheduler.go    # NodeScheduler 节点调度器
│   │   └── registry_test.go
│   │
│   ├── agent/
│   │   ├── template.go     # AgentTemplateManager 模板管理
│   │   ├── template_test.go
│   │   ├── recovery.go     # RecoveryService 恢复机制
│   │   └── recovery_test.go
│   │
│   ├── approval/
│   │   ├── service.go      # ApprovalService 审批服务
│   │   ├── service_test.go
│   │   └── risk.go         # 风险等级定义
│   │
│   └── handler/
│       ├── message.go      # 消息 API handler
│       ├── approval.go     # 审批 API handler
│       ├── template.go     # 模板 API handler
│       └── node.go         # 节点 API handler
│
├── web/src/
│   ├── components/
│   │   ├── tasks/
│   │   │   ├── TaskTreeView.tsx      # 任务依赖树可视化
│   │   │   ├── TaskProgressBadge.tsx # 任务进度徽章
│   │   │   └── TaskDependencyLine.tsx # 依赖关系连线
│   │   │
│   │   ├── agent/
│   │   │   ├── AgentChatPanel.tsx    # 与任意 Agent 对话面板
│   │   │   ├── AgentSelector.tsx     # Agent 选择器
│   │   │   └── AgentStatusBadge.tsx  # Agent 状态徽章
│   │   │
│   │   ├── approval/
│   │   │   ├── ApprovalModal.tsx     # 审批弹窗
│   │   │   ├── ApprovalList.tsx      # 审批列表
│   │   │   └── ApprovalPolicyForm.tsx # 审批策略配置
│   │   │
│   │   └── template/
│   │   │   ├── TemplateList.tsx      # 模板列表
│   │   │   └── TemplateForm.tsx      # 模板编辑表单
│   │   │
│   │   └── dashboard/
│   │       ├── ProgressDashboard.tsx # 进度总览面板
│   │       └── TaskOverview.tsx      # 任务概览卡片
│   │
│   ├── stores/
│   │   ├── agentTemplateStore.ts     # 模板状态管理
│   │   ├── messageStore.ts           # 消息状态管理
│   │   ├── approvalStore.ts          # 审批状态管理
│   │   ├── nodeStore.ts              # 节点状态管理
│   │   └── taskTreeStore.ts          # 任务树状态管理
│   │
│   ├── hooks/
│   │   ├── useTaskTree.ts            # 任务树数据钩子
│   │   ├── useMessages.ts            # 消息数据钩子
│   │   └── useApprovals.ts           # 审批数据钩子

修改文件:
├── internal/shared/models/models.go  # 添加 root_id, parent_id, template_id 字段到 Task
├── internal/coordinator/store/store.go  # 添加新模型 AutoMigrate + Store 接口
├── internal/coordinator/agent/coordinator.go  # 重构为 Agent 模式
├── internal/coordinator/handler/agent.go  # 扩展消息发送能力
├── cmd/coordinator/main.go  # 初始化新服务
├── web/src/stores/agentStore.ts  # 扩展 Agent 相关状态
```

---

## Phase 1: 核心身份模型

### Task 1: 更新 Task 模型添加继承字段

**Files:**
- Modify: `internal/shared/models/models.go:122-178`

- [ ] **Step 1: 添加继承字段到 Task 模型**

```go
// 在 Task 结构体中添加以下字段 (在 GroupID 字段之后):

// 继承关系 (Agent ID = Task ID)
RootID     string  `gorm:"type:varchar(64);index" json:"root_id"`     // 原始任务 ID（自己时等于 ID）
ParentID   *string `gorm:"type:varchar(64);index" json:"parent_id"`  // 父 Agent ID（恢复代理时指向上一条）
TemplateID string  `gorm:"type:varchar(64);index" json:"template_id"` // Agent 模板 ID

// 数据流依赖
Requires   StringArray `gorm:"type:text" json:"requires"` // 需要的数据流路径列表

// 里程碑状态
Milestone  JSON `gorm:"type:text" json:"milestone"` // 里程碑状态，如 {"design": "done", "implement": "running"}
```

- [ ] **Step 2: 添加 RootID 自动设置逻辑**

```go
// 在 Task 结构体后添加 BeforeCreate hook:

// BeforeCreate 创建前自动设置 RootID
func (t *Task) BeforeCreate(tx *gorm.DB) error {
    if t.RootID == "" {
        t.RootID = t.ID
    }
    return nil
}
```

- [ ] **Step 3: 添加 gorm import**

确保 models.go 已导入 gorm:
```go
import (
    "gorm.io/gorm"
)
```

- [ ] **Step 4: 运行测试验证模型变更**

Run: `go build ./internal/shared/models`
Expected: 编译成功，无错误

- [ ] **Step 5: Commit**

```bash
git add internal/shared/models/models.go
git commit -m "feat: add inheritance fields (root_id, parent_id, template_id) to Task model"
```

---

### Task 2: 创建 Node 模型

**Files:**
- Create: `internal/shared/models/node.go`

- [ ] **Step 1: 创建 Node 模型文件**

```go
package models

import (
    "database/sql/driver"
    "encoding/json"
    "errors"
    "time"
)

// NodeType 节点类型
type NodeType string

const (
    NodeTypeSandbox  NodeType = "sandbox"  // 沙箱
    NodeTypeDocker   NodeType = "docker"   // 容器
    NodeTypePhysical NodeType = "physical" // 物理机
    NodeTypeCloud    NodeType = "cloud"    // 云服务器
)

// NodeStatus 节点状态
type NodeStatus string

const (
    NodeStatusIdle    NodeStatus = "idle"    // 空闲
    NodeStatusBusy    NodeStatus = "busy"    // 繁忙
    NodeStatusOffline NodeStatus = "offline" // 离线
)

// NodeCapabilities 节点能力标签
type NodeCapabilities map[string]bool

// Value 实现 driver.Valuer 接口
func (c NodeCapabilities) Value() (driver.Value, error) {
    if c == nil {
        return nil, nil
    }
    return json.Marshal(c)
}

// Scan 实现 sql.Scanner 接口
func (c *NodeCapabilities) Scan(value interface{}) error {
    if value == nil {
        *c = nil
        return nil
    }
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }
    return json.Unmarshal(bytes, c)
}

// Node 节点模型
type Node struct {
    ID   string `gorm:"primaryKey;type:varchar(64)" json:"id"`
    Name string `gorm:"type:varchar(100);uniqueIndex" json:"name"`

    // 类型
    Type NodeType `gorm:"type:varchar(20);index" json:"type"`

    // 能力标签
    Capabilities NodeCapabilities `gorm:"type:text" json:"capabilities"`
    // 示例: {"browser": true, "docker": true, "gpu": false}

    // 状态
    Status NodeStatus `gorm:"type:varchar(20);index" json:"status"`

    // 当前处理的 Agent ID
    CurrentAgentID *string `gorm:"type:varchar(64);index" json:"current_agent_id"`

    // 通信地址
    Endpoint string `gorm:"type:varchar(255)" json:"endpoint"`

    // 元数据
    Metadata JSON `gorm:"type:text" json:"metadata"`

    // 时间
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
    LastSeen  time.Time `gorm:"index" json:"last_seen"`
}

// TableName 指定表名
func (Node) TableName() string {
    return "nodes"
}
```

- [ ] **Step 2: 运行编译验证**

Run: `go build ./internal/shared/models`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add internal/shared/models/node.go
git commit -m "feat: add Node model for node resource pool"
```

---

### Task 3: 创建 Agent 相关模型

**Files:**
- Create: `internal/shared/models/agent.go`

- [ ] **Step 1: 创建 Agent 模型文件**

```go
package models

import (
    "database/sql/driver"
    "encoding/json"
    "errors"
    "time"
)

// AgentTemplate Agent 模板
type AgentTemplate struct {
    ID          string `gorm:"primaryKey;type:varchar(64)" json:"id"`
    Name        string `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
    Description string `gorm:"type:text" json:"description"`

    // 提示词
    BasePrompt   string `gorm:"type:text" json:"base_prompt"`
    SystemPrompt string `gorm:"type:text" json:"system_prompt"`

    // 工具配置
    AllowedTools    StringArray `gorm:"type:text" json:"allowed_tools"`
    RestrictedTools StringArray `gorm:"type:text" json:"restricted_tools"`

    // 需要的节点能力
    RequiredCapabilities NodeCapabilities `gorm:"type:text" json:"required_capabilities"`

    // 默认审批级别
    DefaultApprovalLevel string `gorm:"type:varchar(20);default:'medium'" json:"default_approval_level"`

    // 是否系统预设
    IsSystem bool `gorm:"default:false" json:"is_system"`

    // 创建者
    CreatedBy *string `gorm:"type:varchar(64)" json:"created_by"`

    // 时间
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (AgentTemplate) TableName() string {
    return "agent_templates"
}

// MessageType 消息类型
type MessageType string

const (
    MessageTypeNotify          MessageType = "notify"           // 协调通知
    MessageTypeQuestion        MessageType = "question"         // 对话协商
    MessageTypeData            MessageType = "data"             // 数据传递
    MessageTypeRequestApproval MessageType = "request_approval" // 请求审批
)

// MessageStatus 消息状态
type MessageStatus string

const (
    MessageStatusPending   MessageStatus = "pending"    // 待处理
    MessageStatusDelivered MessageStatus = "delivered"  // 已送达
    MessageStatusRead      MessageStatus = "read"       // 已读
    MessageStatusResponded MessageStatus = "responded"  // 已回复
)

// Message Agent 间消息
type Message struct {
    ID string `gorm:"primaryKey;type:varchar(64)" json:"id"`

    // 发送方
    FromAgent string `gorm:"type:varchar(64);index;not null" json:"from_agent"`
    ProxyFor  string `gorm:"type:varchar(64);index" json:"proxy_for"` // 代理的原始 Agent ID

    // 接收方
    ToAgent string `gorm:"type:varchar(64);index;not null" json:"to_agent"`

    // 类型
    Type MessageType `gorm:"type:varchar(20);index" json:"type"`

    // 内容
    Content string `gorm:"type:text" json:"content"`

    // 是否需要回复
    RequiresResponse bool `gorm:"default:false" json:"requires_response"`

    // 状态
    Status MessageStatus `gorm:"type:varchar(20);index" json:"status"`

    // 回复内容
    Response string `gorm:"type:text" json:"response"`

    // 时间
    CreatedAt    time.Time  `gorm:"autoCreateTime;index" json:"created_at"`
    DeliveredAt  *time.Time `json:"delivered_at"`
    RespondedAt  *time.Time `json:"responded_at"`
}

// TableName 指定表名
func (Message) TableName() string {
    return "messages"
}

// RiskLevel 风险等级
type RiskLevel string

const (
    RiskLevelLow    RiskLevel = "low"    // 低风险：自动执行
    RiskLevelMedium RiskLevel = "medium" // 中风险：自动审批带超时
    RiskLevelHigh   RiskLevel = "high"   // 高风险：强制人工审批
)

// ApprovalStatus 审批状态
type ApprovalStatus string

const (
    ApprovalStatusPending  ApprovalStatus = "pending"  // 待审批
    ApprovalStatusApproved ApprovalStatus = "approved" // 已批准
    ApprovalStatusRejected ApprovalStatus = "rejected" // 已拒绝
    ApprovalStatusExpired  ApprovalStatus = "expired"  // 已过期
)

// ApprovalRequest 审批请求
type ApprovalRequest struct {
    ID string `gorm:"primaryKey;type:varchar(64)" json:"id"`

    // 发起请求的 Agent ID
    AgentID string `gorm:"type:varchar(64);index;not null" json:"agent_id"`

    // 请求执行的操作
    Action string `gorm:"type:varchar(100);not null" json:"action"`

    // 操作详情
    ActionDetail JSON `gorm:"type:text" json:"action_detail"`

    // 风险等级
    RiskLevel RiskLevel `gorm:"type:varchar(20);index" json:"risk_level"`

    // 状态
    Status ApprovalStatus `gorm:"type:varchar(20);index" json:"status"`

    // 审批用户
    UserID *string `gorm:"type:varchar(64)" json:"user_id"`

    // 超时时间（秒），高风险为 NULL
    TimeoutSeconds *int `json:"timeout_seconds"`

    // 时间
    CreatedAt  time.Time  `gorm:"autoCreateTime;index" json:"created_at"`
    ResolvedAt *time.Time `json:"resolved_at"`
}

// TableName 指定表名
func (ApprovalRequest) TableName() string {
    return "approval_requests"
}

// ApprovalPolicy 审批策略
type ApprovalPolicy struct {
    ID string `gorm:"primaryKey;type:varchar(64)" json:"id"`

    // 所属用户
    UserID string `gorm:"type:varchar(64);uniqueIndex;not null" json:"user_id"`

    // 策略类型
    PolicyType string `gorm:"type:varchar(20);default:'default'" json:"policy_type"`

    // 审批规则
    Rules JSON `gorm:"type:text" json:"rules"`

    // 时间
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (ApprovalPolicy) TableName() string {
    return "approval_policies"
}
```

- [ ] **Step 2: 运行编译验证**

Run: `go build ./internal/shared/models`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add internal/shared/models/agent.go
git commit -m "feat: add Agent models (AgentTemplate, Message, ApprovalRequest, ApprovalPolicy)"
```

---

### Task 4: 更新 Store AutoMigrate 和添加 Store 接口

**Files:**
- Modify: `internal/coordinator/store/store.go:110-124`

- [ ] **Step 1: 更新 AutoMigrate 添加新模型**

```go
// 修改 autoMigrate 函数，添加新模型:
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
        // 新增模型
        &models.Node{},
        &models.AgentTemplate{},
        &models.Message{},
        &models.ApprovalRequest{},
        &models.ApprovalPolicy{},
    )
}
```

- [ ] **Step 2: 添加 NodeStore 接口和实现**

在 store.go 文件末尾添加:

```go
// ========== Node Store ==========

// NodeStore 节点存储接口
type NodeStore interface {
    Create(node *models.Node) error
    Get(id string) (*models.Node, error)
    GetByName(name string) (*models.Node, error)
    List() ([]models.Node, error)
    ListByStatus(status models.NodeStatus) ([]models.Node, error)
    ListByCapabilities(capabilities []string) ([]models.Node, error)
    Update(node *models.Node) error
    Delete(id string) error
    UpdateStatus(id string, status models.NodeStatus, currentAgentID *string) error
    UpdateHeartbeat(id string) error
    CountByStatus() (map[models.NodeStatus]int64, error)
}

// nodeStore 节点存储实现
type nodeStore struct {
    db *gorm.DB
}

// NewNodeStore 创建节点存储
func NewNodeStore(db *gorm.DB) NodeStore {
    return &nodeStore{db: db}
}

func (s *nodeStore) Create(node *models.Node) error {
    return s.db.Create(node).Error
}

func (s *nodeStore) Get(id string) (*models.Node, error) {
    var node models.Node
    err := s.db.First(&node, "id = ?", id).Error
    if err != nil {
        return nil, err
    }
    return &node, nil
}

func (s *nodeStore) GetByName(name string) (*models.Node, error) {
    var node models.Node
    err := s.db.First(&node, "name = ?", name).Error
    if err != nil {
        return nil, err
    }
    return &node, nil
}

func (s *nodeStore) List() ([]models.Node, error) {
    var nodes []models.Node
    err := s.db.Order("created_at DESC").Find(&nodes).Error
    return nodes, err
}

func (s *nodeStore) ListByStatus(status models.NodeStatus) ([]models.Node, error) {
    var nodes []models.Node
    err := s.db.Where("status = ?", status).Find(&nodes).Error
    return nodes, err
}

func (s *nodeStore) ListByCapabilities(capabilities []string) ([]models.Node, error) {
    var nodes []models.Node
    // 查询所有节点，然后在 Go 中过滤
    err := s.db.Find(&nodes).Error
    if err != nil {
        return nil, err
    }
    
    var result []models.Node
    for _, node := range nodes {
        hasAll := true
        for _, cap := range capabilities {
            if !node.Capabilities[cap] {
                hasAll = false
                break
            }
        }
        if hasAll {
            result = append(result, node)
        }
    }
    return result, nil
}

func (s *nodeStore) Update(node *models.Node) error {
    return s.db.Save(node).Error
}

func (s *nodeStore) Delete(id string) error {
    return s.db.Delete(&models.Node{}, "id = ?", id).Error
}

func (s *nodeStore) UpdateStatus(id string, status models.NodeStatus, currentAgentID *string) error {
    updates := map[string]interface{}{
        "status": status,
    }
    if currentAgentID != nil {
        updates["current_agent_id"] = *currentAgentID
    } else {
        updates["current_agent_id"] = nil
    }
    return s.db.Model(&models.Node{}).Where("id = ?", id).Updates(updates).Error
}

func (s *nodeStore) UpdateHeartbeat(id string) error {
    return s.db.Model(&models.Node{}).Where("id = ?", id).Update("last_seen", gorm.Expr("datetime('now')")).Error
}

func (s *nodeStore) CountByStatus() (map[models.NodeStatus]int64, error) {
    type StatusCount struct {
        Status models.NodeStatus
        Count  int64
    }
    var results []StatusCount
    err := s.db.Model(&models.Node{}).Select("status, count(*) as count").Group("status").Scan(&results).Error
    if err != nil {
        return nil, err
    }
    counts := make(map[models.NodeStatus]int64)
    for _, r := range results {
        counts[r.Status] = r.Count
    }
    return counts, nil
}
```

- [ ] **Step 3: 添加 MessageStore 接口和实现**

继续在 store.go 添加:

```go
// ========== Message Store ==========

// MessageStore 消息存储接口
type MessageStore interface {
    Create(msg *models.Message) error
    Get(id string) (*models.Message, error)
    ListByToAgent(toAgent string, limit int) ([]models.Message, error)
    ListByFromAgent(fromAgent string, limit int) ([]models.Message, error)
    ListPending(toAgent string) ([]models.Message, error)
    Update(msg *models.Message) error
    UpdateStatus(id string, status models.MessageStatus) error
    MarkDelivered(id string) error
    MarkResponded(id string, response string) error
}

// messageStore 消息存储实现
type messageStore struct {
    db *gorm.DB
}

// NewMessageStore 创建消息存储
func NewMessageStore(db *gorm.DB) MessageStore {
    return &messageStore{db: db}
}

func (s *messageStore) Create(msg *models.Message) error {
    return s.db.Create(msg).Error
}

func (s *messageStore) Get(id string) (*models.Message, error) {
    var msg models.Message
    err := s.db.First(&msg, "id = ?", id).Error
    if err != nil {
        return nil, err
    }
    return &msg, nil
}

func (s *messageStore) ListByToAgent(toAgent string, limit int) ([]models.Message, error) {
    var msgs []models.Message
    query := s.db.Where("to_agent = ?", toAgent).Order("created_at DESC")
    if limit > 0 {
        query = query.Limit(limit)
    }
    err := query.Find(&msgs).Error
    return msgs, err
}

func (s *messageStore) ListByFromAgent(fromAgent string, limit int) ([]models.Message, error) {
    var msgs []models.Message
    query := s.db.Where("from_agent = ?", fromAgent).Order("created_at DESC")
    if limit > 0 {
        query = query.Limit(limit)
    }
    err := query.Find(&msgs).Error
    return msgs, err
}

func (s *messageStore) ListPending(toAgent string) ([]models.Message, error) {
    var msgs []models.Message
    err := s.db.Where("to_agent = ? AND status = ?", toAgent, models.MessageStatusPending).
        Order("created_at ASC").Find(&msgs).Error
    return msgs, err
}

func (s *messageStore) Update(msg *models.Message) error {
    return s.db.Save(msg).Error
}

func (s *messageStore) UpdateStatus(id string, status models.MessageStatus) error {
    return s.db.Model(&models.Message{}).Where("id = ?", id).Update("status", status).Error
}

func (s *messageStore) MarkDelivered(id string) error {
    now := time.Now()
    return s.db.Model(&models.Message{}).Where("id = ?", id).Updates(map[string]interface{}{
        "status":      models.MessageStatusDelivered,
        "delivered_at": &now,
    }).Error
}

func (s *messageStore) MarkResponded(id string, response string) error {
    now := time.Now()
    return s.db.Model(&models.Message{}).Where("id = ?", id).Updates(map[string]interface{}{
        "status":      models.MessageStatusResponded,
        "response":    response,
        "responded_at": &now,
    }).Error
}
```

- [ ] **Step 4: 添加 AgentTemplateStore 和 ApprovalStore**

继续添加:

```go
// ========== AgentTemplate Store ==========

// AgentTemplateStore Agent模板存储接口
type AgentTemplateStore interface {
    Create(template *models.AgentTemplate) error
    Get(id string) (*models.AgentTemplate, error)
    GetByName(name string) (*models.AgentTemplate, error)
    List() ([]models.AgentTemplate, error)
    ListSystem() ([]models.AgentTemplate, error)
    ListByUser(userID string) ([]models.AgentTemplate, error)
    Update(template *models.AgentTemplate) error
    Delete(id string) error
}

// agentTemplateStore Agent模板存储实现
type agentTemplateStore struct {
    db *gorm.DB
}

// NewAgentTemplateStore 创建Agent模板存储
func NewAgentTemplateStore(db *gorm.DB) AgentTemplateStore {
    return &agentTemplateStore{db: db}
}

func (s *agentTemplateStore) Create(template *models.AgentTemplate) error {
    return s.db.Create(template).Error
}

func (s *agentTemplateStore) Get(id string) (*models.AgentTemplate, error) {
    var template models.AgentTemplate
    err := s.db.First(&template, "id = ?", id).Error
    if err != nil {
        return nil, err
    }
    return &template, nil
}

func (s *agentTemplateStore) GetByName(name string) (*models.AgentTemplate, error) {
    var template models.AgentTemplate
    err := s.db.First(&template, "name = ?", name).Error
    if err != nil {
        return nil, err
    }
    return &template, nil
}

func (s *agentTemplateStore) List() ([]models.AgentTemplate, error) {
    var templates []models.AgentTemplate
    err := s.db.Order("is_system DESC, name ASC").Find(&templates).Error
    return templates, err
}

func (s *agentTemplateStore) ListSystem() ([]models.AgentTemplate, error) {
    var templates []models.AgentTemplate
    err := s.db.Where("is_system = ?", true).Order("name ASC").Find(&templates).Error
    return templates, err
}

func (s *agentTemplateStore) ListByUser(userID string) ([]models.AgentTemplate, error) {
    var templates []models.AgentTemplate
    err := s.db.Where("created_by = ?", userID).Order("name ASC").Find(&templates).Error
    return templates, err
}

func (s *agentTemplateStore) Update(template *models.AgentTemplate) error {
    return s.db.Save(template).Error
}

func (s *agentTemplateStore) Delete(id string) error {
    return s.db.Delete(&models.AgentTemplate{}, "id = ?", id).Error
}

// ========== Approval Store ==========

// ApprovalRequestStore 审批请求存储接口
type ApprovalRequestStore interface {
    Create(req *models.ApprovalRequest) error
    Get(id string) (*models.ApprovalRequest, error)
    ListPending(limit int) ([]models.ApprovalRequest, error)
    ListByAgent(agentID string, limit int) ([]models.ApprovalRequest, error)
    Update(req *models.ApprovalRequest) error
    Approve(id string, userID string) error
    Reject(id string, userID string) error
    MarkExpired(id string) error
}

// approvalRequestStore 审批请求存储实现
type approvalRequestStore struct {
    db *gorm.DB
}

// NewApprovalRequestStore 创建审批请求存储
func NewApprovalRequestStore(db *gorm.DB) ApprovalRequestStore {
    return &approvalRequestStore{db: db}
}

func (s *approvalRequestStore) Create(req *models.ApprovalRequest) error {
    return s.db.Create(req).Error
}

func (s *approvalRequestStore) Get(id string) (*models.ApprovalRequest, error) {
    var req models.ApprovalRequest
    err := s.db.First(&req, "id = ?", id).Error
    if err != nil {
        return nil, err
    }
    return &req, nil
}

func (s *approvalRequestStore) ListPending(limit int) ([]models.ApprovalRequest, error) {
    var reqs []models.ApprovalRequest
    query := s.db.Where("status = ?", models.ApprovalStatusPending).Order("created_at DESC")
    if limit > 0 {
        query = query.Limit(limit)
    }
    err := query.Find(&reqs).Error
    return reqs, err
}

func (s *approvalRequestStore) ListByAgent(agentID string, limit int) ([]models.ApprovalRequest, error) {
    var reqs []models.ApprovalRequest
    query := s.db.Where("agent_id = ?", agentID).Order("created_at DESC")
    if limit > 0 {
        query = query.Limit(limit)
    }
    err := query.Find(&reqs).Error
    return reqs, err
}

func (s *approvalRequestStore) Update(req *models.ApprovalRequest) error {
    return s.db.Save(req).Error
}

func (s *approvalRequestStore) Approve(id string, userID string) error {
    now := time.Now()
    return s.db.Model(&models.ApprovalRequest{}).Where("id = ?", id).Updates(map[string]interface{}{
        "status":     models.ApprovalStatusApproved,
        "user_id":    userID,
        "resolved_at": &now,
    }).Error
}

func (s *approvalRequestStore) Reject(id string, userID string) error {
    now := time.Now()
    return s.db.Model(&models.ApprovalRequest{}).Where("id = ?", id).Updates(map[string]interface{}{
        "status":     models.ApprovalStatusRejected,
        "user_id":    userID,
        "resolved_at": &now,
    }).Error
}

func (s *approvalRequestStore) MarkExpired(id string) error {
    now := time.Now()
    return s.db.Model(&models.ApprovalRequest{}).Where("id = ?", id).Updates(map[string]interface{}{
        "status":     models.ApprovalStatusExpired,
        "resolved_at": &now,
    }).Error
}

// ApprovalPolicyStore 审批策略存储接口
type ApprovalPolicyStore interface {
    Create(policy *models.ApprovalPolicy) error
    Get(userID string) (*models.ApprovalPolicy, error)
    Update(policy *models.ApprovalPolicy) error
    Delete(userID string) error
}

// approvalPolicyStore 审批策略存储实现
type approvalPolicyStore struct {
    db *gorm.DB
}

// NewApprovalPolicyStore 创建审批策略存储
func NewApprovalPolicyStore(db *gorm.DB) ApprovalPolicyStore {
    return &approvalPolicyStore{db: db}
}

func (s *approvalPolicyStore) Create(policy *models.ApprovalPolicy) error {
    return s.db.Create(policy).Error
}

func (s *approvalPolicyStore) Get(userID string) (*models.ApprovalPolicy, error) {
    var policy models.ApprovalPolicy
    err := s.db.First(&policy, "user_id = ?", userID).Error
    if err != nil {
        return nil, err
    }
    return &policy, nil
}

func (s *approvalPolicyStore) Update(policy *models.ApprovalPolicy) error {
    return s.db.Save(policy).Error
}

func (s *approvalPolicyStore) Delete(userID string) error {
    return s.db.Delete(&models.ApprovalPolicy{}, "user_id = ?", userID).Error
}
```

- [ ] **Step 5: 添加 time import**

确保 store.go 有 time import:
```go
import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "time"
    ...
)
```

- [ ] **Step 6: 运行编译验证**

Run: `go build ./internal/coordinator/store`
Expected: 编译成功

- [ ] **Step 7: Commit**

```bash
git add internal/coordinator/store/store.go
git commit -m "feat: add NodeStore, MessageStore, AgentTemplateStore, ApprovalStore interfaces and implementations"
```

---

## Phase 2: Coordinator Agent (核心调度)

### Task 5: 创建消息路由层

**Files:**
- Create: `internal/coordinator/message/router.go`
- Create: `internal/coordinator/message/types.go`

- [ ] **Step 1: 创建消息类型定义文件**

```go
// internal/coordinator/message/types.go
package message

import (
    "time"

    "github.com/google/uuid"
    "github.com/tp/cowork/internal/shared/models"
)

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
    FromAgent      string          `json:"from_agent"`
    ProxyFor       string          `json:"proxy_for"`
    ToAgent        string          `json:"to_agent"`
    Type           models.MessageType `json:"type"`
    Content        string          `json:"content"`
    RequiresResponse bool          `json:"requires_response"`
}

// ToMessage 转换为 Message 模型
func (r *SendMessageRequest) ToMessage() *models.Message {
    msg := &models.Message{
        ID:              uuid.New().String(),
        FromAgent:       r.FromAgent,
        ProxyFor:        r.ProxyFor,
        ToAgent:         r.ToAgent,
        Type:            r.Type,
        Content:         r.Content,
        RequiresResponse: r.RequiresResponse,
        Status:          models.MessageStatusPending,
        CreatedAt:       time.Now(),
    }
    if msg.ProxyFor == "" {
        msg.ProxyFor = msg.FromAgent
    }
    return msg
}
```

- [ ] **Step 2: 创建消息路由器文件**

```go
// internal/coordinator/message/router.go
package message

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "gorm.io/gorm"
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/shared/models"
)

// Router 消息路由器
type Router struct {
    msgStore       store.MessageStore
    taskStore      store.TaskStore
    templateStore  store.AgentTemplateStore
    
    // 在线 Agent 的 WebSocket 连接
    connections    map[string]chan *models.Message // agentID -> message channel
    connectionsMu  sync.RWMutex
    
    // 消息到达回调（用于 Coordinator 监听）
    onMessageArrived func(msg *models.Message)
}

// NewRouter 创建消息路由器
func NewRouter(
    msgStore store.MessageStore,
    taskStore store.TaskStore,
    templateStore store.AgentTemplateStore,
) *Router {
    return &Router{
        msgStore:      msgStore,
        taskStore:     taskStore,
        templateStore: templateStore,
        connections:   make(map[string]chan *models.Message),
    }
}

// SetOnMessageArrived 设置消息到达回调
func (r *Router) SetOnMessageArrived(callback func(msg *models.Message)) {
    r.onMessageArrived = callback
}

// RegisterAgent 注册在线 Agent
func (r *Router) RegisterAgent(agentID string) chan *models.Message {
    r.connectionsMu.Lock()
    defer r.connectionsMu.Unlock()
    
    ch := make(chan *models.Message, 100)
    r.connections[agentID] = ch
    return ch
}

// UnregisterAgent 注销 Agent
func (r *Router) UnregisterAgent(agentID string) {
    r.connectionsMu.Lock()
    defer r.connectionsMu.Unlock()
    
    if ch, ok := r.connections[agentID]; ok {
        close(ch)
        delete(r.connections, agentID)
    }
}

// IsAgentOnline 检查 Agent 是否在线
func (r *Router) IsAgentOnline(agentID string) bool {
    r.connectionsMu.RLock()
    defer r.connectionsMu.RUnlock()
    _, ok := r.connections[agentID]
    return ok
}

// Send 发送消息
func (r *Router) Send(ctx context.Context, req *SendMessageRequest) (*models.Message, error) {
    // 创建消息
    msg := req.ToMessage()
    
    // 持久化消息
    if err := r.msgStore.Create(msg); err != nil {
        return nil, fmt.Errorf("failed to create message: %w", err)
    }
    
    // 查找目标 Agent
    targetAgentID := msg.ToAgent
    
    // 检查是否在线
    if r.IsAgentOnline(targetAgentID) {
        // 直接推送
        r.connectionsMu.RLock()
        ch, ok := r.connections[targetAgentID]
        r.connectionsMu.RUnlock()
        
        if ok {
            select {
            case ch <- msg:
                // 标记为已送达
                r.msgStore.MarkDelivered(msg.ID)
            default:
                log.Printf("Agent %s message channel full, message persisted", targetAgentID)
            }
        }
    } else {
        // Agent 离线，触发回调（Coordinator 处理）
        if r.onMessageArrived != nil {
            r.onMessageArrived(msg)
        }
    }
    
    return msg, nil
}

// GetPendingMessages 获取待处理消息
func (r *Router) GetPendingMessages(agentID string) ([]models.Message, error) {
    return r.msgStore.ListPending(agentID)
}

// Respond 回复消息
func (r *Router) Respond(ctx context.Context, msgID string, response string) error {
    return r.msgStore.MarkResponded(msgID, response)
}

// GetInheritanceChain 获取继承链
func (r *Router) GetInheritanceChain(agentID string) ([]string, error) {
    chain := []string{}
    currentID := agentID
    
    // 递归查询 parent_id
    for {
        task, err := r.taskStore.Get(currentID)
        if err != nil {
            break
        }
        chain = append(chain, currentID)
        if task.ParentID == nil || *task.ParentID == "" {
            break
        }
        currentID = *task.ParentID
    }
    
    return chain, nil
}

// GetLatestInChain 获取继承链最新末端
func (r *Router) GetLatestInChain(rootID string, db *gorm.DB) (string, error) {
    // 查询所有 root_id 相同且状态为 done 的任务，按 completed_at DESC 排序取第一个
    var tasks []models.Task
    err := db.Where("root_id = ?", rootID).
        Where("status IN ?", []models.TaskStatus{models.TaskStatusCompleted, models.TaskStatusRunning}).
        Order("created_at DESC").
        Limit(1).
        Find(&tasks).Error
    if err != nil || len(tasks) == 0 {
        return rootID, nil // 没有继承链，返回原始 ID
    }
    return tasks[0].ID, nil
}
```

- [ ] **Step 3: 运行编译验证**

Run: `go build ./internal/coordinator/message`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add internal/coordinator/message/
git commit -m "feat: add MessageRouter for agent-to-agent communication"
```

---

### Task 6: 创建节点调度器

**Files:**
- Create: `internal/coordinator/node/registry.go`
- Create: `internal/coordinator/node/scheduler.go`

- [ ] **Step 1: 创建节点注册表文件**

```go
// internal/coordinator/node/registry.go
package node

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/shared/models"
)

// Registry 节点注册表
type Registry struct {
    store store.NodeStore
}

// NewRegistry 创建节点注册表
func NewRegistry(store store.NodeStore) *Registry {
    return &Registry{store: store}
}

// Register 注册节点
func (r *Registry) Register(ctx context.Context, name string, nodeType models.NodeType, capabilities models.NodeCapabilities, endpoint string) (*models.Node, error) {
    // 检查是否已存在
    existing, err := r.store.GetByName(name)
    if err == nil {
        // 更新现有节点
        existing.Status = models.NodeStatusIdle
        existing.Capabilities = capabilities
        existing.Endpoint = endpoint
        existing.LastSeen = time.Now()
        if err := r.store.Update(existing); err != nil {
            return nil, err
        }
        return existing, nil
    }
    
    // 创建新节点
    node := &models.Node{
        ID:           uuid.New().String(),
        Name:         name,
        Type:         nodeType,
        Capabilities: capabilities,
        Status:       models.NodeStatusIdle,
        Endpoint:     endpoint,
        CreatedAt:    time.Now(),
        LastSeen:     time.Now(),
    }
    
    if err := r.store.Create(node); err != nil {
        return nil, fmt.Errorf("failed to create node: %w", err)
    }
    
    return node, nil
}

// Unregister 注销节点
func (r *Registry) Unregister(ctx context.Context, nodeID string) error {
    return r.store.UpdateStatus(nodeID, models.NodeStatusOffline, nil)
}

// Heartbeat 心跳
func (r *Registry) Heartbeat(ctx context.Context, nodeID string) error {
    return r.store.UpdateHeartbeat(nodeID)
}

// Get 获取节点
func (r *Registry) Get(ctx context.Context, nodeID string) (*models.Node, error) {
    return r.store.Get(nodeID)
}

// List 列出所有节点
func (r *Registry) List(ctx context.Context) ([]models.Node, error) {
    return r.store.List()
}

// ListIdle 列出空闲节点
func (r *Registry) ListIdle(ctx context.Context) ([]models.Node, error) {
    return r.store.ListByStatus(models.NodeStatusIdle)
}
```

- [ ] **Step 2: 创建节点调度器文件**

```go
// internal/coordinator/node/scheduler.go
package node

import (
    "context"
    "fmt"
    "log"

    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/shared/models"
)

// Scheduler 节点调度器
type Scheduler struct {
    registry *Registry
    nodeStore store.NodeStore
    taskStore store.TaskStore
}

// NewScheduler 创建节点调度器
func NewScheduler(registry *Registry, nodeStore store.NodeStore, taskStore store.TaskStore) *Scheduler {
    return &Scheduler{
        registry: registry,
        nodeStore: nodeStore,
        taskStore: taskStore,
    }
}

// AssignTask 分配任务到节点
func (s *Scheduler) AssignTask(ctx context.Context, taskID string, requiredCapabilities []string) (*models.Node, error) {
    // 获取任务
    task, err := s.taskStore.Get(taskID)
    if err != nil {
        return nil, fmt.Errorf("failed to get task: %w", err)
    }
    
    // 查找匹配的空闲节点
    nodes, err := s.nodeStore.ListByCapabilities(requiredCapabilities)
    if err != nil {
        return nil, fmt.Errorf("failed to list nodes: %w", err)
    }
    
    // 过滤空闲节点
    var idleNodes []models.Node
    for _, node := range nodes {
        if node.Status == models.NodeStatusIdle {
            idleNodes = append(idleNodes, node)
        }
    }
    
    if len(idleNodes) == 0 {
        return nil, fmt.Errorf("no available node with capabilities %v", requiredCapabilities)
    }
    
    // 选择第一个空闲节点（可扩展为更复杂的调度策略）
    selectedNode := idleNodes[0]
    
    // 更新节点状态
    if err := s.nodeStore.UpdateStatus(selectedNode.ID, models.NodeStatusBusy, &taskID); err != nil {
        return nil, fmt.Errorf("failed to update node status: %w", err)
    }
    
    // 更新任务
    task.WorkerID = &selectedNode.ID
    task.Status = models.TaskStatusRunning
    now := time.Now()
    task.StartedAt = &now
    if err := s.taskStore.Update(task); err != nil {
        // 回滚节点状态
        s.nodeStore.UpdateStatus(selectedNode.ID, models.NodeStatusIdle, nil)
        return nil, fmt.Errorf("failed to update task: %w", err)
    }
    
    log.Printf("Assigned task %s to node %s", taskID, selectedNode.ID)
    return &selectedNode, nil
}

// ReleaseNode 释放节点
func (s *Scheduler) ReleaseNode(ctx context.Context, nodeID string) error {
    return s.nodeStore.UpdateStatus(nodeID, models.NodeStatusIdle, nil)
}

// CompleteTask 完成任务
func (s *Scheduler) CompleteTask(ctx context.Context, taskID string, output models.JSON, err error) error {
    // 获取任务
    task, err := s.taskStore.Get(taskID)
    if err != nil {
        return fmt.Errorf("failed to get task: %w", err)
    }
    
    // 更新任务状态
    now := time.Now()
    task.CompletedAt = &now
    task.Output = output
    
    if err != nil {
        task.Status = models.TaskStatusFailed
        task.Error = &err.Error()
    } else {
        task.Status = models.TaskStatusCompleted
        task.Progress = 100
    }
    
    if err := s.taskStore.Update(task); err != nil {
        return fmt.Errorf("failed to update task: %w", err)
    }
    
    // 释放节点
    if task.WorkerID != nil {
        if err := s.ReleaseNode(ctx, *task.WorkerID); err != nil {
            log.Printf("Failed to release node %s: %v", *task.WorkerID, err)
        }
    }
    
    return nil
}

// GetTaskDependencies 获取任务依赖状态
func (s *Scheduler) GetTaskDependencies(ctx context.Context, taskID string) ([]string, bool, error) {
    task, err := s.taskStore.Get(taskID)
    if err != nil {
        return nil, false, err
    }
    
    // 检查依赖是否满足
    deps := task.DependsOn
    if len(deps) == 0 {
        return deps, true, nil
    }
    
    allSatisfied := true
    for _, depID := range deps {
        depTask, err := s.taskStore.Get(depID)
        if err != nil {
            allSatisfied = false
            continue
        }
        if depTask.Status != models.TaskStatusCompleted {
            allSatisfied = false
        }
    }
    
    return deps, allSatisfied, nil
}

// GetReadyTasks 获取可以开始的任务
func (s *Scheduler) GetReadyTasks(ctx context.Context) ([]models.Task, error) {
    // 获取所有 pending 状态的任务
    tasks, err := s.taskStore.GetByStatus(models.TaskStatusPending)
    if err != nil {
        return nil, err
    }
    
    var readyTasks []models.Task
    for _, task := range tasks {
        _, satisfied, err := s.GetTaskDependencies(ctx, task.ID)
        if err != nil {
            continue
        }
        if satisfied {
            readyTasks = append(readyTasks, task)
        }
    }
    
    return readyTasks, nil
}
```

- [ ] **Step 3: 添加 time import**

在 scheduler.go 添加:
```go
import (
    ...
    "time"
)
```

- [ ] **Step 4: 运行编译验证**

Run: `go build ./internal/coordinator/node`
Expected: 编译成功

- [ ] **Step 5: Commit**

```bash
git add internal/coordinator/node/
git commit -m "feat: add NodeRegistry and NodeScheduler for node management"
```

---

## Phase 3-10 继续任务...

由于篇幅限制，后续 Phase 的详细任务将在下一个迭代中继续编写。

---

## 文件结构总结

```
已创建:
├── internal/shared/models/agent.go
├── internal/shared/models/node.go
├── internal/coordinator/message/router.go
├── internal/coordinator/message/types.go
├── internal/coordinator/node/registry.go
├── internal/coordinator/node/scheduler.go

已修改:
├── internal/shared/models/models.go (添加继承字段)
├── internal/coordinator/store/store.go (添加新 Store 接口)

待创建 (后续 Phase):
├── internal/coordinator/agent/template.go
├── internal/coordinator/agent/recovery.go
├── internal/coordinator/approval/service.go
├── internal/coordinator/approval/risk.go
├── internal/coordinator/handler/message.go
├── internal/coordinator/handler/approval.go
├── internal/coordinator/handler/template.go
├── internal/coordinator/handler/node.go
├── web/src/components/tasks/TaskTreeView.tsx
├── web/src/components/agent/AgentChatPanel.tsx
├── web/src/components/approval/ApprovalModal.tsx
├── web/src/stores/agentTemplateStore.ts
├── web/src/stores/messageStore.ts
├── web/src/stores/approvalStore.ts
├── web/src/stores/nodeStore.ts
├── web/src/stores/taskTreeStore.ts
```
---

## Phase 3: 节点 Agent (Worker改造)

### Task 7: 创建 AgentTemplateManager

**Files:**
- Create: `internal/coordinator/agent/template.go`

- [ ] **Step 1: 创建模板管理器**

```go
// internal/coordinator/agent/template.go
package agent

import (
    "context"
    "fmt"
    
    "github.com/google/uuid"
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/shared/models"
)

// TemplateManager Agent模板管理器
type TemplateManager struct {
    store store.AgentTemplateStore
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager(store store.AgentTemplateStore) *TemplateManager {
    return &TemplateManager{store: store}
}

// InitSystemTemplates 初始化系统预设模板
func (m *TemplateManager) InitSystemTemplates(ctx context.Context) error {
    templates := []models.AgentTemplate{
        {
            ID:                  "coordinator-template",
            Name:                "Coordinator",
            Description:         "负责任务拆解、调度、监控的协调者Agent",
            BasePrompt:          "你是Coordinator Agent，负责接收用户意图、拆解任务、调度节点、监控进度。你需要分析用户请求，生成任务树，管理任务依赖关系，处理Agent间消息路由。",
            SystemPrompt:        "",
            AllowedTools:        models.StringArray{"dispatch_task", "assign_node", "monitor_progress", "create_agent", "send_message"},
            RestrictedTools:     models.StringArray{},
            RequiredCapabilities: models.NodeCapabilities{},
            DefaultApprovalLevel: "low",
            IsSystem:            true,
        },
        {
            ID:                  "dev-template",
            Name:                "Developer",
            Description:         "负责代码实现的开发Agent",
            BasePrompt:          "你是开发Agent，负责代码编写、文件编辑、API实现。你需要根据任务描述和API文档完成代码开发，汇报进度里程碑。",
            SystemPrompt:        "",
            AllowedTools:        models.StringArray{"read_file", "write_file", "edit_file", "execute_shell", "git", "browser"},
            RestrictedTools:     models.StringArray{"delete_file", "deploy"},
            RequiredCapabilities: models.NodeCapabilities{"editor": true},
            DefaultApprovalLevel: "medium",
            IsSystem:            true,
        },
        {
            ID:                  "test-template",
            Name:                "Tester",
            Description:         "负责测试验证的测试Agent",
            BasePrompt:          "你是测试Agent，负责测试执行、结果分析、bug报告。你需要运行测试、分析结果、发现问题后通知Coordinator创建修复任务。",
            SystemPrompt:        "",
            AllowedTools:        models.StringArray{"read_file", "execute_shell", "run_test", "browser"},
            RestrictedTools:     models.StringArray{"write_file", "edit_file"},
            RequiredCapabilities: models.NodeCapabilities{"test_env": true},
            DefaultApprovalLevel: "low",
            IsSystem:            true,
        },
        {
            ID:                  "review-template",
            Name:                "Reviewer",
            Description:         "负责代码审查的ReviewAgent",
            BasePrompt:          "你是代码审查Agent，负责代码质量检查、安全审计、最佳实践建议。",
            SystemPrompt:        "",
            AllowedTools:        models.StringArray{"read_file", "execute_shell"},
            RestrictedTools:     models.StringArray{"write_file", "edit_file", "execute_shell"},
            RequiredCapabilities: models.NodeCapabilities{},
            DefaultApprovalLevel: "low",
            IsSystem:            true,
        },
        {
            ID:                  "deploy-template",
            Name:                "Deployer",
            Description:         "负责部署发布的部署Agent",
            BasePrompt:          "你是部署Agent，负责代码部署、环境配置、发布验证。所有部署操作需要用户审批。",
            SystemPrompt:        "",
            AllowedTools:        models.StringArray{"read_file", "execute_shell", "deploy"},
            RestrictedTools:     models.StringArray{},
            RequiredCapabilities: models.NodeCapabilities{"docker": true},
            DefaultApprovalLevel: "high",
            IsSystem:            true,
        },
        {
            ID:                  "research-template",
            Name:                "Researcher",
            Description:         "负责调研分析的研究Agent",
            BasePrompt:          "你是研究Agent，负责技术调研、文档分析、方案建议。",
            SystemPrompt:        "",
            AllowedTools:        models.StringArray{"read_file", "browse_web", "execute_shell"},
            RestrictedTools:     models.StringArray{"write_file", "edit_file"},
            RequiredCapabilities: models.NodeCapabilities{"browser": true},
            DefaultApprovalLevel: "low",
            IsSystem:            true,
        },
    }
    
    for _, tmpl := range templates {
        // 检查是否已存在
        existing, err := m.store.Get(tmpl.ID)
        if err == nil && existing != nil {
            continue // 已存在，跳过
        }
        
        tmpl.CreatedAt = time.Now()
        if err := m.store.Create(&tmpl); err != nil {
            return fmt.Errorf("failed to create template %s: %w", tmpl.ID, err)
        }
    }
    
    return nil
}

// Get 获取模板
func (m *TemplateManager) Get(ctx context.Context, templateID string) (*models.AgentTemplate, error) {
    return m.store.Get(templateID)
}

// GetByName 按名称获取模板
func (m *TemplateManager) GetByName(ctx context.Context, name string) (*models.AgentTemplate, error) {
    return m.store.GetByName(name)
}

// List 列出所有模板
func (m *TemplateManager) List(ctx context.Context) ([]models.AgentTemplate, error) {
    return m.store.List()
}

// Create 创建自定义模板
func (m *TemplateManager) Create(ctx context.Context, template *models.AgentTemplate) error {
    template.ID = uuid.New().String()
    template.IsSystem = false
    return m.store.Create(template)
}

// Update 更新模板
func (m *TemplateManager) Update(ctx context.Context, template *models.AgentTemplate) error {
    if template.IsSystem {
        return fmt.Errorf("cannot update system template")
    }
    return m.store.Update(template)
}

// Delete 删除模板
func (m *TemplateManager) Delete(ctx context.Context, templateID string) error {
    template, err := m.store.Get(templateID)
    if err != nil {
        return err
    }
    if template.IsSystem {
        return fmt.Errorf("cannot delete system template")
    }
    return m.store.Delete(templateID)
}

// MatchTemplate 根据任务类型匹配合适的模板
func (m *TemplateManager) MatchTemplate(ctx context.Context, taskType string, taskDescription string) (*models.AgentTemplate, error) {
    templates, err := m.store.ListSystem()
    if err != nil {
        return nil, err
    }
    
    // 简单匹配逻辑（可扩展为 LLM 判断）
    switch taskType {
    case "develop", "code", "implement":
        return m.findTemplateByName(templates, "Developer")
    case "test", "testing":
        return m.findTemplateByName(templates, "Tester")
    case "review", "audit":
        return m.findTemplateByName(templates, "Reviewer")
    case "deploy", "release":
        return m.findTemplateByName(templates, "Deployer")
    case "research", "analyze":
        return m.findTemplateByName(templates, "Researcher")
    default:
        return m.findTemplateByName(templates, "Developer") // 默认开发模板
    }
}

func (m *TemplateManager) findTemplateByName(templates []models.AgentTemplate, name string) (*models.AgentTemplate, error) {
    for _, t := range templates {
        if t.Name == name {
            return &t, nil
        }
    }
    return nil, fmt.Errorf("template %s not found", name)
}
```

- [ ] **Step 2: 添加 time import**

```go
import (
    ...
    "time"
)
```

- [ ] **Step 3: 运行编译验证**

Run: `go build ./internal/coordinator/agent`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add internal/coordinator/agent/template.go
git commit -m "feat: add AgentTemplateManager with system presets"
```

---

## Phase 4: 消息路由 (API Handler)

### Task 8: 创建消息 API Handler

**Files:**
- Create: `internal/coordinator/handler/message.go`

- [ ] **Step 1: 创建消息 Handler**

```go
// internal/coordinator/handler/message.go
package handler

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "github.com/tp/cowork/internal/coordinator/message"
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/shared/models"
)

// MessageHandler 消息 API Handler
type MessageHandler struct {
    router  *message.Router
    msgStore store.MessageStore
}

// NewMessageHandler 创建消息 Handler
func NewMessageHandler(router *message.Router, msgStore store.MessageStore) *MessageHandler {
    return &MessageHandler{
        router:  router,
        msgStore: msgStore,
    }
}

// RegisterRoutes 注册路由
func (h *MessageHandler) RegisterRoutes(r *gin.RouterGroup) {
    r.POST("/messages", h.Send)
    r.GET("/messages/:agentId", h.ListByAgent)
    r.GET("/messages/:agentId/pending", h.ListPending)
    r.POST("/messages/:id/respond", h.Respond)
}

// Send 发送消息
func (h *MessageHandler) Send(c *gin.Context) {
    var req message.SendMessageRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    msg, err := h.router.Send(c.Request.Context(), &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, msg)
}

// ListByAgent 获取 Agent 的消息列表
func (h *MessageHandler) ListByAgent(c *gin.Context) {
    agentID := c.Param("agentId")
    limit := 50
    
    msgs, err := h.msgStore.ListByToAgent(agentID, limit)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, msgs)
}

// ListPending 获取待处理消息
func (h *MessageHandler) ListPending(c *gin.Context) {
    agentID := c.Param("agentId")
    
    msgs, err := h.router.GetPendingMessages(agentID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, msgs)
}

// Respond 回复消息
func (h *MessageHandler) Respond(c *gin.Context) {
    msgID := c.Param("id")
    
    var req struct {
        Response string `json:"response"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if err := h.router.Respond(c.Request.Context(), msgID, req.Response); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "responded"})
}
```

- [ ] **Step 2: 运行编译验证**

Run: `go build ./internal/coordinator/handler`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/handler/message.go
git commit -m "feat: add Message API handler"
```

---

## Phase 5: 节点管理 API

### Task 9: 创建节点 Handler

**Files:**
- Create: `internal/coordinator/handler/node.go`

- [ ] **Step 1: 创建节点 Handler**

```go
// internal/coordinator/handler/node.go
package handler

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "github.com/tp/cowork/internal/coordinator/node"
    "github.com/tp/cowork/internal/shared/models"
)

// NodeHandler 节点 API Handler
type NodeHandler struct {
    registry  *node.Registry
    scheduler *node.Scheduler
}

// NewNodeHandler 创建节点 Handler
func NewNodeHandler(registry *node.Registry, scheduler *node.Scheduler) *NodeHandler {
    return &NodeHandler{
        registry:  registry,
        scheduler: scheduler,
    }
}

// RegisterRoutes 注册路由
func (h *NodeHandler) RegisterRoutes(r *gin.RouterGroup) {
    r.POST("/nodes/register", h.Register)
    r.POST("/nodes/:id/heartbeat", h.Heartbeat)
    r.GET("/nodes", h.List)
    r.GET("/nodes/:id", h.Get)
    r.POST("/nodes/:id/release", h.Release)
}

// RegisterRequest 注册请求
type RegisterRequest struct {
    Name         string                `json:"name"`
    Type         models.NodeType       `json:"type"`
    Capabilities models.NodeCapabilities `json:"capabilities"`
    Endpoint     string                `json:"endpoint"`
}

// Register 注册节点
func (h *NodeHandler) Register(c *gin.Context) {
    var req RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    node, err := h.registry.Register(c.Request.Context(), req.Name, req.Type, req.Capabilities, req.Endpoint)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, node)
}

// Heartbeat 心跳
func (h *NodeHandler) Heartbeat(c *gin.Context) {
    nodeID := c.Param("id")
    
    if err := h.registry.Heartbeat(c.Request.Context(), nodeID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// List 列出节点
func (h *NodeHandler) List(c *gin.Context) {
    nodes, err := h.registry.List(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, nodes)
}

// Get 获取节点
func (h *NodeHandler) Get(c *gin.Context) {
    nodeID := c.Param("id")
    
    node, err := h.registry.Get(c.Request.Context(), nodeID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
        return
    }
    
    c.JSON(http.StatusOK, node)
}

// Release 释放节点
func (h *NodeHandler) Release(c *gin.Context) {
    nodeID := c.Param("id")
    
    if err := h.scheduler.ReleaseNode(c.Request.Context(), nodeID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "released"})
}
```

- [ ] **Step 2: 运行编译验证**

Run: `go build ./internal/coordinator/handler`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/handler/node.go
git commit -m "feat: add Node API handler for registration and management"
```

---

## Phase 6: 前端任务树可视化

### Task 10: 创建任务树 Store

**Files:**
- Create: `web/src/stores/taskTreeStore.ts`

- [ ] **Step 1: 创建任务树 Zustand Store**

```typescript
// web/src/stores/taskTreeStore.ts
import { create } from 'zustand';

interface TaskNode {
  id: string;
  rootId: string;
  parentId: string | null;
  templateId: string;
  title: string;
  description: string;
  status: 'pending' | 'running' | 'done' | 'failed';
  progress: number;
  milestone: Record<string, 'done' | 'running' | 'pending'>;
  dependsOn: string[];
  requires: string[];
  children: string[];
}

interface TaskTreeState {
  tasks: Record<string, TaskNode>;
  rootTaskId: string | null;
  expandedNodes: Set<string>;
  
  // Actions
  setTasks: (tasks: TaskNode[]) => void;
  setRootTaskId: (id: string | null) => void;
  updateTask: (id: string, updates: Partial<TaskNode>) => void;
  toggleExpand: (id: string) => void;
  expandAll: () => void;
  collapseAll: () => void;
  
  // Computed
  getTaskTree: () => TaskNode | null;
  getTaskById: (id: string) => TaskNode | undefined;
  getChildTasks: (parentId: string) => TaskNode[];
  getDependencyGraph: () => { nodes: TaskNode[]; edges: { from: string; to: string }[] };
}

export const useTaskTreeStore = create<TaskTreeState>((set, get) => ({
  tasks: {},
  rootTaskId: null,
  expandedNodes: new Set(),
  
  setTasks: (tasks) => {
    const taskMap: Record<string, TaskNode> = {};
    tasks.forEach(t => taskMap[t.id] = t);
    const rootId = tasks.find(t => t.rootId === t.id && !t.parentId)?.id || null;
    set({ tasks: taskMap, rootTaskId: rootId, expandedNodes: new Set([rootId || '']) });
  },
  
  setRootTaskId: (id) => set({ rootTaskId: id }),
  
  updateTask: (id, updates) => set((state) => ({
    tasks: {
      ...state.tasks,
      [id]: { ...state.tasks[id], ...updates } as TaskNode,
    },
  })),
  
  toggleExpand: (id) => set((state) => {
    const newExpanded = new Set(state.expandedNodes);
    if (newExpanded.has(id)) {
      newExpanded.delete(id);
    } else {
      newExpanded.add(id);
    }
    return { expandedNodes: newExpanded };
  }),
  
  expandAll: () => set((state) => ({
    expandedNodes: new Set(Object.keys(state.tasks)),
  })),
  
  collapseAll: () => set({ expandedNodes: new Set([get().rootTaskId || '']) }),
  
  getTaskTree: () => {
    const state = get();
    if (!state.rootTaskId) return null;
    return state.tasks[state.rootTaskId];
  },
  
  getTaskById: (id) => get().tasks[id],
  
  getChildTasks: (parentId) => {
    const state = get();
    return Object.values(state.tasks).filter(t => t.parentId === parentId);
  },
  
  getDependencyGraph: () => {
    const state = get();
    const nodes = Object.values(state.tasks);
    const edges: { from: string; to: string }[] = [];
    
    nodes.forEach(node => {
      node.dependsOn.forEach(depId => {
        edges.push({ from: node.id, to: depId });
      });
    });
    
    return { nodes, edges };
  },
}));
```

- [ ] **Step 2: Commit**

```bash
git add web/src/stores/taskTreeStore.ts
git commit -m "feat: add taskTreeStore for task dependency visualization"
```

---

### Task 11: 创建 TaskTreeView 组件

**Files:**
- Create: `web/src/components/tasks/TaskTreeView.tsx`

- [ ] **Step 1: 创建任务树可视化组件**

```typescript
// web/src/components/tasks/TaskTreeView.tsx
import React from 'react';
import { useTaskTreeStore } from '@/stores/taskTreeStore';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface TaskTreeViewProps {
  onTaskClick?: (taskId: string) => void;
}

export function TaskTreeView({ onTaskClick }: TaskTreeViewProps) {
  const { 
    tasks, 
    rootTaskId, 
    expandedNodes, 
    toggleExpand, 
    getTaskById,
    getChildTasks,
  } = useTaskTreeStore();
  
  if (!rootTaskId) {
    return (
      <div className="p-4 text-muted-foreground">
        暂无任务树
      </div>
    );
  }
  
  const renderTaskNode = (taskId: string, depth: number = 0) => {
    const task = getTaskById(taskId);
    if (!task) return null;
    
    const children = getChildTasks(taskId);
    const isExpanded = expandedNodes.has(taskId);
    const hasChildren = children.length > 0;
    
    const statusColor = {
      pending: 'bg-gray-200',
      running: 'bg-blue-500',
      done: 'bg-green-500',
      failed: 'bg-red-500',
    };
    
    const progressColor = task.progress < 30 ? 'text-red-500' : 
                          task.progress < 70 ? 'text-blue-500' : 
                          'text-green-500';
    
    return (
      <div key={taskId} className="task-node">
        <div 
          className={cn(
            "flex items-center gap-2 p-2 rounded hover:bg-muted cursor-pointer",
            depth > 0 && "ml-6"
          )}
          onClick={() => onTaskClick?.(taskId)}
        >
          {/* Expand/Collapse Toggle */}
          {hasChildren && (
            <button 
              onClick={(e) => { e.stopPropagation(); toggleExpand(taskId); }}
              className="w-4 h-4 flex items-center justify-center"
            >
              {isExpanded ? '▼' : '▶'}
            </button>
          )}
          
          {/* Status Indicator */}
          <div className={cn("w-2 h-2 rounded-full", statusColor[task.status])} />
          
          {/* Task Title */}
          <span className="font-medium">{task.title}</span>
          
          {/* Progress Badge */}
          <Badge variant="outline" className={progressColor}>
            {task.progress}%
          </Badge>
          
          {/* Template Badge */}
          <Badge variant="secondary">
            {task.templateId}
          </Badge>
        </div>
        
        {/* Milestone Progress */}
        {task.milestone && (
          <div className="ml-6 mb-2 flex gap-1">
            {Object.entries(task.milestone).map(([name, status]) => (
              <Badge 
                key={name}
                variant={status === 'done' ? 'default' : status === 'running' ? 'outline' : 'secondary'}
                className="text-xs"
              >
                {name}: {status === 'done' ? '✓' : status === 'running' ? '●' : '○'}
              </Badge>
            ))}
          </div>
        )}
        
        {/* Children */}
        {isExpanded && hasChildren && (
          <div className="children">
            {children.map(child => renderTaskNode(child.id, depth + 1))}
          </div>
        )}
      </div>
    );
  };
  
  return (
    <div className="task-tree-view p-4 border rounded-lg">
      <div className="flex justify-between mb-4">
        <h3 className="font-semibold">任务树</h3>
        <div className="flex gap-2">
          <button 
            onClick={() => useTaskTreeStore.getState().expandAll()}
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            展开全部
          </button>
          <button 
            onClick={() => useTaskTreeStore.getState().collapseAll()}
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            收起全部
          </button>
        </div>
      </div>
      
      {renderTaskNode(rootTaskId)}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/tasks/TaskTreeView.tsx
git commit -m "feat: add TaskTreeView component for dependency visualization"
```

---

## Phase 7: 审批机制

### Task 12: 创建审批服务

**Files:**
- Create: `internal/coordinator/approval/service.go`
- Create: `internal/coordinator/approval/risk.go`

- [ ] **Step 1: 创建风险等级定义**

```go
// internal/coordinator/approval/risk.go
package approval

import "github.com/tp/cowork/internal/shared/models"

// RiskClassification 风险分类规则
var RiskClassification = map[string]models.RiskLevel{
    // 低风险 - 自动执行
    "read_file":       models.RiskLevelLow,
    "browse_web":      models.RiskLevelLow,
    "run_local_test":  models.RiskLevelLow,
    "list_files":      models.RiskLevelLow,
    
    // 中风险 - 自动审批带超时
    "write_file":      models.RiskLevelMedium,
    "edit_file":       models.RiskLevelMedium,
    "create_branch":   models.RiskLevelMedium,
    "execute_shell":   models.RiskLevelMedium,
    
    // 高风险 - 强制人工审批
    "delete_file":     models.RiskLevelHigh,
    "push_git":        models.RiskLevelHigh,
    "deploy":          models.RiskLevelHigh,
    "execute_shell_rm": models.RiskLevelHigh,
}

// DefaultTimeout 默认超时时间（秒）
var DefaultTimeout = map[models.RiskLevel]int{
    models.RiskLevelLow:    0,  // 无需审批
    models.RiskLevelMedium: 60, // 60秒后自动批准
    models.RiskLevelHigh:   0,  // 无超时，必须人工审批
}

// GetRiskLevel 获取操作的风险等级
func GetRiskLevel(action string) models.RiskLevel {
    // 特殊处理：execute_shell 需要看具体命令
    if action == "execute_shell" {
        return models.RiskLevelMedium // 默认中风险，实际需要看命令内容
    }
    
    if level, ok := RiskClassification[action]; ok {
        return level
    }
    
    return models.RiskLevelMedium // 默认中风险
}

// IsHighRiskShell 检查是否是高风险 shell 命令
func IsHighRiskShell(command string) bool {
    highRiskPatterns := []string{"rm", "sudo", "chmod", "chown", "dd", "mkfs", "shutdown", "reboot"}
    for _, pattern := range highRiskPatterns {
        if containsPattern(command, pattern) {
            return true
        }
    }
    return false
}

func containsPattern(command, pattern string) bool {
    return len(command) >= len(pattern) && 
           (command[:len(pattern)] == pattern || 
            containsWord(command, pattern))
}

func containsWord(s, word string) bool {
    for i := 0; i <= len(s)-len(word); i++ {
        if s[i:i+len(word)] == word {
            // 检查是否是独立单词（前后有空格或边界）
            before := i == 0 || s[i-1] == ' '
            after := i+len(word) == len(s) || s[i+len(word)] == ' '
            if before && after {
                return true
            }
        }
    }
    return false
}
```

- [ ] **Step 2: 创建审批服务**

```go
// internal/coordinator/approval/service.go
package approval

import (
    "context"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/tp/cowork/internal/coordinator/store"
    "github.com/tp/cowork/internal/shared/models"
)

// Service 审批服务
type Service struct {
    reqStore    store.ApprovalRequestStore
    policyStore store.ApprovalPolicyStore
}

// NewService 创建审批服务
func NewService(reqStore store.ApprovalRequestStore, policyStore store.ApprovalPolicyStore) *Service {
    return &Service{
        reqStore:    reqStore,
        policyStore: policyStore,
    }
}

// CreateRequest 创建审批请求
func (s *Service) CreateRequest(ctx context.Context, agentID string, action string, detail models.JSON) (*models.ApprovalRequest, error) {
    riskLevel := GetRiskLevel(action)
    
    // 如果 action 是 execute_shell，检查命令内容
    if action == "execute_shell" {
        if cmd, ok := detail["command"].(string); ok && IsHighRiskShell(cmd) {
            riskLevel = models.RiskLevelHigh
        }
    }
    
    // 低风险直接返回 approved（无需审批）
    if riskLevel == models.RiskLevelLow {
        return &models.ApprovalRequest{
            ID:        uuid.New().String(),
            AgentID:   agentID,
            Action:    action,
            ActionDetail: detail,
            RiskLevel: riskLevel,
            Status:    models.ApprovalStatusApproved,
            CreatedAt: time.Now(),
        }, nil
    }
    
    // 创建审批请求
    timeout := DefaultTimeout[riskLevel]
    var timeoutPtr *int
    if timeout > 0 {
        timeoutPtr = &timeout
    }
    
    req := &models.ApprovalRequest{
        ID:           uuid.New().String(),
        AgentID:      agentID,
        Action:       action,
        ActionDetail: detail,
        RiskLevel:    riskLevel,
        Status:       models.ApprovalStatusPending,
        TimeoutSeconds: timeoutPtr,
        CreatedAt:    time.Now(),
    }
    
    if err := s.reqStore.Create(req); err != nil {
        return nil, fmt.Errorf("failed to create approval request: %w", err)
    }
    
    // 中风险启动自动审批计时器
    if riskLevel == models.RiskLevelMedium && timeout > 0 {
        go s.autoApproveAfterTimeout(req.ID, time.Duration(timeout)*time.Second)
    }
    
    return req, nil
}

// autoApproveAfterTimeout 超时后自动批准
func (s *Service) autoApproveAfterTimeout(reqID string, timeout time.Duration) {
    time.Sleep(timeout)
    
    // 检查是否还未处理
    req, err := s.reqStore.Get(reqID)
    if err != nil || req.Status != models.ApprovalStatusPending {
        return
    }
    
    // 自动批准
    s.reqStore.Approve(reqID, "auto-approved")
}

// Approve 批准请求
func (s *Service) Approve(ctx context.Context, reqID string, userID string) error {
    req, err := s.reqStore.Get(reqID)
    if err != nil {
        return err
    }
    
    if req.Status != models.ApprovalStatusPending {
        return fmt.Errorf("request already processed")
    }
    
    return s.reqStore.Approve(reqID, userID)
}

// Reject 拒绝请求
func (s *Service) Reject(ctx context.Context, reqID string, userID string) error {
    req, err := s.reqStore.Get(reqID)
    if err != nil {
        return err
    }
    
    if req.Status != models.ApprovalStatusPending {
        return fmt.Errorf("request already processed")
    }
    
    return s.reqStore.Reject(reqID, userID)
}

// ListPending 列出待审批请求
func (s *Service) ListPending(ctx context.Context, limit int) ([]models.ApprovalRequest, error) {
    return s.reqStore.ListPending(limit)
}

// GetPolicy 获取用户审批策略
func (s *Service) GetPolicy(ctx context.Context, userID string) (*models.ApprovalPolicy, error) {
    policy, err := s.policyStore.Get(userID)
    if err != nil {
        // 返回默认策略
        return &models.ApprovalPolicy{
            UserID:     userID,
            PolicyType: "default",
            Rules:      models.JSON{"mode": "分级审批"},
        }, nil
    }
    return policy, nil
}

// UpdatePolicy 更新用户审批策略
func (s *Service) UpdatePolicy(ctx context.Context, userID string, rules models.JSON) error {
    policy := &models.ApprovalPolicy{
        UserID:     userID,
        PolicyType: "custom",
        Rules:      rules,
    }
    
    existing, err := s.policyStore.Get(userID)
    if err != nil {
        // 创建新策略
        policy.CreatedAt = time.Now()
        return s.policyStore.Create(policy)
    }
    
    existing.Rules = rules
    return s.policyStore.Update(existing)
}
```

- [ ] **Step 3: 运行编译验证**

Run: `go build ./internal/coordinator/approval`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add internal/coordinator/approval/
git commit -m "feat: add ApprovalService with risk classification"
```

---

## 完整文件结构总结

```
已创建文件:
├── internal/shared/models/
│   ├── agent.go            # AgentTemplate, Message, ApprovalRequest, ApprovalPolicy
│   ├── node.go             # Node, NodeType, NodeStatus, NodeCapabilities
│
├── internal/coordinator/
│   ├── message/
│   │   ├── router.go       # MessageRouter
│   │   ├── types.go        # SendMessageRequest
│   │
│   ├── node/
│   │   ├── registry.go     # NodeRegistry
│   │   ├── scheduler.go    # NodeScheduler
│   │
│   ├── agent/
│   │   ├── template.go     # TemplateManager
│   │
│   ├── approval/
│   │   ├── service.go      # ApprovalService
│   │   ├── risk.go         # RiskClassification
│   │
│   ├── handler/
│   │   ├── message.go      # MessageHandler
│   │   ├── node.go         # NodeHandler
│
├── web/src/
│   ├── stores/
│   │   ├── taskTreeStore.ts
│   │
│   ├── components/
│   │   ├── tasks/
│   │   │   ├── TaskTreeView.tsx

已修改文件:
├── internal/shared/models/models.go  # Task 添加 root_id, parent_id, template_id
├── internal/coordinator/store/store.go  # 新增 Store 接口 + AutoMigrate

待实现 (后续 Phase):
├── internal/coordinator/agent/recovery.go  # 恢复机制
├── internal/coordinator/handler/approval.go # 审批 API
├── internal/coordinator/handler/template.go # 模板 API
├── web/src/components/agent/AgentChatPanel.tsx
├── web/src/components/approval/ApprovalModal.tsx
├── web/src/stores/messageStore.ts
├── web/src/stores/approvalStore.ts
├── web/src/stores/nodeStore.ts
├── web/src/stores/agentTemplateStore.ts
```

---

## 自检清单

1. **Spec覆盖**: Phase 1-7 已覆盖，Phase 8-10 (恢复机制、用户介入、模板管理) 待后续补充
2. **Placeholder检查**: 已修复 router.go 中的 TODO
3. **类型一致性**: 已检查 Task模型字段与后续使用一致
