// Package node defines the Node interface and related types.
package node

import (
	"context"

	"github.com/bherbruck/vibeflow/pkg/message"
)

// TypeInfo describes a node type's metadata for the UI.
type TypeInfo struct {
	Type        string       `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Category    string       `json:"category,omitempty"`
	Inputs      []PortInfo   `json:"inputs,omitempty"`
	Outputs     []PortInfo   `json:"outputs,omitempty"`
	Config      []ConfigSpec `json:"config,omitempty"`
	HasStart    bool         `json:"has_start,omitempty"`
}

// PortInfo describes an input or output port.
type PortInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ConfigSpec describes a configuration option.
type ConfigSpec struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"` // "string", "int", "bool", "float", "object", "array"
	Required    bool         `json:"required,omitempty"`
	Default     any          `json:"default,omitempty"`
	Description string       `json:"description,omitempty"`
	Options     []any        `json:"options,omitempty"` // For enum-like fields
	Items       []ConfigSpec `json:"items,omitempty"`   // For array type: schema of each item's fields
	// Code editor support
	Format   string `json:"format,omitempty"`   // "code" or "template" for code editor fields
	Language string `json:"language,omitempty"` // e.g., "javascript", "json", "yaml", "template"
}

// Node defines the interface that all built-in nodes must implement.
type Node interface {
	// Init initializes the node with configuration.
	// Called once during flow initialization.
	Init(ctx context.Context, cfg *Config) error

	// Process handles an incoming message.
	// The emitter is used to send output messages.
	Process(ctx context.Context, msg *message.Message, emit Emitter) error

	// Stop performs cleanup when the node is shutting down.
	Stop(ctx context.Context) error
}

// Starter is implemented by nodes that need to start background tasks.
type Starter interface {
	// Start is called after Init, before any messages are processed.
	// Use this to start timers, pollers, listeners, etc.
	Start(ctx context.Context, emit Emitter) error
}

// Emitter is used by nodes to emit output messages.
type Emitter interface {
	// Emit sends a message to the specified output port.
	// Use "default" for the default output.
	Emit(output string, msg *message.Message)
}

// Config holds the configuration for a node instance.
type Config struct {
	// ID is the unique node identifier in the flow
	ID string

	// Type is the node type (e.g., "core.inject")
	Type string

	// Name is the optional display name
	Name string

	// Config is the node-specific configuration
	Config map[string]any
}

// GetString returns a string config value or the default.
func (c *Config) GetString(key string, defaultVal string) string {
	if c.Config == nil {
		return defaultVal
	}
	if v, ok := c.Config[key].(string); ok {
		return v
	}
	return defaultVal
}

// GetInt returns an int config value or the default.
func (c *Config) GetInt(key string, defaultVal int) int {
	if c.Config == nil {
		return defaultVal
	}
	switch v := c.Config[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return defaultVal
}

// GetBool returns a bool config value or the default.
func (c *Config) GetBool(key string, defaultVal bool) bool {
	if c.Config == nil {
		return defaultVal
	}
	if v, ok := c.Config[key].(bool); ok {
		return v
	}
	return defaultVal
}

// Get returns a config value or nil.
func (c *Config) Get(key string) any {
	if c.Config == nil {
		return nil
	}
	return c.Config[key]
}
