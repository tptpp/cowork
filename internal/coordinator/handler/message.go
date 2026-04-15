package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tp/cowork/internal/coordinator/message"
	"github.com/tp/cowork/internal/coordinator/store"
)

// MessageHandler 消息 API Handler
type MessageHandler struct {
	router   *message.Router
	msgStore store.MessageStore
}

// NewMessageHandler 创建消息 Handler
func NewMessageHandler(router *message.Router, msgStore store.MessageStore) *MessageHandler {
	return &MessageHandler{
		router:   router,
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