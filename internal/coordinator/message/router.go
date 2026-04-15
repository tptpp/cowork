package message

import (
	"context"
	"fmt"
	"log"
	"sync"

	"gorm.io/gorm"

	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/shared/models"
)

// Router 消息路由器
type Router struct {
	msgStore  store.MessageStore
	taskStore store.TaskStore
	db        *gorm.DB

	// 在线 Agent 的消息通道
	connections   map[string]chan *models.Message
	connectionsMu sync.RWMutex

	// 消息到达回调
	onMessageArrived func(msg *models.Message)
}

// NewRouter 创建消息路由器
func NewRouter(
	msgStore store.MessageStore,
	taskStore store.TaskStore,
	db *gorm.DB,
) *Router {
	return &Router{
		msgStore:    msgStore,
		taskStore:   taskStore,
		db:          db,
		connections: make(map[string]chan *models.Message),
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
	msg := req.ToMessage()
	if err := r.msgStore.Create(msg); err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	targetAgentID := msg.ToAgent

	if r.IsAgentOnline(targetAgentID) {
		r.connectionsMu.RLock()
		ch, ok := r.connections[targetAgentID]
		r.connectionsMu.RUnlock()

		if ok {
			select {
			case ch <- msg:
				r.msgStore.MarkDelivered(msg.ID)
			default:
				log.Printf("Agent %s message channel full", targetAgentID)
			}
		}
	} else {
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
func (r *Router) GetLatestInChain(rootID string) (string, error) {
	var tasks []models.Task
	err := r.db.Where("root_id = ?", rootID).
		Where("status IN ?", []models.TaskStatus{models.TaskStatusCompleted, models.TaskStatusRunning}).
		Order("created_at DESC").
		Limit(1).
		Find(&tasks).Error
	if err != nil || len(tasks) == 0 {
		return rootID, nil
	}
	return tasks[0].ID, nil
}