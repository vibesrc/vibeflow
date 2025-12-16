package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bherbruck/vibeflow/pkg/events"
	"github.com/bherbruck/vibeflow/pkg/flow"
	"github.com/bherbruck/vibeflow/pkg/node"
	"github.com/bherbruck/vibeflow/pkg/runtime"
)

// Engine manages multiple flows and their plugins.
type Engine struct {
	store     *Store
	logger    *slog.Logger
	pluginDir string
	dataDir   string
	events    *events.Bus

	mu        sync.RWMutex
	runtimes  map[string]*flowInstance
	plugins   *PluginManager
	cancelFns map[string]context.CancelFunc
	doneChs   map[string]chan struct{} // Signals when flow has fully stopped
}

type flowInstance struct {
	runtime *runtime.Runtime
	flow    *flow.Flow
	record  *FlowRecord
}

// EngineConfig configures the engine.
type EngineConfig struct {
	Store     *Store
	PluginDir string      // Directory for plugins
	DataDir   string      // Directory for runtime data
	Events    *events.Bus // Event bus for real-time events
}

// NewEngine creates a new flow engine.
func NewEngine(cfg *EngineConfig) *Engine {
	return &Engine{
		store:     cfg.Store,
		logger:    slog.Default(),
		pluginDir: cfg.PluginDir,
		dataDir:   cfg.DataDir,
		events:    cfg.Events,
		runtimes:  make(map[string]*flowInstance),
		plugins:   NewPluginManager(cfg.Store, cfg.PluginDir),
		cancelFns: make(map[string]context.CancelFunc),
		doneChs:   make(map[string]chan struct{}),
	}
}

// Events returns the event bus.
func (e *Engine) Events() *events.Bus {
	return e.events
}

// Start initializes the engine and starts enabled flows.
func (e *Engine) Start(ctx context.Context) error {
	// Load plugins from directory
	if e.pluginDir != "" {
		if err := e.plugins.LoadFromDirectory(ctx); err != nil {
			e.logger.Error("failed to load plugins", "error", err)
		}
	}

	// Load and start enabled flows
	flows, err := e.store.ListFlows()
	if err != nil {
		return fmt.Errorf("failed to list flows: %w", err)
	}

	for _, record := range flows {
		if record.Enabled {
			if err := e.StartFlow(ctx, record.ID); err != nil {
				e.logger.Error("failed to start flow", "id", record.ID, "error", err)
				e.store.UpdateFlowStatus(record.ID, "error")
			}
		}
	}

	return nil
}

// LoadPlugin loads a specific plugin by path.
func (e *Engine) LoadPlugin(ctx context.Context, path string) error {
	key := PluginKey{Name: path} // Use path as name for direct loads
	return e.plugins.loadPlugin(ctx, key, path)
}

// StartFlow starts a flow by ID.
func (e *Engine) StartFlow(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if already running
	if _, running := e.runtimes[id]; running {
		return fmt.Errorf("flow %s is already running", id)
	}

	// Get flow record
	record, err := e.store.GetFlow(id)
	if err != nil {
		return fmt.Errorf("failed to get flow: %w", err)
	}
	if record == nil {
		return fmt.Errorf("flow not found: %s", id)
	}

	// Parse flow YAML
	f, err := flow.Parse([]byte(record.Content))
	if err != nil {
		return fmt.Errorf("failed to parse flow: %w", err)
	}

	// Ensure all required node types are available (auto-download if needed)
	for _, nodeDef := range f.Nodes {
		if !nodeDef.IsEnabled() {
			continue
		}

		// Skip built-in types
		if node.DefaultRegistry.Has(nodeDef.Type) {
			continue
		}

		// Ensure plugin is available
		if err := e.plugins.EnsureType(ctx, nodeDef.Type); err != nil {
			return fmt.Errorf("failed to ensure node type %s: %w", nodeDef.Type, err)
		}
	}

	// Create runtime with event bus
	rt := runtime.New(node.DefaultRegistry, runtime.WithEventBus(e.events))
	rt.SetFlowID(id)

	// Create cancellable context for this flow
	// Use Background() so the flow isn't cancelled when the HTTP request completes
	flowCtx, cancel := context.WithCancel(context.Background())
	e.cancelFns[id] = cancel

	// Create done channel to signal when flow has fully stopped
	doneCh := make(chan struct{})
	e.doneChs[id] = doneCh

	// Load flow into runtime (it will use our plugin manager for external types)
	if err := e.loadFlowWithPlugins(flowCtx, rt, f); err != nil {
		cancel()
		delete(e.doneChs, id)
		return fmt.Errorf("failed to load flow into runtime: %w", err)
	}

	// Store instance
	e.runtimes[id] = &flowInstance{
		runtime: rt,
		flow:    f,
		record:  record,
	}

	// Run flow in background
	go func() {
		e.logger.Info("starting flow", "id", id, "name", f.Metadata.Name)
		e.store.UpdateFlowStatus(id, "running")

		if err := rt.Run(flowCtx); err != nil {
			e.logger.Error("flow error", "id", id, "error", err)
			e.store.UpdateFlowStatus(id, "error")
		} else {
			e.store.UpdateFlowStatus(id, "stopped")
		}

		e.mu.Lock()
		delete(e.runtimes, id)
		delete(e.cancelFns, id)
		delete(e.doneChs, id)
		e.mu.Unlock()

		// Signal that flow has fully stopped
		close(doneCh)
	}()

	return nil
}

