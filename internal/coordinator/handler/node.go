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
	Name         string                  `json:"name"`
	Type         models.NodeType         `json:"type"`
	Capabilities models.NodeCapabilities `json:"capabilities"`
	Endpoint     string                  `json:"endpoint"`
}

// Register 注册节点
func (h *NodeHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	n, err := h.registry.Register(c.Request.Context(), req.Name, req.Type, req.Capabilities, req.Endpoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, n)
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

	n, err := h.registry.Get(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	c.JSON(http.StatusOK, n)
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