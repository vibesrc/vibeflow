package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bherbruck/vibeflow/pkg/external"
	"github.com/bherbruck/vibeflow/pkg/flow"
	"github.com/bherbruck/vibeflow/pkg/node"
	"github.com/google/uuid"
)

// API provides the REST API for Vibeflow.
type API struct {
	engine *Engine
	store  *Store
	logger *slog.Logger
	mux    *http.ServeMux
}

// NewAPI creates a new API handler.
func NewAPI(engine *Engine, store *Store) *API {
	api := &API{
		engine: engine,
		store:  store,
		logger: slog.Default(),
		mux:    http.NewServeMux(),
	}

	api.setupRoutes()
	return api
}

func (a *API) setupRoutes() {
	// Flow endpoints
	a.mux.HandleFunc("GET /api/flows", a.handleListFlows)
	a.mux.HandleFunc("POST /api/flows", a.handleCreateFlow)
	a.mux.HandleFunc("GET /api/flows/{id}", a.handleGetFlow)
	a.mux.HandleFunc("PUT /api/flows/{id}", a.handleUpdateFlow)
	a.mux.HandleFunc("DELETE /api/flows/{id}", a.handleDeleteFlow)

	// Flow actions
	a.mux.HandleFunc("POST /api/flows/{id}/start", a.handleStartFlow)
	a.mux.HandleFunc("POST /api/flows/{id}/stop", a.handleStopFlow)
	a.mux.HandleFunc("POST /api/flows/{id}/restart", a.handleRestartFlow)

	// Node types
	a.mux.HandleFunc("GET /api/node-types", a.handleListNodeTypes)

	// Plugins
	a.mux.HandleFunc("GET /api/plugins", a.handleListPlugins)
	a.mux.HandleFunc("POST /api/plugins/install", a.handleInstallPlugin)

	// Health check
	a.mux.HandleFunc("GET /api/health", a.handleHealth)
}

// ServeHTTP implements http.Handler.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	a.mux.ServeHTTP(w, r)
}

// Response helpers
func (a *API) json(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (a *API) error(w http.ResponseWriter, status int, message string) {
	a.json(w, status, map[string]string{"error": message})
}

// Flow handlers

func (a *API) handleListFlows(w http.ResponseWriter, r *http.Request) {
	flows, err := a.store.ListFlows()
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add runtime status
	type flowWithStatus struct {
		*FlowRecord
		RuntimeStatus string `json:"runtime_status"`
	}

	result := make([]flowWithStatus, len(flows))
	for i, f := range flows {
		result[i] = flowWithStatus{
			FlowRecord:    f,
			RuntimeStatus: a.engine.GetFlowStatus(f.ID),
		}
	}

	a.json(w, http.StatusOK, result)
}

func (a *API) handleCreateFlow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Content     string `json:"content"`
		Enabled     bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Validate flow content
	if req.Content != "" {
		if _, err := flow.Parse([]byte(req.Content)); err != nil {
			a.error(w, http.StatusBadRequest, "invalid flow YAML: "+err.Error())
			return
		}
	}

	record := &FlowRecord{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Enabled:     req.Enabled,
	}

	if err := a.store.CreateFlow(record); err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Start if enabled
	if record.Enabled && record.Content != "" {
		go func() {
			if err := a.engine.StartFlow(context.Background(), record.ID); err != nil {
				a.logger.Error("failed to start flow", "id", record.ID, "error", err)
			}
		}()
	}

	a.json(w, http.StatusCreated, record)
}

func (a *API) handleGetFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	record, err := a.store.GetFlow(id)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if record == nil {
		a.error(w, http.StatusNotFound, "flow not found")
		return
	}

	// Add runtime status
	type flowWithStatus struct {
		*FlowRecord
		RuntimeStatus string `json:"runtime_status"`
	}

	a.json(w, http.StatusOK, flowWithStatus{
		FlowRecord:    record,
		RuntimeStatus: a.engine.GetFlowStatus(record.ID),
	})
}

