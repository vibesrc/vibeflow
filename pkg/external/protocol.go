// Package external provides support for external node processes.
package external

import (
	"github.com/bherbruck/vibeflow/pkg/message"
)

// JSON-RPC 2.0 request/response types

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard error codes
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

// Protocol methods
const (
	MethodGetManifest = "getManifest"
	MethodInit        = "init"
	MethodProcess     = "process"
	MethodStart       = "start"
	MethodStop        = "stop"
	MethodShutdown    = "shutdown"
)

// Manifest describes an external node.
type Manifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	NodeTypes   []NodeTypeInfo    `json:"node_types"`
	Checksum    string            `json:"checksum,omitempty"`
	Protocol    string            `json:"protocol,omitempty"` // "jsonrpc/2.0"
}

// NodeTypeInfo describes a node type provided by an external process.
type NodeTypeInfo struct {
	Type        string       `json:"type"`        // e.g., "vendor.mynode"
	Name        string       `json:"name"`        // Display name
	Description string       `json:"description,omitempty"`
	Category    string       `json:"category,omitempty"`
	Inputs      []PortInfo   `json:"inputs,omitempty"`
	Outputs     []PortInfo   `json:"outputs,omitempty"`
	Config      []ConfigSpec `json:"config,omitempty"`
	HasStart    bool         `json:"has_start,omitempty"` // Implements Start method
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
}

// InitParams are parameters for the init method.
type InitParams struct {
	NodeID   string         `json:"node_id"`
	NodeType string         `json:"node_type"`
	Config   map[string]any `json:"config"`
}

// InitResult is the result of the init method.
type InitResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ProcessParams are parameters for the process method.
type ProcessParams struct {
	NodeID  string          `json:"node_id"`
	Message *message.Message `json:"message"`
}

// ProcessResult is the result of the process method.
type ProcessResult struct {
	Outputs map[string][]*message.Message `json:"outputs,omitempty"` // port -> messages
	Error   string                        `json:"error,omitempty"`
}

// StartParams are parameters for the start method.
type StartParams struct {
	NodeID string `json:"node_id"`
}

// StartResult is the result of the start method.
type StartResult struct {
	Outputs map[string][]*message.Message `json:"outputs,omitempty"`
	Error   string                        `json:"error,omitempty"`
}

// StopParams are parameters for the stop method.
type StopParams struct {
	NodeID string `json:"node_id"`
}

// StopResult is the result of the stop method.
type StopResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// NewRequest creates a new JSON-RPC request.
func NewRequest(id int, method string, params any) *Request {
	return &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewResponse creates a successful JSON-RPC response.
func NewResponse(id int, result any) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates an error JSON-RPC response.
func NewErrorResponse(id int, code int, message string, data any) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}
