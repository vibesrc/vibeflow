# Vibeflow Node Specification

**Version:** 1.0
**Last Updated:** 2025-12-10
**Status:** Draft

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Node Interface Definition](#2-node-interface-definition)
3. [Node Lifecycle](#3-node-lifecycle)
4. [Configuration Handling](#4-configuration-handling)
5. [Input/Output Ports](#5-inputoutput-ports)
6. [Error Handling](#6-error-handling)
7. [Native Node Development](#7-native-node-development)
8. [Script Node Specification](#8-script-node-specification)
9. [WASI Node Specification](#9-wasi-node-specification)
10. [Node Registry](#10-node-registry)
11. [Best Practices](#11-best-practices)
12. [Open Questions](#12-open-questions)

---

## 1. Introduction

### 1.1 Purpose

This specification defines the contract that all Vibeflow nodes must implement. Nodes are the fundamental building blocks of flows, and consistency in their behavior is critical for predictable flow execution.

### 1.2 Node Categories

Vibeflow supports multiple categories of nodes:

| Category | Implementation | Performance | Isolation | Use Case |
|----------|----------------|-------------|-----------|----------|
| **Built-in (core)** | Compiled Go | Highest | Shared runtime | Essential nodes (inject, debug, switch) |
| **Built-in (contrib)** | Compiled Go | Highest | Shared runtime | Common protocols (HTTP, MQTT) |
| **External** | Separate binary + JSON-RPC | High | Process isolation | Custom protocols, hardware access |
| **Script** | Interpreted (goja) | High | VM-isolated | Simple transformations |
| **WASI** | WebAssembly | Moderate | Sandboxed | Portable, sandboxed extensions |

**Note:** This specification covers the interface for built-in nodes. For external nodes, see [07-node-sdk.md](./07-node-sdk.md).

### 1.3 Node Type Naming

**Convention:** `<namespace>.<name>`

**Examples:**
- `core.inject` - Core inject node
- `protocol.mqtt` - MQTT protocol node
- `custom.mynode` - Custom user node

**Rules:**
- Namespace must be lowercase, alphanumeric with hyphens
- Name must be lowercase, alphanumeric with underscores
- Reserved namespaces: `core`, `protocol`, `wasi`, `module`

---

## 2. Node Interface Definition

### 2.1 Go Interface

All native nodes must implement the `Node` interface:

```go
package node

import (
    "context"
    "github.com/bherbruck/vibeflow/pkg/message"
)

// Node defines the interface that all nodes must implement
type Node interface {
    // Type returns the node type identifier (e.g., "core.inject")
    Type() string

    // Init initializes the node with configuration
    // Called once during flow initialization
    // Returns error if configuration is invalid or initialization fails
    Init(ctx context.Context, config Config) error

    // Process handles an incoming message
    // Returns:
    //   - Single message: forward to default output
    //   - Multiple messages: forward to named outputs
    //   - nil: drop message (no output)
    //   - error: message failed processing
    Process(ctx context.Context, msg *message.Message) (*Result, error)

    // Shutdown performs cleanup when node is stopped
    // Must complete within configured timeout
    Shutdown(ctx context.Context) error
}

// Config holds node configuration
type Config struct {
    // ID is the unique node identifier in the flow
    ID string

    // Name is the optional display name
    Name string

    // Config is the node-specific configuration map
    Config map[string]interface{}

    // Outputs defines the output ports
    Outputs []OutputPort

    // Runtime provides access to runtime services
    Runtime *RuntimeContext
}

// OutputPort defines a named output port
type OutputPort struct {
    Name string
}

// Result represents the outcome of message processing
type Result struct {
    // Messages to emit (nil if none)
    Messages []*OutputMessage

    // Metadata to add to all output messages
    Metadata map[string]string
}

// OutputMessage represents a message for a specific output
type OutputMessage struct {
    // Output port name (empty string = "default")
    Output string

    // Message to emit
    Message *message.Message
}

// RuntimeContext provides access to runtime services
type RuntimeContext struct {
    // Logger for structured logging
    Logger Logger

    // Metrics for observability
    Metrics Metrics

    // Environment variables
    Environment map[string]string
}
```

### 2.2 Node Registration

Nodes must register themselves with the runtime registry:

```go
package mynode

import (
    "github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
    node.Register("custom.mynode", &MyNode{})
}

type MyNode struct {
    config myConfig
}

func (n *MyNode) Type() string {
    return "custom.mynode"
}

// ... implement other interface methods
```

### 2.3 Node Factory Pattern

For nodes requiring complex initialization:

```go
package mynode

import (
    "github.com/bherbruck/vibeflow/pkg/node"
)

type MyNodeFactory struct{}

func (f *MyNodeFactory) Create() node.Node {
    return &MyNode{
        // Initialize with defaults
    }
}

func init() {
    node.RegisterFactory("custom.mynode", &MyNodeFactory{})
}
```

---

## 3. Node Lifecycle

### 3.1 Lifecycle Phases

```
+---------------+
| Registration  |  (init() function, startup)
+---------------+
       |
       v
+---------------+
| Instantiation |  (Factory creates instance)
+---------------+
       |
       v
+---------------+
| Init()        |  (Configuration validation, resource allocation)
+---------------+
       |
       v
+---------------+
| Process()     |  (Message handling, called repeatedly)
+---------------+
       | (repeated)
       v
+---------------+
| Shutdown()    |  (Cleanup, flush buffers)
+---------------+
```

### 3.2 Init Phase

**Purpose:** Validate configuration and allocate resources.

**Responsibilities:**
- Parse and validate node-specific config
- Establish external connections (if needed)
- Allocate buffers, goroutines
- Return error if configuration is invalid

**Contract:**
- Must complete within reasonable time (< 1 second recommended)
- Must be idempotent (safe to call multiple times)
- Must not block indefinitely
- Must not start background goroutines (wait for runtime signal)

**Example:**

```go
func (n *HTTPNode) Init(ctx context.Context, config node.Config) error {
    n.id = config.ID
    n.name = config.Name

    // Parse configuration
    var httpConfig HTTPConfig
    if err := mapstructure.Decode(config.Config, &httpConfig); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }

    // Validate required fields
    if httpConfig.URL == "" {
        return errors.New("missing required field: url")
    }

    // Parse URL
    parsedURL, err := url.Parse(httpConfig.URL)
    if err != nil {
        return fmt.Errorf("invalid url: %w", err)
    }

    n.url = parsedURL
    n.method = httpConfig.Method
    n.timeout = time.Duration(httpConfig.TimeoutMS) * time.Millisecond

    // Create HTTP client
    n.client = &http.Client{
        Timeout: n.timeout,
    }

    return nil
}
```

### 3.3 Process Phase

**Purpose:** Handle incoming messages and produce outputs.

**Responsibilities:**
- Process message payload
- Perform node-specific logic
- Return result or error
- Respect context cancellation

**Contract:**
- Must be goroutine-safe (may be called concurrently)
- Must not block indefinitely (respect ctx.Done())
- Must not mutate input message (clone if modification needed)
- Should complete quickly (< 100ms recommended for non-blocking nodes)

**Return Values:**

| Return | Behavior |
|--------|----------|
| `Result{Messages: [...]}, nil` | Forward messages to outputs |
| `Result{Messages: nil}, nil` | Drop message (no output) |
| `nil, error` | Message processing failed |

**Example:**

```go
func (n *TransformNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    // Clone message to avoid mutation
    outMsg := msg.Clone()

    // Extract payload
    payload, ok := msg.Payload.(map[string]interface{})
    if !ok {
        return nil, errors.New("payload must be JSON object")
    }

    // Apply transformation
    if temp, ok := payload["temperature"].(float64); ok {
        // Convert Celsius to Fahrenheit
        payload["temperature"] = (temp * 9 / 5) + 32
        payload["unit"] = "fahrenheit"
    }

    outMsg.Payload = payload

    return &node.Result{
        Messages: []*node.OutputMessage{
            {
                Output:  "default",
                Message: outMsg,
            },
        },
    }, nil
}
```

**Multi-Output Example:**

```go
func (n *RouterNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    payload, ok := msg.Payload.(map[string]interface{})
    if !ok {
        return nil, errors.New("payload must be JSON object")
    }

    temp, ok := payload["temperature"].(float64)
    if !ok {
        return nil, errors.New("missing temperature field")
    }

    // Route based on temperature
    var outputPort string
    if temp > 80 {
        outputPort = "critical"
    } else if temp > 50 {
        outputPort = "warning"
    } else {
        outputPort = "normal"
    }

    return &node.Result{
        Messages: []*node.OutputMessage{
            {
                Output:  outputPort,
                Message: msg,
            },
        },
    }, nil
}
```

### 3.4 Shutdown Phase

**Purpose:** Clean up resources and flush pending data.

**Responsibilities:**
- Close connections
- Flush buffers
- Stop background goroutines
- Release resources

**Contract:**
- Must complete within configured timeout
- Must be idempotent (safe to call multiple times)
- Should attempt graceful cleanup even if errors occur
- Must not panic

**Example:**

```go
func (n *MQTTNode) Shutdown(ctx context.Context) error {
    n.logger.Info("shutting down MQTT node")

    // Stop accepting new messages
    close(n.stopCh)

    // Wait for in-flight messages to complete
    select {
    case <-n.doneCh:
        n.logger.Info("all messages processed")
    case <-ctx.Done():
        n.logger.Warn("shutdown timeout, forcing close")
    }

    // Disconnect from broker
    if n.client != nil {
        n.client.Disconnect(250) // 250ms quiesce timeout
    }

    return nil
}
```

---

## 4. Configuration Handling

### 4.1 Configuration Structure

Node configuration is provided as a `map[string]interface{}` that must be unmarshaled into a typed struct.

**Recommended Approach:** Use `mapstructure` for flexible unmarshaling.

```go
import "github.com/mitchellh/mapstructure"

type MyNodeConfig struct {
    URL       string            `mapstructure:"url"`
    Method    string            `mapstructure:"method"`
    Headers   map[string]string `mapstructure:"headers"`
    TimeoutMS int               `mapstructure:"timeout_ms"`
}

func (n *MyNode) Init(ctx context.Context, config node.Config) error {
    var cfg MyNodeConfig
    if err := mapstructure.Decode(config.Config, &cfg); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }

    // Set defaults
    if cfg.Method == "" {
        cfg.Method = "GET"
    }
    if cfg.TimeoutMS == 0 {
        cfg.TimeoutMS = 5000
    }

    n.config = cfg
    return nil
}
```

### 4.2 Validation

**Required Fields:**

```go
if cfg.URL == "" {
    return errors.New("missing required field: url")
}
```

**Field Constraints:**

```go
if cfg.TimeoutMS < 0 {
    return errors.New("timeout_ms must be >= 0")
}

if cfg.Method != "GET" && cfg.Method != "POST" && cfg.Method != "PUT" {
    return fmt.Errorf("invalid method: %s", cfg.Method)
}
```

**Cross-Field Validation:**

```go
if cfg.Auth == "bearer" && cfg.Token == "" {
    return errors.New("token required when auth is 'bearer'")
}
```

### 4.3 Environment Variable Substitution

The runtime resolves environment variables before passing config to nodes:

**Flow YAML:**
```yaml
nodes:
  - id: api
    type: core.http
    config:
      url: "+{API_URL}"
      token: "+{API_TOKEN}"
```

**Runtime resolves to:**
```go
config.Config = map[string]interface{}{
    "url":   "https://api.example.com",
    "token": "secret-token-value",
}
```

Nodes should **not** perform environment substitution themselves.

### 4.4 Default Values

Nodes should provide sensible defaults:

```go
type HTTPConfig struct {
    URL       string            `mapstructure:"url"`
    Method    string            `mapstructure:"method"`
    Headers   map[string]string `mapstructure:"headers"`
    TimeoutMS int               `mapstructure:"timeout_ms"`
}

func (c *HTTPConfig) SetDefaults() {
    if c.Method == "" {
        c.Method = "GET"
    }
    if c.TimeoutMS == 0 {
        c.TimeoutMS = 5000
    }
    if c.Headers == nil {
        c.Headers = make(map[string]string)
    }
}
```

---

## 5. Input/Output Ports

### 5.1 Default Ports

By default, nodes have:
- **One unnamed input** (receives all messages from wires)
- **One unnamed output** (default output port)

### 5.2 Named Outputs

Nodes can declare multiple named outputs:

```go
func (n *HTTPNode) Init(ctx context.Context, config node.Config) error {
    // Declare outputs
    config.Outputs = []node.OutputPort{
        {Name: "success"},
        {Name: "error"},
        {Name: "timeout"},
    }
    // ...
}
```

**Flow YAML:**
```yaml
nodes:
  - id: http1
    type: core.http
    outputs:
      - name: success
      - name: error
      - name: timeout
wires:
  - from: http1
    output: success
    to: process_response
  - from: http1
    output: error
    to: log_error
  - from: http1
    output: timeout
    to: retry
```

### 5.3 Named Inputs

**Future Feature:** Named inputs are reserved for future implementation.

**Proposed Usage:**
```yaml
nodes:
  - id: join1
    type: core.join
    inputs:
      - name: source1
      - name: source2
wires:
  - from: sensor1
    to: join1
    input: source1
  - from: sensor2
    to: join1
    input: source2
```

### 5.4 Dynamic Outputs

For nodes that emit to different outputs based on runtime logic:

```go
func (n *SwitchNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    value := msg.Payload.(int)

    var output string
    switch {
    case value < 10:
        output = "low"
    case value < 50:
        output = "medium"
    default:
        output = "high"
    }

    return &node.Result{
        Messages: []*node.OutputMessage{
            {
                Output:  output,
                Message: msg,
            },
        },
    }, nil
}
```

---

## 6. Error Handling

### 6.1 Error Types

Nodes can encounter different types of errors:

| Error Type | Handling | Example |
|------------|----------|---------|
| **Configuration Error** | Fail during Init | Invalid URL, missing field |
| **Processing Error** | Return error from Process | Network failure, parse error |
| **Transient Error** | Retry if applicable | Temporary network glitch |
| **Fatal Error** | Stop node execution | Resource exhaustion |

### 6.2 Returning Errors

```go
func (n *MyNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    // Validation error
    if msg.Payload == nil {
        return nil, errors.New("payload cannot be nil")
    }

    // Operation error
    result, err := n.performOperation(msg)
    if err != nil {
        return nil, fmt.Errorf("operation failed: %w", err)
    }

    return result, nil
}
```

### 6.3 Error Outputs

Prefer error outputs over returning errors when the error is part of normal operation:

```go
func (n *HTTPNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    resp, err := n.client.Do(req)
    if err != nil {
        // Network error - emit to error output
        errMsg := msg.Clone()
        errMsg.Metadata["error"] = err.Error()

        return &node.Result{
            Messages: []*node.OutputMessage{
                {
                    Output:  "error",
                    Message: errMsg,
                },
            },
        }, nil
    }

    // Success - emit to success output
    successMsg := msg.Clone()
    successMsg.Payload = resp.Body

    return &node.Result{
        Messages: []*node.OutputMessage{
            {
                Output:  "success",
                Message: successMsg,
            },
        },
    }, nil
}
```

### 6.4 Logging Errors

Use structured logging:

```go
func (n *MyNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    if err := n.validate(msg); err != nil {
        n.logger.Error("validation failed",
            "node_id", n.id,
            "message_id", msg.ID,
            "error", err,
        )
        return nil, err
    }
    // ...
}
```

### 6.5 Panic Recovery

The runtime provides panic recovery, but nodes should avoid panics:

```go
// BAD: May panic
value := msg.Payload.(int)

// GOOD: Safe type assertion
value, ok := msg.Payload.(int)
if !ok {
    return nil, errors.New("payload must be integer")
}
```

---

## 7. Native Node Development

### 7.1 Project Structure

```
mynode/
+ node.go           # Node implementation
+ config.go         # Configuration struct
+ node_test.go      # Unit tests
+ manifest.yaml     # Node manifest
+ README.md         # Documentation
```

### 7.2 Complete Example: Transform Node

**node.go:**

```go
package transform

import (
    "context"
    "fmt"

    "github.com/bherbruck/vibeflow/pkg/message"
    "github.com/bherbruck/vibeflow/pkg/node"
    "github.com/mitchellh/mapstructure"
)

func init() {
    node.Register("core.transform", &TransformNode{})
}

type TransformNode struct {
    id      string
    name    string
    config  TransformConfig
    logger  node.Logger
}

func (n *TransformNode) Type() string {
    return "core.transform"
}

func (n *TransformNode) Init(ctx context.Context, config node.Config) error {
    n.id = config.ID
    n.name = config.Name
    n.logger = config.Runtime.Logger

    var cfg TransformConfig
    if err := mapstructure.Decode(config.Config, &cfg); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }

    if err := cfg.Validate(); err != nil {
        return err
    }

    cfg.SetDefaults()
    n.config = cfg

    n.logger.Info("transform node initialized",
        "id", n.id,
        "operation", cfg.Operation,
    )

    return nil
}

func (n *TransformNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    outMsg := msg.Clone()

    switch n.config.Operation {
    case "uppercase":
        if str, ok := msg.Payload.(string); ok {
            outMsg.Payload = strings.ToUpper(str)
        }
    case "lowercase":
        if str, ok := msg.Payload.(string); ok {
            outMsg.Payload = strings.ToLower(str)
        }
    case "multiply":
        if num, ok := msg.Payload.(float64); ok {
            outMsg.Payload = num * n.config.Factor
        }
    default:
        return nil, fmt.Errorf("unknown operation: %s", n.config.Operation)
    }

    return &node.Result{
        Messages: []*node.OutputMessage{
            {Output: "default", Message: outMsg},
        },
    }, nil
}

func (n *TransformNode) Shutdown(ctx context.Context) error {
    n.logger.Info("transform node shutdown", "id", n.id)
    return nil
}
```

**config.go:**

```go
package transform

import "errors"

type TransformConfig struct {
    Operation string  `mapstructure:"operation"`
    Factor    float64 `mapstructure:"factor"`
}

func (c *TransformConfig) Validate() error {
    validOps := map[string]bool{
        "uppercase": true,
        "lowercase": true,
        "multiply":  true,
    }

    if !validOps[c.Operation] {
        return errors.New("operation must be: uppercase, lowercase, multiply")
    }

    if c.Operation == "multiply" && c.Factor == 0 {
        return errors.New("factor required for multiply operation")
    }

    return nil
}

func (c *TransformConfig) SetDefaults() {
    if c.Factor == 0 && c.Operation == "multiply" {
        c.Factor = 1.0
    }
}
```

### 7.3 Testing Nodes

**node_test.go:**

```go
package transform

import (
    "context"
    "testing"

    "github.com/bherbruck/vibeflow/pkg/message"
    "github.com/bherbruck/vibeflow/pkg/node"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestTransformNode_Uppercase(t *testing.T) {
    n := &TransformNode{}

    cfg := node.Config{
        ID: "transform1",
        Config: map[string]interface{}{
            "operation": "uppercase",
        },
        Runtime: &node.RuntimeContext{
            Logger: &mockLogger{},
        },
    }

    err := n.Init(context.Background(), cfg)
    require.NoError(t, err)

    msg := &message.Message{
        ID:      "msg1",
        Payload: "hello world",
    }

    result, err := n.Process(context.Background(), msg)
    require.NoError(t, err)
    require.NotNil(t, result)
    require.Len(t, result.Messages, 1)

    assert.Equal(t, "HELLO WORLD", result.Messages[0].Message.Payload)
}

func TestTransformNode_InvalidOperation(t *testing.T) {
    n := &TransformNode{}

    cfg := node.Config{
        ID: "transform1",
        Config: map[string]interface{}{
            "operation": "invalid",
        },
        Runtime: &node.RuntimeContext{
            Logger: &mockLogger{},
        },
    }

    err := n.Init(context.Background(), cfg)
    assert.Error(t, err)
}
```

---

## 8. Script Node Specification

### 8.1 Script Node Interface

Script nodes are a special node type that execute user-provided code in an embedded interpreter.

**Supported Languages:**
- JavaScript (ES5.1) via goja
- Go via yaegi
- Tengo
- Starlark (Python-like)

### 8.2 Script Execution Model

```go
type ScriptNode struct {
    id       string
    language string
    code     string
    vm       ScriptVM
}

type ScriptVM interface {
    Execute(ctx context.Context, msg *message.Message) (*message.Message, error)
}
```

### 8.3 JavaScript (goja)

**Configuration:**

```yaml
- id: script1
  type: core.script
  language: javascript
  config:
    code: |
      msg.payload = msg.payload.toUpperCase();
      return msg;
```

**API Available to Scripts:**

```javascript
// Input message
msg = {
  id: "msg-123",
  metadata: {"timestamp": "2025-12-10T..."},
  payload: {...},
  context: {...}
}

// Return modified message
return msg;

// Drop message
return null;

// Emit to named output
return {output: "success", msg: msg};

// Emit multiple messages
return [
  {output: "out1", msg: msg1},
  {output: "out2", msg: msg2}
];
```

**Example:**

```javascript
// Parse JSON string
if (typeof msg.payload === 'string') {
  msg.payload = JSON.parse(msg.payload);
}

// Transform data
msg.payload.temperature = (msg.payload.temperature * 9/5) + 32;
msg.payload.unit = "fahrenheit";

// Add metadata
msg.metadata.processed_at = new Date().toISOString();

return msg;
```

### 8.4 Go Scripts (yaegi)

**Configuration:**

```yaml
- id: script1
  type: core.script
  language: go
  config:
    code: |
      package main
      import "strings"

      func Process(msg Message) Message {
        if s, ok := msg.Payload.(string); ok {
          msg.Payload = strings.ToUpper(s)
        }
        return msg
      }
```

### 8.5 Tengo

**Configuration:**

```yaml
- id: script1
  type: core.script
  language: tengo
  config:
    code: |
      msg := import("msg")
      msg.payload = msg.payload + " processed"
      return msg
```

### 8.6 Starlark

**Configuration:**

```yaml
- id: script1
  type: core.script
  language: starlark
  config:
    code: |
      def process(msg):
        msg["payload"] = msg["payload"].upper()
        return msg
```

### 8.7 Script Node Security

**Restrictions:**
- No filesystem access (except via approved API)
- No network access (except via approved API)
- No subprocess execution
- CPU and memory limits enforced

**Timeout:**
```yaml
runtime:
  script_timeout_ms: 1000  # Kill scripts after 1 second
```

---

## 9. WASI Node Specification

### 9.1 Overview

WASI nodes run WebAssembly modules with WASI (WebAssembly System Interface) support, enabling language-agnostic node development.

### 9.2 WASI Module Interface

**ABI Contract:** WASI modules must export a `process` function:

```wasm
(func +process (param +msg_ptr i32) (param +msg_len i32) (result i32))
```

**Function Signature:**
- **Input:** Pointer and length to serialized message (JSON or MessagePack)
- **Output:** Pointer to serialized result (JSON or MessagePack)
- **Memory:** Module allocates result, runtime frees

### 9.3 Message Serialization

**JSON Format:**

```json
{
  "id": "msg-123",
  "metadata": {"timestamp": "2025-12-10T12:00:00Z"},
  "payload": {"temperature": 25},
  "context": {}
}
```

**Result Format:**

```json
{
  "messages": [
    {
      "output": "default",
      "message": {
        "id": "msg-124",
        "metadata": {"processed": "true"},
        "payload": {"temperature": 77},
        "context": {}
      }
    }
  ]
}
```

### 9.4 Example: Rust WASI Node

**Cargo.toml:**

```toml
[package]
name = "my_wasi_node"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

[lib]
crate-type = ["cdylib"]
```

**lib.rs:**

```rust
use serde::{Deserialize, Serialize};
use std::ffi::CStr;
use std::os::raw::c_char;

#[derive(Deserialize, Serialize)]
struct Message {
    id: String,
    metadata: std::collections::HashMap<String, String>,
    payload: serde_json::Value,
    context: serde_json::Value,
}

#[derive(Serialize)]
struct OutputMessage {
    output: String,
    message: Message,
}

#[derive(Serialize)]
struct Result {
    messages: Vec<OutputMessage>,
}

#[no_mangle]
pub extern "C" fn process(msg_ptr: *const c_char, msg_len: usize) -> *const c_char {
    // Parse input message
    let msg_bytes = unsafe { std::slice::from_raw_parts(msg_ptr as *const u8, msg_len) };
    let mut msg: Message = serde_json::from_slice(msg_bytes).unwrap();

    // Transform payload
    if let Some(temp) = msg.payload.get("temperature").and_then(|v| v.as_f64()) {
        let fahrenheit = (temp * 9.0 / 5.0) + 32.0;
        msg.payload["temperature"] = serde_json::json!(fahrenheit);
        msg.payload["unit"] = serde_json::json!("fahrenheit");
    }

    // Build result
    let result = Result {
        messages: vec![OutputMessage {
            output: "default".to_string(),
            message: msg,
        }],
    };

    // Serialize result
    let result_json = serde_json::to_string(&result).unwrap();
    let c_str = std::ffi::CString::new(result_json).unwrap();
    c_str.into_raw()
}
```

**Build:**

```bash
cargo build --target wasm32-wasi --release
```

### 9.5 WASI Node Configuration

```yaml
- id: wasi1
  type: wasi.custom
  config:
    module_path: "./wasm/my_node.wasm"
    function: "process"
    memory_limit_mb: 64
    timeout_ms: 5000
```

### 9.6 WASI Runtime Integration

The Vibeflow runtime loads WASI modules using `wazero`:

```go
package wasi

import (
    "context"
    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type WASINode struct {
    runtime wazero.Runtime
    module  wazero.CompiledModule
    config  WASIConfig
}

func (n *WASINode) Init(ctx context.Context, config node.Config) error {
    // Create WASI runtime
    n.runtime = wazero.NewRuntime(ctx)

    // Instantiate WASI
    wasi_snapshot_preview1.Instantiate(ctx, n.runtime)

    // Load module
    wasmBytes, err := os.ReadFile(n.config.ModulePath)
    if err != nil {
        return err
    }

    n.module, err = n.runtime.CompileModule(ctx, wasmBytes)
    return err
}

func (n *WASINode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    // Serialize message to JSON
    msgJSON, err := json.Marshal(msg)
    if err != nil {
        return nil, err
    }

    // Call WASI function
    result, err := n.module.ExportedFunction(n.config.Function).Call(ctx, msgJSON)
    if err != nil {
        return nil, err
    }

    // Deserialize result
    var nodeResult node.Result
    if err := json.Unmarshal(result, &nodeResult); err != nil {
        return nil, err
    }

    return &nodeResult, nil
}
```

---

## 10. Node Registry

### 10.1 Registration

Nodes register themselves during package initialization:

```go
package inject

import "github.com/bherbruck/vibeflow/pkg/node"

func init() {
    node.Register("core.inject", &InjectNode{})
}
```

### 10.2 Registry Interface

```go
package node

var registry = make(map[string]Node)

func Register(nodeType string, instance Node) {
    if _, exists := registry[nodeType]; exists {
        panic(fmt.Sprintf("node type already registered: %s", nodeType))
    }
    registry[nodeType] = instance
}

func Get(nodeType string) (Node, error) {
    instance, exists := registry[nodeType]
    if !exists {
        return nil, fmt.Errorf("unknown node type: %s", nodeType)
    }
    return instance, nil
}

func List() []string {
    types := make([]string, 0, len(registry))
    for nodeType := range registry {
        types = append(types, nodeType)
    }
    return types
}
```

### 10.3 Dynamic Loading

**Future Feature:** Load nodes from external plugins:

```go
func LoadPlugin(path string) error {
    plugin, err := plugin.Open(path)
    if err != nil {
        return err
    }

    registerFunc, err := plugin.Lookup("Register")
    if err != nil {
        return err
    }

    registerFunc.(func())()
    return nil
}
```

---

## 11. Best Practices

### 11.1 Performance

1. **Avoid Allocations in Hot Path**
```go
// BAD: Allocates on every call
func (n *MyNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    result := &node.Result{
        Messages: []*node.OutputMessage{
            {Output: "default", Message: msg},
        },
    }
    return result, nil
}

// GOOD: Reuse pool
var resultPool = sync.Pool{
    New: func() interface{} {
        return &node.Result{
            Messages: make([]*node.OutputMessage, 0, 1),
        }
    },
}

func (n *MyNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    result := resultPool.Get().(*node.Result)
    result.Messages = append(result.Messages, &node.OutputMessage{
        Output: "default", Message: msg,
    })
    return result, nil
}
```

2. **Clone Messages Only When Necessary**
```go
// If not modifying message, return original
if !needsTransformation {
    return &node.Result{
        Messages: []*node.OutputMessage{{Output: "default", Message: msg}},
    }, nil
}

// Clone only if modifying
outMsg := msg.Clone()
outMsg.Payload = transformed
```

### 11.2 Concurrency

1. **Make Nodes Goroutine-Safe**
```go
type MyNode struct {
    mu     sync.RWMutex
    cache  map[string]interface{}
}

func (n *MyNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    n.mu.RLock()
    val := n.cache[key]
    n.mu.RUnlock()
    // ...
}
```

2. **Respect Context Cancellation**
```go
func (n *MyNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Long operation
    result := n.slowOperation(ctx)
    return result, nil
}
```

### 11.3 Error Handling

1. **Use Structured Errors**
```go
type ValidationError struct {
    Field string
    Value interface{}
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("invalid %s: %v", e.Field, e.Value)
}
```

2. **Log Errors with Context**
```go
n.logger.Error("processing failed",
    "node_id", n.id,
    "message_id", msg.ID,
    "error", err,
    "retry_count", retryCount,
)
```

### 11.4 Documentation

1. **Document Configuration**
```go
// HTTPConfig defines the configuration for HTTP request nodes
type HTTPConfig struct {
    // URL is the target endpoint (required)
    URL string `mapstructure:"url"`

    // Method is the HTTP method (default: GET)
    Method string `mapstructure:"method"`

    // TimeoutMS is the request timeout in milliseconds (default: 5000)
    TimeoutMS int `mapstructure:"timeout_ms"`
}
```

2. **Provide Examples**
```yaml
# Example: Fetch data from API
- id: fetch_weather
  type: core.http
  config:
    url: "https://api.weather.com/current"
    method: "GET"
    headers:
      Authorization: "Bearer +{API_TOKEN}"
    timeout_ms: 5000
```

---

## 12. Open Questions

### 12.1 Named Inputs

**Question:** Should nodes support named inputs?

**Use Case:** Join nodes that correlate messages from multiple sources.

**Proposal:** Add `Inputs` field to node interface.

### 12.2 Streaming Nodes

**Question:** How should nodes handle streaming data (e.g., TCP connections)?

**Options:**
1. Generate multiple messages (one per chunk)
2. Stream into single message with stream API
3. Background goroutine with message emission

### 12.3 Stateful Nodes

**Question:** Should nodes have built-in state management?

**Current:** Nodes manage state internally (maps, caches)
**Alternative:** Runtime provides key-value store API

### 12.4 Node Versioning

**Question:** How should node versioning work?

**Proposal:**
```yaml
- id: http1
  type: core.http
  version: "^2.0.0"
```

Runtime loads compatible version or fails.

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-10 | Initial | First draft |

---

## References

- [01-overview.md](./01-overview.md) - Project Overview
- [02-flow-format.md](./02-flow-format.md) - Flow Format Specification
- [04-message-model.md](./04-message-model.md) - Message Model Specification
- [05-runtime.md](./05-runtime.md) - Runtime Architecture
- [06-node-manifest.md](./06-node-manifest.md) - Node Manifest and Distribution
- [07-node-sdk.md](./07-node-sdk.md) - Node SDK and RPC Protocol (for external nodes)