func (a *API) handleUpdateFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	record, err := a.store.GetFlow(id)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if record == nil {
		a.error(w, http.StatusNotFound, "flow not found")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Content     *string `json:"content"`
		Enabled     *bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Apply updates
	if req.Name != nil {
		record.Name = *req.Name
	}
	if req.Description != nil {
		record.Description = *req.Description
	}
	if req.Content != nil {
		// Validate new content
		if _, err := flow.Parse([]byte(*req.Content)); err != nil {
			a.error(w, http.StatusBadRequest, "invalid flow YAML: "+err.Error())
			return
		}
		record.Content = *req.Content
	}
	if req.Enabled != nil {
		record.Enabled = *req.Enabled
	}

	if err := a.store.UpdateFlow(record); err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Restart if running and content changed
	if a.engine.IsFlowRunning(id) && req.Content != nil {
		go func() {
			if err := a.engine.RestartFlow(context.Background(), id); err != nil {
				a.logger.Error("failed to restart flow", "id", id, "error", err)
			}
		}()
	}

	// Return with runtime status (same structure as GET)
	type flowWithStatus struct {
		*FlowRecord
		RuntimeStatus string `json:"runtime_status"`
	}

	a.json(w, http.StatusOK, flowWithStatus{
		FlowRecord:    record,
		RuntimeStatus: a.engine.GetFlowStatus(record.ID),
	})
}

func (a *API) handleDeleteFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Stop if running and wait for it to stop
	if a.engine.IsFlowRunning(id) {
		doneCh, _ := a.engine.StopFlow(id)
		if doneCh != nil {
			<-doneCh
		}
	}

	if err := a.store.DeleteFlow(id); err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusOK, map[string]bool{"deleted": true})
}

// Flow action handlers

func (a *API) handleStartFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := a.engine.StartFlow(r.Context(), id); err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Mark flow as enabled so it starts on server restart
	a.store.SetFlowEnabled(id, true)

	a.json(w, http.StatusOK, map[string]string{"status": "started"})
}

func (a *API) handleStopFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	doneCh, err := a.engine.StopFlow(id)
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Wait for flow to fully stop before responding
	<-doneCh

	// Mark flow as disabled so it doesn't start on server restart
	a.store.SetFlowEnabled(id, false)

	a.json(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (a *API) handleRestartFlow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := a.engine.RestartFlow(r.Context(), id); err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusOK, map[string]string{"status": "restarted"})
}

// Node type handlers

type portInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type configSpec struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Required    bool         `json:"required,omitempty"`
	Default     any          `json:"default,omitempty"`
	Description string       `json:"description,omitempty"`
	Options     []any        `json:"options,omitempty"`
	Items       []configSpec `json:"items,omitempty"`
	Format      string       `json:"format,omitempty"`
	Language    string       `json:"language,omitempty"`
}

