package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/bherbruck/vibeflow/pkg/events"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// WSMessage is the JSON message format sent over WebSocket.
type WSMessage struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	FlowID    string    `json:"flow_id,omitempty"`
	NodeID    string    `json:"node_id,omitempty"`
	Data      any       `json:"data,omitempty"`
}

// WSHub manages WebSocket connections and broadcasts events.
type WSHub struct {
	bus    *events.Bus
	logger *slog.Logger

	// Connected clients
	clients   map[*wsClient]bool
	clientsMu sync.RWMutex

	// Subscription to event bus
	sub *events.Subscription

	// Shutdown
	done chan struct{}
}

type wsClient struct {
	hub     *WSHub
	conn    *websocket.Conn
	send    chan []byte
	pattern string // Subscription pattern from client
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub(bus *events.Bus) *WSHub {
	hub := &WSHub{
		bus:     bus,
		logger:  slog.Default(),
		clients: make(map[*wsClient]bool),
		done:    make(chan struct{}),
	}

	// Subscribe to all events
	hub.sub = bus.Subscribe("*")

	return hub
}

// Start begins processing events and broadcasting to clients.
func (h *WSHub) Start() {
	go h.run()
}

// Stop shuts down the hub.
func (h *WSHub) Stop() {
	close(h.done)
	h.sub.Unsubscribe()

	h.clientsMu.Lock()
	for client := range h.clients {
		close(client.send)
		client.conn.Close()
	}
	h.clientsMu.Unlock()
}

func (h *WSHub) run() {
	for {
		select {
		case <-h.done:
			return
		case event, ok := <-h.sub.Events():
			if !ok {
				return
			}
			h.broadcast(event)
		}
	}
}

// isDebugPanelEvent returns true for events that should be sent to the debug panel.
// Only send: debug node output and flow lifecycle (start/stop).
func isDebugPanelEvent(t events.EventType) bool {
	switch t {
	case events.EventDebug,
		events.EventFlowStart,
		events.EventFlowStop:
		return true
	default:
		return false
	}
}

func (h *WSHub) broadcast(e events.Event) {
	// Filter: only send debug-relevant events to the UI
	if !isDebugPanelEvent(e.Type) {
		return
	}

	msg := WSMessage{
		Type:      e.Type.String(),
		Timestamp: e.Timestamp,
		FlowID:    e.FlowID,
		NodeID:    e.NodeID,
		Data:      e.Data,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal event", "error", err)
		return
	}

	h.clientsMu.RLock()
	clients := make([]*wsClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.clientsMu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- data:
		default:
			// Buffer full, close connection
			h.removeClient(client)
		}
	}
}

func (h *WSHub) addClient(client *wsClient) {
	h.clientsMu.Lock()
	h.clients[client] = true
	h.clientsMu.Unlock()
	h.logger.Info("websocket client connected", "remote", client.conn.RemoteAddr())
}

func (h *WSHub) removeClient(client *wsClient) {
	h.clientsMu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	h.clientsMu.Unlock()
	h.logger.Info("websocket client disconnected", "remote", client.conn.RemoteAddr())
}

// ServeHTTP handles WebSocket upgrade requests.
func (h *WSHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := &wsClient{
		hub:     h,
		conn:    conn,
		send:    make(chan []byte, 256),
		pattern: "*", // Default to all events
	}

	h.addClient(client)

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// ClientCount returns the number of connected clients.
func (h *WSHub) ClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

func (c *wsClient) readPump() {
	defer func() {
		c.hub.removeClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("websocket read error", "error", err)
			}
			break
		}

		// Handle incoming messages (e.g., subscription changes)
		var req struct {
			Subscribe string `json:"subscribe"`
		}
		if err := json.Unmarshal(message, &req); err == nil && req.Subscribe != "" {
			c.pattern = req.Subscribe
			c.hub.logger.Debug("client subscription changed", "pattern", c.pattern)
		}
	}
}

func (c *wsClient) writePump() {
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
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Batch any queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

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
