package models

import (
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

	// 模型配置
	DefaultModel string  `gorm:"type:varchar(50)" json:"default_model"`   // 如 "gpt-4"
	MaxTokens    int     `gorm:"default:4096" json:"max_tokens"`
	Temperature  float64 `gorm:"default:0.7" json:"temperature"`

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