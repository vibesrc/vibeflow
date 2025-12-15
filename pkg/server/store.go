// Package server provides the Vibeflow server with web UI and API.
package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages flow persistence in SQLite.
type Store struct {
	db *sql.DB
}

// FlowRecord represents a stored flow.
type FlowRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Content     string    `json:"content"` // YAML content
	Enabled     bool      `json:"enabled"`
	Status      string    `json:"status"` // "stopped", "running", "error"
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewStore creates a new SQLite store.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return store, nil
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS flows (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		content TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		status TEXT DEFAULT 'stopped',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS plugins (
		name TEXT PRIMARY KEY,
		source TEXT NOT NULL,
		version TEXT DEFAULT '',
		checksum TEXT DEFAULT '',
		installed_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateFlow creates a new flow.
func (s *Store) CreateFlow(flow *FlowRecord) error {
	now := time.Now()
	flow.CreatedAt = now
	flow.UpdatedAt = now
	if flow.Status == "" {
		flow.Status = "stopped"
	}

	_, err := s.db.Exec(
		`INSERT INTO flows (id, name, description, content, enabled, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		flow.ID, flow.Name, flow.Description, flow.Content,
		flow.Enabled, flow.Status, flow.CreatedAt, flow.UpdatedAt,
	)
	return err
}

// GetFlow retrieves a flow by ID.
func (s *Store) GetFlow(id string) (*FlowRecord, error) {
	row := s.db.QueryRow(
		`SELECT id, name, description, content, enabled, status, created_at, updated_at
		 FROM flows WHERE id = ?`, id,
	)

	flow := &FlowRecord{}
	err := row.Scan(
		&flow.ID, &flow.Name, &flow.Description, &flow.Content,
		&flow.Enabled, &flow.Status, &flow.CreatedAt, &flow.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return flow, nil
}

// UpdateFlow updates an existing flow.
func (s *Store) UpdateFlow(flow *FlowRecord) error {
	flow.UpdatedAt = time.Now()
	_, err := s.db.Exec(
		`UPDATE flows SET name=?, description=?, content=?, enabled=?, status=?, updated_at=?
		 WHERE id=?`,
		flow.Name, flow.Description, flow.Content, flow.Enabled, flow.Status,
		flow.UpdatedAt, flow.ID,
	)
	return err
}

// DeleteFlow deletes a flow.
func (s *Store) DeleteFlow(id string) error {
	_, err := s.db.Exec(`DELETE FROM flows WHERE id=?`, id)
	return err
}

// ListFlows returns all flows.
func (s *Store) ListFlows() ([]*FlowRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, content, enabled, status, created_at, updated_at
		 FROM flows ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flows []*FlowRecord
	for rows.Next() {
		flow := &FlowRecord{}
		err := rows.Scan(
			&flow.ID, &flow.Name, &flow.Description, &flow.Content,
			&flow.Enabled, &flow.Status, &flow.CreatedAt, &flow.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		flows = append(flows, flow)
	}
	return flows, rows.Err()
}

// UpdateFlowStatus updates just the status field.
func (s *Store) UpdateFlowStatus(id, status string) error {
	_, err := s.db.Exec(
		`UPDATE flows SET status=?, updated_at=? WHERE id=?`,
		status, time.Now(), id,
	)
	return err
}

// SetFlowEnabled updates just the enabled field.
func (s *Store) SetFlowEnabled(id string, enabled bool) error {
	_, err := s.db.Exec(
		`UPDATE flows SET enabled=?, updated_at=? WHERE id=?`,
		enabled, time.Now(), id,
	)
	return err
}

// GetSetting retrieves a setting value.
func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting stores a setting value.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`,
		key, value,
	)
	return err
}

// PluginRecord represents an installed plugin.
type PluginRecord struct {
	Name        string    `json:"name"`
	Source      string    `json:"source"`
	Version     string    `json:"version"`
	Checksum    string    `json:"checksum"`
	InstalledAt time.Time `json:"installed_at"`
}

// SavePlugin records an installed plugin.
func (s *Store) SavePlugin(plugin *PluginRecord) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO plugins (name, source, version, checksum, installed_at)
		 VALUES (?, ?, ?, ?, ?)`,
		plugin.Name, plugin.Source, plugin.Version, plugin.Checksum, time.Now(),
	)
	return err
}

// GetPlugin retrieves a plugin by name.
func (s *Store) GetPlugin(name string) (*PluginRecord, error) {
	row := s.db.QueryRow(
		`SELECT name, source, version, checksum, installed_at FROM plugins WHERE name=?`,
		name,
	)

	plugin := &PluginRecord{}
	err := row.Scan(&plugin.Name, &plugin.Source, &plugin.Version, &plugin.Checksum, &plugin.InstalledAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return plugin, err
}

// ListPlugins returns all installed plugins.
func (s *Store) ListPlugins() ([]*PluginRecord, error) {
	rows, err := s.db.Query(
		`SELECT name, source, version, checksum, installed_at FROM plugins ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []*PluginRecord
	for rows.Next() {
		plugin := &PluginRecord{}
		err := rows.Scan(&plugin.Name, &plugin.Source, &plugin.Version, &plugin.Checksum, &plugin.InstalledAt)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return plugins, rows.Err()
}

// ExportFlows exports all flows as JSON.
func (s *Store) ExportFlows() ([]byte, error) {
	flows, err := s.ListFlows()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(flows, "", "  ")
}
