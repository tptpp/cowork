package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
)

// MessageRouter 消息路由服务
// 负责 Agent 间消息的路由和投递，支持恢复代理机制
type MessageRouter struct {
	msgStore  store.MessageStore
	taskStore store.TaskStore
}

// NewMessageRouter 创建服务
func NewMessageRouter(
	msgStore store.MessageStore,
	taskStore store.TaskStore,
) *MessageRouter {
	return &MessageRouter{
		msgStore:  msgStore,
		taskStore: taskStore,
	}
}

// Route 路由消息
// 保存消息并根据目标 Agent 状态进行投递或创建恢复代理
func (s *MessageRouter) Route(msg *models.Message) error {
	// 生成消息 ID
	msg.ID = uuid.New().String()
	msg.CreatedAt = time.Now()
	msg.Status = models.MessageStatusPending

	// 保存消息
	if err := s.msgStore.Create(msg); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	// 检查目标 Agent 状态 (Agent = Task in this design)
	targetAgent, err := s.taskStore.Get(msg.ToAgent)
	if err != nil {
		// Agent 不存在或已完成，可能需要创建恢复代理
		return s.handleRecovery(msg)
	}

	// Agent 正在运行，直接路由
	if targetAgent.Status == models.TaskStatusRunning {
		return s.deliverToAgent(msg, targetAgent)
	}

	// Agent 已完成，创建恢复代理
	return s.handleRecovery(msg)
}

// handleRecovery 创建恢复代理
// 当目标 Agent 不存在或已完成时，创建恢复代理继续处理消息
func (s *MessageRouter) handleRecovery(msg *models.Message) error {
	// 查询原始 Agent
	originalAgent, err := s.taskStore.Get(msg.ToAgent)
	if err != nil {
		// Agent 不存在，无法创建恢复代理
		return fmt.Errorf("original agent not found: %s", msg.ToAgent)
	}

	// 创建恢复代理 (Agent = Task)
	recoveryAgent := &models.Task{
		ID:         uuid.New().String(),
		RootID:     originalAgent.RootID, // 保持相同的 RootID
		ParentID:   &msg.ToAgent,         // 父 Agent ID
		TemplateID: originalAgent.TemplateID,
		Status:     models.TaskStatusPending,
		Title:      fmt.Sprintf("Recovery for %s", originalAgent.Title),
		Type:       originalAgent.Type,
	}

	if err := s.taskStore.Create(recoveryAgent); err != nil {
		return fmt.Errorf("failed to create recovery agent: %w", err)
	}

	// 更新消息目标为恢复代理
	msg.ProxyFor = msg.ToAgent
	msg.ToAgent = recoveryAgent.ID

	// 更新消息
	if err := s.msgStore.Update(msg); err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	return s.deliverToAgent(msg, recoveryAgent)
}

// deliverToAgent 投递消息给 Agent
// 将消息状态更新为已送达
func (s *MessageRouter) deliverToAgent(msg *models.Message, agent *models.Task) error {
	// 使用 MessageStore 的 MarkDelivered 方法更新消息状态
	return s.msgStore.MarkDelivered(msg.ID)
}

// GetPendingMessages 获取待处理消息
// 返回指定 Agent 的所有待处理消息
func (s *MessageRouter) GetPendingMessages(agentID string) ([]models.Message, error) {
	return s.msgStore.ListPending(agentID)
}

// MarkResponded 标记消息已回复
// 更新消息状态为已回复并记录回复内容
func (s *MessageRouter) MarkResponded(msgID string, response string) error {
	return s.msgStore.MarkResponded(msgID, response)
}