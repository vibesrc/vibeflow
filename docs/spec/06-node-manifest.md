# Vibeflow Node Manifest and Packaging Specification

**Version:** 2.0
**Last Updated:** 2025-12-10
**Status:** Draft

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Node Types](#2-node-types)
3. [Manifest Schema](#3-manifest-schema)
4. [Configuration Schema](#4-configuration-schema)
5. [Built-in Nodes](#5-built-in-nodes)
6. [External Nodes](#6-external-nodes)
7. [Script Nodes](#7-script-nodes)
8. [Module Format](#8-module-format)
9. [GitHub Release Distribution](#9-github-release-distribution)
10. [Versioning Requirements](#10-versioning-requirements)
11. [Permission Model](#11-permission-model)
12. [Concurrency Model](#12-concurrency-model)
13. [Node Discovery and Installation](#13-node-discovery-and-installation)
14. [Publishing Guidelines](#14-publishing-guidelines)
15. [Best Practices](#15-best-practices)

---

## 1. Introduction

### 1.1 Purpose

This specification defines how nodes are packaged, distributed, and integrated into Vibeflow. A node manifest describes a node's interface, configuration schema, dependencies, and resource requirements.

### 1.2 Design Goals

1. **Zero-friction distribution**: External nodes distributed via GitHub releases
2. **Docker-native**: Users run a single Docker image, nodes are downloaded on demand
3. **Full OS access**: External nodes can use TCP, UDP, serial, GPIO, SPI, I2C, etc.
4. **Crash isolation**: External nodes run as separate processes
5. **Simple authoring**: Go SDK handles all boilerplate

### 1.3 Deployment Model

Users deploy Vibeflow via Docker with a configuration file:

```bash
docker run -v ./config:/config ghcr.io/bherbruck/vibeflow
```

```yaml
# /config/vibeflow.yaml
nodes:
  - github.com/vibeflow/nodes-contrib/modbus@v1.2.0
  - github.com/company/custom-gpio@v0.5.0

flows:
  - /config/flows/main.yaml
```

On startup, the runtime:
1. Downloads external node binaries from GitHub releases
2. Verifies checksums
3. Caches in a volume
4. Spawns node processes as needed

---

## 2. Node Types

### 2.1 Overview

| Type | Location | Execution | Use Case |
|------|----------|-----------|----------|
| **Built-in (core)** | Compiled into runtime | In-process | Essential nodes (inject, debug, switch) |
| **Built-in (contrib)** | Compiled into runtime | In-process | Common protocols (HTTP, MQTT) |
| **External** | Separate binary | Subprocess + JSON-RPC | Custom protocols, hardware, third-party |
| **Script** | Inline in flow YAML | Embedded interpreter | Simple transformations |
| **Module** | Flow YAML file | Sub-flow expansion | Reusable flow patterns |

### 2.2 Decision Guide

```
Need custom protocol or hardware access?
  |
  +--> YES --> External Node (separate binary)
  |
  +--> NO --> Is it a simple transformation?
                |
                +--> YES --> Script Node (inline JS)
                |
                +--> NO --> Is it reusable logic?
                              |
                              +--> YES --> Module (sub-flow)
                              |
                              +--> NO --> Use built-in nodes
```

### 2.3 Performance Characteristics

| Type | Call Overhead | Memory | Crash Isolation |
|------|---------------|--------|-----------------|
| Built-in | ~10ns | Shared | No |
| External | ~1-10us (JSON-RPC) | Separate process | Yes |
| Script | ~100-500ns | Shared | No |
| Module | ~10ns (inlined) | Shared | No |

---

## 3. Manifest Schema

### 3.1 Manifest File

**File Name:** `vibeflow-node.yaml`

**Location:**
- Built-in: Embedded in source code
- External: In release tarball root
- Module: Alongside module YAML

### 3.2 Top-Level Schema

```yaml
# Manifest schema version
manifest_version: "2.0"

# Node metadata
metadata:
  name: string (required)        # Human-readable name
  namespace: string (required)   # e.g., "core", "contrib", "protocol", "hardware"
  type_name: string (required)   # Node type identifier, e.g., "modbus-tcp"
  version: string (required)     # Semver
  description: string
  author: string
  license: string
  repository: string             # GitHub repo URL
  tags: list[string]

# Node type
type: string (required)  # builtin, external, script, module

# Node interface
interface:
  inputs: list[Input]
  outputs: list[Output]
  config_schema: object (JSON Schema)

# For external nodes: execution settings
execution:
  binary_name: string            # e.g., "modbus-node"

# Concurrency settings
concurrency:
  mode: string                   # single, parallel, worker_pool
  max_parallel: integer          # For parallel/worker_pool modes

# Resource requirements
resources:
  max_memory_mb: integer
  max_cpu_percent: integer

# Permissions (informational for external nodes)
permissions:
  network: bool
  serial: bool
  gpio: bool
  filesystem: bool

# Dependencies
dependencies:
  runtime_version: string        # Minimum Vibeflow version

# Documentation
examples:
  - name: string
    description: string
    flow: string (YAML)

# Testing
tests:
  - name: string
    input: object
    expected_output: object
```

### 3.3 Example: External Modbus TCP Node

```yaml
manifest_version: "2.0"

metadata:
  name: "Modbus TCP"
  namespace: "protocol"
  type_name: "modbus-tcp"
  version: "1.2.0"
  description: "Modbus TCP client for reading/writing registers"
  author: "Vibeflow Contributors"
  license: "MIT"
  repository: "https://github.com/vibeflow/nodes-contrib"
  tags:
    - modbus
    - protocol
    - industrial
    - plc

type: external

interface:
  inputs:
    - name: default
      description: "Trigger a read/write operation"
    - name: trigger
      description: "Alternative trigger input"
  outputs:
    - name: default
      description: "Successful read/write result"
    - name: error
      description: "Operation failed"
  config_schema:
    type: object
    required:
      - address
      - slave_id
    properties:
      address:
        type: string
        pattern: "^[^:]+:[0-9]+$"
        description: "Modbus TCP address (host:port)"
        examples:
          - "192.168.1.100:502"
      slave_id:
        type: integer
        minimum: 1
        maximum: 247
        description: "Modbus slave ID"
      function:
        type: string
        enum:
          - read_coils
          - read_discrete_inputs
          - read_holding_registers
          - read_input_registers
          - write_single_coil
          - write_single_register
          - write_multiple_coils
          - write_multiple_registers
        default: read_holding_registers
      start_address:
        type: integer
        minimum: 0
        maximum: 65535
        default: 0
      count:
        type: integer
        minimum: 1
        maximum: 125
        default: 1
      poll_interval_ms:
        type: integer
        minimum: 0
        default: 0
        description: "Polling interval (0 = trigger only)"

execution:
  binary_name: "vibeflow-node-modbus-tcp"

concurrency:
  mode: single

resources:
  max_memory_mb: 64

permissions:
  network: true

dependencies:
  runtime_version: ">=0.1.0"

examples:
  - name: "Poll holding registers"
    description: "Read 10 registers every second"
    flow: |
      nodes:
        - id: modbus
          type: protocol.modbus-tcp
          config:
            address: "192.168.1.100:502"
            slave_id: 1
            function: read_holding_registers
            start_address: 30001
            count: 10
            poll_interval_ms: 1000
        - id: debug
          type: core.debug
      wires:
        - from: modbus
          to: debug
```

---

## 4. Configuration Schema

### 4.1 JSON Schema Format

Node configuration uses JSON Schema (Draft 7+) for validation.

**Benefits:**
- Standard format with tooling support
- Automatic validation at startup
- Documentation generation
- IDE autocomplete (with LSP)
- Auto-generation from Go struct tags (via SDK)

### 4.2 SDK Auto-Generation

The Vibeflow SDK generates JSON Schema from Go struct tags:

```go
type Config struct {
    Address   string `json:"address" vf:"required,pattern=^[^:]+:[0-9]+$"`
    SlaveID   int    `json:"slave_id" vf:"required,min=1,max=247"`
    Function  string `json:"function" vf:"enum=read_coils|read_holding_registers,default=read_holding_registers"`
    PollMS    int    `json:"poll_interval_ms" vf:"min=0,default=0"`
}
```

Generates:

```yaml
config_schema:
  type: object
  required: [address, slave_id]
  properties:
    address:
      type: string
      pattern: "^[^:]+:[0-9]+$"
    slave_id:
      type: integer
      minimum: 1
      maximum: 247
    function:
      type: string
      enum: [read_coils, read_holding_registers]
      default: read_holding_registers
    poll_interval_ms:
      type: integer
      minimum: 0
      default: 0
```

### 4.3 Schema Extensions

**Vibeflow-specific extensions:**

```yaml
config_schema:
  properties:
    api_key:
      type: string
      x-vf-secret: true           # Mask in logs/UI
      x-vf-env-var: true          # Allow ${VAR} substitution
    serial_port:
      type: string
      x-vf-serial-port: true      # UI hint: show port picker
    gpio_pin:
      type: integer
      x-vf-gpio-pin: true         # UI hint: show pin selector
```

---

## 5. Built-in Nodes

### 5.1 Core Nodes

Core nodes are essential functionality compiled into the runtime. They cannot be removed.

| Node | Type | Description |
|------|------|-------------|
| `core.inject` | Source | Inject messages on timer or startup |
| `core.debug` | Sink | Log messages for debugging |
| `core.switch` | Router | Route messages based on conditions |
| `core.change` | Transform | Set, change, delete message properties |
| `core.template` | Transform | Generate output using templates |
| `core.delay` | Flow | Delay or rate-limit messages |
| `core.filter` | Flow | Filter messages by condition |
| `core.join` | Flow | Join multiple messages into one |
| `core.split` | Flow | Split message into multiple |

### 5.2 Contrib Nodes

Contrib nodes are common functionality compiled into the runtime. They ship with the official Docker image.

| Node | Type | Description |
|------|------|-------------|
| `contrib.http-request` | IO | Make HTTP/HTTPS requests |
| `contrib.http-in` | Source | HTTP server endpoint |
| `contrib.mqtt-in` | Source | MQTT subscriber |
| `contrib.mqtt-out` | Sink | MQTT publisher |
| `contrib.tcp-in` | Source | TCP server/client |
| `contrib.tcp-out` | Sink | TCP client |
| `contrib.udp-in` | Source | UDP listener |
| `contrib.udp-out` | Sink | UDP sender |
| `contrib.websocket-in` | Source | WebSocket server/client |
| `contrib.websocket-out` | Sink | WebSocket client |
| `contrib.file-in` | Source | Read/watch files |
| `contrib.file-out` | Sink | Write files |
| `contrib.exec` | IO | Execute shell commands |

### 5.3 Built-in Node Implementation

Built-in nodes implement the `node.Node` interface directly:

```go
package core

import (
    "github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
    node.Register("core.inject", &InjectNodeFactory{})
}

type InjectNode struct {
    config InjectConfig
    ticker *time.Ticker
}

func (n *InjectNode) Init(ctx node.InitContext) error {
    return ctx.ConfigInto(&n.config)
}

func (n *InjectNode) Start(ctx node.RunContext) error {
    if n.config.IntervalMS > 0 {
        n.ticker = time.NewTicker(time.Duration(n.config.IntervalMS) * time.Millisecond)
        go n.poll(ctx)
    }
    if n.config.OnStart {
        ctx.Emit("default", n.config.Payload)
    }
    return nil
}

func (n *InjectNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    // Inject nodes don't process input messages
}

func (n *InjectNode) Stop() error {
    if n.ticker != nil {
        n.ticker.Stop()
    }
    return nil
}
```

---

## 6. External Nodes

### 6.1 Architecture

External nodes run as separate processes, communicating with the runtime via JSON-RPC over stdin/stdout:

```
+-------------------+          stdin/stdout         +-------------------+
|    Vibeflow       |  <=========================>  |   External Node   |
|    Runtime        |         JSON-RPC 2.0          |   (Go binary)     |
+-------------------+                               +-------------------+
                                                    - Full OS access
                                                    - TCP, UDP, serial
                                                    - GPIO, SPI, I2C
                                                    - Any Go library
```

### 6.2 Why External Nodes?

| Requirement | Solution |
|-------------|----------|
| Custom protocol (Modbus RTU, BACnet, etc.) | Full access to serial ports |
| Hardware access (GPIO, SPI, I2C) | Direct hardware control |
| Third-party libraries | Use any Go library |
| Crash isolation | Node crash doesn't kill runtime |
| Platform-specific code | Compile per-platform binaries |

### 6.3 External Node Lifecycle

```
1. Runtime parses flow, finds external node reference
         |
         v
2. Download binary from GitHub release (if not cached)
         |
         v
3. Verify SHA256 checksum
         |
         v
4. Spawn process: ./vibeflow-node-modbus-tcp
         |
         v
5. Send "init" RPC with config
         |
         v
6. Node responds "ready"
         |
         v
7. Runtime sends messages via "process" RPC
         |
         v
8. Node sends messages back via "emit" notification
         |
         v
9. On shutdown, runtime sends "stop" RPC
         |
         v
10. Node exits cleanly
```

### 6.4 JSON-RPC Protocol

See [07-node-sdk.md](./07-node-sdk.md) for the complete RPC protocol specification.

**Key methods:**

| Method | Direction | Description |
|--------|-----------|-------------|
| `init` | Runtime -> Node | Initialize with config |
| `process` | Runtime -> Node | Process incoming message |
| `emit` | Node -> Runtime | Emit output message |
| `log` | Node -> Runtime | Send log message |
| `stop` | Runtime -> Node | Graceful shutdown |

### 6.5 Writing External Nodes

External nodes use the Vibeflow SDK for Go:

```go
package main

import (
    "github.com/bherbruck/vibeflow/sdk/node"
    "github.com/goburrow/modbus"
)

type ModbusTCPNode struct {
    client modbus.Client
    config Config
}

type Config struct {
    Address  string `json:"address" vf:"required"`
    SlaveID  int    `json:"slave_id" vf:"required,min=1,max=247"`
    Function string `json:"function" vf:"default=read_holding_registers"`
}

func (n *ModbusTCPNode) Init(ctx node.InitContext) error {
    if err := ctx.ConfigInto(&n.config); err != nil {
        return err
    }

    handler := modbus.NewTCPClientHandler(n.config.Address)
    handler.SlaveId = byte(n.config.SlaveID)
    n.client = modbus.NewClient(handler)
    return handler.Connect()
}

func (n *ModbusTCPNode) OnMessage(ctx node.MessageContext, msg node.Message) {
    results, err := n.client.ReadHoldingRegisters(0, 10)
    if err != nil {
        ctx.Emit("error", node.ErrorPayload(err))
        return
    }
    ctx.Emit("default", results)
}

func (n *ModbusTCPNode) Stop() error {
    return nil
}

func main() {
    node.Run(&ModbusTCPNode{})
}
```

---

## 7. Script Nodes

### 7.1 Purpose

Script nodes allow inline code for simple transformations without creating external binaries.

### 7.2 Supported Languages

| Language | Engine | Use Case |
|----------|--------|----------|
| JavaScript (ES5.1) | goja | General transformations |

### 7.3 Inline Script Example

```yaml
nodes:
  - id: transform
    type: core.script
    config:
      language: javascript
      code: |
        // Convert Fahrenheit to Celsius
        var fahrenheit = msg.payload.temperature;
        var celsius = (fahrenheit - 32) * 5/9;
        msg.payload.temperature = celsius;
        msg.payload.unit = "celsius";
        return msg;
```

### 7.4 Script API

Scripts receive a `msg` object and must return a message (or null to drop):

```javascript
// msg object structure
{
  id: "uuid",
  payload: { /* any data */ },
  metadata: { /* key-value pairs */ },
  context: { /* flow context */ }
}

// Return message to default output
return msg;

// Return to specific output
return { output: "error", msg: msg };

// Drop message
return null;
```

### 7.5 Script Limitations

- No network access
- No file access
- No external modules/require
- 5 second execution timeout (configurable)
- 1000 call stack limit

---

## 8. Module Format

### 8.1 Purpose

Modules are reusable sub-flows that can be shared and versioned.

### 8.2 Module Structure

```
my-module/
  vibeflow-node.yaml    # Module manifest
  module.yaml           # Sub-flow definition
  README.md
```

### 8.3 Module Manifest

```yaml
manifest_version: "2.0"

metadata:
  name: "Temperature Normalizer"
  namespace: "modules"
  type_name: "temp-normalizer"
  version: "1.0.0"
  description: "Normalize temperature readings to Celsius"

type: module

interface:
  inputs:
    - name: default
      description: "Raw temperature reading"
  outputs:
    - name: default
      description: "Normalized Celsius value"
    - name: invalid
      description: "Invalid readings"
  config_schema:
    type: object
    properties:
      min_valid:
        type: number
        default: -40
      max_valid:
        type: number
        default: 100
```

### 8.4 Module Implementation

```yaml
# module.yaml
version: "0.1"

parameters:
  min_valid: -40
  max_valid: 100

nodes:
  - id: convert
    type: core.script
    config:
      language: javascript
      code: |
        var temp = msg.payload.value;
        if (msg.payload.unit === "fahrenheit") {
          temp = (temp - 32) * 5/9;
        } else if (msg.payload.unit === "kelvin") {
          temp = temp - 273.15;
        }
        msg.payload = { value: temp, unit: "celsius" };
        return msg;

  - id: validate
    type: core.switch
    config:
      rules:
        - condition: "payload.value < ${min_valid} || payload.value > ${max_valid}"
          output: invalid
        - condition: "true"
          output: default

wires:
  - from: convert
    to: validate
```

### 8.5 Using Modules

```yaml
nodes:
  - id: normalize
    type: modules.temp-normalizer
    config:
      min_valid: -20
      max_valid: 80
```

---

## 9. GitHub Release Distribution

### 9.1 Overview

External nodes are distributed via GitHub releases. This provides:
- Versioned releases with changelogs
- Multi-architecture binaries
- Checksum verification
- No infrastructure to maintain

### 9.2 Release Structure

```
github.com/vibeflow/nodes-contrib/releases/v1.2.0/
  |
  +-- vibeflow-node-modbus-tcp-linux-amd64.tar.gz
  +-- vibeflow-node-modbus-tcp-linux-arm64.tar.gz
  +-- vibeflow-node-modbus-tcp-linux-arm.tar.gz
  +-- vibeflow-node-modbus-tcp-darwin-amd64.tar.gz
  +-- vibeflow-node-modbus-tcp-darwin-arm64.tar.gz
  +-- vibeflow-node-modbus-tcp-windows-amd64.zip
  +-- checksums.txt
  +-- manifest.yaml
```

### 9.3 Tarball Contents

```
vibeflow-node-modbus-tcp-linux-amd64.tar.gz
  |
  +-- vibeflow-node-modbus-tcp    # Binary
  +-- vibeflow-node.yaml          # Manifest
  +-- README.md
  +-- LICENSE
```

### 9.4 Checksums File

```
# checksums.txt (SHA256)
a1b2c3d4... vibeflow-node-modbus-tcp-linux-amd64.tar.gz
e5f6g7h8... vibeflow-node-modbus-tcp-linux-arm64.tar.gz
i9j0k1l2... vibeflow-node-modbus-tcp-darwin-amd64.tar.gz
...
```

### 9.5 Node Reference Format

```yaml
# vibeflow.yaml
nodes:
  # Full format
  - source: github.com/vibeflow/nodes-contrib
    name: modbus-tcp
    version: v1.2.0

  # Short format
  - github.com/vibeflow/nodes-contrib/modbus-tcp@v1.2.0

  # Latest version
  - github.com/company/custom-node@latest
```

### 9.6 Download and Cache

On startup, the runtime:

1. Parses node references
2. Checks local cache (`/data/nodes/`)
3. If not cached, downloads from GitHub releases
4. Verifies SHA256 checksum
5. Extracts to cache directory
6. Loads manifest

```
/data/nodes/
  github.com/
    vibeflow/
      nodes-contrib/
        modbus-tcp/
          v1.2.0/
            vibeflow-node-modbus-tcp
            vibeflow-node.yaml
```

### 9.7 Building Releases

Use the `vibeflow build` CLI:

```bash
cd my-node/
vibeflow build --all-platforms
```

This:
1. Cross-compiles for all supported platforms
2. Generates manifest from Go struct tags
3. Creates tarballs with proper structure
4. Generates checksums.txt

Output:
```
dist/
  vibeflow-node-mynode-linux-amd64.tar.gz
  vibeflow-node-mynode-linux-arm64.tar.gz
  vibeflow-node-mynode-darwin-amd64.tar.gz
  ...
  checksums.txt
  manifest.yaml
```

---

## 10. Versioning Requirements

### 10.1 Semantic Versioning

**Format:** `MAJOR.MINOR.PATCH`

- **MAJOR**: Breaking changes (config schema changes, removed outputs)
- **MINOR**: New features (new config options, new outputs)
- **PATCH**: Bug fixes

### 10.2 Version Constraints

```yaml
nodes:
  - github.com/company/node@v1.2.0     # Exact version
  - github.com/company/node@^1.2.0     # Compatible (>=1.2.0 <2.0.0)
  - github.com/company/node@~1.2.0     # Patch updates (>=1.2.0 <1.3.0)
  - github.com/company/node@latest     # Latest release
```

### 10.3 Runtime Compatibility

```yaml
# In manifest
dependencies:
  runtime_version: ">=0.1.0 <2.0.0"
```

---

## 11. Permission Model

### 11.1 Permission Types

External nodes declare what system resources they need:

```yaml
permissions:
  network: true           # TCP, UDP, HTTP
  serial: true            # Serial ports
  gpio: true              # GPIO pins
  spi: true               # SPI bus
  i2c: true               # I2C bus
  filesystem: true        # File read/write
  environment: true       # Environment variables
```

### 11.2 Runtime Enforcement

Permissions are currently **informational** for external nodes (they run as separate processes with full OS access).

Future: Container-based sandboxing for untrusted nodes.

### 11.3 Display in UI

Permissions are displayed when installing nodes:

```
Installing: github.com/company/modbus-node@v1.2.0

Permissions requested:
  - Network: Yes (TCP connections)
  - Serial: No
  - GPIO: No
  - Filesystem: No

Continue? [y/N]
```

---

## 12. Concurrency Model

### 12.1 Concurrency Modes

```yaml
concurrency:
  mode: single          # Only one message at a time
  # OR
  mode: parallel        # Multiple messages concurrently
  max_parallel: 10      # Limit concurrent processing
  # OR
  mode: worker_pool     # Multiple node instances
  pool_size: 4          # Number of instances
```

### 12.2 Mode Descriptions

**single**: Messages are queued and processed one at a time. Use for stateful nodes or when order matters.

**parallel**: Node SDK handles concurrent message processing. Use for stateless IO operations.

**worker_pool**: Runtime spawns multiple node processes. Use for CPU-intensive work.

### 12.3 SDK Enforcement

The SDK enforces concurrency limits:

```go
// In SDK - user doesn't see this
func (r *runner) processMessage(msg Message) {
    r.semaphore.Acquire()
    defer r.semaphore.Release()

    r.node.OnMessage(r.ctx, msg)
}
```

---

## 13. Node Discovery and Installation

### 13.1 Configuration

```yaml
# vibeflow.yaml
nodes:
  # External nodes from GitHub
  - github.com/vibeflow/nodes-contrib/modbus-tcp@v1.2.0
  - github.com/company/custom-gpio@v0.5.0

  # Local development
  - file:///path/to/my-node
```

### 13.2 CLI Commands

```bash
# List installed nodes
vibeflow nodes list

# Install a node
vibeflow nodes install github.com/company/node@v1.0.0

# Remove a node
vibeflow nodes remove github.com/company/node

# Update nodes
vibeflow nodes update

# Search (future: registry)
vibeflow nodes search modbus
```

### 13.3 Cache Location

```
$VIBEFLOW_CACHE/nodes/           # Default: /data/nodes/ in Docker
  github.com/
    vibeflow/
      nodes-contrib/
        modbus-tcp/v1.2.0/
        mqtt/v2.0.0/
    company/
      custom-gpio/v0.5.0/
```

---

## 14. Publishing Guidelines

### 14.1 Repository Structure

```
my-vibeflow-node/
  cmd/
    vibeflow-node-mynode/
      main.go
  internal/
    mynode/
      node.go
      config.go
  vibeflow-node.yaml          # Manifest (or auto-generated)
  README.md
  LICENSE
  go.mod
  go.sum
  .github/
    workflows/
      release.yml             # GitHub Actions for releases
```

### 14.2 GitHub Actions Release Workflow

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Build releases
        run: |
          vibeflow build --all-platforms

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            dist/*.tar.gz
            dist/*.zip
            dist/checksums.txt
            dist/manifest.yaml
```

### 14.3 Pre-Publication Checklist

- [ ] Manifest is valid (`vibeflow validate`)
- [ ] All config fields documented
- [ ] Examples provided
- [ ] Tests pass (`vibeflow test`)
- [ ] README with installation and usage
- [ ] LICENSE file present
- [ ] Semantic version tag

---

## 15. Best Practices

### 15.1 Node Design

**Do:**
- Use the SDK - don't reinvent RPC handling
- Declare accurate permissions
- Handle errors gracefully - emit to "error" output
- Support graceful shutdown
- Log at appropriate levels

**Don't:**
- Block forever in OnMessage
- Panic on bad input
- Ignore context cancellation
- Hardcode configuration

### 15.2 Configuration

**Do:**
- Use JSON Schema validation
- Provide sensible defaults
- Document all options
- Use x-vf-secret for sensitive fields

**Don't:**
- Require unnecessary config
- Use ambiguous field names
- Change config schema in patch releases

### 15.3 Performance

**Do:**
- Use appropriate concurrency mode
- Release resources on Stop()
- Buffer appropriately for high-throughput

**Don't:**
- Create goroutines without limits
- Hold connections open unnecessarily
- Log excessively in hot paths

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-10 | Initial | First draft |
| 2.0 | 2025-12-10 | Updated | External binary model, GitHub distribution |

---

## References

- [01-overview.md](./01-overview.md) - Project Overview
- [02-flow-format.md](./02-flow-format.md) - Flow Format Specification
- [03-node-spec.md](./03-node-spec.md) - Node Interface Specification
- [04-message-model.md](./04-message-model.md) - Message Model Specification
- [05-runtime.md](./05-runtime.md) - Runtime Architecture
- [07-node-sdk.md](./07-node-sdk.md) - Node SDK and RPC Protocol
- [JSON Schema Specification](https://json-schema.org/)
- [Semantic Versioning](https://semver.org/)
- [Hashicorp go-plugin](https://github.com/hashicorp/go-plugin)