type nodeTypeInfo struct {
	Type        string       `json:"type"`
	Category    string       `json:"category"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	IsExternal  bool         `json:"is_external"`
	Inputs      []portInfo   `json:"inputs,omitempty"`
	Outputs     []portInfo   `json:"outputs,omitempty"`
	Config      []configSpec `json:"config,omitempty"`
}

// convertConfigSpec converts node.ConfigSpec to API configSpec (including nested items)
func convertConfigSpec(c node.ConfigSpec) configSpec {
	spec := configSpec{
		Name:        c.Name,
		Type:        c.Type,
		Required:    c.Required,
		Default:     c.Default,
		Description: c.Description,
		Options:     c.Options,
		Format:      c.Format,
		Language:    c.Language,
	}
	if len(c.Items) > 0 {
		spec.Items = make([]configSpec, len(c.Items))
		for i, item := range c.Items {
			spec.Items[i] = convertConfigSpec(item)
		}
	}
	return spec
}

// convertExternalConfigSpec converts external.ConfigSpec to API configSpec (including nested items)
func convertExternalConfigSpec(c external.ConfigSpec) configSpec {
	spec := configSpec{
		Name:        c.Name,
		Type:        c.Type,
		Required:    c.Required,
		Default:     c.Default,
		Description: c.Description,
		Options:     c.Options,
	}
	if len(c.Items) > 0 {
		spec.Items = make([]configSpec, len(c.Items))
		for i, item := range c.Items {
			spec.Items[i] = convertExternalConfigSpec(item)
		}
	}
	return spec
}

func (a *API) handleListNodeTypes(w http.ResponseWriter, r *http.Request) {
	var types []nodeTypeInfo

	// Built-in types - use full type info from registry
	for _, info := range node.DefaultRegistry.TypeInfos() {
		category := info.Category
		if category == "" {
			parts := strings.SplitN(info.Type, ".", 2)
			category = "core"
			if len(parts) > 1 {
				category = parts[0]
			}
		}

		// Convert port info
		inputs := make([]portInfo, len(info.Inputs))
		for i, p := range info.Inputs {
			inputs[i] = portInfo{Name: p.Name, Description: p.Description}
		}
		outputs := make([]portInfo, len(info.Outputs))
		for i, p := range info.Outputs {
			outputs[i] = portInfo{Name: p.Name, Description: p.Description}
		}

		// Convert config spec
		config := make([]configSpec, len(info.Config))
		for i, c := range info.Config {
			config[i] = convertConfigSpec(c)
		}

		name := info.Name
		if name == "" {
			name = info.Type
		}

		types = append(types, nodeTypeInfo{
			Type:        info.Type,
			Category:    category,
			Name:        name,
			Description: info.Description,
			IsExternal:  false,
			Inputs:      inputs,
			Outputs:     outputs,
			Config:      config,
		})
	}

	// Plugin types - use full type info
	for _, info := range a.engine.Plugins().TypeInfos() {
		parts := strings.SplitN(info.Type, ".", 2)
		category := info.Category
		if category == "" {
			category = "external"
			if len(parts) > 1 {
				category = parts[0]
			}
		}

		// Convert port info
		inputs := make([]portInfo, len(info.Inputs))
		for i, p := range info.Inputs {
			inputs[i] = portInfo{Name: p.Name, Description: p.Description}
		}
		outputs := make([]portInfo, len(info.Outputs))
		for i, p := range info.Outputs {
			outputs[i] = portInfo{Name: p.Name, Description: p.Description}
		}

		// Convert config spec
		config := make([]configSpec, len(info.Config))
		for i, c := range info.Config {
			config[i] = convertExternalConfigSpec(c)
		}

		// Default to one input/output if none specified
		if len(inputs) == 0 {
			inputs = []portInfo{{Name: "input", Description: "Default input"}}
		}
		if len(outputs) == 0 {
			outputs = []portInfo{{Name: "output", Description: "Default output"}}
		}

		name := info.Name
		if name == "" {
			name = info.Type
		}

		types = append(types, nodeTypeInfo{
			Type:        info.Type,
			Category:    category,
			Name:        name,
			Description: info.Description,
			IsExternal:  true,
			Inputs:      inputs,
			Outputs:     outputs,
			Config:      config,
		})
	}

	a.json(w, http.StatusOK, types)
}

// Plugin handlers

func (a *API) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := a.engine.Plugins().InstalledPlugins()
	a.json(w, http.StatusOK, plugins)
}

func (a *API) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source  string `json:"source"`  // "github:owner/repo"
		Version string `json:"version"` // Optional, defaults to "latest"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.error(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if !strings.HasPrefix(req.Source, "github:") {
		a.error(w, http.StatusBadRequest, "only GitHub sources supported currently")
		return
	}

	parts := strings.TrimPrefix(req.Source, "github:")
	ownerRepo := strings.Split(parts, "/")
	if len(ownerRepo) != 2 {
		a.error(w, http.StatusBadRequest, "invalid source format, expected github:owner/repo")
		return
	}

	version := req.Version
	if version == "" {
		version = "latest"
	}

	if err := a.engine.Plugins().InstallFromGitHub(r.Context(), ownerRepo[0], ownerRepo[1], version); err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.json(w, http.StatusOK, map[string]string{"status": "installed"})
}

// Health handler

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	a.json(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"running_flows": len(a.engine.ListRunningFlows()),
	})
}
