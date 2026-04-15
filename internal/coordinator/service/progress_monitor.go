package service

import (
	"encoding/json"

	"github.com/tp/cowork/internal/coordinator/ws"
	"github.com/tp/cowork/internal/shared/models"
)

// ProgressMonitor 进度监控服务
type ProgressMonitor struct {
	hub *ws.Hub
}

// NewProgressMonitor 创建服务
func NewProgressMonitor(hub *ws.Hub) *ProgressMonitor {
	return &ProgressMonitor{
		hub: hub,
	}
}

// OnTaskStatusChange 任务状态变更时推送
func (s *ProgressMonitor) OnTaskStatusChange(task *models.Task) {
	msg := map[string]interface{}{
		"type":     "task_update",
		"task_id":  task.ID,
		"status":   task.Status,
		"title":    task.Title,
		"progress": task.Progress,
	}

	data, _ := json.Marshal(msg)
	s.hub.BroadcastToChannel("tasks", data)
}

// OnAgentMessage Agent 消息时推送
func (s *ProgressMonitor) OnAgentMessage(msg *models.AgentMessage) {
	wsMsg := map[string]interface{}{
		"type":       "agent_message",
		"session_id": msg.SessionID,
		"role":       msg.Role,
		"content":    msg.Content,
	}

	data, _ := json.Marshal(wsMsg)
	s.hub.BroadcastToChannel("messages", data)
}

// OnApprovalRequest 审批请求时推送
func (s *ProgressMonitor) OnApprovalRequest(approval *models.ApprovalRequest) {
	msg := map[string]interface{}{
		"type":       "approval_request",
		"id":         approval.ID,
		"agent_id":   approval.AgentID,
		"action":     approval.Action,
		"risk_level": approval.RiskLevel,
	}

	data, _ := json.Marshal(msg)
	s.hub.BroadcastToChannel("approvals", data)
}