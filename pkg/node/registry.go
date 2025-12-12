package node

import (
	"fmt"
	"sync"
)

// Factory creates new node instances.
type Factory func() Node

// registeredType holds a factory and optional type info.
type registeredType struct {
	factory Factory
	info    *TypeInfo
}

// Registry holds registered node factories.
type Registry struct {
	mu    sync.RWMutex
	types map[string]registeredType
}

// NewRegistry creates a new node registry.
func NewRegistry() *Registry {
	return &Registry{
		types: make(map[string]registeredType),
	}
}

// Register adds a node factory to the registry.
func (r *Registry) Register(nodeType string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.types[nodeType] = registeredType{factory: factory}
}

// RegisterWithInfo adds a node factory with type info to the registry.
func (r *Registry) RegisterWithInfo(nodeType string, factory Factory, info TypeInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.types[nodeType] = registeredType{factory: factory, info: &info}
}

// Create creates a new node instance of the given type.
func (r *Registry) Create(nodeType string) (Node, error) {
	r.mu.RLock()
	rt, ok := r.types[nodeType]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown node type: %s", nodeType)
	}

	return rt.factory(), nil
}

// Has returns true if the registry has a factory for the given type.
func (r *Registry) Has(nodeType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.types[nodeType]
	return ok
}

// Types returns all registered node types.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.types))
	for t := range r.types {
		types = append(types, t)
	}
	return types
}

// TypeInfos returns type info for all registered node types.
func (r *Registry) TypeInfos() []TypeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]TypeInfo, 0, len(r.types))
	for nodeType, rt := range r.types {
		if rt.info != nil {
			infos = append(infos, *rt.info)
		} else {
			// Return basic info for nodes without full type info
			infos = append(infos, TypeInfo{
				Type:    nodeType,
				Name:    nodeType,
				Inputs:  []PortInfo{{Name: "input", Description: "Default input"}},
				Outputs: []PortInfo{{Name: "output", Description: "Default output"}},
			})
		}
	}
	return infos
}

// GetTypeInfo returns type info for a specific node type.
func (r *Registry) GetTypeInfo(nodeType string) *TypeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rt, ok := r.types[nodeType]
	if !ok {
		return nil
	}
	return rt.info
}

// DefaultRegistry is the global node registry.
var DefaultRegistry = NewRegistry()

// Register adds a node factory to the default registry.
func Register(nodeType string, factory Factory) {
	DefaultRegistry.Register(nodeType, factory)
}

// RegisterWithInfo adds a node factory with type info to the default registry.
func RegisterWithInfo(nodeType string, factory Factory, info TypeInfo) {
	DefaultRegistry.RegisterWithInfo(nodeType, factory, info)
}
