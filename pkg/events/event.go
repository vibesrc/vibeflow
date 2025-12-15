// Package events provides a high-performance event bus with pluggable hooks.
package events

import (
	"sync"
	"time"
)

// EventType identifies the kind of event.
type EventType uint8

const (
	// Flow lifecycle events
	EventFlowStart EventType = iota
	EventFlowStop
	EventFlowError

	// Node lifecycle events
	EventNodeInit
	EventNodeStart
	EventNodeStop

	// Message processing events
	EventNodeProcess // Node received a message
	EventNodeEmit    // Node emitted a message
	EventNodeError   // Node encountered an error

	// Debug and observability
	EventDebug    // Debug node output
	EventLog      // General log message
	EventVariable // Variable was set/changed
)

// String returns the event type name.
func (t EventType) String() string {
	switch t {
	case EventFlowStart:
		return "flow.start"
	case EventFlowStop:
		return "flow.stop"
	case EventFlowError:
		return "flow.error"
	case EventNodeInit:
		return "node.init"
	case EventNodeStart:
		return "node.start"
	case EventNodeStop:
		return "node.stop"
	case EventNodeProcess:
		return "node.process"
	case EventNodeEmit:
		return "node.emit"
	case EventNodeError:
		return "node.error"
	case EventDebug:
		return "debug"
	case EventLog:
		return "log"
	case EventVariable:
		return "variable"
	default:
		return "unknown"
	}
}

// Event represents something that happened in the runtime.
// Data is passed by pointer for zero-copy in the hot path.
type Event struct {
	Type      EventType
	Timestamp time.Time
	FlowID    string
	NodeID    string
	Data      any // Payload - type depends on EventType
}

// DebugData is the payload for EventDebug.
type DebugData struct {
	NodeName string `json:"node_name"`
	Topic    string `json:"topic"`
	Payload  any    `json:"payload"`
	Level    string `json:"level"` // "log", "warn", "error"
}

// ErrorData is the payload for error events.
type ErrorData struct {
	Error   error
	Message string
}

// VariableData is the payload for EventVariable.
type VariableData struct {
	Scope string // "flow", "node", "global"
	Key   string
	Value any
}

// ProcessData is the payload for EventNodeProcess/EventNodeEmit.
type ProcessData struct {
	MessageID string
	Port      string // Input/output port name
	Payload   any    // Message payload (may be truncated for large payloads)
}

// Event pool for reducing allocations
var eventPool = sync.Pool{
	New: func() any {
		return &Event{}
	},
}

// AcquireEvent gets an event from the pool.
func AcquireEvent() *Event {
	return eventPool.Get().(*Event)
}

// ReleaseEvent returns an event to the pool.
func ReleaseEvent(e *Event) {
	e.Type = 0
	e.Timestamp = time.Time{}
	e.FlowID = ""
	e.NodeID = ""
	e.Data = nil
	eventPool.Put(e)
}
