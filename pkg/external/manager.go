package external

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bherbruck/vibeflow/pkg/node"
)

// Manager manages external node plugins.
type Manager struct {
	logger *slog.Logger

	mu      sync.RWMutex
	hosts   map[string]*Host    // path -> host
	types   map[string]typeInfo // nodeType -> typeInfo
}

type typeInfo struct {
	host     *Host
	info     NodeTypeInfo
}

// NewManager creates a new plugin manager.
func NewManager() *Manager {
	return &Manager{
		logger: slog.Default(),
		hosts:  make(map[string]*Host),
		types:  make(map[string]typeInfo),
	}
}

// LoadPlugin loads a plugin from the given path and registers its node types.
func (m *Manager) LoadPlugin(ctx context.Context, path string, args ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already loaded
	if _, ok := m.hosts[path]; ok {
		return nil
	}

	// Create and start host
	host := NewHost(path, args...)
	if err := host.Start(ctx); err != nil {
		return fmt.Errorf("failed to start plugin %s: %w", path, err)
	}

	// Get manifest
	manifest, err := host.GetManifest(ctx)
	if err != nil {
		host.Kill()
		return fmt.Errorf("failed to get manifest from %s: %w", path, err)
	}

	m.logger.Info("loaded plugin",
		"path", path,
		"name", manifest.Name,
		"version", manifest.Version,
		"types", len(manifest.NodeTypes),
	)

	// Register host and types
	m.hosts[path] = host

	for _, nodeType := range manifest.NodeTypes {
		m.types[nodeType.Type] = typeInfo{
			host: host,
			info: nodeType,
		}
		m.logger.Debug("registered external node type",
			"type", nodeType.Type,
			"name", nodeType.Name,
		)
	}

	return nil
}

// HasType checks if a node type is registered.
func (m *Manager) HasType(nodeType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.types[nodeType]
	return ok
}

// CreateNode creates a proxy node for an external node type.
func (m *Manager) CreateNode(nodeType string) (node.Node, error) {
	m.mu.RLock()
	info, ok := m.types[nodeType]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown external node type: %s", nodeType)
	}

	return NewProxyNode(info.host, nodeType, info.info.HasStart), nil
}

// Types returns all registered external node types.
func (m *Manager) Types() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	types := make([]string, 0, len(m.types))
	for t := range m.types {
		types = append(types, t)
	}
	return types
}

// Shutdown shuts down all plugins.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for path, host := range m.hosts {
		if err := host.Shutdown(ctx); err != nil {
			m.logger.Error("failed to shutdown plugin", "path", path, "error", err)
			lastErr = err
		}
	}

	m.hosts = make(map[string]*Host)
	m.types = make(map[string]typeInfo)

	return lastErr
}
