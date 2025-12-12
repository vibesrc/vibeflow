// Package runtime provides the flow execution engine.
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bherbruck/vibeflow/pkg/external"
	"github.com/bherbruck/vibeflow/pkg/flow"
	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

// Runtime executes flows.
type Runtime struct {
	registry *node.Registry
	plugins  *external.Manager
	logger   *slog.Logger

	mu       sync.RWMutex
	nodes    map[string]node.Node
	wires    map[string][]wire // nodeID -> outgoing wires
	running  bool
	cancelFn context.CancelFunc

	// Message channels for each node
	nodeChans map[string]chan *message.Message
}

type wire struct {
	output string // Source output port
	toNode string // Target node ID
	input  string // Target input port
}

// New creates a new runtime.
func New(registry *node.Registry) *Runtime {
	if registry == nil {
		registry = node.DefaultRegistry
	}
	return &Runtime{
		registry:  registry,
		plugins:   external.NewManager(),
		logger:    slog.Default(),
		nodes:     make(map[string]node.Node),
		wires:     make(map[string][]wire),
		nodeChans: make(map[string]chan *message.Message),
	}
}

// LoadPlugin loads an external node plugin.
func (r *Runtime) LoadPlugin(ctx context.Context, path string, args ...string) error {
	return r.plugins.LoadPlugin(ctx, path, args...)
}

// Load loads a flow and initializes all nodes.
func (r *Runtime) Load(ctx context.Context, f *flow.Flow) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create and initialize nodes
	for _, nodeDef := range f.Nodes {
		if !nodeDef.IsEnabled() {
			r.logger.Info("skipping disabled node", "id", nodeDef.ID)
			continue
		}

		// Create node instance - try built-in registry first, then plugins
		var n node.Node
		var err error

		if r.registry.Has(nodeDef.Type) {
			n, err = r.registry.Create(nodeDef.Type)
		} else if r.plugins.HasType(nodeDef.Type) {
			n, err = r.plugins.CreateNode(nodeDef.Type)
		} else {
			err = fmt.Errorf("unknown node type: %s", nodeDef.Type)
		}

		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", nodeDef.ID, err)
		}

		// Initialize node
		cfg := &node.Config{
			ID:     nodeDef.ID,
			Type:   nodeDef.Type,
			Name:   nodeDef.Name,
			Config: nodeDef.Config,
		}

		if err := n.Init(ctx, cfg); err != nil {
			// Node init failed - log warning and skip this node (don't fail the whole flow)
			r.logger.Warn("failed to init node, skipping", "id", nodeDef.ID, "type", nodeDef.Type, "error", err)
			continue
		}

		r.nodes[nodeDef.ID] = n
		r.nodeChans[nodeDef.ID] = make(chan *message.Message, 100)

		r.logger.Info("initialized node", "id", nodeDef.ID, "type", nodeDef.Type)
	}

	// Build wire map
	for _, w := range f.Wires {
		output := w.Output
		if output == "" {
			output = "default"
		}
		input := w.Input
		if input == "" {
			input = "default"
		}

		r.wires[w.From] = append(r.wires[w.From], wire{
			output: output,
			toNode: w.To,
			input:  input,
		})
	}

	return nil
}

// Run starts the flow execution.
func (r *Runtime) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("runtime already running")
	}
	r.running = true

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFn = cancel
	r.mu.Unlock()

	var wg sync.WaitGroup

	// Start message processing goroutines for each node
	for nodeID, n := range r.nodes {
		wg.Add(1)
		go func(id string, node node.Node) {
			defer wg.Done()
			r.runNode(ctx, id, node)
		}(nodeID, n)
	}

	// Start nodes that implement Starter
	for nodeID, n := range r.nodes {
		if starter, ok := n.(node.Starter); ok {
			emitter := r.createEmitter(nodeID)
			if err := starter.Start(ctx, emitter); err != nil {
				r.logger.Error("failed to start node", "id", nodeID, "error", err)
			}
		}
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Close all node channels to signal shutdown
	r.mu.Lock()
	for _, ch := range r.nodeChans {
		close(ch)
	}
	r.mu.Unlock()

	// Wait for all node goroutines to finish
	wg.Wait()

	// Stop all nodes
	for nodeID, n := range r.nodes {
		if err := n.Stop(context.Background()); err != nil {
			r.logger.Error("failed to stop node", "id", nodeID, "error", err)
		}
	}

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	return nil
}

// runNode processes messages for a single node.
func (r *Runtime) runNode(ctx context.Context, nodeID string, n node.Node) {
	ch := r.nodeChans[nodeID]
	emitter := r.createEmitter(nodeID)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := n.Process(ctx, msg, emitter); err != nil {
				r.logger.Error("node processing error",
					"node", nodeID,
					"message", msg.ID,
					"error", err,
				)
			}
		}
	}
}

// createEmitter creates an emitter for a node.
func (r *Runtime) createEmitter(nodeID string) node.Emitter {
	return &emitter{
		runtime: r,
		nodeID:  nodeID,
	}
}

type emitter struct {
	runtime *Runtime
	nodeID  string
}

func (e *emitter) Emit(output string, msg *message.Message) {
	if output == "" {
		output = "default"
	}

	e.runtime.mu.RLock()
	wires := e.runtime.wires[e.nodeID]
	e.runtime.mu.RUnlock()

	for _, w := range wires {
		if w.output == output {
			e.runtime.mu.RLock()
			ch, ok := e.runtime.nodeChans[w.toNode]
			e.runtime.mu.RUnlock()

			if ok {
				// Clone message for each destination
				select {
				case ch <- msg.Clone():
				default:
					e.runtime.logger.Warn("message dropped (buffer full)",
						"from", e.nodeID,
						"to", w.toNode,
					)
				}
			}
		}
	}
}

// Stop stops the runtime.
func (r *Runtime) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancelFn != nil {
		r.cancelFn()
	}
}

// Shutdown shuts down the runtime and all plugins.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.Stop()
	return r.plugins.Shutdown(ctx)
}
