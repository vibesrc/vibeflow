// Command vibeflow-server runs the Vibeflow server with web UI and API.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bherbruck/vibeflow/pkg/server"

	// Import core nodes to register them
	_ "github.com/bherbruck/vibeflow/pkg/nodes/core"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse flags
	var (
		addr      string
		dataDir   string
		pluginDir string
		debug     bool
	)

	flag.StringVar(&addr, "addr", ":8080", "HTTP server address")
	flag.StringVar(&dataDir, "data", "./data", "Data directory")
	flag.StringVar(&pluginDir, "plugins", "./plugins", "Plugin directory")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	flag.Parse()

	// Setup logging
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Ensure directories exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Create server
	srv, err := server.New(&server.Config{
		Addr:      addr,
		DataDir:   dataDir,
		PluginDir: pluginDir,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Start server
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	logger.Info("vibeflow server started",
		"addr", addr,
		"data_dir", dataDir,
		"plugin_dir", pluginDir,
	)

	// Wait for shutdown
	<-ctx.Done()

	// Graceful shutdown
	if err := srv.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	logger.Info("server stopped")
	return nil
}
