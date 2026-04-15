package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType 消息类型
type MessageType string

const (
	MessageTypeSubscribe        MessageType = "subscribe"
	MessageTypeUnsubscribe      MessageType = "unsubscribe"
	MessageTypePing             MessageType = "ping"
	MessageTypePong             MessageType = "pong"
	MessageTypeTaskUpdate       MessageType = "task_update"
	MessageTypeTaskLog          MessageType = "task_log"
	MessageTypeWorkerUpdate     MessageType = "worker_update"
	MessageTypeNotification     MessageType = "notification"
	MessageTypeError            MessageType = "error"
	MessageTypeApprovalRequest  MessageType = "approval_request"
)

// Message WebSocket 消息
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Channels []string       `json:"channels,omitempty"`
}

// Client WebSocket 客户端
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	channels map[string]bool
	mu       sync.RWMutex
}

// Hub WebSocket 连接管理中心
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub 创建新的 Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run 运行 Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected, total: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected, total: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 广播消息到所有客户端
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// BroadcastToChannel 广播消息到特定频道
func (h *Hub) BroadcastToChannel(channel string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		client.mu.RLock()
		if client.channels[channel] {
			select {
			case client.send <- message:
			default:
				// 发送失败，忽略
			}
		}
		client.mu.RUnlock()
	}
}

// BroadcastApprovalRequest broadcasts an approval request to clients subscribed to the approvals channel
func (h *Hub) BroadcastApprovalRequest(approval interface{}) {
	msg := Message{
		Type: MessageTypeApprovalRequest,
	}
	payload, _ := json.Marshal(approval)
	msg.Payload = payload
	data, _ := json.Marshal(msg)
	h.BroadcastToChannel("approvals", data)
}

// ClientCount 获取客户端数量
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// NewClient 创建新的客户端
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		channels: make(map[string]bool),
	}
}

// Register 注册客户端
func (c *Client) Register() {
	c.hub.register <- c
}

// Unregister 注销客户端
func (c *Client) Unregister() {
	c.hub.unregister <- c
}

// Subscribe 订阅频道
func (c *Client) Subscribe(channels []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range channels {
		c.channels[ch] = true
	}
	log.Printf("Client subscribed to: %v", channels)
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(channels []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range channels {
		delete(c.channels, ch)
	}
}

// ReadPump 读取消息循环
func (c *Client) ReadPump() {
	defer func() {
		c.Unregister()
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		switch msg.Type {
		case MessageTypeSubscribe:
			var channels []string
			if err := json.Unmarshal(msg.Payload, &channels); err == nil {
				c.Subscribe(channels)
			}
		case MessageTypeUnsubscribe:
			var channels []string
			if err := json.Unmarshal(msg.Payload, &channels); err == nil {
				c.Unsubscribe(channels)
			}
		case MessageTypePing:
			c.sendPong()
		}
	}
}

// WritePump 写入消息循环
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 批量发送
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendPong() {
	msg := Message{Type: MessageTypePong}
	data, _ := json.Marshal(msg)
	c.send <- data
}

// SendMessage 发送消息
func (c *Client) SendMessage(msgType MessageType, payload interface{}) {
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return
	}
	msg := Message{
		Type:    msgType,
		Payload: payloadData,
	}
	data, _ := json.Marshal(msg)
	select {
	case c.send <- data:
	default:
		// 发送失败
	}
}