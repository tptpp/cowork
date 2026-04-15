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