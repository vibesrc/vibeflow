package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/bherbruck/vibeflow/pkg/external"
	"github.com/bherbruck/vibeflow/pkg/node"
)

// PluginManager manages the global plugin palette with versioning support.
type PluginManager struct {
	store     *Store
	logger    *slog.Logger
	pluginDir string // Base directory for plugins

	mu       sync.RWMutex
	hosts    map[string]*external.Host // "name@version" -> host
	types    map[string]*pluginType    // "vendor.nodetype" -> pluginType
	registry map[string]string         // "prefix" -> "github:owner/repo" (plugin registry)
}

// pluginType tracks which plugin provides a node type
type pluginType struct {
	host       *external.Host
	pluginKey  string // "name@version"
	info       external.NodeTypeInfo
}

// PluginKey represents a versioned plugin identifier
type PluginKey struct {
	Name    string
	Version string
}

func (k PluginKey) String() string {
	if k.Version == "" {
		return k.Name
	}
	return fmt.Sprintf("%s@%s", k.Name, k.Version)
}

// ParsePluginKey parses "name@version" or "name"
func ParsePluginKey(s string) PluginKey {
	parts := strings.SplitN(s, "@", 2)
	key := PluginKey{Name: parts[0]}
	if len(parts) > 1 {
		key.Version = parts[1]
	}
	return key
}

// NewPluginManager creates a new plugin manager.
func NewPluginManager(store *Store, pluginDir string) *PluginManager {
	pm := &PluginManager{
		store:     store,
		logger:    slog.Default(),
		pluginDir: pluginDir,
		hosts:     make(map[string]*external.Host),
		types:     make(map[string]*pluginType),
		registry:  make(map[string]string),
	}

	// Default plugin registry - maps node type prefixes to GitHub repos
	// Users can extend this via settings or a registry file
	pm.registry["modbus"] = "github:vibeflow/vibeflow-modbus"
	pm.registry["http"] = "github:vibeflow/vibeflow-http"
	pm.registry["mqtt"] = "github:vibeflow/vibeflow-mqtt"
	pm.registry["tcp"] = "github:vibeflow/vibeflow-tcp"
	pm.registry["udp"] = "github:vibeflow/vibeflow-udp"
	pm.registry["file"] = "github:vibeflow/vibeflow-file"
	pm.registry["sql"] = "github:vibeflow/vibeflow-sql"
	pm.registry["redis"] = "github:vibeflow/vibeflow-redis"

	return pm
}

// RegisterSource registers a node type prefix to a plugin source.
func (m *PluginManager) RegisterSource(prefix, source string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registry[prefix] = source
}

// LoadFromDirectory loads all plugins from the plugin directory.
func (m *PluginManager) LoadFromDirectory(ctx context.Context) error {
	if m.pluginDir == "" {
		return nil
	}

	// Plugin directory structure:
	// plugins/
	//   name/
	//     v1.0.0/
	//       plugin (executable)
	//     v2.0.0/
	//       plugin

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginName := entry.Name()
		pluginPath := filepath.Join(m.pluginDir, pluginName)

		// Check for versioned subdirectories
		versionDirs, err := os.ReadDir(pluginPath)
		if err != nil {
			continue
		}

		for _, vdir := range versionDirs {
			if !vdir.IsDir() {
				// Could be a single executable (no versioning)
				if isExecutable(filepath.Join(pluginPath, vdir.Name())) {
					key := PluginKey{Name: pluginName}
					if err := m.loadPlugin(ctx, key, filepath.Join(pluginPath, vdir.Name())); err != nil {
						m.logger.Error("failed to load plugin", "name", pluginName, "error", err)
					}
				}
				continue
			}

			version := vdir.Name()
			execPath := filepath.Join(pluginPath, version, "plugin")

			// Try platform-specific executable name
			if runtime.GOOS == "windows" {
				execPath = filepath.Join(pluginPath, version, "plugin.exe")
			}

			if !isExecutable(execPath) {
				continue
			}

			key := PluginKey{Name: pluginName, Version: version}
			if err := m.loadPlugin(ctx, key, execPath); err != nil {
				m.logger.Error("failed to load plugin", "key", key.String(), "error", err)
			}
		}
	}

	return nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// On Unix, check execute permission
	if runtime.GOOS != "windows" {
		return info.Mode()&0111 != 0
	}
	return true
}

// LoadPlugin loads a single plugin executable.
func (m *PluginManager) LoadPlugin(ctx context.Context, key PluginKey, execPath string) error {
	return m.loadPlugin(ctx, key, execPath)
}

