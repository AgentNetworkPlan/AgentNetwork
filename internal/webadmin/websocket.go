package webadmin

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// WSClient represents a WebSocket client.
type WSClient struct {
	conn    *websocket.Conn
	send    chan []byte
	channel string
	hub     *WebSocketHub
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log error if needed
			}
			break
		}
		// We don't expect messages from clients, just keep connection alive
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WebSocketHub maintains the set of active clients and broadcasts messages.
type WebSocketHub struct {
	// Registered clients by channel
	clients map[string]map[*WSClient]bool

	// Inbound messages to broadcast
	broadcast chan *BroadcastMessage

	// Register requests from clients
	register chan *WSClient

	// Unregister requests from clients
	unregister chan *WSClient

	// Close signal
	done chan struct{}

	mu sync.RWMutex
}

// BroadcastMessage represents a message to broadcast.
type BroadcastMessage struct {
	Channel string
	Data    []byte
}

// NewWebSocketHub creates a new WebSocketHub.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[string]map[*WSClient]bool),
		broadcast:  make(chan *BroadcastMessage, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		done:       make(chan struct{}),
	}
}

// Run starts the hub event loop.
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.channel] == nil {
				h.clients[client.channel] = make(map[*WSClient]bool)
			}
			h.clients[client.channel][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.channel]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			clients := h.clients[message.Channel]
			for client := range clients {
				select {
				case client.send <- message.Data:
				default:
					// Client is slow, skip this message
				}
			}
			h.mu.RUnlock()

		case <-h.done:
			return
		}
	}
}

// Broadcast sends a message to all clients in a channel.
func (h *WebSocketHub) Broadcast(channel string, data []byte) {
	select {
	case h.broadcast <- &BroadcastMessage{Channel: channel, Data: data}:
	default:
		// Channel full, drop message
	}
}

// Close closes the hub.
func (h *WebSocketHub) Close() {
	close(h.done)

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, clients := range h.clients {
		for client := range clients {
			close(client.send)
		}
	}
	h.clients = make(map[string]map[*WSClient]bool)
}

// ClientCount returns the number of clients in a channel.
func (h *WebSocketHub) ClientCount(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[channel])
}

// TotalClientCount returns the total number of connected clients.
func (h *WebSocketHub) TotalClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}
