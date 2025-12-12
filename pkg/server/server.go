package server

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/bherbruck/vibeflow/ui"
)

// Server is the main Vibeflow server.
type Server struct {
	config     *Config
	logger     *slog.Logger
	store      *Store
	engine     *Engine
	api        *API
	httpServer *http.Server
}

// Config configures the server.
type Config struct {
	// HTTP server settings
	Addr string // Default: ":8080"

	// Data paths
	DataDir   string // Directory for SQLite and data files
	PluginDir string // Directory for plugins

	// Feature flags
	EnableAuth bool
}

// New creates a new server.
func New(cfg *Config) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if cfg.PluginDir == "" {
		cfg.PluginDir = "./plugins"
	}

	logger := slog.Default()

	// Create store
	dbPath := cfg.DataDir + "/vibeflow.db"
	store, err := NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Create engine
	engine := NewEngine(&EngineConfig{
		Store:     store,
		PluginDir: cfg.PluginDir,
		DataDir:   cfg.DataDir,
	})

	// Create API
	api := NewAPI(engine, store)

	// Create HTTP server
	mux := http.NewServeMux()

	// API routes
	mux.Handle("/api/", api)

	// Serve embedded UI assets
	distFS, err := fs.Sub(ui.Assets, "dist")
	if err != nil {
		return nil, fmt.Errorf("failed to access embedded UI: %w", err)
	}

	// Read index.html for SPA fallback
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read index.html: %w", err)
	}

	// Serve static files, with SPA fallback to index.html
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Try to serve static file if it exists (and has an extension)
		if path != "/" {
			// Check if file exists in dist
			if f, err := distFS.Open(path[1:]); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for client-side routing
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return &Server{
		config:     cfg,
		logger:     logger,
		store:      store,
		engine:     engine,
		api:        api,
		httpServer: httpServer,
	}, nil
}

// Start starts the server.
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("starting engine")

	// Start the flow engine
	if err := s.engine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}

	s.logger.Info("starting HTTP server", "addr", s.config.Addr)

	// Start HTTP server in background
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("HTTP server shutdown error", "error", err)
	}

	// Shutdown engine
	if err := s.engine.Shutdown(ctx); err != nil {
		s.logger.Error("engine shutdown error", "error", err)
	}

	// Close store
	if err := s.store.Close(); err != nil {
		s.logger.Error("store close error", "error", err)
	}

	return nil
}

// Engine returns the flow engine.
func (s *Server) Engine() *Engine {
	return s.engine
}

// Store returns the data store.
func (s *Server) Store() *Store {
	return s.store
}
