# Vibeflow Node SDK Specification

**Version:** 1.0
**Last Updated:** 2025-12-10
**Status:** Draft

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [SDK Architecture](#2-sdk-architecture)
3. [Node Interface](#3-node-interface)
4. [JSON-RPC Protocol](#4-json-rpc-protocol)
5. [Message Format](#5-message-format)
6. [Context API](#6-context-api)
7. [Configuration Handling](#7-configuration-handling)
8. [Lifecycle Management](#8-lifecycle-management)
9. [Concurrency Control](#9-concurrency-control)
10. [Logging and Observability](#10-logging-and-observability)
11. [Error Handling](#11-error-handling)
12. [Testing Framework](#12-testing-framework)
13. [CLI Tools](#13-cli-tools)
14. [Complete Examples](#14-complete-examples)

---

## 1. Introduction

### 1.1 Purpose

The Vibeflow Node SDK is a Go library that simplifies building external nodes. It handles:

- JSON-RPC communication with the runtime
- Lifecycle management (init, start, stop)
- Concurrency control
- Configuration parsing and validation
- Manifest generation from Go struct tags
- Logging and error handling

### 1.2 Design Goals

1. **Minimal boilerplate**: Node authors focus on business logic
2. **Type-safe configuration**: Struct tags generate JSON Schema
3. **Concurrency-safe**: SDK manages goroutines and message dispatch
4. **Testable**: Built-in testing utilities
5. **Observable**: Structured logging and metrics

### 1.3 Installation

```bash
go get github.com/bherbruck/vibeflow/sdk
```

---

## 2. SDK Architecture

### 2.1 Component Overview

```
+------------------------------------------------------------------+
|                        Node Binary                                |
+------------------------------------------------------------------+
|                                                                   |
|  +-------------------+    +-------------------+                   |
|  |   Your Node Code  |    |   Vibeflow SDK    |                   |
|  |                   |    |                   |                   |
|  |  - Init()         |    |  - RPC Handler    |                   |
|  |  - OnMessage()    |--->|  - Lifecycle Mgr  |                   |
|  |  - Stop()         |    |  - Config Parser  |                   |
|  |                   |    |  - Concurrency    |                   |
|  +-------------------+    +-------------------+                   |
|                                   |                               |
|                                   v                               |
|                          +-------------------+                    |
|                          |   JSON-RPC I/O    |                    |
|                          |  stdin / stdout   |                    |
|                          +-------------------+                    |
|                                   |                               |
+-----------------------------------|-------------------------------+
                                    |
                          stdin/stdout (JSON-RPC 2.0)
                                    |
+-----------------------------------|-------------------------------+
|                     Vibeflow Runtime                              |
+------------------------------------------------------------------+
```

### 2.2 Package Structure

```
github.com/bherbruck/vibeflow/sdk/
  node/           # Core node interface and runner
    node.go       # Node interface
    runner.go     # Main execution loop
    context.go    # Context types
    config.go     # Configuration handling
  message/        # Message types
    message.go    # Message struct
    payload.go    # Payload helpers
  rpc/            # JSON-RPC protocol
    protocol.go   # RPC types
    transport.go  # stdin/stdout transport
  manifest/       # Manifest generation
    generate.go   # Generate from struct tags
    schema.go     # JSON Schema generation
  testing/        # Test utilities
    harness.go    # Test harness
    mock.go       # Mock runtime
```

---

## 3. Node Interface

### 3.1 Core Interface

```go
package node

// Node is the interface that all external nodes must implement.
type Node interface {
    // Init is called once when the node is first created.
    // Use this to parse configuration, establish connections,
    // and allocate resources.
    Init(ctx InitContext) error

    // OnMessage is called for each incoming message.
    // Process the message and emit outputs via ctx.Emit().
    OnMessage(ctx MessageContext, msg Message)

    // Stop is called when the node is shutting down.
    // Use this to close connections and release resources.
    Stop() error
}
```

### 3.2 Optional Interfaces

```go
// Starter is implemented by nodes that need to perform
// actions after Init but before receiving messages.
// Use for starting background goroutines, timers, etc.
type Starter interface {
    Start(ctx RunContext) error
}

// Ticker is implemented by nodes that need periodic execution.
// The runtime calls Tick() at the configured interval.
type Ticker interface {
    Tick(ctx MessageContext)
}

// HealthChecker is implemented by nodes that support health checks.
type HealthChecker interface {
    HealthCheck() error
}
```

### 3.3 Running a Node

```go
package main

import (
    "github.com/bherbruck/vibeflow/sdk/node"
)

func main() {
    node.Run(&MyNode{})
}
```

The `node.Run()` function:
1. Sets up JSON-RPC communication over stdin/stdout
2. Waits for the `init` RPC call
3. Calls your `Init()` method
4. Enters the message processing loop
5. Calls `Stop()` on shutdown

---

## 4. JSON-RPC Protocol

### 4.1 Overview

Communication between the runtime and external nodes uses JSON-RPC 2.0 over stdin/stdout.

**Message framing**: Each JSON-RPC message is a single line terminated by `\n`.

### 4.2 Runtime -> Node Methods

#### 4.2.1 init

Initialize the node with configuration.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "init",
  "params": {
    "node_id": "modbus1",
    "node_type": "protocol.modbus-tcp",
    "config": {
      "address": "192.168.1.100:502",
      "slave_id": 1,
      "function": "read_holding_registers"
    }
  }
}
```

**Response (success):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "status": "ready"
  }
}
```

**Response (error):**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32000,
    "message": "failed to connect to 192.168.1.100:502: connection refused"
  }
}
```

#### 4.2.2 process

Process an incoming message.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "process",
  "params": {
    "input": "default",
    "message": {
      "id": "msg-uuid-123",
      "payload": {"trigger": true},
      "metadata": {
        "timestamp": "2025-12-10T10:30:00Z",
        "source_node": "inject1"
      },
      "context": {}
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "status": "processed"
  }
}
```

Note: Output messages are sent via the `emit` notification (node -> runtime).

#### 4.2.3 stop

Graceful shutdown.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "stop",
  "params": {
    "timeout_ms": 5000
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "status": "stopped"
  }
}
```

#### 4.2.4 health

Health check (optional).

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "health",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "status": "healthy",
    "details": {
      "connection": "connected",
      "last_poll": "2025-12-10T10:30:00Z"
    }
  }
}
```

### 4.3 Node -> Runtime Notifications

#### 4.3.1 emit

Emit an output message.

**Notification:**
```json
{
  "jsonrpc": "2.0",
  "method": "emit",
  "params": {
    "output": "default",
    "message": {
      "id": "msg-uuid-456",
      "payload": {
        "registers": [100, 200, 300, 400]
      },
      "metadata": {
        "timestamp": "2025-12-10T10:30:01Z"
      },
      "context": {}
    }
  }
}
```

#### 4.3.2 log

Send a log message.

**Notification:**
```json
{
  "jsonrpc": "2.0",
  "method": "log",
  "params": {
    "level": "info",
    "message": "Connected to Modbus device",
    "fields": {
      "address": "192.168.1.100:502",
      "slave_id": 1
    }
  }
}
```

Log levels: `debug`, `info`, `warn`, `error`

#### 4.3.3 metric

Report a metric value.

**Notification:**
```json
{
  "jsonrpc": "2.0",
  "method": "metric",
  "params": {
    "name": "modbus_read_duration_ms",
    "type": "histogram",
    "value": 12.5,
    "labels": {
      "function": "read_holding_registers"
    }
  }
}
```

### 4.4 Error Codes

| Code | Meaning |
|------|---------|
| -32700 | Parse error |
| -32600 | Invalid request |
| -32601 | Method not found |
| -32602 | Invalid params |
| -32603 | Internal error |
| -32000 | Node initialization failed |
| -32001 | Configuration error |
| -32002 | Connection error |
| -32003 | Timeout |

---

## 5. Message Format

### 5.1 Message Structure

```go
package message

// Message represents a single unit of data flowing through the system.
type Message struct {
    // ID is a unique identifier (UUID v7, time-ordered)
    ID string `json:"id"`

    // Payload is the message data (any JSON-serializable value)
    Payload any `json:"payload"`

    // Metadata contains routing hints, timestamps, correlation IDs
    Metadata map[string]string `json:"metadata"`

    // Context is flow-scoped shared state
    Context map[string]any `json:"context"`
}
```

### 5.2 Creating Messages

```go
// In your node's OnMessage handler:

func (n *MyNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    // Create a new message with transformed payload
    result := msg.Clone()
    result.Payload = transformedData

    // Or create a fresh message
    newMsg := node.NewMessage(myPayload)

    ctx.Emit("default", result)
}
```

### 5.3 Payload Helpers

```go
package message

// Common payload operations

func (m *Message) PayloadAs(target any) error
func (m *Message) PayloadBytes() ([]byte, error)
func (m *Message) PayloadString() (string, error)
func (m *Message) PayloadMap() (map[string]any, error)

// Error payloads
func ErrorPayload(err error) map[string]any
func IsErrorPayload(payload any) bool
```

---

## 6. Context API

### 6.1 InitContext

Provided during `Init()`:

```go
type InitContext interface {
    // NodeID returns this node's unique identifier
    NodeID() string

    // NodeType returns the node type (e.g., "protocol.modbus-tcp")
    NodeType() string

    // ConfigInto parses the configuration into the provided struct
    ConfigInto(target any) error

    // ConfigRaw returns the raw configuration map
    ConfigRaw() map[string]any

    // Logger returns a structured logger
    Logger() Logger
}
```

### 6.2 RunContext

Provided during `Start()`:

```go
type RunContext interface {
    // Done returns a channel that's closed when the node should stop
    Done() <-chan struct{}

    // Emit sends a message to the specified output
    Emit(output string, payload any)

    // EmitMessage sends a full message object
    EmitMessage(output string, msg Message)

    // Logger returns a structured logger
    Logger() Logger
}
```

### 6.3 MessageContext

Provided during `OnMessage()`:

```go
type MessageContext interface {
    // Emit sends a message to the specified output
    Emit(output string, payload any)

    // EmitMessage sends a full message object
    EmitMessage(output string, msg Message)

    // EmitError sends an error to the "error" output
    EmitError(err error)

    // Logger returns a structured logger
    Logger() Logger

    // InputName returns which input received this message
    InputName() string
}
```

---

## 7. Configuration Handling

### 7.1 Struct Tags

The SDK uses struct tags to define configuration schema:

```go
type Config struct {
    // Basic types
    Address string `json:"address" vf:"required,pattern=^[^:]+:[0-9]+$"`
    Port    int    `json:"port" vf:"min=1,max=65535,default=502"`
    Timeout int    `json:"timeout_ms" vf:"min=0,default=5000"`

    // Enums
    Function string `json:"function" vf:"enum=read|write,default=read"`

    // Secrets (masked in logs)
    Password string `json:"password" vf:"secret"`

    // Environment variable substitution
    APIKey string `json:"api_key" vf:"env"`

    // Optional with default
    Retries int `json:"retries" vf:"default=3"`

    // Nested structs
    TLS TLSConfig `json:"tls"`
}

type TLSConfig struct {
    Enabled  bool   `json:"enabled" vf:"default=false"`
    CertFile string `json:"cert_file" vf:"required_if=Enabled:true"`
    KeyFile  string `json:"key_file" vf:"required_if=Enabled:true"`
}
```

### 7.2 Tag Reference

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Field must be provided | `vf:"required"` |
| `default=X` | Default value | `vf:"default=5000"` |
| `min=X` | Minimum value (numbers) | `vf:"min=1"` |
| `max=X` | Maximum value (numbers) | `vf:"max=65535"` |
| `pattern=X` | Regex pattern (strings) | `vf:"pattern=^[a-z]+$"` |
| `enum=X\|Y\|Z` | Allowed values | `vf:"enum=tcp\|udp"` |
| `secret` | Mask in logs/UI | `vf:"secret"` |
| `env` | Allow ${VAR} substitution | `vf:"env"` |
| `required_if=F:V` | Required if field F equals V | `vf:"required_if=TLS:true"` |

### 7.3 Parsing Configuration

```go
func (n *ModbusNode) Init(ctx node.InitContext) error {
    var config Config
    if err := ctx.ConfigInto(&config); err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }

    n.config = config
    return nil
}
```

### 7.4 Generated JSON Schema

The SDK generates JSON Schema from your config struct:

```bash
vibeflow manifest generate --config-type=Config
```

Output:
```yaml
config_schema:
  type: object
  required: [address]
  properties:
    address:
      type: string
      pattern: "^[^:]+:[0-9]+$"
    port:
      type: integer
      minimum: 1
      maximum: 65535
      default: 502
    timeout_ms:
      type: integer
      minimum: 0
      default: 5000
    function:
      type: string
      enum: [read, write]
      default: read
    password:
      type: string
      x-vf-secret: true
```

---

## 8. Lifecycle Management

### 8.1 Lifecycle Phases

```
1. Binary spawned by runtime
         |
         v
2. SDK waits for "init" RPC
         |
         v
3. SDK calls Node.Init(ctx)
         |
         +---> Error? Send error response, exit
         |
         v
4. SDK calls Node.Start(ctx) [if implemented]
         |
         +---> Error? Send error response, exit
         |
         v
5. SDK sends "ready" response
         |
         v
6. Message loop: SDK receives "process" RPC
         |
         +---> SDK calls Node.OnMessage(ctx, msg)
         |
         +---> (repeat)
         |
         v
7. SDK receives "stop" RPC
         |
         v
8. SDK calls Node.Stop()
         |
         v
9. SDK sends "stopped" response
         |
         v
10. Binary exits
```

### 8.2 Graceful Shutdown

The SDK handles shutdown signals:

```go
func (n *ModbusNode) Stop() error {
    // Close connections
    if n.client != nil {
        n.client.Close()
    }

    // Stop background goroutines
    close(n.stopCh)

    // Wait for goroutines to finish (with timeout)
    select {
    case <-n.done:
        return nil
    case <-time.After(5 * time.Second):
        return errors.New("shutdown timeout")
    }
}
```

### 8.3 Background Goroutines

For nodes that need background tasks (polling, etc.):

```go
type PollingNode struct {
    config Config
    stopCh chan struct{}
    done   chan struct{}
}

func (n *PollingNode) Init(ctx node.InitContext) error {
    ctx.ConfigInto(&n.config)
    n.stopCh = make(chan struct{})
    n.done = make(chan struct{})
    return nil
}

func (n *PollingNode) Start(ctx node.RunContext) error {
    go n.pollLoop(ctx)
    return nil
}

func (n *PollingNode) pollLoop(ctx node.RunContext) {
    defer close(n.done)

    ticker := time.NewTicker(time.Duration(n.config.IntervalMS) * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-n.stopCh:
            return
        case <-ticker.C:
            data := n.poll()
            ctx.Emit("default", data)
        }
    }
}

func (n *PollingNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    // Can also trigger poll on incoming message
    data := n.poll()
    ctx.Emit("default", data)
}

func (n *PollingNode) Stop() error {
    close(n.stopCh)
    <-n.done
    return nil
}
```

---

## 9. Concurrency Control

### 9.1 Concurrency Modes

Declared in manifest, enforced by SDK:

```yaml
concurrency:
  mode: single        # Default: one message at a time
  # OR
  mode: parallel
  max_parallel: 10    # Up to 10 concurrent OnMessage calls
```

### 9.2 Single Mode (Default)

Messages are processed sequentially:

```go
// SDK implementation (simplified)
func (r *runner) messageLoop() {
    for msg := range r.inbound {
        r.node.OnMessage(r.ctx, msg)  // Blocks until complete
    }
}
```

Use when:
- Node has shared state
- Order matters
- External resource has concurrency limits

### 9.3 Parallel Mode

Messages are processed concurrently with a semaphore:

```go
// SDK implementation (simplified)
func (r *runner) messageLoop() {
    sem := make(chan struct{}, r.maxParallel)

    for msg := range r.inbound {
        sem <- struct{}{}  // Acquire
        go func(m Message) {
            defer func() { <-sem }()  // Release
            r.node.OnMessage(r.ctx, m)
        }(msg)
    }
}
```

Use when:
- Node is stateless
- Each message is independent
- Want to maximize throughput

### 9.4 Thread-Safe Emit

`ctx.Emit()` is always thread-safe:

```go
func (n *MyNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    // Safe to emit from multiple goroutines
    go func() {
        result := expensiveOperation()
        ctx.Emit("default", result)  // Thread-safe
    }()
}
```

---

## 10. Logging and Observability

### 10.1 Structured Logging

```go
func (n *MyNode) Init(ctx node.InitContext) error {
    log := ctx.Logger()

    log.Info("initializing node",
        "address", n.config.Address,
        "slave_id", n.config.SlaveID,
    )

    log.Debug("detailed config", "config", n.config)

    if err := n.connect(); err != nil {
        log.Error("connection failed", "error", err)
        return err
    }

    return nil
}
```

### 10.2 Log Levels

| Level | Use Case |
|-------|----------|
| `debug` | Detailed debugging info |
| `info` | Normal operational messages |
| `warn` | Potential issues |
| `error` | Errors (node still running) |

### 10.3 Metrics

```go
func (n *MyNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    start := time.Now()

    result, err := n.process()

    // Report timing metric
    ctx.Metric("process_duration_ms", time.Since(start).Milliseconds())

    // Report counter
    if err != nil {
        ctx.MetricInc("process_errors_total")
    }
}
```

---

## 11. Error Handling

### 11.1 Init Errors

Return an error from `Init()` to fail node startup:

```go
func (n *MyNode) Init(ctx node.InitContext) error {
    if err := ctx.ConfigInto(&n.config); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }

    if err := n.connect(); err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }

    return nil
}
```

### 11.2 Message Processing Errors

Emit errors to the "error" output:

```go
func (n *MyNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    result, err := n.process(msg)
    if err != nil {
        ctx.EmitError(err)
        // Or with more context:
        ctx.Emit("error", map[string]any{
            "error": err.Error(),
            "input": msg.Payload,
        })
        return
    }

    ctx.Emit("default", result)
}
```

### 11.3 Panic Recovery

The SDK recovers from panics in `OnMessage()`:

```go
// SDK implementation
func (r *runner) safeOnMessage(ctx MessageContext, msg Message) {
    defer func() {
        if p := recover(); p != nil {
            r.logger.Error("panic in OnMessage",
                "panic", p,
                "stack", string(debug.Stack()),
            )
            ctx.EmitError(fmt.Errorf("panic: %v", p))
        }
    }()

    r.node.OnMessage(ctx, msg)
}
```

---

## 12. Testing Framework

### 12.1 Test Harness

```go
package mynode_test

import (
    "testing"
    "github.com/bherbruck/vibeflow/sdk/testing"
)

func TestModbusNode(t *testing.T) {
    harness := testing.NewHarness(t, &ModbusNode{})

    // Initialize with config
    err := harness.Init(map[string]any{
        "address":  "192.168.1.100:502",
        "slave_id": 1,
    })
    if err != nil {
        t.Fatalf("init failed: %v", err)
    }

    // Send a message
    harness.SendMessage("default", map[string]any{
        "trigger": true,
    })

    // Assert output
    msg := harness.ExpectOutput("default", 5*time.Second)
    if msg == nil {
        t.Fatal("expected output message")
    }

    registers, ok := msg.Payload.([]int)
    if !ok {
        t.Fatalf("expected []int payload, got %T", msg.Payload)
    }

    if len(registers) != 10 {
        t.Errorf("expected 10 registers, got %d", len(registers))
    }
}
```

### 12.2 Mock Connections

```go
func TestModbusNodeWithMock(t *testing.T) {
    // Create mock Modbus server
    mockServer := testing.NewMockModbusServer()
    mockServer.SetHoldingRegisters(0, []uint16{100, 200, 300})
    defer mockServer.Close()

    harness := testing.NewHarness(t, &ModbusNode{})
    harness.Init(map[string]any{
        "address":  mockServer.Addr(),
        "slave_id": 1,
    })

    harness.SendMessage("default", nil)

    msg := harness.ExpectOutput("default", time.Second)
    // ... assertions
}
```

### 12.3 Manifest-Based Tests

Define tests in the manifest:

```yaml
tests:
  - name: "Read holding registers"
    config:
      address: "mock://localhost"
      slave_id: 1
    input:
      payload: {"trigger": true}
    expected_output:
      output: default
      payload:
        registers: [100, 200, 300]

  - name: "Connection error"
    config:
      address: "mock://invalid"
      slave_id: 1
    input:
      payload: {"trigger": true}
    expected_output:
      output: error
```

Run with:
```bash
vibeflow test vibeflow-node.yaml
```

---

## 13. CLI Tools

### 13.1 vibeflow init

Create a new node project:

```bash
vibeflow init node mycompany/my-node

# Creates:
# my-node/
#   cmd/vibeflow-node-my-node/main.go
#   internal/mynode/node.go
#   internal/mynode/config.go
#   vibeflow-node.yaml
#   go.mod
#   README.md
#   LICENSE
#   .github/workflows/release.yml
```

### 13.2 vibeflow manifest

Generate or validate manifest:

```bash
# Generate manifest from Go types
vibeflow manifest generate --config-type=Config --output=vibeflow-node.yaml

# Validate existing manifest
vibeflow manifest validate vibeflow-node.yaml
```

### 13.3 vibeflow build

Build release binaries:

```bash
# Build for current platform
vibeflow build

# Build for all platforms
vibeflow build --all-platforms

# Build specific platforms
vibeflow build --platforms=linux/amd64,linux/arm64
```

Output:
```
dist/
  vibeflow-node-mynode-linux-amd64.tar.gz
  vibeflow-node-mynode-linux-arm64.tar.gz
  checksums.txt
  manifest.yaml
```

### 13.4 vibeflow test

Run manifest-defined tests:

```bash
vibeflow test vibeflow-node.yaml

# With verbose output
vibeflow test -v vibeflow-node.yaml

# Run specific test
vibeflow test --name="Read holding registers" vibeflow-node.yaml
```

### 13.5 vibeflow run

Run node in development mode:

```bash
# Interactive mode (stdin/stdout)
vibeflow run ./cmd/vibeflow-node-mynode

# With test config
vibeflow run --config='{"address":"localhost:502"}' ./cmd/vibeflow-node-mynode
```

---

## 14. Complete Examples

### 14.1 Modbus TCP Node

```go
// cmd/vibeflow-node-modbus-tcp/main.go
package main

import (
    "fmt"
    "github.com/bherbruck/vibeflow/sdk/node"
    "github.com/goburrow/modbus"
)

type ModbusTCPNode struct {
    config  Config
    handler *modbus.TCPClientHandler
    client  modbus.Client
}

type Config struct {
    Address      string `json:"address" vf:"required,pattern=^[^:]+:[0-9]+$"`
    SlaveID      int    `json:"slave_id" vf:"required,min=1,max=247"`
    Function     string `json:"function" vf:"enum=read_coils|read_holding_registers|write_single_register,default=read_holding_registers"`
    StartAddress int    `json:"start_address" vf:"min=0,max=65535,default=0"`
    Count        int    `json:"count" vf:"min=1,max=125,default=1"`
    PollMS       int    `json:"poll_interval_ms" vf:"min=0,default=0"`
}

func (n *ModbusTCPNode) Init(ctx node.InitContext) error {
    if err := ctx.ConfigInto(&n.config); err != nil {
        return err
    }

    n.handler = modbus.NewTCPClientHandler(n.config.Address)
    n.handler.SlaveId = byte(n.config.SlaveID)

    if err := n.handler.Connect(); err != nil {
        return fmt.Errorf("failed to connect: %w", err)
    }

    n.client = modbus.NewClient(n.handler)

    ctx.Logger().Info("connected to Modbus device",
        "address", n.config.Address,
        "slave_id", n.config.SlaveID,
    )

    return nil
}

func (n *ModbusTCPNode) Start(ctx node.RunContext) error {
    if n.config.PollMS > 0 {
        go n.pollLoop(ctx)
    }
    return nil
}

func (n *ModbusTCPNode) pollLoop(ctx node.RunContext) {
    ticker := time.NewTicker(time.Duration(n.config.PollMS) * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            n.readAndEmit(ctx)
        }
    }
}

func (n *ModbusTCPNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    n.readAndEmit(ctx)
}

func (n *ModbusTCPNode) readAndEmit(ctx interface{ Emit(string, any) }) {
    var results []byte
    var err error

    switch n.config.Function {
    case "read_coils":
        results, err = n.client.ReadCoils(uint16(n.config.StartAddress), uint16(n.config.Count))
    case "read_holding_registers":
        results, err = n.client.ReadHoldingRegisters(uint16(n.config.StartAddress), uint16(n.config.Count))
    default:
        err = fmt.Errorf("unsupported function: %s", n.config.Function)
    }

    if err != nil {
        ctx.Emit("error", map[string]any{"error": err.Error()})
        return
    }

    ctx.Emit("default", map[string]any{
        "registers":     results,
        "start_address": n.config.StartAddress,
        "count":         n.config.Count,
    })
}

func (n *ModbusTCPNode) Stop() error {
    if n.handler != nil {
        n.handler.Close()
    }
    return nil
}

func main() {
    node.Run(&ModbusTCPNode{})
}
```

### 14.2 GPIO Node (Raspberry Pi)

```go
// cmd/vibeflow-node-gpio/main.go
package main

import (
    "fmt"
    "github.com/bherbruck/vibeflow/sdk/node"
    "github.com/stianeikeland/go-rpio/v4"
)

type GPIONode struct {
    config Config
    pin    rpio.Pin
}

type Config struct {
    Pin       int    `json:"pin" vf:"required,min=0,max=27"`
    Direction string `json:"direction" vf:"enum=input|output,default=input"`
    Pull      string `json:"pull" vf:"enum=up|down|off,default=off"`
    Edge      string `json:"edge" vf:"enum=rising|falling|both|none,default=none"`
}

func (n *GPIONode) Init(ctx node.InitContext) error {
    if err := ctx.ConfigInto(&n.config); err != nil {
        return err
    }

    if err := rpio.Open(); err != nil {
        return fmt.Errorf("failed to open GPIO: %w", err)
    }

    n.pin = rpio.Pin(n.config.Pin)

    switch n.config.Direction {
    case "input":
        n.pin.Input()
    case "output":
        n.pin.Output()
    }

    switch n.config.Pull {
    case "up":
        n.pin.PullUp()
    case "down":
        n.pin.PullDown()
    case "off":
        n.pin.PullOff()
    }

    return nil
}

func (n *GPIONode) OnMessage(ctx node.MessageContext, msg node.Message) {
    if n.config.Direction == "input" {
        // Read pin state
        state := n.pin.Read()
        ctx.Emit("default", map[string]any{
            "pin":   n.config.Pin,
            "state": int(state),
        })
    } else {
        // Write pin state
        value, ok := msg.Payload.(map[string]any)["state"].(float64)
        if !ok {
            ctx.EmitError(fmt.Errorf("expected {state: 0|1} payload"))
            return
        }
        if value > 0 {
            n.pin.High()
        } else {
            n.pin.Low()
        }
        ctx.Emit("default", map[string]any{
            "pin":   n.config.Pin,
            "state": int(value),
        })
    }
}

func (n *GPIONode) Stop() error {
    rpio.Close()
    return nil
}

func main() {
    node.Run(&GPIONode{})
}
```

### 14.3 Serial Port Node

```go
// cmd/vibeflow-node-serial/main.go
package main

import (
    "bufio"
    "fmt"
    "github.com/bherbruck/vibeflow/sdk/node"
    "go.bug.st/serial"
)

type SerialNode struct {
    config Config
    port   serial.Port
    stopCh chan struct{}
    done   chan struct{}
}

type Config struct {
    Port     string `json:"port" vf:"required"`
    BaudRate int    `json:"baud_rate" vf:"default=9600"`
    DataBits int    `json:"data_bits" vf:"enum=5|6|7|8,default=8"`
    StopBits string `json:"stop_bits" vf:"enum=1|1.5|2,default=1"`
    Parity   string `json:"parity" vf:"enum=none|odd|even|mark|space,default=none"`
}

func (n *SerialNode) Init(ctx node.InitContext) error {
    if err := ctx.ConfigInto(&n.config); err != nil {
        return err
    }

    mode := &serial.Mode{
        BaudRate: n.config.BaudRate,
        DataBits: n.config.DataBits,
    }

    port, err := serial.Open(n.config.Port, mode)
    if err != nil {
        return fmt.Errorf("failed to open serial port: %w", err)
    }

    n.port = port
    n.stopCh = make(chan struct{})
    n.done = make(chan struct{})

    return nil
}

func (n *SerialNode) Start(ctx node.RunContext) error {
    go n.readLoop(ctx)
    return nil
}

func (n *SerialNode) readLoop(ctx node.RunContext) {
    defer close(n.done)

    reader := bufio.NewReader(n.port)

    for {
        select {
        case <-ctx.Done():
            return
        case <-n.stopCh:
            return
        default:
            line, err := reader.ReadString('\n')
            if err != nil {
                continue
            }
            ctx.Emit("default", map[string]any{
                "data": line,
            })
        }
    }
}

func (n *SerialNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    // Write to serial port
    data, ok := msg.Payload.(map[string]any)["data"].(string)
    if !ok {
        ctx.EmitError(fmt.Errorf("expected {data: string} payload"))
        return
    }

    _, err := n.port.Write([]byte(data))
    if err != nil {
        ctx.EmitError(err)
        return
    }

    ctx.Emit("default", map[string]any{"written": len(data)})
}

func (n *SerialNode) Stop() error {
    close(n.stopCh)
    <-n.done
    return n.port.Close()
}

func main() {
    node.Run(&SerialNode{})
}
```

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-10 | Initial | First draft |

---

## References

- [06-node-manifest.md](./06-node-manifest.md) - Node Manifest Specification
- [03-node-spec.md](./03-node-spec.md) - Node Interface Specification
- [04-message-model.md](./04-message-model.md) - Message Model Specification
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [Hashicorp go-plugin](https://github.com/hashicorp/go-plugin)
