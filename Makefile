.PHONY: all build build-ui build-server clean run stop dev dev-ui dev-server install-ui help

# Binary names
SERVER_BIN := vibeflow-server
CLI_BIN := vibeflow

# Directories
UI_DIR := ui
DIST_DIR := $(UI_DIR)/dist
DATA_DIR := ./data
PLUGIN_DIR := ./plugins

# PID file for background server
PID_FILE := .vibeflow.pid

# Default target
all: build

# Build everything
build: build-ui build-server

# Build frontend (only if sources changed)
build-ui: $(DIST_DIR)

$(DIST_DIR): $(shell find $(UI_DIR)/src -type f -name '*.ts' -o -name '*.tsx' -o -name '*.css' 2>/dev/null) $(UI_DIR)/package.json
	@echo "Building frontend..."
	@cd $(UI_DIR) && npm run build
	@touch $(DIST_DIR)

# Build server (depends on frontend for embedding)
build-server: build-ui
	@echo "Building server..."
	@go build -o $(SERVER_BIN) ./cmd/vibeflow-server

# Build CLI only
build-cli:
	@echo "Building CLI..."
	@go build -o $(CLI_BIN) ./cmd/vibeflow

# Install frontend dependencies
install-ui:
	@echo "Installing frontend dependencies..."
	@cd $(UI_DIR) && npm install

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(SERVER_BIN) $(CLI_BIN)
	@rm -rf $(DIST_DIR)
	@rm -f $(PID_FILE)

# Run the server (foreground)
run: build
	@echo "Starting server on :8080..."
	@./$(SERVER_BIN) -addr :8080 -data $(DATA_DIR) -plugins $(PLUGIN_DIR)

# Run server in background
start: build
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "Server already running (PID: $$(cat $(PID_FILE)))"; \
	else \
		echo "Starting server in background..."; \
		./$(SERVER_BIN) -addr :8080 -data $(DATA_DIR) -plugins $(PLUGIN_DIR) & \
		echo $$! > $(PID_FILE); \
		echo "Server started (PID: $$(cat $(PID_FILE)))"; \
		echo "Listening on http://localhost:8080"; \
	fi

# Stop background server
stop:
	@if [ -f $(PID_FILE) ]; then \
		PID=$$(cat $(PID_FILE)); \
		if kill -0 $$PID 2>/dev/null; then \
			echo "Stopping server (PID: $$PID)..."; \
			kill $$PID; \
			rm -f $(PID_FILE); \
			echo "Server stopped"; \
		else \
			echo "Server not running (stale PID file)"; \
			rm -f $(PID_FILE); \
		fi \
	else \
		echo "No PID file found"; \
	fi

# Check server status
status:
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "Server running (PID: $$(cat $(PID_FILE)))"; \
	else \
		echo "Server not running"; \
	fi

# Restart server
restart: stop start

# Development mode - run frontend dev server with hot reload
dev-ui:
	@echo "Starting frontend dev server..."
	@cd $(UI_DIR) && npm run dev

# Development mode - run backend only (without embedded UI)
dev-server: build-cli
	@echo "Starting server in dev mode..."
	@go run ./cmd/vibeflow-server -addr :8080 -data $(DATA_DIR) -plugins $(PLUGIN_DIR) -debug

# Full dev mode - run both (frontend proxies to backend)
dev:
	@echo "Starting dev mode..."
	@echo "Run 'make dev-server' in one terminal"
	@echo "Run 'make dev-ui' in another terminal"
	@echo "Frontend dev server will proxy API requests to backend"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Build example plugin
build-plugin-echo:
	@echo "Building echo plugin..."
	@go build -o examples/plugins/echo/echo-plugin ./examples/plugins/echo

# Format code
fmt:
	@echo "Formatting Go code..."
	@go fmt ./...
	@echo "Formatting frontend code..."
	@cd $(UI_DIR) && npm run format 2>/dev/null || true

# Lint code
lint:
	@echo "Linting Go code..."
	@golangci-lint run 2>/dev/null || go vet ./...
	@echo "Linting frontend code..."
	@cd $(UI_DIR) && npm run lint 2>/dev/null || true

# Show help
help:
	@echo "Vibeflow Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build targets:"
	@echo "  all          Build everything (default)"
	@echo "  build        Build frontend and server"
	@echo "  build-ui     Build frontend only"
	@echo "  build-server Build server only"
	@echo "  build-cli    Build CLI only"
	@echo "  clean        Remove build artifacts"
	@echo ""
	@echo "Run targets:"
	@echo "  run          Run server in foreground"
	@echo "  start        Start server in background"
	@echo "  stop         Stop background server"
	@echo "  restart      Restart background server"
	@echo "  status       Check if server is running"
	@echo ""
	@echo "Development targets:"
	@echo "  dev          Instructions for dev mode"
	@echo "  dev-ui       Run frontend with hot reload"
	@echo "  dev-server   Run backend in debug mode"
	@echo "  install-ui   Install frontend dependencies"
	@echo ""
	@echo "Other targets:"
	@echo "  test         Run tests"
	@echo "  fmt          Format code"
	@echo "  lint         Lint code"
	@echo "  help         Show this help"
