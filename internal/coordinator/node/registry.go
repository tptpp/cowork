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
	existing, err := r.store.GetByName(name)
	if err == nil {
		existing.Status = models.NodeStatusIdle
		existing.Capabilities = capabilities
		existing.Endpoint = endpoint
		existing.LastSeen = time.Now()
		if err := r.store.Update(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

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