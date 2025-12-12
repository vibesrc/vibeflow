// Command vibeflow is the main entry point for the Vibeflow runtime.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bherbruck/vibeflow/pkg/flow"
	"github.com/bherbruck/vibeflow/pkg/node"
	"github.com/bherbruck/vibeflow/pkg/runtime"

	// Import core nodes to register them
	_ "github.com/bherbruck/vibeflow/pkg/nodes/core"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// pluginList is a flag that can be repeated
type pluginList []string

func (p *pluginList) String() string {
	return fmt.Sprintf("%v", *p)
}

func (p *pluginList) Set(value string) error {
	*p = append(*p, value)
	return nil
}

func run() error {
	// Parse flags
	var flowPath string
	var debug bool
	var plugins pluginList

	flag.StringVar(&flowPath, "flow", "", "Path to flow YAML file")
	flag.StringVar(&flowPath, "f", "", "Path to flow YAML file (shorthand)")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	flag.Var(&plugins, "plugin", "External plugin to load (can be repeated)")
	flag.Var(&plugins, "p", "External plugin to load (shorthand)")
	flag.Parse()

	// Check for positional argument
	if flowPath == "" && flag.NArg() > 0 {
		flowPath = flag.Arg(0)
	}

	if flowPath == "" {
		return fmt.Errorf("usage: vibeflow [-f] <flow.yaml>")
	}

	// Setup logging
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Load flow
	logger.Info("loading flow", "path", flowPath)
	f, err := flow.LoadFromFile(flowPath)
	if err != nil {
		return fmt.Errorf("failed to load flow: %w", err)
	}

	logger.Info("loaded flow",
		"name", f.Metadata.Name,
		"nodes", len(f.Nodes),
		"wires", len(f.Wires),
	)

	// List available node types
	logger.Debug("registered node types", "types", node.DefaultRegistry.Types())

	// Create runtime
	rt := runtime.New(nil)

	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load plugins
	for _, pluginPath := range plugins {
		logger.Info("loading plugin", "path", pluginPath)
		if err := rt.LoadPlugin(ctx, pluginPath); err != nil {
			return fmt.Errorf("failed to load plugin %s: %w", pluginPath, err)
		}
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Load and run flow
	if err := rt.Load(ctx, f); err != nil {
		return fmt.Errorf("failed to load flow into runtime: %w", err)
	}

	logger.Info("starting flow")
	if err := rt.Run(ctx); err != nil {
		return fmt.Errorf("runtime error: %w", err)
	}

	// Shutdown plugins
	rt.Shutdown(context.Background())

	logger.Info("flow stopped")
	return nil
}
