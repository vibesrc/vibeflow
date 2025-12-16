package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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

// ErrorSnapshotFunc returns current node errors for all running flows.
// Returns map[flowID]map[nodeID]errorMessage
type ErrorSnapshotFunc func() map[string]map[string]string

// VariableSnapshotFunc returns current variables from all running flows.
// Returns map[flowID]map[key]value where key includes scope prefix.
type VariableSnapshotFunc func() map[string]map[string]any

// WSHub manages WebSocket connections and broadcasts events.
type WSHub struct {
	bus              *events.Bus
	logger           *slog.Logger
	errorSnapshot    ErrorSnapshotFunc
	variableSnapshot VariableSnapshotFunc

	// Connected clients
	clients   map[*wsClient]bool
	clientsMu sync.RWMutex

	// Subscription to event bus
	sub *events.Subscription

	// Shutdown
	done chan struct{}
}

// Subscription represents a client's event subscription.
type Subscription struct {
	FlowID string   `json:"flow_id,omitempty"` // Flow to subscribe to (empty = global only)
	Types  []string `json:"types,omitempty"`   // Event types: "debug", "errors", "variables", "lifecycle"
}

type wsClient struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte

	// Subscription state
	mu           sync.RWMutex
	subscription *Subscription // nil = not subscribed to anything
}

// isSubscribed checks if the client wants to receive this event.
func (c *wsClient) isSubscribed(eventType string, flowID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.subscription == nil {
		return false // Not subscribed to anything
	}

	// Check flow match (empty subscription flow = global events only)
	if c.subscription.FlowID != "" && c.subscription.FlowID != flowID {
		return false
	}

	// Check type match
	if len(c.subscription.Types) == 0 {
		return true // No type filter = all types
	}

	// Map event types to subscription categories
	category := eventTypeToCategory(eventType)
	for _, t := range c.subscription.Types {
		if t == category {
			return true
		}
	}
	return false
}

// setSubscription updates the client's subscription.
func (c *wsClient) setSubscription(sub *Subscription) {
	c.mu.Lock()
	c.subscription = sub
	c.mu.Unlock()
}

// getSubscription returns a copy of the current subscription.
func (c *wsClient) getSubscription() *Subscription {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.subscription == nil {
		return nil
	}
	// Return a copy
	return &Subscription{
		FlowID: c.subscription.FlowID,
		Types:  append([]string{}, c.subscription.Types...),
	}
}

