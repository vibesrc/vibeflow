# Vibeflow Runtime Architecture Specification

**Version:** 1.0
**Last Updated:** 2025-12-10
**Status:** Draft

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Runtime Initialization](#2-runtime-initialization)
3. [Flow Loading and Validation](#3-flow-loading-and-validation)
4. [Node Registry and Instantiation](#4-node-registry-and-instantiation)
5. [Scheduler Design](#5-scheduler-design)
6. [Message Routing Algorithm](#6-message-routing-algorithm)
7. [Backpressure Mechanisms](#7-backpressure-mechanisms)
8. [Error Handling and Recovery](#8-error-handling-and-recovery)
9. [Hot Reloading Protocol](#9-hot-reloading-protocol)
10. [Diagnostics and Observability](#10-diagnostics-and-observability)
11. [Script Engine Integration](#11-script-engine-integration)
12. [Performance Considerations](#12-performance-considerations)
13. [Open Questions](#13-open-questions)

---

## 1. Introduction

### 1.1 Purpose

The Vibeflow runtime is the execution engine responsible for loading flows, instantiating nodes, routing messages, and managing the lifecycle of the entire system. This document specifies the runtime architecture and behavior.

### 1.2 Runtime Responsibilities

1. **Flow Management**
   - Parse and validate YAML flow definitions
   - Instantiate node instances from registry
   - Build message routing graph

2. **Execution**
   - Schedule node execution on goroutines
   - Route messages between nodes via channels
   - Handle backpressure and flow control

3. **Lifecycle Management**
   - Initialize nodes in dependency order
   - Graceful shutdown with timeout
   - Hot reload flows when safe

4. **Observability**
   - Emit metrics (Prometheus)
   - Distributed tracing (OpenTelemetry)
   - Structured logging (slog)

5. **Error Handling**
   - Catch panics in node execution
   - Retry transient errors
   - Dead letter queue for failed messages

### 1.3 Architecture Overview

```
+-------------------------------------------------------------+
|                     Vibeflow Runtime                        |
+-------------------------------------------------------------+
|                                                             |
| +---------------+     +--------------+                      |
| |  Flow Loader  | --> |  Validator   |                      |
| +---------------+     +--------------+                      |
|                             |                               |
|                             v                               |
| +-------------------------------------------------------+   |
| |           Flow Graph Builder                          |   |
| |  - Dependency analysis                                |   |
| |  - Cycle detection                                    |   |
| |  - Topological sort                                   |   |
| +-------------------------------------------------------+   |
|                             |                               |
|                             v                               |
| +-------------------------------------------------------+   |
| |          Node Instantiation                           |   |
| |  - Registry lookup                                    |   |
| |  - Configuration binding                              |   |
| |  - Init() calls                                       |   |
| +-------------------------------------------------------+   |
|                             |                               |
|                             v                               |
| +-------------------------------------------------------+   |
| |          Message Router                               |   |
| |  - Channel creation (buffered)                        |   |
| |  - Wire connections                                   |   |
| |  - Routing table                                      |   |
| +-------------------------------------------------------+   |
|                             |                               |
|                             v                               |
| +-------------------------------------------------------+   |
| |          Scheduler                                    |   |
| |  - Goroutine pool                                     |   |
| |  - Node execution loops                               |   |
| |  - Backpressure control                               |   |
| +-------------------------------------------------------+   |
|                             |                               |
|                             v                               |
| +-------------------------------------------------------+   |
| |          Observability                                |   |
| |  - Metrics collector                                  |   |
| |  - Trace exporter                                     |   |
| |  - Logger                                             |   |
| +-------------------------------------------------------+   |
|                                                             |
+-------------------------------------------------------------+
```

---

## 2. Runtime Initialization

### 2.1 Startup Sequence

```
1. Parse CLI arguments / Load config file
         |
         v
2. Initialize logging
         |
         v
3. Initialize metrics/tracing
         |
         v
4. Load flow file(s)
         |
         v
5. Validate flow schema
         |
         v
6. Build flow graph
         |
         v
7. Instantiate nodes
         |
         v
8. Initialize nodes (Init())
         |
         v
9. Create channels and wire connections
         |
         v
10. Start node goroutines
         |
         v
11. Start scheduler
         |
         v
12. Enter main loop (wait for shutdown signal)
```

### 2.2 Runtime Struct

```go
package runtime

import (
    "context"
    "sync"

    "github.com/bherbruck/vibeflow/pkg/flow"
    "github.com/bherbruck/vibeflow/pkg/node"
)

type Runtime struct {
    // Configuration
    config Config

    // Flow definitions
    flows []*flow.Flow

    // Node instances
    nodes map[string]*NodeInstance

    // Routing table
    router *Router

    // Scheduler
    scheduler *Scheduler

    // Observability
    logger  Logger
    metrics *Metrics
    tracer  *Tracer

    // Lifecycle
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup

    // Hot reload
    reloadCh chan *flow.Flow
}

type Config struct {
    LogLevel           string
    MaxGoroutines      int
    BufferSize         int
    ShutdownTimeoutMS  int
    HotReload          bool
    MetricsEnabled     bool
    TracingEnabled     bool
    ScriptTimeoutMS    int
}

type NodeInstance struct {
    ID      string
    Node    node.Node
    InputCh chan *message.Message
    Outputs map[string]chan *message.Message
    Config  node.Config
}
```

### 2.3 Initialization Code

```go
func NewRuntime(config Config) (*Runtime, error) {
    ctx, cancel := context.WithCancel(context.Background())

    rt := &Runtime{
        config:   config,
        nodes:    make(map[string]*NodeInstance),
        ctx:      ctx,
        cancel:   cancel,
        reloadCh: make(chan *flow.Flow, 1),
    }

    // Initialize logging
    rt.logger = initLogger(config.LogLevel)

    // Initialize metrics
    if config.MetricsEnabled {
        rt.metrics = initMetrics()
    }

    // Initialize tracing
    if config.TracingEnabled {
        rt.tracer = initTracer()
    }

    return rt, nil
}

func (rt *Runtime) LoadFlow(path string) error {
    rt.logger.Info("loading flow", "path", path)

    // Load YAML
    flowDef, err := flow.LoadFromFile(path)
    if err != nil {
        return fmt.Errorf("failed to load flow: %w", err)
    }

    // Validate
    if err := flowDef.Validate(); err != nil {
        return fmt.Errorf("invalid flow: %w", err)
    }

    rt.flows = append(rt.flows, flowDef)
    return nil
}

func (rt *Runtime) Start() error {
    rt.logger.Info("starting runtime")

    // Build flow graph
    if err := rt.buildGraph(); err != nil {
        return err
    }

    // Instantiate nodes
    if err := rt.instantiateNodes(); err != nil {
        return err
    }

    // Initialize nodes
    if err := rt.initializeNodes(); err != nil {
        return err
    }

    // Create routing channels
    if err := rt.createRouting(); err != nil {
        return err
    }

    // Start scheduler
    rt.scheduler = NewScheduler(rt)
    if err := rt.scheduler.Start(); err != nil {
        return err
    }

    rt.logger.Info("runtime started")
    return nil
}
```

---

## 3. Flow Loading and Validation

### 3.1 YAML Parsing

```go
package flow

import (
    "os"
    "gopkg.in/yaml.v3"
)

func LoadFromFile(path string) (*Flow, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var flow Flow
    if err := yaml.Unmarshal(data, &flow); err != nil {
        return nil, fmt.Errorf("invalid YAML: %w", err)
    }

    return &flow, nil
}
```

### 3.2 Schema Validation

```go
func (f *Flow) Validate() error {
    validator := &FlowValidator{}

    // Check version
    if err := validator.validateVersion(f.Version); err != nil {
        return err
    }

    // Check for duplicate node IDs
    if err := validator.validateUniqueIDs(f.Nodes); err != nil {
        return err
    }

    // Check wire references
    if err := validator.validateWires(f.Wires, f.Nodes); err != nil {
        return err
    }

    // Check for cycles (if DAG-only mode)
    if err := validator.checkCycles(f); err != nil {
        return err
    }

    // Validate node configs
    for _, node := range f.Nodes {
        if err := validator.validateNodeConfig(node); err != nil {
            return err
        }
    }

    return nil
}
```

### 3.3 Dependency Analysis

```go
type FlowGraph struct {
    Nodes map[string]*GraphNode
    Edges []*GraphEdge
}

type GraphNode struct {
    ID         string
    Definition *flow.NodeDefinition
    Inputs     []*GraphEdge
    Outputs    []*GraphEdge
}

type GraphEdge struct {
    From       *GraphNode
    To         *GraphNode
    OutputPort string
    InputPort  string
}

func buildGraph(f *Flow) (*FlowGraph, error) {
    graph := &FlowGraph{
        Nodes: make(map[string]*GraphNode),
    }

    // Create nodes
    for _, nodeDef := range f.Nodes {
        graph.Nodes[nodeDef.ID] = &GraphNode{
            ID:         nodeDef.ID,
            Definition: nodeDef,
        }
    }

    // Create edges
    for _, wire := range f.Wires {
        from := graph.Nodes[wire.From]
        to := graph.Nodes[wire.To]

        edge := &GraphEdge{
            From:       from,
            To:         to,
            OutputPort: wire.Output,
            InputPort:  wire.Input,
        }

        from.Outputs = append(from.Outputs, edge)
        to.Inputs = append(to.Inputs, edge)
        graph.Edges = append(graph.Edges, edge)
    }

    return graph, nil
}
```

### 3.4 Cycle Detection

```go
func (g *FlowGraph) HasCycle() (bool, []string) {
    visited := make(map[string]bool)
    recStack := make(map[string]bool)
    path := []string{}

    var detectCycle func(nodeID string) bool
    detectCycle = func(nodeID string) bool {
        visited[nodeID] = true
        recStack[nodeID] = true
        path = append(path, nodeID)

        node := g.Nodes[nodeID]
        for _, edge := range node.Outputs {
            nextID := edge.To.ID
            if !visited[nextID] {
                if detectCycle(nextID) {
                    return true
                }
            } else if recStack[nextID] {
                // Cycle detected
                return true
            }
        }

        recStack[nodeID] = false
        path = path[:len(path)-1]
        return false
    }

    for nodeID := range g.Nodes {
        if !visited[nodeID] {
            if detectCycle(nodeID) {
                return true, path
            }
        }
    }

    return false, nil
}
```

---

## 4. Node Registry and Instantiation

### 4.1 Node Registry

```go
package node

import "sync"

var (
    registry   = make(map[string]Factory)
    registryMu sync.RWMutex
)

type Factory interface {
    Create() Node
}

func Register(nodeType string, factory Factory) {
    registryMu.Lock()
    defer registryMu.Unlock()

    if _, exists := registry[nodeType]; exists {
        panic(fmt.Sprintf("node type already registered: %s", nodeType))
    }

    registry[nodeType] = factory
}

func Get(nodeType string) (Node, error) {
    registryMu.RLock()
    defer registryMu.RUnlock()

    factory, exists := registry[nodeType]
    if !exists {
        return nil, fmt.Errorf("unknown node type: %s", nodeType)
    }

    return factory.Create(), nil
}

func List() []string {
    registryMu.RLock()
    defer registryMu.RUnlock()

    types := make([]string, 0, len(registry))
    for nodeType := range registry {
        types = append(types, nodeType)
    }
    return types
}
```

### 4.2 Node Instantiation

```go
func (rt *Runtime) instantiateNodes() error {
    for _, flow := range rt.flows {
        for _, nodeDef := range flow.Nodes {
            // Skip disabled nodes
            if !nodeDef.Enabled {
                rt.logger.Info("skipping disabled node", "id", nodeDef.ID)
                continue
            }

            // Lookup node type in registry
            nodeInstance, err := node.Get(nodeDef.Type)
            if err != nil {
                return fmt.Errorf("failed to instantiate node %s: %w", nodeDef.ID, err)
            }

            // Create node instance wrapper
            instance := &NodeInstance{
                ID:      nodeDef.ID,
                Node:    nodeInstance,
                InputCh: make(chan *message.Message, rt.config.BufferSize),
                Outputs: make(map[string]chan *message.Message),
                Config: node.Config{
                    ID:      nodeDef.ID,
                    Name:    nodeDef.Name,
                    Config:  nodeDef.Config,
                    Outputs: nodeDef.Outputs,
                    Runtime: &node.RuntimeContext{
                        Logger:      rt.logger,
                        Metrics:     rt.metrics,
                        Environment: flow.Environment,
                    },
                },
            }

            rt.nodes[nodeDef.ID] = instance
            rt.logger.Info("instantiated node", "id", nodeDef.ID, "type", nodeDef.Type)
        }
    }

    return nil
}
```

### 4.3 Node Initialization

```go
func (rt *Runtime) initializeNodes() error {
    rt.logger.Info("initializing nodes")

    // Initialize in topological order (sources first)
    order, err := rt.topologicalSort()
    if err != nil {
        return err
    }

    for _, nodeID := range order {
        instance := rt.nodes[nodeID]

        rt.logger.Debug("initializing node", "id", nodeID)

        ctx, cancel := context.WithTimeout(rt.ctx, 5*time.Second)
        defer cancel()

        if err := instance.Node.Init(ctx, instance.Config); err != nil {
            return fmt.Errorf("node %s initialization failed: %w", nodeID, err)
        }

        rt.logger.Info("node initialized", "id", nodeID)
    }

    return nil
}

func (rt *Runtime) topologicalSort() ([]string, error) {
    graph := rt.buildDependencyGraph()

    visited := make(map[string]bool)
    order := []string{}

    var visit func(nodeID string) error
    visit = func(nodeID string) error {
        if visited[nodeID] {
            return nil
        }

        visited[nodeID] = true

        // Visit dependencies first
        for _, dep := range graph[nodeID] {
            if err := visit(dep); err != nil {
                return err
            }
        }

        order = append(order, nodeID)
        return nil
    }

    for nodeID := range rt.nodes {
        if err := visit(nodeID); err != nil {
            return nil, err
        }
    }

    return order, nil
}
```

---

## 5. Scheduler Design

### 5.1 Goroutine-Per-Node Model

**Design:** Each node runs in its own goroutine, reading from input channel and writing to output channels.

**Benefits:**
- Simple mental model
- Natural concurrency
- Go scheduler handles load balancing
- Backpressure via buffered channels

**Drawbacks:**
- Goroutine overhead for many nodes (acceptable for Go)
- No global priority scheduling

### 5.2 Scheduler Implementation

```go
package runtime

type Scheduler struct {
    runtime *Runtime
    wg      sync.WaitGroup
}

func NewScheduler(rt *Runtime) *Scheduler {
    return &Scheduler{runtime: rt}
}

func (s *Scheduler) Start() error {
    s.runtime.logger.Info("starting scheduler")

    // Start goroutine for each node
    for nodeID, instance := range s.runtime.nodes {
        s.wg.Add(1)
        go s.nodeLoop(nodeID, instance)
    }

    // Start source nodes (inject timers, listeners, etc.)
    for _, instance := range s.runtime.nodes {
        if isSourceNode(instance.Node) {
            s.wg.Add(1)
            go s.sourceLoop(instance)
        }
    }

    return nil
}

func (s *Scheduler) nodeLoop(nodeID string, instance *NodeInstance) {
    defer s.wg.Done()
    defer s.recoverPanic(nodeID)

    logger := s.runtime.logger.With("node_id", nodeID)
    logger.Info("node loop started")

    for {
        select {
        case <-s.runtime.ctx.Done():
            logger.Info("node loop stopping")
            return

        case msg := <-instance.InputCh:
            if msg == nil {
                continue
            }

            // Process message
            result, err := s.processMessage(instance, msg)
            if err != nil {
                logger.Error("message processing failed",
                    "message_id", msg.ID,
                    "error", err,
                )
                s.handleError(instance, msg, err)
                continue
            }

            // Route output messages
            if result != nil {
                s.routeMessages(instance, result.Messages)
            }
        }
    }
}

func (s *Scheduler) processMessage(instance *NodeInstance, msg *message.Message) (*node.Result, error) {
    ctx := s.runtime.ctx

    // Add timeout for node processing
    if s.runtime.config.ScriptTimeoutMS > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, time.Duration(s.runtime.config.ScriptTimeoutMS)*time.Millisecond)
        defer cancel()
    }

    // Emit metrics
    start := time.Now()
    defer func() {
        s.runtime.metrics.NodeProcessingDuration.WithLabelValues(instance.ID).Observe(time.Since(start).Seconds())
        s.runtime.metrics.NodeMessagesProcessed.WithLabelValues(instance.ID).Inc()
    }()

    // Call node's Process method
    return instance.Node.Process(ctx, msg)
}

func (s *Scheduler) recoverPanic(nodeID string) {
    if r := recover(); r != nil {
        s.runtime.logger.Error("node panic",
            "node_id", nodeID,
            "panic", r,
            "stack", string(debug.Stack()),
        )

        // Increment panic counter
        s.runtime.metrics.NodePanics.WithLabelValues(nodeID).Inc()

        // Optionally restart node
        if s.runtime.config.RestartOnPanic {
            s.runtime.logger.Info("restarting node after panic", "node_id", nodeID)
            go s.nodeLoop(nodeID, s.runtime.nodes[nodeID])
        }
    }
}
```

### 5.3 Source Node Handling

```go
func (s *Scheduler) sourceLoop(instance *NodeInstance) {
    defer s.wg.Done()

    logger := s.runtime.logger.With("node_id", instance.ID)
    logger.Info("source node started")

    // Source nodes emit messages without input
    // Example: timer nodes, HTTP listeners

    // This is node-specific - some nodes start their own loops
    // Runtime just provides lifecycle hooks
}
```

---

## 6. Message Routing Algorithm

### 6.1 Routing Table

```go
type Router struct {
    // Map: (fromNodeID, outputPort) -> []targetChannel
    routes map[string][]chan *message.Message
}

func (rt *Runtime) createRouting() error {
    rt.router = &Router{
        routes: make(map[string][]chan *message.Message),
    }

    // Build routing table from wires
    for _, flow := range rt.flows {
        for _, wire := range flow.Wires {
            fromNode := rt.nodes[wire.From]
            toNode := rt.nodes[wire.To]

            if fromNode == nil || toNode == nil {
                return fmt.Errorf("invalid wire: from=%s to=%s", wire.From, wire.To)
            }

            // Default output port
            outputPort := wire.Output
            if outputPort == "" {
                outputPort = "default"
            }

            // Create routing key
            routeKey := routingKey(wire.From, outputPort)

            // Add target channel to routing table
            rt.router.routes[routeKey] = append(
                rt.router.routes[routeKey],
                toNode.InputCh,
            )

            rt.logger.Debug("created route",
                "from", wire.From,
                "output", outputPort,
                "to", wire.To,
            )
        }
    }

    return nil
}

func routingKey(nodeID, outputPort string) string {
    return fmt.Sprintf("%s:%s", nodeID, outputPort)
}
```

### 6.2 Message Dispatch

```go
func (s *Scheduler) routeMessages(instance *NodeInstance, messages []*node.OutputMessage) {
    for _, outMsg := range messages {
        outputPort := outMsg.Output
        if outputPort == "" {
            outputPort = "default"
        }

        routeKey := routingKey(instance.ID, outputPort)
        targets := s.runtime.router.routes[routeKey]

        if len(targets) == 0 {
            s.runtime.logger.Warn("no route found",
                "node_id", instance.ID,
                "output", outputPort,
                "message_id", outMsg.Message.ID,
            )
            continue
        }

        // Multicast: clone message for each target
        if len(targets) > 1 {
            for i, targetCh := range targets {
                msg := outMsg.Message
                if i > 0 {
                    // Clone for subsequent targets
                    msg = outMsg.Message.Clone()
                }

                select {
                case targetCh <- msg:
                    s.runtime.metrics.MessagesRouted.Inc()
                case <-s.runtime.ctx.Done():
                    return
                default:
                    // Target buffer full - backpressure
                    s.runtime.logger.Warn("target buffer full, blocking",
                        "from", instance.ID,
                        "message_id", msg.ID,
                    )
                    targetCh <- msg // Block until space available
                }
            }
        } else {
            // Unicast: send without cloning
            targetCh := targets[0]
            select {
            case targetCh <- outMsg.Message:
                s.runtime.metrics.MessagesRouted.Inc()
            case <-s.runtime.ctx.Done():
                return
            default:
                s.runtime.logger.Warn("target buffer full, blocking",
                    "from", instance.ID,
                    "message_id", outMsg.Message.ID,
                )
                targetCh <- outMsg.Message
            }
        }
    }
}
```

---

## 7. Backpressure Mechanisms

### 7.1 Buffered Channels

**Default Strategy:** Use buffered channels between nodes.

**Configuration:**

```yaml
runtime:
  buffer_size: 100  # Default channel buffer
```

**Per-Wire Buffer:**

```yaml
wires:
  - from: fast_producer
    to: slow_consumer
    buffer_size: 1000  # Larger buffer for this connection
```

### 7.2 Blocking vs. Dropping

**Blocking (Default):** When channel is full, sender blocks.

```go
targetCh <- msg  // Blocks if buffer full
```

**Dropping (Optional):** Drop messages instead of blocking.

```go
select {
case targetCh <- msg:
    // Sent successfully
default:
    // Buffer full - drop message
    s.runtime.logger.Warn("dropping message, buffer full")
    s.runtime.metrics.MessagesDropped.Inc()
}
```

**Configuration:**

```yaml
runtime:
  backpressure_strategy: "block"  # block, drop, drop_oldest
```

### 7.3 Flow Control

**Rate Limiting:** Limit message rate for specific nodes.

```yaml
nodes:
  - id: rate_limited
    type: core.http
    config:
      rate_limit: 100  # Max 100 requests/second
```

**Implementation:**

```go
import "golang.org/x/time/rate"

type RateLimitedNode struct {
    limiter *rate.Limiter
}

func (n *RateLimitedNode) Process(ctx context.Context, msg *message.Message) (*node.Result, error) {
    // Wait for rate limiter
    if err := n.limiter.Wait(ctx); err != nil {
        return nil, err
    }

    // Process message
    return n.doProcess(ctx, msg)
}
```

---

## 8. Error Handling and Recovery

### 8.1 Error Categories

| Category | Handling | Example |
|----------|----------|---------|
| **Configuration Error** | Fail at startup | Invalid YAML, missing node type |
| **Initialization Error** | Fail at startup | Node init() failure |
| **Processing Error** | Retry or route to error output | Network timeout, parse error |
| **Panic** | Recover, log, optionally restart | Nil pointer dereference |

### 8.2 Error Handling in Scheduler

```go
func (s *Scheduler) handleError(instance *NodeInstance, msg *message.Message, err error) {
    s.runtime.logger.Error("message processing error",
        "node_id", instance.ID,
        "message_id", msg.ID,
        "error", err,
    )

    s.runtime.metrics.NodeErrors.WithLabelValues(instance.ID).Inc()

    // Check if error is retryable
    if isRetryable(err) && msg.RetryCount < s.runtime.config.MaxRetries {
        msg.RetryCount++
        s.runtime.logger.Info("retrying message",
            "node_id", instance.ID,
            "message_id", msg.ID,
            "retry_count", msg.RetryCount,
        )

        // Re-enqueue message
        instance.InputCh <- msg
        return
    }

    // Send to dead letter queue
    s.sendToDeadLetter(msg, err)
}

func (s *Scheduler) sendToDeadLetter(msg *message.Message, err error) {
    if s.runtime.deadLetterCh != nil {
        deadMsg := &DeadLetter{
            Message:   msg,
            Error:     err.Error(),
            Timestamp: time.Now(),
        }
        s.runtime.deadLetterCh <- deadMsg
    }
}
```

### 8.3 Panic Recovery

```go
func (s *Scheduler) recoverPanic(nodeID string) {
    if r := recover(); r != nil {
        s.runtime.logger.Error("node panic recovered",
            "node_id", nodeID,
            "panic", r,
            "stack", string(debug.Stack()),
        )

        s.runtime.metrics.NodePanics.WithLabelValues(nodeID).Inc()

        // Optionally restart node
        if s.runtime.config.RestartOnPanic {
            time.Sleep(1 * time.Second) // Avoid restart loop
            instance := s.runtime.nodes[nodeID]
            s.wg.Add(1)
            go s.nodeLoop(nodeID, instance)
        }
    }
}
```

---

## 9. Hot Reloading Protocol

### 9.1 Hot Reload Phases

```
1. Receive reload signal (SIGHUP or API call)
         |
         v
2. Load new flow definition
         |
         v
3. Validate new flow
         |
         v
4. Compare with current flow (diff)
         |
         v
5. Drain message buffers
         |
         v
6. Shutdown changed/removed nodes
         |
         v
7. Instantiate new nodes
         |
         v
8. Update routing table
         |
         v
9. Resume message processing
```

### 9.2 Hot Reload Implementation

```go
func (rt *Runtime) Reload(newFlowPath string) error {
    rt.logger.Info("hot reload requested", "path", newFlowPath)

    // Load new flow
    newFlow, err := flow.LoadFromFile(newFlowPath)
    if err != nil {
        return fmt.Errorf("failed to load new flow: %w", err)
    }

    // Validate
    if err := newFlow.Validate(); err != nil {
        return fmt.Errorf("invalid new flow: %w", err)
    }

    // Compute diff
    diff := rt.computeFlowDiff(rt.flows[0], newFlow)

    rt.logger.Info("flow diff computed",
        "added", len(diff.AddedNodes),
        "removed", len(diff.RemovedNodes),
        "modified", len(diff.ModifiedNodes),
    )

    // Drain buffers
    rt.logger.Info("draining message buffers")
    rt.drainBuffers()

    // Shutdown removed nodes
    for _, nodeID := range diff.RemovedNodes {
        rt.shutdownNode(nodeID)
    }

    // Instantiate new nodes
    for _, nodeDef := range diff.AddedNodes {
        rt.instantiateNode(nodeDef)
    }

    // Update routing
    rt.updateRouting(newFlow)

    // Start new nodes
    for _, nodeID := range diff.AddedNodes {
        instance := rt.nodes[nodeID]
        rt.scheduler.wg.Add(1)
        go rt.scheduler.nodeLoop(nodeID, instance)
    }

    rt.logger.Info("hot reload completed")
    return nil
}

func (rt *Runtime) drainBuffers() {
    // Wait for all channels to be empty or timeout
    timeout := time.After(time.Duration(rt.config.ShutdownTimeoutMS) * time.Millisecond)

    for {
        allEmpty := true
        for _, instance := range rt.nodes {
            if len(instance.InputCh) > 0 {
                allEmpty = false
                break
            }
        }

        if allEmpty {
            return
        }

        select {
        case <-timeout:
            rt.logger.Warn("drain timeout, some messages may be lost")
            return
        case <-time.After(100 * time.Millisecond):
            // Continue waiting
        }
    }
}
```

### 9.3 Hot Reload Constraints

**Safe to Reload:**
- Adding new nodes
- Removing nodes with empty buffers
- Modifying node configuration (if node supports)

**Unsafe to Reload:**
- Nodes with active connections (TCP listeners, MQTT clients)
- Nodes with in-flight transactions
- Nodes with persistent state

**Solution:** Define node capabilities:

```go
type ReloadableNode interface {
    node.Node
    CanReload() bool
    PrepareReload() error
}
```

---

## 10. Diagnostics and Observability

### 10.1 Metrics (Prometheus)

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
    // Message metrics
    MessagesProcessed  *prometheus.CounterVec
    MessagesRouted     prometheus.Counter
    MessagesDropped    prometheus.Counter

    // Node metrics
    NodeMessagesProcessed   *prometheus.CounterVec
    NodeProcessingDuration  *prometheus.HistogramVec
    NodeErrors              *prometheus.CounterVec
    NodePanics              *prometheus.CounterVec

    // Runtime metrics
    ActiveGoroutines   prometheus.Gauge
    ChannelBufferUsage *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
    m := &Metrics{
        MessagesProcessed: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "vibeflow_messages_processed_total",
                Help: "Total number of messages processed",
            },
            []string{"node_id"},
        ),
        NodeProcessingDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "vibeflow_node_processing_duration_seconds",
                Help:    "Node processing duration in seconds",
                Buckets: prometheus.ExponentialBuckets(0.000001, 10, 8), // 1->s to 10s
            },
            []string{"node_id"},
        ),
        // ... initialize other metrics
    }

    prometheus.MustRegister(m.MessagesProcessed)
    prometheus.MustRegister(m.NodeProcessingDuration)
    // ... register other metrics

    return m
}
```

### 10.2 Distributed Tracing (OpenTelemetry)

```go
package tracing

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func (s *Scheduler) processMessage(instance *NodeInstance, msg *message.Message) (*node.Result, error) {
    // Start trace span
    ctx, span := s.runtime.tracer.Start(s.runtime.ctx, "process_message",
        trace.WithAttributes(
            attribute.String("node.id", instance.ID),
            attribute.String("message.id", msg.ID),
        ),
    )
    defer span.End()

    // Extract trace context from message metadata
    if traceID, ok := msg.GetMetadata(message.MetaTraceID); ok {
        span.SetAttributes(attribute.String("trace.id", traceID))
    }

    // Process message
    result, err := instance.Node.Process(ctx, msg)

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }

    return result, err
}
```

### 10.3 Structured Logging (slog)

```go
package logging

import "log/slog"

func initLogger(level string) *slog.Logger {
    var logLevel slog.Level
    switch level {
    case "debug":
        logLevel = slog.LevelDebug
    case "info":
        logLevel = slog.LevelInfo
    case "warn":
        logLevel = slog.LevelWarn
    case "error":
        logLevel = slog.LevelError
    default:
        logLevel = slog.LevelInfo
    }

    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: logLevel,
    })

    return slog.New(handler)
}
```

**Usage:**

```go
logger.Info("message processed",
    "node_id", nodeID,
    "message_id", msg.ID,
    "duration_ms", duration.Milliseconds(),
)
```

### 10.4 Runtime Inspection API

**HTTP API for diagnostics:**

```go
func (rt *Runtime) StartDiagnosticsServer(addr string) error {
    mux := http.NewServeMux()

    // Metrics endpoint
    mux.Handle("/metrics", promhttp.Handler())

    // Health check
    mux.HandleFunc("/health", rt.handleHealth)

    // Flow status
    mux.HandleFunc("/status", rt.handleStatus)

    // Node list
    mux.HandleFunc("/nodes", rt.handleNodes)

    // Message tracing
    mux.HandleFunc("/trace/{message_id}", rt.handleTrace)

    srv := &http.Server{
        Addr:    addr,
        Handler: mux,
    }

    rt.logger.Info("diagnostics server started", "addr", addr)
    return srv.ListenAndServe()
}

func (rt *Runtime) handleStatus(w http.ResponseWriter, r *http.Request) {
    status := map[string]interface{}{
        "runtime_version": VERSION,
        "uptime":          time.Since(rt.startTime).String(),
        "nodes":           len(rt.nodes),
        "goroutines":      runtime.NumGoroutine(),
    }

    json.NewEncoder(w).Encode(status)
}
```

---

## 11. Script Engine Integration

### 11.1 Script Engine Manager

```go
package script

type EngineManager struct {
    engines map[string]Engine
}

type Engine interface {
    Execute(ctx context.Context, code string, msg *message.Message) (*message.Message, error)
}

func NewEngineManager() *EngineManager {
    return &EngineManager{
        engines: map[string]Engine{
            "javascript": &GojaEngine{},
            "go":         &YaegiEngine{},
            "tengo":      &TengoEngine{},
            "starlark":   &StarlarkEngine{},
        },
    }
}

func (m *EngineManager) Execute(language, code string, msg *message.Message) (*message.Message, error) {
    engine, exists := m.engines[language]
    if !exists {
        return nil, fmt.Errorf("unknown script language: %s", language)
    }

    return engine.Execute(context.Background(), code, msg)
}
```

### 11.2 JavaScript Engine (goja)

```go
package script

import "github.com/dop251/goja"

type GojaEngine struct {
    vm *goja.Runtime
}

func (e *GojaEngine) Execute(ctx context.Context, code string, msg *message.Message) (*message.Message, error) {
    vm := goja.New()

    // Set timeout
    vm.SetMaxCallStackSize(1000)

    // Expose message to script
    msgObj := vm.NewObject()
    msgObj.Set("id", msg.ID)
    msgObj.Set("metadata", msg.Metadata)
    msgObj.Set("payload", msg.Payload)
    msgObj.Set("context", msg.Context)
    vm.Set("msg", msgObj)

    // Execute script
    _, err := vm.RunString(code)
    if err != nil {
        return nil, fmt.Errorf("script error: %w", err)
    }

    // Extract result
    result := vm.Get("msg")
    if goja.IsUndefined(result) || goja.IsNull(result) {
        return nil, nil // Drop message
    }

    // Convert back to message
    resultObj := result.ToObject(vm)
    outMsg := msg.Clone()
    outMsg.Payload = resultObj.Get("payload").Export()

    return outMsg, nil
}
```

---

## 12. Performance Considerations

### 12.1 Memory Management

**Object Pooling:**

```go
var messagePool = sync.Pool{
    New: func() interface{} {
        return &message.Message{
            Metadata: make(map[string]string),
            Context:  make(map[string]interface{}),
        }
    },
}

func NewMessage(payload interface{}) *message.Message {
    msg := messagePool.Get().(*message.Message)
    msg.ID = uuid.NewString()
    msg.Payload = payload
    return msg
}

func ReleaseMessage(msg *message.Message) {
    // Clear fields
    msg.Payload = nil
    for k := range msg.Metadata {
        delete(msg.Metadata, k)
    }
    for k := range msg.Context {
        delete(msg.Context, k)
    }

    messagePool.Put(msg)
}
```

### 12.2 CPU Optimization

**Reduce Allocations:**

```go
// Pre-allocate slices
result := &node.Result{
    Messages: make([]*node.OutputMessage, 0, 1),
}
```

**Avoid Reflection:**

```go
// Use type assertions instead of reflection
if payload, ok := msg.Payload.(map[string]interface{}); ok {
    // Direct access
}
```

### 12.3 Goroutine Management

**Limit Goroutines:**

```yaml
runtime:
  max_goroutines: 1000  # Limit concurrent goroutines
```

**Worker Pool (Alternative):**

```go
type WorkerPool struct {
    workers   int
    taskQueue chan Task
}

func (p *WorkerPool) Start() {
    for i := 0; i < p.workers; i++ {
        go p.worker()
    }
}

func (p *WorkerPool) worker() {
    for task := range p.taskQueue {
        task.Execute()
    }
}
```

---

## 13. Open Questions

### 13.1 Priority Scheduling

**Question:** Should messages support priority levels?

**Use Case:** Critical alerts processed before routine telemetry.

### 13.2 Distributed Runtime

**Question:** Should Vibeflow support distributed execution across multiple machines?

**Challenges:**
- Network serialization overhead
- Coordination and synchronization
- Failure handling

### 13.3 Stateful Stream Processing

**Question:** How should windowing and aggregation work?

**Proposal:** Special stateful nodes with checkpoint/restore.

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
- [04-message-model.md](./04-message-model.md) - Message Model Specification
- [06-node-manifest.md](./06-node-manifest.md) - Node Manifest Format
