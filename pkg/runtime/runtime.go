// Package runtime provides the flow execution engine.
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bherbruck/vibeflow/pkg/events"
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
	events   *events.Bus

	mu       sync.RWMutex
	flowID   string // Current flow ID
	nodes    map[string]node.Node
	wires    map[string][]wire // nodeID -> outgoing wires
	running  bool
	cancelFn context.CancelFunc

	// Message channels for each node - carries both message and input port
	nodeChans map[string]chan *routedMessage
}

// routedMessage wraps a message with routing info
type routedMessage struct {
	msg       *message.Message
	inputPort string
}

type wire struct {
	output string // Source output port
	toNode string // Target node ID
	input  string // Target input port
}

// inputHandle implements node.Input
type inputHandle struct {
	name string
}

func (i *inputHandle) Name() string { return i.name }

// inputSet implements node.Inputs
type inputSet struct {
	ports map[string]*inputHandle
}

func (s *inputSet) Get(name string) (node.Input, error) {
	if h, ok := s.ports[name]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("input port %q not wired", name)
}

func (s *inputSet) Has(name string) bool {
	_, ok := s.ports[name]
	return ok
}

// outputHandle implements node.Output
type outputHandle struct {
	runtime *Runtime
	nodeID  string
	port    string
}

func (o *outputHandle) Send(msg *message.Message) {
	o.runtime.mu.RLock()
	wires := o.runtime.wires[o.nodeID]
	flowID := o.runtime.flowID
	o.runtime.mu.RUnlock()

	// Emit the emit event
	o.runtime.emitEvent(events.Event{
		Type:   events.EventNodeEmit,
		FlowID: flowID,
		NodeID: o.nodeID,
		Data: &events.ProcessData{
			MessageID: msg.ID,
			Port:      o.port,
			Payload:   msg.Payload,
		},
	})

	// If there's only one wire total from this node, route to it regardless of port name
	singleWire := len(wires) == 1

	for _, w := range wires {
		if w.output == o.port || singleWire {
			o.runtime.mu.RLock()
			ch, ok := o.runtime.nodeChans[w.toNode]
			o.runtime.mu.RUnlock()

			if ok {
				select {
				case ch <- &routedMessage{msg: msg.Clone(), inputPort: w.input}:
				default:
					o.runtime.logger.Warn("message dropped (buffer full)",
						"from", o.nodeID,
						"to", w.toNode,
					)
				}
			}
		}
	}
}

// outputSet implements node.Outputs
type outputSet struct {
	runtime *Runtime
	nodeID  string
	ports   map[string]*outputHandle
}

func (s *outputSet) Get(name string) (node.Output, error) {
	if h, ok := s.ports[name]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("output port %q not wired", name)
}

func (s *outputSet) Has(name string) bool {
	_, ok := s.ports[name]
	return ok
}

// Option configures a Runtime.
type Option func(*Runtime)

// WithEventBus sets the event bus for the runtime.
func WithEventBus(bus *events.Bus) Option {
	return func(r *Runtime) {
		r.events = bus
	}
}

// WithLogger sets the logger for the runtime.
func WithLogger(logger *slog.Logger) Option {
	return func(r *Runtime) {
		r.logger = logger
	}
}

