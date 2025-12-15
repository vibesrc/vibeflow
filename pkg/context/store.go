// Package context provides state management for flows and nodes.
package context

// Store is the interface for context storage backends.
// Implementations must be safe for concurrent use.
type Store interface {
	// Get retrieves a value by key. Returns (nil, false) if not found.
	Get(key string) (any, bool)

	// Set stores a value by key.
	Set(key string, value any)

	// Delete removes a key.
	Delete(key string)

	// Keys returns all keys with the given prefix.
	Keys(prefix string) []string

	// Clear removes all keys with the given prefix.
	Clear(prefix string)
}

// FlowContext provides scoped access to flow-level state.
// State is shared across all nodes in a flow.
type FlowContext struct {
	store  Store
	flowID string
}

// NewFlowContext creates a new flow context.
func NewFlowContext(store Store, flowID string) *FlowContext {
	return &FlowContext{store: store, flowID: flowID}
}

func (f *FlowContext) prefix() string {
	return "flow:" + f.flowID + ":"
}

// Get retrieves a value from flow context.
func (f *FlowContext) Get(key string) (any, bool) {
	return f.store.Get(f.prefix() + key)
}

// Set stores a value in flow context.
func (f *FlowContext) Set(key string, value any) {
	f.store.Set(f.prefix()+key, value)
}

// Delete removes a key from flow context.
func (f *FlowContext) Delete(key string) {
	f.store.Delete(f.prefix() + key)
}

// Keys returns all keys in this flow's context.
func (f *FlowContext) Keys() []string {
	prefix := f.prefix()
	keys := f.store.Keys(prefix)
	// Strip prefix from keys
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = k[len(prefix):]
	}
	return result
}

// Clear removes all state for this flow.
func (f *FlowContext) Clear() {
	f.store.Clear(f.prefix())
}

// NodeContext provides scoped access to node-level state.
// State is specific to a single node instance.
type NodeContext struct {
	store  Store
	flowID string
	nodeID string
}

// NewNodeContext creates a new node context.
func NewNodeContext(store Store, flowID, nodeID string) *NodeContext {
	return &NodeContext{store: store, flowID: flowID, nodeID: nodeID}
}

func (n *NodeContext) prefix() string {
	return "node:" + n.flowID + ":" + n.nodeID + ":"
}

// Get retrieves a value from node context.
func (n *NodeContext) Get(key string) (any, bool) {
	return n.store.Get(n.prefix() + key)
}

// Set stores a value in node context.
func (n *NodeContext) Set(key string, value any) {
	n.store.Set(n.prefix()+key, value)
}

// Delete removes a key from node context.
func (n *NodeContext) Delete(key string) {
	n.store.Delete(n.prefix() + key)
}

// Keys returns all keys in this node's context.
func (n *NodeContext) Keys() []string {
	prefix := n.prefix()
	keys := n.store.Keys(prefix)
	// Strip prefix from keys
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = k[len(prefix):]
	}
	return result
}

// Clear removes all state for this node.
func (n *NodeContext) Clear() {
	n.store.Clear(n.prefix())
}

// GlobalContext provides access to global state.
// State is shared across all flows.
type GlobalContext struct {
	store Store
}

// NewGlobalContext creates a new global context.
func NewGlobalContext(store Store) *GlobalContext {
	return &GlobalContext{store: store}
}

func (g *GlobalContext) prefix() string {
	return "global:"
}

// Get retrieves a value from global context.
func (g *GlobalContext) Get(key string) (any, bool) {
	return g.store.Get(g.prefix() + key)
}

// Set stores a value in global context.
func (g *GlobalContext) Set(key string, value any) {
	g.store.Set(g.prefix()+key, value)
}

// Delete removes a key from global context.
func (g *GlobalContext) Delete(key string) {
	g.store.Delete(g.prefix() + key)
}

// Keys returns all keys in global context.
func (g *GlobalContext) Keys() []string {
	prefix := g.prefix()
	keys := g.store.Keys(prefix)
	// Strip prefix from keys
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = k[len(prefix):]
	}
	return result
}

// Clear removes all global state.
func (g *GlobalContext) Clear() {
	g.store.Clear(g.prefix())
}