// loadPlugin loads a single plugin executable (internal).
func (m *PluginManager) loadPlugin(ctx context.Context, key PluginKey, execPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyStr := key.String()

	// Check if already loaded
	if _, ok := m.hosts[keyStr]; ok {
		return nil
	}

	// Create and start host
	host := external.NewHost(execPath)
	if err := host.Start(ctx); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	// Get manifest
	manifest, err := host.GetManifest(ctx)
	if err != nil {
		host.Kill()
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	m.logger.Info("loaded plugin",
		"key", keyStr,
		"name", manifest.Name,
		"version", manifest.Version,
		"types", len(manifest.NodeTypes),
	)

	m.hosts[keyStr] = host

	// Register node types
	for _, nodeType := range manifest.NodeTypes {
		m.types[nodeType.Type] = &pluginType{
			host:      host,
			pluginKey: keyStr,
			info:      nodeType,
		}
	}

	// Save to store
	if m.store != nil {
		m.store.SavePlugin(&PluginRecord{
			Name:    key.Name,
			Source:  execPath,
			Version: key.Version,
		})
	}

	return nil
}

// InstallFromGitHub downloads and installs a plugin from GitHub releases.
func (m *PluginManager) InstallFromGitHub(ctx context.Context, owner, repo, version string) error {
	if version == "" {
		version = "latest"
	}

	// Determine platform
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// GitHub release asset naming convention
	assetName := fmt.Sprintf("%s_%s_%s", repo, goos, goarch)
	if goos == "windows" {
		assetName += ".exe"
	}

	// Download URL
	var downloadURL string
	if version == "latest" {
		downloadURL = fmt.Sprintf(
			"https://github.com/%s/%s/releases/latest/download/%s",
			owner, repo, assetName,
		)
	} else {
		downloadURL = fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/%s/%s",
			owner, repo, version, assetName,
		)
	}

	m.logger.Info("downloading plugin", "url", downloadURL)

	// Create plugin directory
	pluginName := fmt.Sprintf("%s-%s", owner, repo)
	pluginPath := filepath.Join(m.pluginDir, pluginName, version)
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	execPath := filepath.Join(pluginPath, "plugin")
	if goos == "windows" {
		execPath += ".exe"
	}

	// Download
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Write to file
	out, err := os.Create(execPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Calculate checksum while downloading
	hash := sha256.New()
	writer := io.MultiWriter(out, hash)

	_, err = io.Copy(writer, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(execPath)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Make executable
	if err := os.Chmod(execPath, 0755); err != nil {
		os.Remove(execPath)
		return fmt.Errorf("failed to make executable: %w", err)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))

	// Load the plugin
	key := PluginKey{Name: pluginName, Version: version}
	if err := m.loadPlugin(ctx, key, execPath); err != nil {
		os.Remove(execPath)
		return err
	}

	// Update checksum in store
	if m.store != nil {
		m.store.SavePlugin(&PluginRecord{
			Name:     pluginName,
			Source:   fmt.Sprintf("github:%s/%s", owner, repo),
			Version:  version,
			Checksum: checksum,
		})
	}

	return nil
}

// HasType checks if a node type is available.
func (m *PluginManager) HasType(nodeType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.types[nodeType]
	return ok
}

// EnsureType ensures a node type is available, auto-downloading if needed.
func (m *PluginManager) EnsureType(ctx context.Context, nodeType string) error {
	// Check if already available
	if m.HasType(nodeType) {
		return nil
	}

	// Extract prefix (e.g., "modbus" from "modbus.read")
	prefix := strings.Split(nodeType, ".")[0]

	// Look up in registry
	m.mu.RLock()
	source, ok := m.registry[prefix]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown node type: %s (no plugin registered for prefix %q)", nodeType, prefix)
	}

	// Parse source
	if strings.HasPrefix(source, "github:") {
		parts := strings.TrimPrefix(source, "github:")
		ownerRepo := strings.Split(parts, "/")
		if len(ownerRepo) != 2 {
			return fmt.Errorf("invalid GitHub source: %s", source)
		}

		m.logger.Info("auto-downloading plugin for node type",
			"nodeType", nodeType,
			"source", source,
		)

		return m.InstallFromGitHub(ctx, ownerRepo[0], ownerRepo[1], "latest")
	}

	return fmt.Errorf("unsupported plugin source: %s", source)
}

// CreateNode creates a proxy node for an external node type.
func (m *PluginManager) CreateNode(nodeType string) (node.Node, error) {
	m.mu.RLock()
	pt, ok := m.types[nodeType]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown node type: %s", nodeType)
	}

	return external.NewProxyNode(pt.host, nodeType, pt.info.HasStart), nil
}

// Types returns all available node types from plugins.
func (m *PluginManager) Types() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	types := make([]string, 0, len(m.types))
	for t := range m.types {
		types = append(types, t)
	}
	return types
}

// TypeInfos returns full type info for all plugin node types.
func (m *PluginManager) TypeInfos() []external.NodeTypeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]external.NodeTypeInfo, 0, len(m.types))
	for _, pt := range m.types {
		infos = append(infos, pt.info)
	}
	return infos
}

// InstalledPlugins returns info about installed plugins.
func (m *PluginManager) InstalledPlugins() []PluginKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]PluginKey, 0, len(m.hosts))
	for keyStr := range m.hosts {
		keys = append(keys, ParsePluginKey(keyStr))
	}
	return keys
}

// Shutdown stops all plugin processes.
func (m *PluginManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for key, host := range m.hosts {
		if err := host.Shutdown(ctx); err != nil {
			m.logger.Error("failed to shutdown plugin", "key", key, "error", err)
			lastErr = err
		}
	}

	m.hosts = make(map[string]*external.Host)
	m.types = make(map[string]*pluginType)

	return lastErr
}
