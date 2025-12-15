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

// ShowWhen defines conditions for when a config field should be visible.
// Multiple conditions are ANDed together.
type ShowWhen struct {
	Field    string `json:"field"`              // Name of the field to check
	Eq       any    `json:"eq,omitempty"`       // Show when field equals this value
	Ne       any    `json:"ne,omitempty"`       // Show when field does not equal this value
	In       []any  `json:"in,omitempty"`       // Show when field value is in this list
	NotIn    []any  `json:"notIn,omitempty"`    // Show when field value is not in this list
	Present  bool   `json:"present,omitempty"`  // Show when field has a non-empty value
	Absent   bool   `json:"absent,omitempty"`   // Show when field is empty/unset
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
	// Conditional visibility
	ShowWhen *ShowWhen `json:"showWhen,omitempty"` // Conditions for when this field is visible
}

// Output represents a handle to an output port.
// Obtained during Init via Outputs.Get() - errors if port doesn't exist.
type Output interface {
	// Send sends a message to this output port.
	Send(msg *message.Message)
}

// Outputs provides access to output ports during initialization.
// Use Get() to obtain Output handles - errors if port isn't wired.
type Outputs interface {
	// Get returns an Output handle for the named port.
	// Returns error if the port doesn't exist or isn't wired.
	Get(name string) (Output, error)

	// Has returns true if the named output port exists and is wired.
	Has(name string) bool
}

// Input represents a handle to an input port.
// Used to identify which port a message arrived on.
type Input interface {
	// Name returns the port name.
	Name() string
}

// Inputs provides access to input ports during initialization.
type Inputs interface {
	// Get returns an Input handle for the named port.
	// Returns error if the port doesn't exist or isn't wired.
	Get(name string) (Input, error)

	// Has returns true if the named input port exists and is wired.
	Has(name string) bool
}

// Node defines the interface that all built-in nodes must implement.
type Node interface {
	// Init initializes the node with configuration.
	// Called once during flow initialization.
	// Inputs/Outputs allow LBYL port validation - get handles upfront.
	Init(ctx context.Context, cfg *Config, inputs Inputs, outputs Outputs) error

	// Process handles an incoming message.
	// inputPort identifies which input the message arrived on.
	Process(ctx context.Context, msg *message.Message, inputPort string) error

	// Stop performs cleanup when the node is shutting down.
	Stop(ctx context.Context) error
}

// Starter is implemented by nodes that need to start background tasks.
type Starter interface {
	// Start is called after Init, before any messages are processed.
	// Use this to start timers, pollers, listeners, etc.
	// Outputs were already obtained during Init.
	Start(ctx context.Context) error
}

// Emitter is used by nodes to emit output messages.
// Deprecated: Use Output.Send() instead. Kept for backwards compatibility.
type Emitter interface {
	// Emit sends a message to the specified output port.
	// Use "default" for the default output.
	Emit(output string, msg *message.Message)
}

// DebugEmitter allows nodes to emit debug events to the UI sidebar.
// Implementations are provided by the runtime and made available via context.
type DebugEmitter interface {
	// Debug emits a debug event that appears in the UI sidebar.
	// topic is optional and can provide context (like Node-RED's msg.topic).
	// payload is the data to display (will be JSON-serialized if not a string).
	Debug(topic string, payload any)
}

// DebugEmitterKey is the context key for accessing the DebugEmitter.
type debugEmitterKeyType struct{}

// DebugEmitterKey is used to store/retrieve DebugEmitter from context.
var DebugEmitterKey = debugEmitterKeyType{}

// GetDebugEmitter retrieves the DebugEmitter from the context, or nil if not present.
func GetDebugEmitter(ctx context.Context) DebugEmitter {
	if v := ctx.Value(DebugEmitterKey); v != nil {
		if de, ok := v.(DebugEmitter); ok {
			return de
		}
	}
	return nil
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
