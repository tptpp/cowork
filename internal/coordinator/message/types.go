package message

import (
	"time"

	"github.com/google/uuid"
	"github.com/tp/cowork/internal/shared/models"
)

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	FromAgent        string             `json:"from_agent"`
	ProxyFor         string             `json:"proxy_for"`
	ToAgent          string             `json:"to_agent"`
	Type             models.MessageType `json:"type"`
	Content          string             `json:"content"`
	RequiresResponse bool               `json:"requires_response"`
}

// ToMessage 转换为 Message 模型
func (r *SendMessageRequest) ToMessage() *models.Message {
	msg := &models.Message{
		ID:               uuid.New().String(),
		FromAgent:        r.FromAgent,
		ProxyFor:         r.ProxyFor,
		ToAgent:          r.ToAgent,
		Type:             r.Type,
		Content:          r.Content,
		RequiresResponse: r.RequiresResponse,
		Status:           models.MessageStatusPending,
		CreatedAt:        time.Now(),
	}
	if msg.ProxyFor == "" {
		msg.ProxyFor = msg.FromAgent
	}
	return msg
}