// eventTypeToCategory maps event type strings to subscription categories.
func eventTypeToCategory(eventType string) string {
	switch eventType {
	case "debug":
		return "debug"
	case "node.error", "flow.error":
		return "errors"
	case "variable":
		return "variables"
	case "flow.start", "flow.stop":
		return "lifecycle"
	default:
		return eventType
	}
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub(bus *events.Bus, errorSnapshot ErrorSnapshotFunc, variableSnapshot VariableSnapshotFunc) *WSHub {
	hub := &WSHub{
		bus:              bus,
		logger:           slog.Default(),
		errorSnapshot:    errorSnapshot,
		variableSnapshot: variableSnapshot,
		clients:          make(map[*wsClient]bool),
		done:             make(chan struct{}),
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
// Only send: debug node output, flow lifecycle (start/stop), errors, and variable changes.
func isDebugPanelEvent(t events.EventType) bool {
	switch t {
	case events.EventDebug,
		events.EventFlowStart,
		events.EventFlowStop,
		events.EventFlowError,
		events.EventNodeError,
		events.EventVariable:
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

	eventType := e.Type.String()

	// Sanitize event data to prevent marshal panics (goja values, etc.)
	safeData := sanitizeForJSON(e.Data)

	msg := WSMessage{
		Type:      eventType,
		Timestamp: e.Timestamp,
		FlowID:    e.FlowID,
		NodeID:    e.NodeID,
		Data:      safeData,
	}

	data, err := safeMarshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal event", "error", err, "type", eventType)
		return
	}

	h.clientsMu.RLock()
	clients := make([]*wsClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.clientsMu.RUnlock()

	for _, client := range clients {
		// Check if client is subscribed to this event
		if !client.isSubscribed(eventType, e.FlowID) {
			continue
		}

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
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
		// subscription starts as nil - client must subscribe to receive events
	}

	h.addClient(client)

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()

	// Don't send snapshots automatically - wait for client to subscribe
}

// sendErrorSnapshot sends current node errors to a client as node.error events.
// Only sends errors for flows the client is subscribed to.
func (h *WSHub) sendErrorSnapshot(client *wsClient) {
	if h.errorSnapshot == nil {
		return
	}

	sub := client.getSubscription()
	if sub == nil {
		return
	}

	// Check if client wants errors
	wantsErrors := len(sub.Types) == 0 // no filter = all types
	for _, t := range sub.Types {
		if t == "errors" {
			wantsErrors = true
			break
		}
	}
	if !wantsErrors {
		return
	}

	allErrors := h.errorSnapshot()
	for flowID, nodeErrors := range allErrors {
		// Only send for subscribed flow
		if sub.FlowID != "" && sub.FlowID != flowID {
			continue
		}

		for nodeID, message := range nodeErrors {
			msg := WSMessage{
				Type:      "node.error",
				Timestamp: time.Now(),
				FlowID:    flowID,
				NodeID:    nodeID,
				Data:      map[string]string{"message": message},
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			select {
			case client.send <- data:
			default:
				// Buffer full, skip
			}
		}
	}
}

// sendVariableSnapshot sends current variables to a client as variable events.
// Only sends variables for flows the client is subscribed to.
func (h *WSHub) sendVariableSnapshot(client *wsClient) {
	if h.variableSnapshot == nil {
		return
	}

	sub := client.getSubscription()
	if sub == nil {
		return
	}

	// Check if client wants variables
	wantsVars := len(sub.Types) == 0 // no filter = all types
	for _, t := range sub.Types {
		if t == "variables" {
			wantsVars = true
			break
		}
	}
	if !wantsVars {
		return
	}

	allVars := h.variableSnapshot()
	for flowID, vars := range allVars {
		// Only send for subscribed flow
		if sub.FlowID != "" && sub.FlowID != flowID {
			continue
		}

		for key, value := range vars {
			// Parse key to extract scope, nodeID, and variable name
			// Key formats: "global:name", "flow:flowID:name", "node:flowID:nodeID:name"
			scope, nodeID, varKey := parseVariableKey(key)

			// Sanitize value to prevent panics
			safeValue := sanitizeForJSON(value)

			msg := WSMessage{
				Type:      "variable",
				Timestamp: time.Now(),
				FlowID:    flowID,
				NodeID:    nodeID,
				Data: map[string]any{
					"scope": scope,
					"key":   varKey,
					"value": safeValue,
				},
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			select {
			case client.send <- data:
			default:
				// Buffer full, skip
			}
		}
	}
}

// parseVariableKey extracts scope, nodeID, and key name from a prefixed key.
func parseVariableKey(key string) (scope, nodeID, varKey string) {
	parts := strings.SplitN(key, ":", 4)
	switch parts[0] {
	case "global":
		scope = "global"
		if len(parts) >= 2 {
			varKey = parts[1]
		}
	case "flow":
		scope = "flow"
		if len(parts) >= 3 {
			varKey = parts[2]
		}
	case "node":
		scope = "node"
		if len(parts) >= 4 {
			nodeID = parts[2]
			varKey = parts[3]
		}
	}
	return
}

// sanitizeForJSON converts a value to a JSON-serializable form.
func sanitizeForJSON(value any) (result any) {
	if value == nil {
		return nil
	}

	defer func() {
		if r := recover(); r != nil {
			result = fmt.Sprintf("%v", value)
		}
	}()

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Sprintf("%v", value)
	}

	return result
}

// safeMarshal wraps json.Marshal with panic recovery.
func safeMarshal(v any) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("marshal panic: %v", r)
			data = nil
		}
	}()

	return json.Marshal(v)
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

		// Handle incoming messages
		var req struct {
			Subscribe   *Subscription `json:"subscribe,omitempty"`
			Unsubscribe bool          `json:"unsubscribe,omitempty"`
		}
		if err := json.Unmarshal(message, &req); err != nil {
			c.hub.logger.Debug("failed to parse websocket message", "error", err)
			continue
		}

		if req.Unsubscribe {
			// Unsubscribe from all events
			c.setSubscription(nil)
			c.hub.logger.Debug("client unsubscribed", "remote", c.conn.RemoteAddr())
		} else if req.Subscribe != nil {
			// Update subscription and send snapshots
			c.setSubscription(req.Subscribe)
			c.hub.logger.Debug("client subscribed",
				"remote", c.conn.RemoteAddr(),
				"flow_id", req.Subscribe.FlowID,
				"types", req.Subscribe.Types,
			)

			// Send current state snapshots for the subscribed flow/types
			c.hub.sendErrorSnapshot(c)
			c.hub.sendVariableSnapshot(c)
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
