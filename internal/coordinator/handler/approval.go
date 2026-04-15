package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/tp/cowork/internal/coordinator/approval"
	"github.com/tp/cowork/internal/shared/models"
)

// ApprovalHandler 审批处理器
type ApprovalHandler struct {
	service *approval.Service
}

// NewApprovalHandler 创建审批处理器
func NewApprovalHandler(service *approval.Service) *ApprovalHandler {
	return &ApprovalHandler{service: service}
}

// CreateApprovalRequest 创建审批请求
type CreateApprovalRequest struct {
	AgentID string      `json:"agent_id"`
	Action  string      `json:"action"`
	Detail  models.JSON `json:"detail"`
}

// Create 创建审批请求 (Worker 调用)
func (h *ApprovalHandler) Create(c *gin.Context) {
	var req CreateApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	approvalReq, err := h.service.CreateRequest(c.Request.Context(), req.AgentID, req.Action, req.Detail)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, approvalReq)
}

// Get 查询审批状态 (Worker 调用)
func (h *ApprovalHandler) Get(c *gin.Context) {
	id := c.Param("id")

	req, err := h.service.Get(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "approval not found"})
		return
	}

	c.JSON(200, req)
}

// Approve 批准审批 (前端调用)
func (h *ApprovalHandler) Approve(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Approve(c.Request.Context(), id, req.UserID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "approved"})
}

// Reject 拒绝审批 (前端调用)
func (h *ApprovalHandler) Reject(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Reject(c.Request.Context(), id, req.UserID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "rejected"})
}

// ListPending 列出待审批 (前端调用)
func (h *ApprovalHandler) ListPending(c *gin.Context) {
	reqs, err := h.service.ListPending(c.Request.Context(), 50)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, reqs)
}

// RegisterRoutes 注册路由
func (h *ApprovalHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/approvals", h.Create)
	r.GET("/approvals/:id", h.Get)
	r.POST("/approvals/:id/approve", h.Approve)
	r.POST("/approvals/:id/reject", h.Reject)
	r.GET("/approvals/pending", h.ListPending)
}