// New creates a new runtime.
func New(registry *node.Registry, opts ...Option) *Runtime {
	if registry == nil {
		registry = node.DefaultRegistry
	}
	r := &Runtime{
		registry:  registry,
		plugins:   external.NewManager(),
		logger:    slog.Default(),
		nodes:     make(map[string]node.Node),
		wires:     make(map[string][]wire),
		nodeChans: make(map[string]chan *routedMessage),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// SetFlowID sets the flow ID for event correlation.
func (r *Runtime) SetFlowID(id string) {
	r.mu.Lock()
	r.flowID = id
	r.mu.Unlock()
}

// LoadPlugin loads an external node plugin.
func (r *Runtime) LoadPlugin(ctx context.Context, path string, args ...string) error {
	return r.plugins.LoadPlugin(ctx, path, args...)
}

// Load loads a flow and initializes all nodes.
func (r *Runtime) Load(ctx context.Context, f *flow.Flow) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Build wire map FIRST (deduplicate wires with same from/output/to/input)
	// This is needed to build input/output sets for each node
	seenWires := make(map[string]bool)
	nodeInputs := make(map[string]map[string]bool)  // nodeID -> set of input port names
	nodeOutputs := make(map[string]map[string]bool) // nodeID -> set of output port names

	for _, w := range f.Wires {
		output := w.Output
		// Normalize port names - "output" and empty string both map to "default"
		if output == "" || output == "output" {
			output = "default"
		}
		input := w.Input
		// Normalize port names - "input" and empty string both map to "default"
		if input == "" || input == "input" {
			input = "default"
		}

		// Deduplicate: skip if we've seen this exact wire before
		wireKey := fmt.Sprintf("%s:%s->%s:%s", w.From, output, w.To, input)
		if seenWires[wireKey] {
			r.logger.Warn("skipping duplicate wire", "from", w.From, "output", output, "to", w.To, "input", input)
			continue
		}
		seenWires[wireKey] = true

		r.wires[w.From] = append(r.wires[w.From], wire{
			output: output,
			toNode: w.To,
			input:  input,
		})

		// Track which ports are wired for each node
		if nodeOutputs[w.From] == nil {
			nodeOutputs[w.From] = make(map[string]bool)
		}
		nodeOutputs[w.From][output] = true

		if nodeInputs[w.To] == nil {
			nodeInputs[w.To] = make(map[string]bool)
		}
		nodeInputs[w.To][input] = true
	}

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

		// Build input set for this node
		inputs := &inputSet{ports: make(map[string]*inputHandle)}
		for portName := range nodeInputs[nodeDef.ID] {
			inputs.ports[portName] = &inputHandle{name: portName}
		}

		// Build output set for this node
		outputs := &outputSet{
			runtime: r,
			nodeID:  nodeDef.ID,
			ports:   make(map[string]*outputHandle),
		}
		for portName := range nodeOutputs[nodeDef.ID] {
			outputs.ports[portName] = &outputHandle{
				runtime: r,
				nodeID:  nodeDef.ID,
				port:    portName,
			}
		}

		// Initialize node
		cfg := &node.Config{
			ID:     nodeDef.ID,
			Type:   nodeDef.Type,
			Name:   nodeDef.Name,
			Config: nodeDef.Config,
		}

		if err := n.Init(ctx, cfg, inputs, outputs); err != nil {
			// Node init failed - log warning and skip this node (don't fail the whole flow)
			r.logger.Warn("failed to init node, skipping", "id", nodeDef.ID, "type", nodeDef.Type, "error", err)
			continue
		}

		r.nodes[nodeDef.ID] = n
		r.nodeChans[nodeDef.ID] = make(chan *routedMessage, 100)

		r.logger.Info("initialized node", "id", nodeDef.ID, "type", nodeDef.Type)

		// Emit node init event
		r.emitEvent(events.Event{
			Type:   events.EventNodeInit,
			FlowID: r.flowID,
			NodeID: nodeDef.ID,
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
	flowID := r.flowID

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFn = cancel
	r.mu.Unlock()

	// Emit flow start event
	r.emitEvent(events.Event{
		Type:   events.EventFlowStart,
		FlowID: flowID,
	})

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
			if err := starter.Start(ctx); err != nil {
				r.logger.Error("failed to start node", "id", nodeID, "error", err)
				r.emitEvent(events.Event{
					Type:   events.EventNodeError,
					FlowID: flowID,
					NodeID: nodeID,
					Data:   &events.ErrorData{Error: err, Message: err.Error()},
				})
			} else {
				r.emitEvent(events.Event{
					Type:   events.EventNodeStart,
					FlowID: flowID,
					NodeID: nodeID,
				})
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
		r.emitEvent(events.Event{
			Type:   events.EventNodeStop,
			FlowID: flowID,
			NodeID: nodeID,
		})
	}

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	// Emit flow stop event
	r.emitEvent(events.Event{
		Type:   events.EventFlowStop,
		FlowID: flowID,
	})

	return nil
}

// runNode processes messages for a single node.
func (r *Runtime) runNode(ctx context.Context, nodeID string, n node.Node) {
	ch := r.nodeChans[nodeID]

	r.mu.RLock()
	flowID := r.flowID
	r.mu.RUnlock()

	for {
		select {
		case <-ctx.Done():
			return
		case routed, ok := <-ch:
			if !ok {
				return
			}

			// Emit process event
			r.emitEvent(events.Event{
				Type:   events.EventNodeProcess,
				FlowID: flowID,
				NodeID: nodeID,
				Data: &events.ProcessData{
					MessageID: routed.msg.ID,
					Port:      routed.inputPort,
					Payload:   routed.msg.Payload,
				},
			})

			if err := n.Process(ctx, routed.msg, routed.inputPort); err != nil {
				r.logger.Error("node processing error",
					"node", nodeID,
					"message", routed.msg.ID,
					"error", err,
				)
				r.emitEvent(events.Event{
					Type:   events.EventNodeError,
					FlowID: flowID,
					NodeID: nodeID,
					Data:   &events.ErrorData{Error: err, Message: err.Error()},
				})
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

// emitEvent sends an event to the event bus if configured.
func (r *Runtime) emitEvent(e events.Event) {
	if r.events != nil {
		r.events.Emit(e)
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
	flowID := e.runtime.flowID
	e.runtime.mu.RUnlock()

	// Emit the emit event
	e.runtime.emitEvent(events.Event{
		Type:   events.EventNodeEmit,
		FlowID: flowID,
		NodeID: e.nodeID,
		Data: &events.ProcessData{
			MessageID: msg.ID,
			Port:      output,
			Payload:   msg.Payload,
		},
	})

	// If there's only one wire total from this node, route to it regardless of port name
	// This lets node developers use any output name when they only have one output
	singleWire := len(wires) == 1

	for _, w := range wires {
		if w.output == output || singleWire {
			e.runtime.mu.RLock()
			ch, ok := e.runtime.nodeChans[w.toNode]
			e.runtime.mu.RUnlock()

			if ok {
				// Clone message for each destination
				select {
				case ch <- &routedMessage{msg: msg.Clone(), inputPort: w.input}:
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
