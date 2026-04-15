package approval

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/coordinator/store"
	"github.com/tp/cowork/internal/coordinator/ws"
	"github.com/tp/cowork/internal/shared/models"
)

// Service 审批服务
type Service struct {
	reqStore    store.ApprovalRequestStore
	policyStore store.ApprovalPolicyStore
	hub         *ws.Hub
}

// NewService 创建审批服务
func NewService(reqStore store.ApprovalRequestStore, policyStore store.ApprovalPolicyStore, hub *ws.Hub) *Service {
	return &Service{
		reqStore:    reqStore,
		policyStore: policyStore,
		hub:         hub,
	}
}

// CreateRequest 创建审批请求
func (s *Service) CreateRequest(ctx context.Context, agentID string, action string, detail models.JSON) (*models.ApprovalRequest, error) {
	riskLevel := GetRiskLevel(action)

	if action == "execute_shell" {
		if cmd, ok := detail["command"].(string); ok && IsHighRiskShell(cmd) {
			riskLevel = models.RiskLevelHigh
		}
	}

	if riskLevel == models.RiskLevelLow {
		return &models.ApprovalRequest{
			ID:           uuid.New().String(),
			AgentID:      agentID,
			Action:       action,
			ActionDetail: detail,
			RiskLevel:    riskLevel,
			Status:       models.ApprovalStatusApproved,
			CreatedAt:    time.Now(),
		}, nil
	}

	timeout := DefaultTimeout[riskLevel]
	var timeoutPtr *int
	if timeout > 0 {
		timeoutPtr = &timeout
	}

	req := &models.ApprovalRequest{
		ID:             uuid.New().String(),
		AgentID:        agentID,
		Action:         action,
		ActionDetail:   detail,
		RiskLevel:      riskLevel,
		Status:         models.ApprovalStatusPending,
		TimeoutSeconds: timeoutPtr,
		CreatedAt:      time.Now(),
	}

	if err := s.reqStore.Create(req); err != nil {
		return nil, fmt.Errorf("failed to create approval request: %w", err)
	}

	// Broadcast approval request via WebSocket
	if s.hub != nil {
		s.hub.BroadcastApprovalRequest(req)
	}

	if riskLevel == models.RiskLevelMedium && timeout > 0 {
		go s.autoApproveAfterTimeout(req.ID, time.Duration(timeout)*time.Second)
	}

	return req, nil
}

func (s *Service) autoApproveAfterTimeout(reqID string, timeout time.Duration) {
	time.Sleep(timeout)
	req, err := s.reqStore.Get(reqID)
	if err != nil || req.Status != models.ApprovalStatusPending {
		return
	}
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

// Get 获取审批请求
func (s *Service) Get(reqID string) (*models.ApprovalRequest, error) {
	return s.reqStore.Get(reqID)
}

// ListPending 列出待审批请求
func (s *Service) ListPending(ctx context.Context, limit int) ([]models.ApprovalRequest, error) {
	return s.reqStore.ListPending(limit)
}

// GetPolicy 获取用户审批策略
func (s *Service) GetPolicy(ctx context.Context, userID string) (*models.ApprovalPolicy, error) {
	policy, err := s.policyStore.Get(userID)
	if err != nil {
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
		policy.CreatedAt = time.Now()
		return s.policyStore.Create(policy)
	}

	existing.Rules = rules
	return s.policyStore.Update(existing)
}