// loadFlowWithPlugins loads a flow using the engine's plugin manager for external types.
func (e *Engine) loadFlowWithPlugins(ctx context.Context, rt *runtime.Runtime, f *flow.Flow) error {
	// We need to intercept node creation to use our plugin manager
	// For now, let the runtime handle built-in types and we'll add external support
	return rt.Load(ctx, f)
}

// StopFlow stops a running flow and returns a channel that closes when the flow has fully stopped.
func (e *Engine) StopFlow(id string) (<-chan struct{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	cancel, ok := e.cancelFns[id]
	if !ok {
		return nil, fmt.Errorf("flow %s is not running", id)
	}

	doneCh := e.doneChs[id]

	e.logger.Info("stopping flow", "id", id)
	cancel()

	return doneCh, nil
}

// RestartFlow restarts a flow, waiting for the old instance to fully stop first.
func (e *Engine) RestartFlow(ctx context.Context, id string) error {
	// Stop if running and wait for it to fully stop
	e.mu.RLock()
	_, running := e.runtimes[id]
	e.mu.RUnlock()

	if running {
		doneCh, err := e.StopFlow(id)
		if err != nil {
			return err
		}
		// Wait for the flow to fully stop before starting the new one
		<-doneCh
	}

	return e.StartFlow(ctx, id)
}

// GetFlowStatus returns the status of a flow.
func (e *Engine) GetFlowStatus(id string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, running := e.runtimes[id]; running {
		return "running"
	}
	return "stopped"
}

// IsFlowRunning checks if a flow is running.
func (e *Engine) IsFlowRunning(id string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, running := e.runtimes[id]
	return running
}

// ListRunningFlows returns IDs of running flows.
func (e *Engine) ListRunningFlows() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.runtimes))
	for id := range e.runtimes {
		ids = append(ids, id)
	}
	return ids
}

// Shutdown stops all flows and cleans up.
func (e *Engine) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	// Copy cancel functions to avoid holding lock during cancellation
	cancels := make([]context.CancelFunc, 0, len(e.cancelFns))
	for _, cancel := range e.cancelFns {
		cancels = append(cancels, cancel)
	}
	e.mu.Unlock()

	// Cancel all flows
	for _, cancel := range cancels {
		cancel()
	}

	// Shutdown plugin manager
	return e.plugins.Shutdown(ctx)
}

// AvailableNodeTypes returns all available node types.
func (e *Engine) AvailableNodeTypes() []string {
	types := node.DefaultRegistry.Types()
	types = append(types, e.plugins.Types()...)
	return types
}

// Plugins returns the plugin manager.
func (e *Engine) Plugins() *PluginManager {
	return e.plugins
}

// GetNodeErrors returns the current node errors for a running flow.
func (e *Engine) GetNodeErrors(id string) map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	instance, ok := e.runtimes[id]
	if !ok {
		return nil
	}
	return instance.runtime.NodeErrors()
}

// GetAllNodeErrors returns current node errors for all running flows.
// Returns map[flowID]map[nodeID]errorMessage
func (e *Engine) GetAllNodeErrors() map[string]map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]map[string]string)
	for id, instance := range e.runtimes {
		errors := instance.runtime.NodeErrors()
		if len(errors) > 0 {
			result[id] = errors
		}
	}
	return result
}

// GetAllVariables returns current variables from all running flows.
// Returns map[flowID]map[key]value where key includes scope prefix.
func (e *Engine) GetAllVariables() map[string]map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]map[string]any)
	for id, instance := range e.runtimes {
		vars := instance.runtime.Variables()
		if len(vars) > 0 {
			result[id] = vars
		}
	}
	return result
}
