# Vibeflow Message Model Specification

**Version:** 1.0
**Last Updated:** 2025-12-10
**Status:** Draft

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Message Structure](#2-message-structure)
3. [Zero-Copy Implementation](#3-zero-copy-implementation)
4. [Metadata Schema](#4-metadata-schema)
5. [Payload Type System](#5-payload-type-system)
6. [Serialization Formats](#6-serialization-formats)
7. [Message Routing Semantics](#7-message-routing-semantics)
8. [Multi-Output Handling](#8-multi-output-handling)
9. [Message Lifecycle](#9-message-lifecycle)
10. [Best Practices](#10-best-practices)
11. [Open Questions](#11-open-questions)

---

## 1. Introduction

### 1.1 Purpose

Messages are the fundamental unit of data flow in Vibeflow. This specification defines the structure, semantics, and lifecycle of messages to ensure consistent behavior across nodes and efficient runtime execution.

### 1.2 Design Principles

1. **Zero-Copy by Default**: Messages pass by reference when possible to minimize memory allocation
2. **Immutability When Appropriate**: Metadata and context should be immutable for tracing
3. **Type Safety**: Payloads are typed to enable compile-time validation where possible
4. **Extensibility**: Support for custom payload types and metadata
5. **Debuggability**: Messages carry enough context for debugging and tracing

### 1.3 Message Lifecycle

```
+--------------+
| Created by   |  (Source node: inject, HTTP listener, etc.)
| Source Node  |
+--------------+
       |
       v
+--------------+
| Routed by    |  (Runtime determines next node)
| Runtime      |
+--------------+
       |
       v
+--------------+
| Processed by |  (Node transforms payload, updates metadata)
| Node         |
+--------------+
       |
       v
+--------------+
| Cloned (if   |  (Copy-on-write for multi-output)
| needed)      |
+--------------+
       |
       v
+--------------+
| Routed to    |  (Repeat until reaching sink node)
| Next Node(s) |
+--------------+
       |
       v
+--------------+
| Garbage      |  (Go GC reclaims when no references remain)
| Collected    |
+--------------+
```

---

## 2. Message Structure

### 2.1 Go Struct Definition

```go
package message

import (
    "time"
)

// Message represents a single unit of data flowing through the system
type Message struct {
    // ID is a unique identifier for this message
    // Format: UUID v7 (time-ordered) for tracing
    ID string

    // Metadata contains immutable key-value pairs for routing and tracing
    // Examples: timestamps, correlation IDs, trace spans
    Metadata Metadata

    // Payload is the message data (zero-copy when possible)
    // Type: interface{} to support any Go type
    Payload interface{}

    // Context is flow-scoped mutable state shared across nodes
    // Use sparingly - prefer stateless transformations
    Context Context

    // Internal fields (not exposed to nodes)
    createdAt time.Time
    refCount  int32 // Reference count for zero-copy optimization
}

// Metadata is an immutable map of string key-value pairs
type Metadata map[string]string

// Context is a mutable map for flow-scoped state
type Context map[string]interface{}
```

### 2.2 Field Descriptions

#### 2.2.1 ID Field

**Type:** `string`
**Format:** UUID v7 (time-ordered UUID)
**Purpose:** Unique identifier for message tracing and correlation

**Generation:**

```go
import "github.com/google/uuid"

func NewMessage(payload interface{}) *Message {
    return &Message{
        ID:        uuid.NewString(), // Generates UUID v4 or v7
        Metadata:  make(Metadata),
        Payload:   payload,
        Context:   make(Context),
        createdAt: time.Now(),
        refCount:  1,
    }
}
```

**Usage:**
- Logs: `"processing message" message_id=msg-123`
- Tracing: Span ID or correlation ID
- Debugging: Track message through flow

#### 2.2.2 Metadata Field

**Type:** `map[string]string`
**Mutability:** Immutable (copy-on-write)
**Purpose:** Routing hints, timestamps, tracing information

**Standard Metadata Keys:**

| Key | Type | Description | Example |
|-----|------|-------------|---------|
| `timestamp` | ISO 8601 | Message creation time | `2025-12-10T12:00:00Z` |
| `source_node` | string | Node that created message | `inject1` |
| `trace_id` | string | Distributed trace ID | `abc123...` |
| `span_id` | string | Span within trace | `def456...` |
| `correlation_id` | string | Business correlation ID | `order-789` |
| `priority` | string | Message priority | `high`, `normal`, `low` |
| `content_type` | string | Payload MIME type | `application/json` |

**Adding Metadata:**

```go
// Nodes should not mutate metadata directly
// Instead, clone and add:
func (n *MyNode) Process(ctx context.Context, msg *Message) (*Result, error) {
    outMsg := msg.Clone()
    outMsg.Metadata["processed_by"] = n.id
    outMsg.Metadata["processed_at"] = time.Now().Format(time.RFC3339)
    return &Result{Messages: []*OutputMessage{{Message: outMsg}}}, nil
}
```

#### 2.2.3 Payload Field

**Type:** `interface{}`
**Mutability:** Depends on type (prefer immutable)
**Purpose:** Actual message data

**Common Payload Types:**

| Go Type | Use Case | Example |
|---------|----------|---------|
| `string` | Text data | `"hello world"` |
| `[]byte` | Binary data | Raw bytes from TCP socket |
| `map[string]interface{}` | JSON-like objects | `{"temp": 25}` |
| `int`, `float64`, `bool` | Primitive values | `42`, `3.14`, `true` |
| Custom struct | Typed data | `&SensorReading{...}` |

**Type Safety:**

```go
// Type assertion with safety check
if payload, ok := msg.Payload.(map[string]interface{}); ok {
    // Process JSON-like payload
}

// Prefer specific types when possible
type SensorReading struct {
    Temperature float64
    Humidity    float64
    Timestamp   time.Time
}

if reading, ok := msg.Payload.(*SensorReading); ok {
    // Type-safe access
}
```

#### 2.2.4 Context Field

**Type:** `map[string]interface{}`
**Mutability:** Mutable
**Purpose:** Flow-scoped shared state

**Use Cases:**
- Accumulate data across nodes (e.g., join operations)
- Pass configuration from upstream to downstream nodes
- Store intermediate results

**Caution:** Excessive use of context increases coupling and reduces testability. Prefer stateless transformations.

**Example:**

```yaml
# Node 1: Set context
- id: init
  type: core.script
  language: javascript
  config:
    code: |
      msg.context.batch_id = "batch-" + Date.now();
      return msg;

# Node 2: Use context
- id: process
  type: core.script
  language: javascript
  config:
    code: |
      msg.payload.batch = msg.context.batch_id;
      return msg;
```

### 2.3 Message Methods

```go
package message

// Clone creates a deep copy of the message
// Use when modifying payload or sending to multiple outputs
func (m *Message) Clone() *Message {
    return &Message{
        ID:        m.ID,  // Keep same ID for tracing
        Metadata:  m.Metadata.Clone(),
        Payload:   m.clonePayload(),
        Context:   m.Context.Clone(),
        createdAt: m.createdAt,
        refCount:  1,
    }
}

// WithPayload creates a new message with updated payload (immutable pattern)
func (m *Message) WithPayload(payload interface{}) *Message {
    clone := m.Clone()
    clone.Payload = payload
    return clone
}

// WithMetadata creates a new message with additional metadata
func (m *Message) WithMetadata(key, value string) *Message {
    clone := m.Clone()
    clone.Metadata[key] = value
    return clone
}

// Age returns how long ago the message was created
func (m *Message) Age() time.Duration {
    return time.Since(m.createdAt)
}
```

**Helper Methods:**

```go
// Clone creates a copy of metadata
func (m Metadata) Clone() Metadata {
    clone := make(Metadata, len(m))
    for k, v := range m {
        clone[k] = v
    }
    return clone
}

// Clone creates a copy of context
func (c Context) Clone() Context {
    clone := make(Context, len(c))
    for k, v := range c {
        // Deep copy if possible
        clone[k] = deepCopy(v)
    }
    return clone
}
```

---

## 3. Zero-Copy Implementation

### 3.1 Motivation

Copying large payloads on every node transition is expensive:
- Memory allocations stress GC
- CPU cycles wasted on copying
- Increased latency

**Goal:** Pass messages by reference when possible, copy only when necessary.

### 3.2 Reference Counting

```go
package message

import "sync/atomic"

type Message struct {
    // ... fields ...
    refCount int32  // Atomic reference count
}

// AddRef increments reference count (thread-safe)
func (m *Message) AddRef() {
    atomic.AddInt32(&m.refCount, 1)
}

// Release decrements reference count
// Returns true if this was the last reference
func (m *Message) Release() bool {
    return atomic.AddInt32(&m.refCount, -1) == 0
}

// RefCount returns current reference count
func (m *Message) RefCount() int32 {
    return atomic.LoadInt32(&m.refCount)
}
```

### 3.3 Copy-on-Write

**Strategy:** Share message references until a node needs to modify it.

**Single Output (No Copy):**

```go
// Node doesn't modify message - pass by reference
func (n *PassthroughNode) Process(ctx context.Context, msg *Message) (*Result, error) {
    // No modification needed
    return &Result{
        Messages: []*OutputMessage{{Message: msg}},
    }, nil
}
```

**Single Output (Copy):**

```go
// Node modifies message - clone first
func (n *TransformNode) Process(ctx context.Context, msg *Message) (*Result, error) {
    outMsg := msg.Clone()
    outMsg.Payload = transform(msg.Payload)
    return &Result{
        Messages: []*OutputMessage{{Message: outMsg}},
    }, nil
}
```

**Multiple Outputs (Copy):**

```go
// Message sent to multiple outputs - clone for each
func (n *FanOutNode) Process(ctx context.Context, msg *Message) (*Result, error) {
    return &Result{
        Messages: []*OutputMessage{
            {Output: "out1", Message: msg.Clone()},
            {Output: "out2", Message: msg.Clone()},
            {Output: "out3", Message: msg.Clone()},
        },
    }, nil
}
```

### 3.4 Payload Zero-Copy

For large binary payloads (e.g., images, video frames):

**Shared Memory:**

```go
type Message struct {
    Payload interface{}  // Points to shared []byte
}

// No copy - multiple nodes read same bytes
msg1.Payload = sharedBuffer  // Node 1
msg2.Payload = sharedBuffer  // Node 2 (same buffer)
```

**Copy-on-Write Buffers:**

```go
package message

// CowBuffer is a copy-on-write byte buffer
type CowBuffer struct {
    data     []byte
    refCount int32
}

func NewCowBuffer(data []byte) *CowBuffer {
    return &CowBuffer{data: data, refCount: 1}
}

func (b *CowBuffer) Get() []byte {
    return b.data  // Read-only access
}

func (b *CowBuffer) Set(data []byte) *CowBuffer {
    // Create new buffer if shared
    if atomic.LoadInt32(&b.refCount) > 1 {
        return NewCowBuffer(data)
    }
    // Reuse buffer if sole owner
    b.data = data
    return b
}
```

### 3.5 Zero-Copy Constraints

**When Zero-Copy is NOT Possible:**

1. **WASI Nodes**: Must serialize message to WASM memory
2. **Script Nodes**: May require serialization depending on VM
3. **External Systems**: Network I/O requires serialization
4. **Logging/Debugging**: Often requires serialization for persistence

**Trade-offs:**
- Native nodes: Zero-copy possible
- Script nodes: Overhead of VM boundary
- WASI nodes: Serialization overhead

---

## 4. Metadata Schema

### 4.1 Standard Metadata Keys

**Vibeflow defines standard metadata keys for interoperability:**

```go
package message

const (
    // Timestamps
    MetaTimestamp      = "timestamp"        // ISO 8601 creation time
    MetaProcessedAt    = "processed_at"     // ISO 8601 processing time

    // Tracing
    MetaTraceID        = "trace_id"         // Distributed trace ID
    MetaSpanID         = "span_id"          // Span within trace
    MetaParentSpanID   = "parent_span_id"   // Parent span ID

    // Correlation
    MetaCorrelationID  = "correlation_id"   // Business correlation ID
    MetaSessionID      = "session_id"       // Session identifier

    // Routing
    MetaPriority       = "priority"         // Message priority
    MetaSourceNode     = "source_node"      // Originating node ID
    MetaFlowID         = "flow_id"          // Flow identifier

    // Content
    MetaContentType    = "content_type"     // MIME type
    MetaEncoding       = "encoding"         // Encoding (e.g., "gzip")
    MetaSchema         = "schema"           // Schema identifier
)
```

### 4.2 Custom Metadata

**Naming Convention:** Use namespaced keys to avoid collisions.

**Examples:**
- `custom.company.user_id`
- `protocol.mqtt.topic`
- `app.order_status`

**Go Constants:**

```go
const (
    MetaCustomPrefix = "custom."
    MetaProtocolPrefix = "protocol."
    MetaAppPrefix = "app."
)
```

### 4.3 Metadata Helpers

```go
package message

// WithTimestamp adds current timestamp
func (m *Message) WithTimestamp() *Message {
    return m.WithMetadata(MetaTimestamp, time.Now().Format(time.RFC3339))
}

// WithTrace adds trace and span IDs
func (m *Message) WithTrace(traceID, spanID string) *Message {
    clone := m.Clone()
    clone.Metadata[MetaTraceID] = traceID
    clone.Metadata[MetaSpanID] = spanID
    return clone
}

// GetMetadata safely retrieves metadata value
func (m *Message) GetMetadata(key string) (string, bool) {
    val, exists := m.Metadata[key]
    return val, exists
}

// GetMetadataOrDefault retrieves metadata with fallback
func (m *Message) GetMetadataOrDefault(key, defaultVal string) string {
    if val, exists := m.Metadata[key]; exists {
        return val
    }
    return defaultVal
}
```

---

## 5. Payload Type System

### 5.1 Primitive Types

| Go Type | Description | Example |
|---------|-------------|---------|
| `string` | UTF-8 text | `"hello"` |
| `int`, `int64` | Integer | `42` |
| `float64` | Floating point | `3.14` |
| `bool` | Boolean | `true`, `false` |
| `nil` | Null value | `nil` |

### 5.2 Structured Types

**JSON-Compatible Map:**

```go
payload := map[string]interface{}{
    "temperature": 25.5,
    "humidity":    60,
    "timestamp":   "2025-12-10T12:00:00Z",
}
```

**Slices:**

```go
payload := []interface{}{1, 2, 3, 4, 5}
payload := []map[string]interface{}{
    {"id": 1, "name": "Alice"},
    {"id": 2, "name": "Bob"},
}
```

### 5.3 Binary Types

**Byte Slice:**

```go
payload := []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F}  // "Hello"
```

**Copy-on-Write Buffer:**

```go
payload := message.NewCowBuffer(largeData)
```

### 5.4 Custom Types

**Struct:**

```go
type SensorReading struct {
    DeviceID    string    `json:"device_id"`
    Temperature float64   `json:"temperature"`
    Humidity    float64   `json:"humidity"`
    Timestamp   time.Time `json:"timestamp"`
}

payload := &SensorReading{
    DeviceID:    "sensor-01",
    Temperature: 25.5,
    Humidity:    60,
    Timestamp:   time.Now(),
}
```

**Type Registry (Future):**

```go
package message

var typeRegistry = make(map[string]reflect.Type)

func RegisterType(name string, typ reflect.Type) {
    typeRegistry[name] = typ
}

func NewPayload(typeName string) (interface{}, error) {
    typ, exists := typeRegistry[typeName]
    if !exists {
        return nil, fmt.Errorf("unknown type: %s", typeName)
    }
    return reflect.New(typ).Interface(), nil
}
```

### 5.5 Type Assertions

**Safe Type Assertions:**

```go
func processMessage(msg *message.Message) error {
    // String payload
    if str, ok := msg.Payload.(string); ok {
        return processString(str)
    }

    // JSON-like payload
    if obj, ok := msg.Payload.(map[string]interface{}); ok {
        return processObject(obj)
    }

    // Binary payload
    if data, ok := msg.Payload.([]byte); ok {
        return processBinary(data)
    }

    return errors.New("unsupported payload type")
}
```

**Type Switch:**

```go
func routeByType(msg *message.Message) string {
    switch msg.Payload.(type) {
    case string:
        return "text_handler"
    case []byte:
        return "binary_handler"
    case map[string]interface{}:
        return "json_handler"
    default:
        return "unknown_handler"
    }
}
```

---

## 6. Serialization Formats

### 6.1 When Serialization is Required

1. **WASI Nodes**: Messages cross WASM boundary
2. **Logging**: Persisting messages to disk/database
3. **Export**: Sending messages over network (HTTP, MQTT)
4. **Debugging**: Inspecting message contents

### 6.2 JSON Serialization

**Standard Format:**

```json
{
  "id": "01HQJXZ3F8T9Y2K6E7M5P1N4C6",
  "metadata": {
    "timestamp": "2025-12-10T12:00:00Z",
    "source_node": "inject1",
    "trace_id": "abc123"
  },
  "payload": {
    "temperature": 25.5,
    "humidity": 60
  },
  "context": {
    "batch_id": "batch-001"
  }
}
```

**Go Implementation:**

```go
package message

import "encoding/json"

// MarshalJSON serializes message to JSON
func (m *Message) MarshalJSON() ([]byte, error) {
    return json.Marshal(map[string]interface{}{
        "id":       m.ID,
        "metadata": m.Metadata,
        "payload":  m.Payload,
        "context":  m.Context,
    })
}

// UnmarshalJSON deserializes message from JSON
func (m *Message) UnmarshalJSON(data []byte) error {
    var raw map[string]interface{}
    if err := json.Unmarshal(data, &raw); err != nil {
        return err
    }

    m.ID = raw["id"].(string)
    m.Metadata = raw["metadata"].(map[string]string)
    m.Payload = raw["payload"]
    m.Context = raw["context"].(map[string]interface{})
    return nil
}
```

### 6.3 MessagePack Serialization

**More compact than JSON, faster serialization:**

```go
import "github.com/vmihailenco/msgpack/v5"

func (m *Message) MarshalMsgpack() ([]byte, error) {
    return msgpack.Marshal(map[string]interface{}{
        "id":       m.ID,
        "metadata": m.Metadata,
        "payload":  m.Payload,
        "context":  m.Context,
    })
}

func (m *Message) UnmarshalMsgpack(data []byte) error {
    var raw map[string]interface{}
    if err := msgpack.Unmarshal(data, &raw); err != nil {
        return err
    }

    // ... unmarshal fields
    return nil
}
```

### 6.4 Protocol Buffers (Future)

**For high-performance serialization:**

```protobuf
syntax = "proto3";

message Message {
  string id = 1;
  map<string, string> metadata = 2;
  bytes payload = 3;  // Serialized payload
  map<string, bytes> context = 4;
}
```

### 6.5 Serialization Configuration

**Runtime Configuration:**

```yaml
runtime:
  serialization_format: "json"  # json, msgpack, protobuf
  pretty_print: false           # Pretty-print JSON (debugging)
```

---

## 7. Message Routing Semantics

### 7.1 Unicast Routing

**Definition:** Message sent to a single downstream node.

```
[Node A] -> [Node B]
```

**YAML:**

```yaml
wires:
  - from: nodeA
    to: nodeB
```

**Behavior:** Message passed by reference (zero-copy).

### 7.2 Multicast Routing (Fan-Out)

**Definition:** Message sent to multiple downstream nodes.

```
[Node A] -> [Node B]
         -> [Node C]
         -> [Node D]
```

**YAML:**

```yaml
wires:
  - from: nodeA
    to: nodeB
  - from: nodeA
    to: nodeC
  - from: nodeA
    to: nodeD
```

**Behavior:** Message cloned for each output (copy-on-write).

### 7.3 Named Output Routing

**Definition:** Message emitted to specific named output port.

```
[HTTP Node] -> success -> [Process]
           -> error -> [Log Error]
           -> timeout -> [Retry]
```

**YAML:**

```yaml
wires:
  - from: http1
    output: success
    to: process
  - from: http1
    output: error
    to: log_error
  - from: http1
    output: timeout
    to: retry
```

**Node Implementation:**

```go
func (n *HTTPNode) Process(ctx context.Context, msg *Message) (*Result, error) {
    resp, err := n.client.Do(req)

    var outputPort string
    if err != nil {
        if isTimeout(err) {
            outputPort = "timeout"
        } else {
            outputPort = "error"
        }
    } else {
        outputPort = "success"
    }

    return &Result{
        Messages: []*OutputMessage{
            {Output: outputPort, Message: msg},
        },
    }, nil
}
```

### 7.4 Broadcast Routing

**Definition:** Same message cloned to all connected outputs (semantic equivalent to multicast).

**Implementation:** Runtime clones message for each wire.

### 7.5 Conditional Routing (Future)

**Proposal:** Wire-level conditions for routing without script nodes.

```yaml
wires:
  - from: sensor
    to: alert
    condition: "payload.temperature > 80"
  - from: sensor
    to: logger
    condition: "payload.temperature <= 80"
```

---

## 8. Multi-Output Handling

### 8.1 Single Message to Multiple Outputs

```go
return &Result{
    Messages: []*OutputMessage{
        {Output: "success", Message: msg},
    },
}, nil
```

### 8.2 Multiple Messages to Single Output

```go
return &Result{
    Messages: []*OutputMessage{
        {Output: "default", Message: msg1},
        {Output: "default", Message: msg2},
        {Output: "default", Message: msg3},
    },
}, nil
```

### 8.3 Multiple Messages to Multiple Outputs

```go
return &Result{
    Messages: []*OutputMessage{
        {Output: "out1", Message: msg1},
        {Output: "out1", Message: msg2},
        {Output: "out2", Message: msg3},
        {Output: "out2", Message: msg4},
    },
}, nil
```

### 8.4 Null Output (Drop Message)

```go
// Drop message - no output
return &Result{
    Messages: nil,
}, nil
```

---

## 9. Message Lifecycle

### 9.1 Message Creation

**Source Nodes:** Create new messages.

```go
func (n *InjectNode) emit() {
    msg := message.NewMessage(n.config.Payload)
    msg.Metadata[message.MetaTimestamp] = time.Now().Format(time.RFC3339)
    msg.Metadata[message.MetaSourceNode] = n.id

    n.outputChan <- msg
}
```

### 9.2 Message Transit

**Routing:** Runtime routes messages between nodes via channels.

```go
// Simplified routing loop
for msg := range node.inputChan {
    result, err := node.Process(ctx, msg)
    if err != nil {
        handleError(err)
        continue
    }

    // Route output messages
    for _, out := range result.Messages {
        targetNode := n.router.GetTarget(node.ID, out.Output)
        targetNode.inputChan <- out.Message
    }
}
```

### 9.3 Message Modification

**Transformation Nodes:** Clone and modify messages.

```go
func (n *TransformNode) Process(ctx context.Context, msg *Message) (*Result, error) {
    outMsg := msg.Clone()
    outMsg.Payload = n.transform(msg.Payload)
    outMsg.Metadata[message.MetaProcessedAt] = time.Now().Format(time.RFC3339)

    return &Result{
        Messages: []*OutputMessage{{Message: outMsg}},
    }, nil
}
```

### 9.4 Message Termination

**Sink Nodes:** Consume messages without forwarding.

```go
func (n *DebugNode) Process(ctx context.Context, msg *Message) (*Result, error) {
    n.logger.Info("message received",
        "id", msg.ID,
        "payload", msg.Payload,
    )

    // No output - message terminates here
    return &Result{Messages: nil}, nil
}
```

### 9.5 Message Garbage Collection

**Go GC:** Automatically reclaims messages when no references remain.

**Manual Cleanup:**

```go
// For large payloads, explicitly release
if cowBuf, ok := msg.Payload.(*CowBuffer); ok {
    if cowBuf.Release() {
        // Last reference - can free buffer
    }
}
```

---

## 10. Best Practices

### 10.1 Message Cloning

**Rule:** Clone only when modifying payload or routing to multiple outputs.

```go
// BAD: Unnecessary clone
func (n *Node) Process(ctx context.Context, msg *Message) (*Result, error) {
    outMsg := msg.Clone()  // Not needed if not modifying
    return &Result{Messages: []*OutputMessage{{Message: outMsg}}}, nil
}

// GOOD: Clone only if modifying
func (n *Node) Process(ctx context.Context, msg *Message) (*Result, error) {
    if needsModification {
        outMsg := msg.Clone()
        outMsg.Payload = modified
        return &Result{Messages: []*OutputMessage{{Message: outMsg}}}, nil
    }
    // Pass by reference
    return &Result{Messages: []*OutputMessage{{Message: msg}}}, nil
}
```

### 10.2 Metadata Usage

**Do:**
- Use metadata for routing decisions
- Add timestamps for tracing
- Include correlation IDs

**Don't:**
- Store large data in metadata (use payload)
- Mutate metadata without cloning
- Use metadata as primary data store

### 10.3 Payload Design

**Prefer Immutable Payloads:**

```go
// Immutable struct
type Reading struct {
    Value     float64
    Timestamp time.Time
}

// Return new instance instead of modifying
func transform(r Reading) Reading {
    return Reading{
        Value:     r.Value * 2,
        Timestamp: time.Now(),
    }
}
```

### 10.4 Context Usage

**Minimize Context:**

```go
// BAD: Accumulating data in context
msg.Context["step1_result"] = result1
msg.Context["step2_result"] = result2
msg.Context["step3_result"] = result3

// GOOD: Aggregate in payload
msg.Payload = map[string]interface{}{
    "step1": result1,
    "step2": result2,
    "step3": result3,
}
```

### 10.5 Large Payloads

**Use Streaming or References:**

```go
// Instead of loading entire file
msg.Payload = []byte{...} // 1GB

// Store file path or stream handle
msg.Payload = &FileReference{
    Path: "/data/large-file.bin",
    Size: 1_000_000_000,
}
```

---

## 11. Open Questions

### 11.1 Message Priority

**Question:** Should messages support priority levels for scheduling?

**Proposal:**
```go
type Message struct {
    Priority Priority  // High, Normal, Low
}
```

**Use Case:** Critical alerts processed before routine telemetry.

### 11.2 Message Expiry

**Question:** Should messages have TTL (time-to-live)?

**Proposal:**
```go
type Message struct {
    ExpiresAt time.Time
}

// Runtime drops expired messages
if time.Now().After(msg.ExpiresAt) {
    return // Drop message
}
```

### 11.3 Message Batching

**Question:** Should runtime support automatic message batching?

**Use Case:** Accumulate 100 messages, then emit batch to reduce overhead.

**Proposal:**
```yaml
wires:
  - from: sensor
    to: batch_processor
    batch_size: 100
    batch_timeout_ms: 1000
```

### 11.4 Schema Validation

**Question:** Should messages support schema validation?

**Proposal:**
```go
msg.Metadata[message.MetaSchema] = "sensor-reading-v1"

// Runtime validates against registered schema
if err := validateSchema(msg); err != nil {
    // Reject message
}
```

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-10 | Initial | First draft |

---

## References

- [01-overview.md](./01-overview.md) - Project Overview
- [02-flow-format.md](./02-flow-format.md) - Flow Format Specification
- [03-node-spec.md](./03-node-spec.md) - Node Interface Specification
- [05-runtime.md](./05-runtime.md) - Runtime Architecture
- [Go Memory Model](https://go.dev/ref/mem)
