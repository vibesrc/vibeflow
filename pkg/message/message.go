// Package message defines the Message type that flows between nodes.
package message

import (
	"time"

	"github.com/google/uuid"
)

// Message represents a single unit of data flowing through the system.
type Message struct {
	// ID is a unique identifier for this message
	ID string `json:"id"`

	// Payload is the message data
	Payload any `json:"payload"`

	// Metadata contains routing hints, timestamps, correlation IDs
	Metadata map[string]string `json:"metadata,omitempty"`

	// Context is flow-scoped shared state
	Context map[string]any `json:"context,omitempty"`
}

// New creates a new message with the given payload.
func New(payload any) *Message {
	return &Message{
		ID:       uuid.New().String(),
		Payload:  payload,
		Metadata: make(map[string]string),
		Context:  make(map[string]any),
	}
}

// Clone creates a shallow copy of the message with a new ID.
func (m *Message) Clone() *Message {
	metadata := make(map[string]string, len(m.Metadata))
	for k, v := range m.Metadata {
		metadata[k] = v
	}

	ctx := make(map[string]any, len(m.Context))
	for k, v := range m.Context {
		ctx[k] = v
	}

	return &Message{
		ID:       uuid.New().String(),
		Payload:  m.Payload, // Shallow copy of payload
		Metadata: metadata,
		Context:  ctx,
	}
}

// WithPayload returns a clone with a new payload.
func (m *Message) WithPayload(payload any) *Message {
	clone := m.Clone()
	clone.Payload = payload
	return clone
}

// SetMeta sets a metadata value.
func (m *Message) SetMeta(key, value string) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
}

// GetMeta gets a metadata value.
func (m *Message) GetMeta(key string) string {
	if m.Metadata == nil {
		return ""
	}
	return m.Metadata[key]
}

// SetTimestamp sets the timestamp metadata.
func (m *Message) SetTimestamp() {
	m.SetMeta("timestamp", time.Now().UTC().Format(time.RFC3339Nano))
}
