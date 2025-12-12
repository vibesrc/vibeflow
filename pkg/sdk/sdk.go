// Package sdk provides helpers for building external Vibeflow nodes.
package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/bherbruck/vibeflow/pkg/external"
	"github.com/bherbruck/vibeflow/pkg/message"
)

// NodeHandler is the interface for external node implementations.
type NodeHandler interface {
	// Init initializes the node with configuration.
	Init(ctx context.Context, nodeID string, config map[string]any) error

	// Process handles an incoming message.
	// Returns a map of output port -> messages to emit.
	Process(ctx context.Context, msg *message.Message) (map[string][]*message.Message, error)

	// Stop performs cleanup.
	Stop(ctx context.Context) error
}

// StarterHandler is for nodes that emit messages without input.
type StarterHandler interface {
	NodeHandler

	// Start is called to begin message production.
	// Returns initial messages to emit.
	Start(ctx context.Context) (map[string][]*message.Message, error)
}

// Plugin is the main entry point for external nodes.
type Plugin struct {
	manifest *external.Manifest
	handlers map[string]func() NodeHandler // Factory functions per node type
	nodes    map[string]NodeHandler        // Instantiated nodes by ID

	stdin  io.Reader
	stdout io.Writer
}

// NewPlugin creates a new plugin.
func NewPlugin(name, version, description string) *Plugin {
	return &Plugin{
		manifest: &external.Manifest{
			Name:        name,
			Version:     version,
			Description: description,
			Protocol:    "jsonrpc/2.0",
			NodeTypes:   make([]external.NodeTypeInfo, 0),
		},
		handlers: make(map[string]func() NodeHandler),
		nodes:    make(map[string]NodeHandler),
		stdin:    os.Stdin,
		stdout:   os.Stdout,
	}
}

// RegisterNode registers a node type with the plugin.
func (p *Plugin) RegisterNode(info external.NodeTypeInfo, factory func() NodeHandler) {
	p.manifest.NodeTypes = append(p.manifest.NodeTypes, info)
	p.handlers[info.Type] = factory
}

// Run starts the plugin and processes requests.
func (p *Plugin) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(p.stdin)

	for scanner.Scan() {
		line := scanner.Bytes()

		var req external.Request
		if err := json.Unmarshal(line, &req); err != nil {
			p.sendError(0, external.ErrCodeParse, "Parse error", err.Error())
			continue
		}

		resp := p.handleRequest(ctx, &req)
		p.sendResponse(resp)
	}

	return scanner.Err()
}

func (p *Plugin) handleRequest(ctx context.Context, req *external.Request) *external.Response {
	switch req.Method {
	case external.MethodGetManifest:
		return external.NewResponse(req.ID, p.manifest)

	case external.MethodInit:
		return p.handleInit(ctx, req)

	case external.MethodProcess:
		return p.handleProcess(ctx, req)

	case external.MethodStart:
		return p.handleStart(ctx, req)

	case external.MethodStop:
		return p.handleStop(ctx, req)

	case external.MethodShutdown:
		// Clean shutdown - stop all nodes
		for id, handler := range p.nodes {
			handler.Stop(ctx)
			delete(p.nodes, id)
		}
		return external.NewResponse(req.ID, map[string]bool{"success": true})

	default:
		return external.NewErrorResponse(req.ID, external.ErrCodeMethodNotFound,
			"Method not found", req.Method)
	}
}

func (p *Plugin) handleInit(ctx context.Context, req *external.Request) *external.Response {
	// Parse params
	data, _ := json.Marshal(req.Params)
	var params external.InitParams
	if err := json.Unmarshal(data, &params); err != nil {
		return external.NewErrorResponse(req.ID, external.ErrCodeInvalidParams,
			"Invalid params", err.Error())
	}

	// Find factory
	factory, ok := p.handlers[params.NodeType]
	if !ok {
		return external.NewResponse(req.ID, &external.InitResult{
			Success: false,
			Error:   fmt.Sprintf("unknown node type: %s", params.NodeType),
		})
	}

	// Create handler
	handler := factory()

	// Initialize
	if err := handler.Init(ctx, params.NodeID, params.Config); err != nil {
		return external.NewResponse(req.ID, &external.InitResult{
			Success: false,
			Error:   err.Error(),
		})
	}

	p.nodes[params.NodeID] = handler

	return external.NewResponse(req.ID, &external.InitResult{Success: true})
}

func (p *Plugin) handleProcess(ctx context.Context, req *external.Request) *external.Response {
	data, _ := json.Marshal(req.Params)
	var params external.ProcessParams
	if err := json.Unmarshal(data, &params); err != nil {
		return external.NewErrorResponse(req.ID, external.ErrCodeInvalidParams,
			"Invalid params", err.Error())
	}

	handler, ok := p.nodes[params.NodeID]
	if !ok {
		return external.NewResponse(req.ID, &external.ProcessResult{
			Error: fmt.Sprintf("node not found: %s", params.NodeID),
		})
	}

	outputs, err := handler.Process(ctx, params.Message)
	if err != nil {
		return external.NewResponse(req.ID, &external.ProcessResult{
			Error: err.Error(),
		})
	}

	return external.NewResponse(req.ID, &external.ProcessResult{Outputs: outputs})
}

func (p *Plugin) handleStart(ctx context.Context, req *external.Request) *external.Response {
	data, _ := json.Marshal(req.Params)
	var params external.StartParams
	if err := json.Unmarshal(data, &params); err != nil {
		return external.NewErrorResponse(req.ID, external.ErrCodeInvalidParams,
			"Invalid params", err.Error())
	}

	handler, ok := p.nodes[params.NodeID]
	if !ok {
		return external.NewResponse(req.ID, &external.StartResult{
			Error: fmt.Sprintf("node not found: %s", params.NodeID),
		})
	}

	starter, ok := handler.(StarterHandler)
	if !ok {
		return external.NewResponse(req.ID, &external.StartResult{})
	}

	outputs, err := starter.Start(ctx)
	if err != nil {
		return external.NewResponse(req.ID, &external.StartResult{
			Error: err.Error(),
		})
	}

	return external.NewResponse(req.ID, &external.StartResult{Outputs: outputs})
}

func (p *Plugin) handleStop(ctx context.Context, req *external.Request) *external.Response {
	data, _ := json.Marshal(req.Params)
	var params external.StopParams
	if err := json.Unmarshal(data, &params); err != nil {
		return external.NewErrorResponse(req.ID, external.ErrCodeInvalidParams,
			"Invalid params", err.Error())
	}

	handler, ok := p.nodes[params.NodeID]
	if !ok {
		return external.NewResponse(req.ID, &external.StopResult{
			Success: false,
			Error:   fmt.Sprintf("node not found: %s", params.NodeID),
		})
	}

	if err := handler.Stop(ctx); err != nil {
		return external.NewResponse(req.ID, &external.StopResult{
			Success: false,
			Error:   err.Error(),
		})
	}

	delete(p.nodes, params.NodeID)

	return external.NewResponse(req.ID, &external.StopResult{Success: true})
}

func (p *Plugin) sendResponse(resp *external.Response) {
	data, _ := json.Marshal(resp)
	fmt.Fprintln(p.stdout, string(data))
}

func (p *Plugin) sendError(id int, code int, message, data string) {
	resp := external.NewErrorResponse(id, code, message, data)
	p.sendResponse(resp)
}
