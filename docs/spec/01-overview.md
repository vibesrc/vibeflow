# Vibeflow Overview Specification

**Version:** 1.0
**Last Updated:** 2025-12-10
**Status:** Draft

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Project Vision and Philosophy](#2-project-vision-and-philosophy)
3. [Goals with Rationale](#3-goals-with-rationale)
4. [Non-Goals and Boundaries](#4-non-goals-and-boundaries)
5. [Core Concepts](#5-core-concepts)
6. [Architecture Overview](#6-architecture-overview)
7. [Terminology Glossary](#7-terminology-glossary)
8. [Target Audience and Use Cases](#8-target-audience-and-use-cases)
9. [Technology Stack](#9-technology-stack)
10. [Open Questions](#10-open-questions)

---

## 1. Executive Summary

Vibeflow is a high-performance, low-code flow-based programming engine built in Go. It enables developers and operators to build automation workflows, protocol integrations, and data transformation pipelines through declarative YAML configurations or visual tooling.

Unlike traditional workflow engines that prioritize features over performance, Vibeflow is designed from the ground up for speed and resource efficiency, making it suitable for deployment on edge devices, IoT gateways, industrial controllers, and cloud environments alike.

**Key Differentiators:**

- **Near wire-speed performance** through zero-copy message handling
- **Multiple scripting engines** (JavaScript/goja, Go/yaegi, Tengo, Starlark)
- **WASI support** for portable, sandboxed WebAssembly nodes
- **Protocol-first design** with native support for Modbus, MQTT, HTTP, TCP
- **Deterministic behavior** through immutable YAML configuration
- **Minimal dependencies** enabling deployment on resource-constrained devices

---

## 2. Project Vision and Philosophy

### 2.1 Vision Statement

**"Make automation accessible without sacrificing performance."**

Vibeflow aims to democratize automation by providing a tool that is:
- Simple enough for operators to configure
- Powerful enough for engineers to extend
- Fast enough for production-critical systems
- Portable enough to run anywhere

### 2.2 Core Philosophy

#### 2.2.1 Configuration Over Code

Flows are defined in YAML, not compiled into binaries. This enables:
- Version control of automation logic
- GitOps workflows for deployment
- Non-programmers to understand and modify flows
- Tooling validation before deployment

#### 2.2.2 Performance Without Compromise

Low-code should not mean low performance. Vibeflow achieves high throughput through:
- Zero-copy message passing where possible
- Minimal allocations in hot paths
- Native goroutine-based concurrency
- Efficient routing algorithms

#### 2.2.3 Extensibility Through Standards

Rather than building a monolithic system, Vibeflow defines clear interfaces:
- Native Go nodes for maximum performance
- Script nodes for rapid development
- WASI nodes for language-agnostic extensions
- Standard message and manifest formats

#### 2.2.4 Predictability and Determinism

Flows should behave identically given the same configuration:
- No hidden state or magic behavior
- Explicit routing and transformation
- Reproducible execution for testing
- Clear error boundaries

---

## 3. Goals with Rationale

### 3.1 High Performance

**Goal:** Process messages with minimal overhead, targeting microsecond-scale latencies for native nodes.

**Rationale:**
- Many automation scenarios are latency-sensitive (industrial control, real-time monitoring)
- Edge devices have limited CPU/memory resources
- High throughput reduces infrastructure costs in cloud deployments
- Competitive advantage over Node-RED and similar tools

**Success Metrics:**
- Message routing overhead < 1s for native nodes
- Support 100k+ messages/second on commodity hardware
- Memory footprint < 50MB for typical flows
- Startup time < 100ms

### 3.2 Low-Code Usability

**Goal:** Enable non-programmers to create and modify flows through declarative configuration.

**Rationale:**
- Operations teams should not need to compile code for configuration changes
- Visual editors can generate YAML flows
- Reduces time-to-deployment for common automation tasks
- Lowers barrier to entry for new users

**Success Metrics:**
- Flow creation without writing Go code
- Schema validation catches 90%+ configuration errors
- Standard library covers common use cases
- Documentation accessible to non-developers

### 3.3 Modular Extensibility

**Goal:** Support three extension mechanisms+native Go, scripting, and WASI+with consistent interfaces.

**Rationale:**
- Native nodes provide maximum performance for critical paths
- Script nodes enable rapid prototyping and simple transformations
- WASI nodes allow community contributions in any language
- Consistent interfaces reduce cognitive load

**Success Metrics:**
- Third-party nodes installable via manifests
- Script nodes have < 10x overhead vs native
- WASI nodes load and execute successfully
- Node development SDK available

### 3.4 Portability

**Goal:** Run on Linux, Windows, macOS, ARM32, ARM64, x86-64 without modification.

**Rationale:**
- Edge devices span diverse architectures (Raspberry Pi, industrial PCs, routers)
- Development on laptops, deployment on servers
- Cross-compilation simplifies distribution
- Single binary deployment reduces operational complexity

**Success Metrics:**
- Successful builds for all targets
- No platform-specific code in hot paths
- Docker images < 20MB compressed
- Static binary with no external dependencies

### 3.5 Robust Ecosystem

**Goal:** Third-party developers can publish nodes with minimal friction.

**Rationale:**
- Success of Node-RED demonstrates value of community extensions
- Standard manifest format enables discovery and validation
- Go modules provide existing distribution mechanism
- Security model protects users from malicious nodes

**Success Metrics:**
- Node manifest specification published
- Example nodes available as templates
- Registry or discovery mechanism defined
- Security audit process documented

### 3.6 Deterministic Configuration

**Goal:** Flows defined in YAML produce reproducible behavior.

**Rationale:**
- GitOps requires immutable, auditable configuration
- Testing requires deterministic behavior
- Debugging requires reproducible issues
- Migration between environments requires consistency

**Success Metrics:**
- YAML-defined flows are version-controllable
- No runtime-only configuration (except secrets)
- Schema validation before execution
- Migration tools between versions

---

## 4. Non-Goals and Boundaries

To maintain focus and quality, Vibeflow explicitly **does not** aim to:

### 4.1 Node-RED Compatibility

**Why:** Node-RED's JSON format, JavaScript-centric model, and runtime assumptions are incompatible with Vibeflow's design. Attempting compatibility would compromise performance and design clarity.

**Alternative:** Migration tools may be provided, but wire-level compatibility is not a goal.

### 4.2 Embedded V8 or Heavy Runtimes

**Why:** V8 is large (10s of MB), complex to embed, and overkill for most scripting needs. Lightweight alternatives (goja, Tengo) provide 90% of functionality at 10% of the size.

**Alternative:** Multiple lightweight scripting engines supported.

### 4.3 Full ETL/Workflow Orchestration

**Why:** Vibeflow focuses on real-time message processing, not batch ETL or long-running workflow orchestration (like Airflow or Temporal). Different domains require different architectures.

**Boundary:** Vibeflow handles streaming data and event-driven automation. For batch processing or distributed workflows spanning days, use purpose-built tools.

### 4.4 General-Purpose Application Framework

**Why:** Vibeflow is not a web framework, API gateway, or application platform. It's a specialized tool for flow-based programming.

**Boundary:** Vibeflow integrates with applications via HTTP, MQTT, TCP, etc., but does not replace application frameworks like Gin or Echo.

### 4.5 Built-in Visual Editor

**Why:** Building production-quality UIs is a separate discipline requiring frontend expertise. Vibeflow focuses on the runtime engine.

**Alternative:** Third-party editors can generate YAML flows. Reference implementation may be provided separately.

### 4.6 Database or State Management

**Why:** Persistent state management introduces complexity (transactions, consistency, recovery) beyond flow execution.

**Boundary:** Nodes can interact with external databases (Redis, PostgreSQL), but Vibeflow does not provide built-in persistence.

---

## 5. Core Concepts

### 5.1 Flows

**Definition:** A flow is a directed acyclic graph (DAG) or directed graph of interconnected nodes that process messages.

**Characteristics:**
- Defined declaratively in YAML
- Validated before execution
- Nodes execute asynchronously where possible
- Messages route deterministically based on wiring

**Lifecycle:**
1. **Load**: Parse YAML, validate schema
2. **Initialize**: Instantiate nodes, establish connections
3. **Execute**: Route messages, process in nodes
4. **Shutdown**: Cleanup, flush buffers

**Example Flow (Conceptual):**

```
[Inject Timer] --> [Transform] --> [HTTP Request] --> [Debug]
                        |
                     [Logger]
```

### 5.2 Nodes

**Definition:** A node is a computational unit that receives messages, performs operations, and outputs messages.

**Types:**

| Type | Description | Performance | Use Case |
|------|-------------|-------------|----------|
| **Built-in (core/contrib)** | Compiled Go code | Fastest | Essential nodes, common protocols |
| **External** | Separate binary + JSON-RPC | High | Custom protocols, hardware access |
| **Script** | Interpreted (JavaScript/goja) | Fast | Data transformation, simple logic |
| **WASI** | WebAssembly | Moderate | Sandboxed, portable extensions |

**External nodes** are distributed via GitHub releases and downloaded on demand. They run as separate processes with full OS access (TCP, UDP, serial, GPIO, etc.). See [07-node-sdk.md](./07-node-sdk.md) for details.

**Anatomy of a Node:**

```
+-------------------------+
|      Node: "node1"      |
|   Type: "core.http"     |
+-------------------------+
| Inputs:                 |
|   - default             |
|   - trigger (optional)  |
+-------------------------+
| Configuration:          |
|   url: "..."            |
|   method: "GET"         |
+-------------------------+
| Outputs:                |
|   - success             |
|   - error               |
+-------------------------+
```

**Node Responsibilities:**
- Validate configuration on initialization
- Process incoming messages
- Emit output messages (or none)
- Handle errors gracefully
- Cleanup resources on shutdown

### 5.3 Messages

**Definition:** A message is the atomic unit of data passed between nodes.

**Structure (Conceptual):**

```go
type Message struct {
    ID       string                 // Unique message identifier
    Metadata map[string]string      // Routing hints, timestamps, correlation IDs
    Payload  interface{}            // Actual data (zero-copy when possible)
    Context  map[string]interface{} // Flow-scoped context
}
```

**Properties:**
- **Immutable Metadata**: Once set, metadata should not change (for tracing)
- **Zero-Copy Payload**: Payloads reference shared memory when possible
- **Typed Payloads**: Payloads have well-defined types (byte slices, JSON objects, custom structs)
- **Contextual**: Messages carry context from upstream nodes

**Message Flow:**

```
Node A --> [Message ID: msg-123] --> Node B --> [Message ID: msg-124] --> Node C
           Payload: {"temp": 25}                Payload: {"temp": 77}
```

### 5.4 Wires

**Definition:** A wire connects an output from one node to an input on another node.

**Characteristics:**
- Unidirectional data flow
- Can be named (for multi-output scenarios)
- Buffered channels (configurable depth)
- Backpressure-aware

**Example Wire Configuration:**

```yaml
wires:
  - from: inject1
    to: transform1
    output: default
    input: default
  - from: transform1
    to: debug1
    output: success
  - from: transform1
    to: logger1
    output: error
```

### 5.5 Runtime

**Definition:** The runtime is the execution engine that loads flows, manages nodes, and routes messages.

**Responsibilities:**

1. **Flow Management**
   - Load and parse YAML
   - Validate schema and references
   - Instantiate nodes

2. **Message Routing**
   - Dispatch messages to connected nodes
   - Handle backpressure
   - Implement delivery guarantees (at-most-once, at-least-once)

3. **Lifecycle Management**
   - Initialize nodes in dependency order
   - Graceful shutdown with flush
   - Hot reload flows (if safe)

4. **Observability**
   - Metrics (message throughput, latency)
   - Tracing (message lineage)
   - Logging (errors, warnings)

**Runtime Architecture (Simplified):**

```
+---------------------------------------+
|           Flow Runtime                |
+---------------------------------------+
|  +-----------+  +-----------+         |
|  |   Node    |  |   Node    |         |
|  | Registry  |  |  Factory  |         |
|  +-----------+  +-----------+         |
+---------------------------------------+
|  +-----------------------------------+|
|  |   Message Router & Scheduler     ||
|  +-----------------------------------+|
+---------------------------------------+
|  +-----------+  +-----------+         |
|  |  Script   |  |   WASI    |         |
|  |  Engines  |  |  Runtime  |         |
|  +-----------+  +-----------+         |
+---------------------------------------+
```

---

## 6. Architecture Overview

### 6.1 High-Level Component Diagram

```
+----------------------------------------------------+
|                 Vibeflow Runtime                   |
+----------------------------------------------------+
|                                                    |
| +----------------+     +------------------+        |
| |  Flow Loader   | --> |  Flow Validator  |        |
| +----------------+     +------------------+        |
|         |                       |                  |
|         v                       v                  |
| +----------------------------------------------+   |
| |             Node Registry                    |   |
| |  - Native Nodes                              |   |
| |  - Script Nodes (goja, yaegi, tengo, starlark)|  |
| |  - WASI Nodes                                |   |
| +----------------------------------------------+   |
|                       |                            |
|                       v                            |
| +----------------------------------------------+   |
| |           Message Scheduler                  |   |
| |  - Goroutine pool                            |   |
| |  - Message queues                            |   |
| |  - Backpressure control                      |   |
| +----------------------------------------------+   |
|                       |                            |
|                       v                            |
| +----------------------------------------------+   |
| |           Node Instances                     |   |
| |  [Node A] [Node B] [Node C] ...              |   |
| +----------------------------------------------+   |
|                                                    |
+----------------------------------------------------+
                        |
         +--------------+--------------+
         |                             |
         v                             v
+------------------+         +------------------+
|   Observability  |         |   External       |
|     (Metrics,    |         |   Systems        |
|      Traces)     |         | (MQTT, HTTP)     |
+------------------+         +------------------+
```

### 6.2 Data Flow

```
1. YAML Flow Definition
         |
         v
2. Parse & Validate
         |
         v
3. Instantiate Nodes
         |
         v
4. Wire Connections (Channels)
         |
         v
5. Start Execution
         |
         v
6. Messages Flow Through Nodes
         |
         v
7. Graceful Shutdown
```

### 6.3 Concurrency Model

**Approach:** Each node runs in its own goroutine(s), communicating via channels.

**Benefits:**
- Natural concurrency via Go's scheduler
- Backpressure via buffered channels
- Simple mental model

**Example:**

```
Node A (goroutine) --> Channel --> Node B (goroutine)
                                       |
                                  Channel --> Node C (goroutine)
```

---

## 7. Terminology Glossary

| Term | Definition |
|------|------------|
| **Flow** | A directed graph of interconnected nodes defined in YAML |
| **Node** | A computational unit that processes messages |
| **Message** | The atomic data structure passed between nodes |
| **Wire** | A connection between two nodes (from output to input) |
| **Runtime** | The execution engine that manages flows |
| **Payload** | The data content of a message |
| **Metadata** | Key-value annotations on a message (timestamps, IDs) |
| **Context** | Flow-scoped shared state accessible to nodes |
| **Native Node** | A node implemented in compiled Go |
| **Script Node** | A node implemented in an interpreted language |
| **WASI Node** | A node implemented in WebAssembly with WASI interface |
| **Manifest** | A YAML file describing a node's interface and configuration schema |
| **Zero-Copy** | Passing data by reference without duplication |
| **Backpressure** | Flow control mechanism when downstream is slower than upstream |
| **Hot Reload** | Updating a flow without stopping the runtime |
| **DAG** | Directed Acyclic Graph (flows without cycles) |
| **Goroutine** | Go's lightweight thread for concurrent execution |
| **Channel** | Go's communication primitive for passing data between goroutines |

---

## 8. Target Audience and Use Cases

### 8.1 Target Audience

**Primary Users:**

1. **Automation Engineers**
   - Configure industrial automation workflows
   - Integrate PLCs, sensors, and SCADA systems
   - Need: Low latency, deterministic behavior

2. **IoT Developers**
   - Build edge gateways and data pipelines
   - Handle MQTT, Modbus, HTTP protocols
   - Need: Low resource usage, portability

3. **DevOps Engineers**
   - Create monitoring and alerting flows
   - Integrate APIs and webhooks
   - Need: Easy configuration, GitOps support

4. **System Integrators**
   - Connect disparate systems
   - Transform data between protocols
   - Need: Extensibility, rapid prototyping

**Secondary Users:**

5. **Node Developers**
   - Extend Vibeflow with custom nodes
   - Publish reusable components
   - Need: Clear APIs, good documentation

6. **Operators**
   - Deploy and monitor flows
   - Troubleshoot issues
   - Need: Observability, clear error messages

### 8.2 Use Cases

#### Use Case 1: IoT Gateway

**Scenario:** A factory floor has 100 Modbus sensors that need data published to an MQTT broker and stored in TimescaleDB.

**Flow:**
```
[Modbus Poll: 100 devices] -> [Transform: Normalize] -> [Split]
                                                        +-> [MQTT Publish]
                                                        +-> [HTTP: TimescaleDB]
```

**Why Vibeflow:**
- Native Modbus support
- High throughput (1000s of readings/sec)
- Runs on ARM64 gateway hardware

#### Use Case 2: API Integration

**Scenario:** Aggregate data from 5 REST APIs, correlate by timestamp, and send alerts to Slack.

**Flow:**
```
[Timer: 1min] -> [HTTP: API 1]
             -> [HTTP: API 2] -> [Join] -> [Script: Filter] -> [HTTP: Slack]
             -> [HTTP: API 3]
```

**Why Vibeflow:**
- Low-code configuration
- Built-in HTTP client
- JavaScript for complex logic

#### Use Case 3: Protocol Translation

**Scenario:** Legacy system speaks proprietary binary protocol, modern system needs JSON over HTTP.

**Flow:**
```
[TCP Listen: :9000] -> [Script: Parse Binary] -> [Transform: To JSON] -> [HTTP POST]
```

**Why Vibeflow:**
- Raw TCP socket support
- Scripting for parsing logic
- Zero-copy for high throughput

#### Use Case 4: Real-Time Monitoring

**Scenario:** Monitor application logs, extract errors, and publish metrics to Prometheus.

**Flow:**
```
[File Tail: app.log] -> [Script: Parse Log] -> [Filter: errors] -> [Metrics: Counter]
```

**Why Vibeflow:**
- File watching
- Pattern matching
- Prometheus integration

#### Use Case 5: Edge ML Inference

**Scenario:** Run TensorFlow Lite model on edge device, process camera frames in real-time.

**Flow:**
```
[Camera: USB] -> [Image Decode] -> [WASI: TFLite] -> [Filter: Confidence > 0.8] -> [MQTT: Alert]
```

**Why Vibeflow:**
- WASI for ML runtimes
- Low-latency pipeline
- Edge-compatible

---

## 9. Technology Stack

### 9.1 Core Language

**Go 1.25+**

**Rationale:**
- Native concurrency (goroutines, channels)
- Fast compilation, static binaries
- Excellent cross-compilation
- Strong standard library
- Growing ecosystem

### 9.2 Configuration Format

**YAML**

**Rationale:**
- Human-readable
- Supports comments
- Wide tooling support (linters, validators)
- Standard library (gopkg.in/yaml.v3)

### 9.3 Scripting Engines

| Engine | Language | Use Case | Maturity |
|--------|----------|----------|----------|
| **goja** | JavaScript (ES5.1) | General scripting | Stable |
| **yaegi** | Go | Go scripts, type safety | Stable |
| **tengo** | Tengo | Simple scripts | Stable |
| **starlark** | Starlark (Python-like) | Configuration as code | Stable |

### 9.4 WASI Runtime

**wazero**

**Rationale:**
- Pure Go (no CGO)
- Full WASI support
- Good performance
- Active development

### 9.5 Protocol Libraries

| Protocol | Library | Notes |
|----------|---------|-------|
| HTTP/HTTPS | `net/http` | Standard library |
| MQTT | `paho.mqtt.golang` | Eclipse Paho |
| Modbus | `goburrow/modbus` | Modbus TCP/RTU |
| TCP/UDP | `net` | Standard library |
| WebSocket | `gorilla/websocket` | De facto standard |

### 9.6 Testing

- **Unit Tests:** `testing` (stdlib)
- **Integration Tests:** `testcontainers-go`
- **Benchmarks:** `testing.B`
- **Fuzzing:** `testing.F` (Go 1.18+)

### 9.7 Observability

- **Metrics:** Prometheus (`prometheus/client_golang`)
- **Tracing:** OpenTelemetry
- **Logging:** `slog` (Go 1.21+)

---

## 10. Open Questions

### 10.1 Flow Execution Model

**Question:** Should flows support cyclic graphs (loops) or remain DAGs?

**Implications:**
- DAGs simplify scheduling and deadlock avoidance
- Cycles enable iterative processing (e.g., retry loops)
- Cycles complicate backpressure and termination

**Decision:** To be determined based on user needs.

### 10.2 State Management

**Question:** Should Vibeflow provide built-in state persistence, or delegate to external systems?

**Options:**
- **Built-in:** SQLite or embedded KV store (BoltDB, BadgerDB)
- **External:** Nodes connect to Redis, PostgreSQL, etc.

**Trade-offs:**
- Built-in simplifies deployment but adds complexity
- External maintains separation of concerns

**Decision:** Start with external, revisit if demand exists.

### 10.3 Hot Reload Safety

**Question:** What constraints ensure hot reloading is safe?

**Considerations:**
- Stateful nodes (e.g., TCP servers) may have in-flight connections
- Message buffers may contain data
- Graceful shutdown required

**Approach:** Define reload contract (draining, flushing, reconnection).

### 10.4 Multi-Flow Coordination

**Question:** Should a single runtime manage multiple independent flows?

**Benefits:**
- Resource sharing (node instances, connections)
- Centralized monitoring

**Drawbacks:**
- Isolation concerns (one flow crash affects others)
- Complexity in scheduling

**Decision:** Support both single-flow and multi-flow modes.

### 10.5 Visual Editor

**Question:** Should Vibeflow provide an official visual editor?

**Options:**
1. **No editor:** Focus on runtime, let community build UIs
2. **Reference editor:** Simple web-based tool
3. **Full-featured editor:** Professional tool

**Decision:** Start without, provide JSON schema for third-party tools.

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-10 | Initial | First draft |

---

## References

- [02-flow-format.md](./02-flow-format.md) - YAML Schema Specification
- [03-node-spec.md](./03-node-spec.md) - Node Interface Specification
- [04-message-model.md](./04-message-model.md) - Message Model Specification
- [05-runtime.md](./05-runtime.md) - Runtime Architecture
- [06-node-manifest.md](./06-node-manifest.md) - Node Manifest and Distribution
- [07-node-sdk.md](./07-node-sdk.md) - Node SDK and RPC Protocol
