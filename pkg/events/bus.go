package events

import (
	"strings"
	"sync"
	"time"
)

// Hook is an interface for event observers.
// Implementations must be fast and non-blocking.
// For expensive operations, hooks should use internal buffering/goroutines.
type Hook interface {
	OnEvent(Event)
}

// HookFunc is a function adapter for Hook interface.
type HookFunc func(Event)

func (f HookFunc) OnEvent(e Event) { f(e) }

// Subscription represents an active subscription to the bus.
type Subscription struct {
	id      uint64
	pattern string
	ch      chan Event
	bus     *Bus
}

// Unsubscribe removes this subscription from the bus.
func (s *Subscription) Unsubscribe() {
	s.bus.unsubscribe(s)
}

// Events returns the channel for receiving events.
func (s *Subscription) Events() <-chan Event {
	return s.ch
}

// Bus is a high-performance event bus with pluggable hooks.
type Bus struct {
	// Hooks for metrics, logging, tracing, etc.
	hooks   []Hook
	hooksMu sync.RWMutex

	// Channel-based subscribers for real-time streaming
	subs     map[uint64]*Subscription
	subsMu   sync.RWMutex
	subID    uint64
	subIDMu  sync.Mutex

	// Buffer size for subscriber channels
	bufferSize int
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		hooks:      make([]Hook, 0),
		subs:       make(map[uint64]*Subscription),
		bufferSize: 256, // Default buffer size
	}
}

// SetBufferSize sets the buffer size for new subscriber channels.
func (b *Bus) SetBufferSize(size int) {
	b.bufferSize = size
}

// AddHook registers a hook to receive all events.
// Hooks are called synchronously, so they must be fast.
func (b *Bus) AddHook(h Hook) {
	b.hooksMu.Lock()
	b.hooks = append(b.hooks, h)
	b.hooksMu.Unlock()
}

// RemoveHook unregisters a hook.
func (b *Bus) RemoveHook(h Hook) {
	b.hooksMu.Lock()
	defer b.hooksMu.Unlock()

	for i, hook := range b.hooks {
		if hook == h {
			b.hooks = append(b.hooks[:i], b.hooks[i+1:]...)
			return
		}
	}
}

// Subscribe creates a subscription for events matching the pattern.
// Pattern supports:
//   - "*" matches all events
//   - "flow.*" matches all flow events
//   - "node.*" matches all node events
//   - "debug" matches only debug events
//   - "flow.start,flow.stop" matches multiple specific events
func (b *Bus) Subscribe(pattern string) *Subscription {
	b.subIDMu.Lock()
	b.subID++
	id := b.subID
	b.subIDMu.Unlock()

	sub := &Subscription{
		id:      id,
		pattern: pattern,
		ch:      make(chan Event, b.bufferSize),
		bus:     b,
	}

	b.subsMu.Lock()
	b.subs[id] = sub
	b.subsMu.Unlock()

	return sub
}

func (b *Bus) unsubscribe(s *Subscription) {
	b.subsMu.Lock()
	delete(b.subs, s.id)
	b.subsMu.Unlock()
	close(s.ch)
}

// Emit publishes an event to all hooks and matching subscribers.
func (b *Bus) Emit(e Event) {
	// Set timestamp if not already set
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	// 1. Notify hooks (synchronous, must be fast)
	b.hooksMu.RLock()
	hooks := b.hooks
	b.hooksMu.RUnlock()

	for _, h := range hooks {
		h.OnEvent(e)
	}

	// 2. Fan out to channel subscribers (non-blocking)
	b.subsMu.RLock()
	subs := make([]*Subscription, 0, len(b.subs))
	for _, sub := range b.subs {
		subs = append(subs, sub)
	}
	b.subsMu.RUnlock()

	eventType := e.Type.String()
	for _, sub := range subs {
		if matchPattern(sub.pattern, eventType) {
			// Non-blocking send - drop if buffer full
			select {
			case sub.ch <- e:
			default:
				// Buffer full, event dropped
				// In production, you might want to track this
			}
		}
	}
}

// EmitAsync publishes an event asynchronously.
// Useful when you don't want to block the caller.
func (b *Bus) EmitAsync(e Event) {
	go b.Emit(e)
}

// Helper methods for common event types

// FlowStarted emits a flow start event.
func (b *Bus) FlowStarted(flowID string) {
	b.Emit(Event{
		Type:   EventFlowStart,
		FlowID: flowID,
	})
}

// FlowStopped emits a flow stop event.
func (b *Bus) FlowStopped(flowID string) {
	b.Emit(Event{
		Type:   EventFlowStop,
		FlowID: flowID,
	})
}

// FlowError emits a flow error event.
func (b *Bus) FlowError(flowID string, err error) {
	b.Emit(Event{
		Type:   EventFlowError,
		FlowID: flowID,
		Data:   &ErrorData{Error: err, Message: err.Error()},
	})
}

// NodeProcessed emits a node process event.
func (b *Bus) NodeProcessed(flowID, nodeID, messageID, port string, payload any) {
	b.Emit(Event{
		Type:   EventNodeProcess,
		FlowID: flowID,
		NodeID: nodeID,
		Data: &ProcessData{
			MessageID: messageID,
			Port:      port,
			Payload:   payload,
		},
	})
}

// NodeEmitted emits a node emit event.
func (b *Bus) NodeEmitted(flowID, nodeID, messageID, port string, payload any) {
	b.Emit(Event{
		Type:   EventNodeEmit,
		FlowID: flowID,
		NodeID: nodeID,
		Data: &ProcessData{
			MessageID: messageID,
			Port:      port,
			Payload:   payload,
		},
	})
}

// NodeError emits a node error event.
func (b *Bus) NodeError(flowID, nodeID string, err error) {
	b.Emit(Event{
		Type:   EventNodeError,
		FlowID: flowID,
		NodeID: nodeID,
		Data:   &ErrorData{Error: err, Message: err.Error()},
	})
}

// Debug emits a debug event.
func (b *Bus) Debug(flowID, nodeID, nodeName, topic string, payload any, level string) {
	b.Emit(Event{
		Type:   EventDebug,
		FlowID: flowID,
		NodeID: nodeID,
		Data: &DebugData{
			NodeName: nodeName,
			Topic:    topic,
			Payload:  payload,
			Level:    level,
		},
	})
}

// matchPattern checks if an event type matches a subscription pattern.
func matchPattern(pattern, eventType string) bool {
	if pattern == "*" {
		return true
	}

	// Check for comma-separated patterns
	if strings.Contains(pattern, ",") {
		patterns := strings.Split(pattern, ",")
		for _, p := range patterns {
			if matchSinglePattern(strings.TrimSpace(p), eventType) {
				return true
			}
		}
		return false
	}

	return matchSinglePattern(pattern, eventType)
}

func matchSinglePattern(pattern, eventType string) bool {
	if pattern == "*" {
		return true
	}

	// Wildcard suffix match: "flow.*" matches "flow.start", "flow.stop", etc.
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(eventType, prefix+".")
	}

	// Exact match
	return pattern == eventType
}
