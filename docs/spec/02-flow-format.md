# Vibeflow Flow Format Specification

**Version:** 1.0
**Last Updated:** 2025-12-10
**Status:** Draft

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [YAML Schema Overview](#2-yaml-schema-overview)
3. [Field Definitions](#3-field-definitions)
4. [Validation Rules](#4-validation-rules)
5. [JSON Schema Definition](#5-json-schema-definition)
6. [Example Flows](#6-example-flows)
7. [Best Practices](#7-best-practices)
8. [Schema Versioning](#8-schema-versioning)
9. [Migration Guide](#9-migration-guide)
10. [Open Questions](#10-open-questions)

---

## 1. Introduction

### 1.1 Purpose

This document defines the canonical YAML format for Vibeflow flow definitions. A flow file is the primary interface for configuring automation workflows and must be:

- **Human-readable**: Easy to understand and edit manually
- **Machine-parsable**: Strict schema for validation and tooling
- **Version-controllable**: Git-friendly format with meaningful diffs
- **Portable**: Platform-independent, no embedded binaries

### 1.2 Design Principles

1. **Explicit over Implicit**: All connections and configurations are explicit
2. **Fail Fast**: Invalid configurations are rejected at load time, not runtime
3. **No Magic Values**: Special behavior requires explicit configuration
4. **Composable**: Flows can reference modules (sub-flows)
5. **Extensible**: Schema accommodates new node types without breaking changes

### 1.3 File Conventions

- **Extension**: `.yaml` or `.yml`
- **Encoding**: UTF-8
- **Indentation**: 2 spaces (recommended)
- **Naming**: `kebab-case` for file names (e.g., `sensor-pipeline.yaml`)

---

## 2. YAML Schema Overview

### 2.1 Top-Level Structure

```yaml
version: string          # Schema version (required)
metadata: object         # Flow metadata (optional)
environment: object      # Environment variables (optional)
runtime: object          # Runtime configuration (optional)
nodes: array[Node]       # Node definitions (required)
wires: array[Wire]       # Connection definitions (required)
modules: object          # Module/sub-flow references (optional)
```

### 2.2 Minimal Flow

```yaml
version: "0.1"
nodes:
  - id: inject1
    type: core.inject
    config:
      payload: "Hello, World!"
      interval_ms: 1000
  - id: debug1
    type: core.debug
wires:
  - from: inject1
    to: debug1
```

---

## 3. Field Definitions

### 3.1 Version Field

**Type:** `string`
**Required:** Yes
**Format:** Semantic versioning (`MAJOR.MINOR`)

**Description:** Specifies the flow schema version. Used for backward compatibility and migration.

**Valid Values:**
- `"0.1"`: Initial schema version
- Future versions will follow semantic versioning

**Example:**

```yaml
version: "0.1"
```

**Validation:**
- Must match regex: `^\d+\.\d++`
- Runtime must support specified version or fail with clear error

---

### 3.2 Metadata Section

**Type:** `object`
**Required:** No

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Human-readable flow name |
| `description` | string | No | Flow description |
| `version` | string | No | Flow semantic version (not schema version) |
| `author` | string | No | Flow author |
| `license` | string | No | SPDX license identifier |
| `tags` | array[string] | No | Searchable tags |
| `created` | string | No | ISO 8601 timestamp |
| `updated` | string | No | ISO 8601 timestamp |

**Example:**

```yaml
metadata:
  name: "Temperature Monitoring Flow"
  description: "Polls Modbus sensors and publishes to MQTT"
  version: "1.2.0"
  author: "Jane Doe <jane@example.com>"
  license: "MIT"
  tags:
    - iot
    - modbus
    - mqtt
  created: "2025-01-15T10:30:00Z"
  updated: "2025-12-10T14:22:00Z"
```

**Validation:**
- `version` must follow semantic versioning if provided
- `created` and `updated` must be valid ISO 8601 timestamps
- `license` should be SPDX identifier (warning if unknown)
- `tags` must be lowercase, alphanumeric with hyphens

---

### 3.3 Environment Section

**Type:** `object` (map of `string` -> `string`)
**Required:** No

**Description:** Environment variables accessible to nodes. Secrets should be loaded from external sources (env vars, vault), not hardcoded.

**Example:**

```yaml
environment:
  MQTT_BROKER: "mqtt://broker.example.com:1883"
  DEVICE_ID: "sensor-+{HOSTNAME}"
  LOG_LEVEL: "info"
```

**Validation:**
- Keys must match regex: `^[A-Z][A-Z0-9_]*+`
- Values are strings (nodes perform type conversion if needed)
- Circular references not allowed (e.g., `FOO: "+{BAR}"`, `BAR: "+{FOO}"`)

**Variable Substitution:**

Values can reference other environment variables using `+{VAR_NAME}` syntax:

```yaml
environment:
  BASE_URL: "https://api.example.com"
  ENDPOINT: "+{BASE_URL}/v1/data"
```

Runtime resolves substitutions before passing to nodes.

---

### 3.4 Runtime Section

**Type:** `object`
**Required:** No

**Fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `log_level` | string | `"info"` | Logging level: `debug`, `info`, `warn`, `error` |
| `max_goroutines` | integer | `0` (unlimited) | Max concurrent goroutines for node execution |
| `buffer_size` | integer | `100` | Default channel buffer size |
| `shutdown_timeout_ms` | integer | `5000` | Graceful shutdown timeout |
| `hot_reload` | boolean | `false` | Enable hot reloading |
| `metrics_enabled` | boolean | `true` | Enable Prometheus metrics |
| `tracing_enabled` | boolean | `false` | Enable OpenTelemetry tracing |

**Example:**

```yaml
runtime:
  log_level: "debug"
  max_goroutines: 1000
  buffer_size: 50
  shutdown_timeout_ms: 10000
  hot_reload: true
  metrics_enabled: true
  tracing_enabled: true
```

**Validation:**
- `log_level` must be one of: `debug`, `info`, `warn`, `error`
- `max_goroutines` must be >= 0 (0 means unlimited)
- `buffer_size` must be > 0
- `shutdown_timeout_ms` must be > 0

---

### 3.5 Nodes Section

**Type:** `array[Node]`
**Required:** Yes

**Description:** Array of node definitions.

#### 3.5.1 Node Schema

```yaml
id: string                # Unique identifier (required)
type: string              # Node type (required)
name: string              # Display name (optional)
enabled: boolean          # Enable/disable node (default: true)
config: object            # Node-specific configuration (optional)
language: string          # Script language (script nodes only)
outputs: array[Output]    # Output port definitions (optional)
```

#### 3.5.2 Node Fields

**`id`**
- **Type:** `string`
- **Required:** Yes
- **Format:** Must be unique within flow, valid Go identifier
- **Regex:** `^[a-zA-Z_][a-zA-Z0-9_]*+`
- **Example:** `sensor_poll_1`, `httpRequest`, `transform_temp`

**`type`**
- **Type:** `string`
- **Required:** Yes
- **Format:** Namespace-qualified type: `<namespace>.<name>`
- **Examples:** `core.inject`, `protocol.mqtt`, `custom.mynode`
- **Validation:** Must be registered in node registry

**`name`**
- **Type:** `string`
- **Required:** No
- **Description:** Human-readable label for UI/logging
- **Example:** `"Temperature Sensor Poll"`

**`enabled`**
- **Type:** `boolean`
- **Required:** No
- **Default:** `true`
- **Description:** Disabled nodes are skipped during initialization

**`config`**
- **Type:** `object`
- **Required:** Depends on node type
- **Description:** Node-specific configuration validated against node manifest
- **Example:**

```yaml
config:
  url: "https://api.example.com/data"
  method: "POST"
  headers:
    Content-Type: "application/json"
  timeout_ms: 5000
```

**`language`**
- **Type:** `string`
- **Required:** Yes (for script nodes only)
- **Values:** `javascript`, `go`, `tengo`, `starlark`
- **Example:**

```yaml
type: core.script
language: javascript
config:
  code: |
    msg.payload = msg.payload.toUpperCase();
    return msg;
```

**`outputs`**
- **Type:** `array[Output]`
- **Required:** No (defaults to single unnamed output)
- **Description:** Named output ports for multi-output nodes

```yaml
outputs:
  - name: success
  - name: error
  - name: timeout
```

#### 3.5.3 Node Examples

**Inject Node:**

```yaml
- id: inject1
  type: core.inject
  name: "Periodic Timer"
  config:
    payload: "tick"
    interval_ms: 1000
    repeat: true
```

**HTTP Request Node:**

```yaml
- id: api_call
  type: core.http
  name: "Fetch Weather Data"
  config:
    url: "https://api.weather.com/v1/current"
    method: "GET"
    headers:
      Authorization: "Bearer +{API_TOKEN}"
    timeout_ms: 5000
  outputs:
    - name: success
    - name: error
```

**Script Node:**

```yaml
- id: transform
  type: core.script
  language: javascript
  config:
    code: |
      // Convert Celsius to Fahrenheit
      if (typeof msg.payload === 'number') {
        msg.payload = (msg.payload * 9/5) + 32;
      }
      return msg;
```

**Modbus Node:**

```yaml
- id: modbus_read
  type: protocol.modbus
  config:
    address: "192.168.1.100:502"
    slave_id: 1
    function: "read_holding_registers"
    start_address: 0
    count: 10
    poll_interval_ms: 1000
```

---

### 3.6 Wires Section

**Type:** `array[Wire]`
**Required:** Yes

**Description:** Defines connections between nodes.

#### 3.6.1 Wire Schema

```yaml
from: string          # Source node ID (required)
to: string            # Target node ID (required)
output: string        # Source output port (default: "default")
input: string         # Target input port (default: "default")
buffer_size: integer  # Override default buffer size (optional)
```

#### 3.6.2 Wire Fields

**`from`**
- **Type:** `string`
- **Required:** Yes
- **Description:** Source node ID
- **Validation:** Must reference existing node

**`to`**
- **Type:** `string`
- **Required:** Yes
- **Description:** Target node ID
- **Validation:** Must reference existing node

**`output`**
- **Type:** `string`
- **Required:** No
- **Default:** `"default"`
- **Description:** Named output port on source node
- **Validation:** Must match declared output port

**`input`**
- **Type:** `string`
- **Required:** No
- **Default:** `"default"`
- **Description:** Named input port on target node
- **Validation:** Must match declared input port (if node declares inputs)

**`buffer_size`**
- **Type:** `integer`
- **Required:** No
- **Default:** Uses runtime default
- **Description:** Channel buffer size for this connection
- **Validation:** Must be > 0

#### 3.6.3 Wire Examples

**Simple Connection:**

```yaml
wires:
  - from: inject1
    to: debug1
```

**Named Outputs:**

```yaml
wires:
  - from: http_request
    output: success
    to: process_response
  - from: http_request
    output: error
    to: log_error
```

**Custom Buffer Size:**

```yaml
wires:
  - from: high_volume_source
    to: slow_processor
    buffer_size: 1000  # Larger buffer for backpressure
```

**Multiple Outputs (Fan-out):**

```yaml
wires:
  - from: sensor_read
    to: mqtt_publish
  - from: sensor_read
    to: database_insert
  - from: sensor_read
    to: debug_logger
```

---

### 3.7 Modules Section

**Type:** `object` (map of `string` -> `ModuleRef`)
**Required:** No

**Description:** References to reusable sub-flows (modules).

#### 3.7.1 Module Reference Schema

```yaml
modules:
  <module_id>:
    path: string           # File path or URL to module
    version: string        # Module version constraint (optional)
    params: object         # Parameters passed to module (optional)
```

#### 3.7.2 Example

```yaml
modules:
  sensor_processor:
    path: "./modules/sensor-processing.yaml"
    version: "^1.0.0"
    params:
      threshold: 100
      units: "celsius"

nodes:
  - id: use_module
    type: module.sensor_processor
    config:
      input_source: "modbus"
```

**Status:** Modules are a future feature. Spec reserved for forward compatibility.

---

## 4. Validation Rules

### 4.1 Structural Validation

**Rule 4.1.1: Unique Node IDs**
- All node `id` fields must be unique within the flow
- Error: `duplicate node ID: "node1"`

**Rule 4.1.2: Valid References**
- All wire `from` and `to` fields must reference existing nodes
- Error: `wire references unknown node: "nonexistent"`

**Rule 4.1.3: No Self-Loops**
- A node cannot wire to itself directly
- Error: `self-loop detected: "node1" -> "node1"`

**Rule 4.1.4: Valid Output Ports**
- If a node declares outputs, wires must reference declared ports
- Error: `node "http1" has no output port "invalid"`

**Rule 4.1.5: Disabled Node Isolation**
- Disabled nodes cannot be source or target of wires
- Warning: `wire references disabled node: "node1"`

### 4.2 Type Validation

**Rule 4.2.1: Node Type Registration**
- Node `type` must be registered in runtime
- Error: `unknown node type: "custom.undefined"`

**Rule 4.2.2: Config Schema Validation**
- Node `config` must match node manifest schema
- Error: `node "inject1" config validation failed: missing required field "interval_ms"`

**Rule 4.2.3: Script Language Validation**
- Script nodes must specify valid `language`
- Error: `script node "transform1" missing language field`

### 4.3 Semantic Validation

**Rule 4.3.1: Reachability**
- Warning if nodes are unreachable (no path from source nodes)
- Warning: `node "debug1" is unreachable`

**Rule 4.3.2: Source Nodes**
- Flow must have at least one source node (node with no inputs)
- Error: `flow has no source nodes`

**Rule 4.3.3: Dead Ends**
- Warning if nodes have no outputs and are not sinks
- Warning: `node "transform1" has no output wires`

**Rule 4.3.4: Cycles**
- Optional: Detect cycles and warn (if DAG-only mode)
- Warning: `cycle detected: node1 -> node2 -> node3 -> node1`

### 4.4 Validation Levels

| Level | Behavior | Use Case |
|-------|----------|----------|
| **Strict** | Reject on any error or warning | Production deployment |
| **Lenient** | Warn but allow warnings | Development |
| **Permissive** | Only reject on errors | Testing/experimentation |

**Configuration:**

```yaml
runtime:
  validation_level: "strict"  # strict, lenient, permissive
```

---

## 5. JSON Schema Definition

### 5.1 Full JSON Schema

```json
{
  "+schema": "http://json-schema.org/draft-07/schema#",
  "title": "Vibeflow Flow Schema",
  "type": "object",
  "required": ["version", "nodes", "wires"],
  "properties": {
    "version": {
      "type": "string",
      "pattern": "^\\d+\\.\\d++"
    },
    "metadata": {
      "type": "object",
      "properties": {
        "name": {"type": "string"},
        "description": {"type": "string"},
        "version": {
          "type": "string",
          "pattern": "^\\d+\\.\\d+\\.\\d++"
        },
        "author": {"type": "string"},
        "license": {"type": "string"},
        "tags": {
          "type": "array",
          "items": {"type": "string"}
        },
        "created": {
          "type": "string",
          "format": "date-time"
        },
        "updated": {
          "type": "string",
          "format": "date-time"
        }
      }
    },
    "environment": {
      "type": "object",
      "patternProperties": {
        "^[A-Z][A-Z0-9_]*+": {"type": "string"}
      },
      "additionalProperties": false
    },
    "runtime": {
      "type": "object",
      "properties": {
        "log_level": {
          "type": "string",
          "enum": ["debug", "info", "warn", "error"]
        },
        "max_goroutines": {
          "type": "integer",
          "minimum": 0
        },
        "buffer_size": {
          "type": "integer",
          "minimum": 1
        },
        "shutdown_timeout_ms": {
          "type": "integer",
          "minimum": 0
        },
        "hot_reload": {"type": "boolean"},
        "metrics_enabled": {"type": "boolean"},
        "tracing_enabled": {"type": "boolean"}
      }
    },
    "nodes": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "type"],
        "properties": {
          "id": {
            "type": "string",
            "pattern": "^[a-zA-Z_][a-zA-Z0-9_]*+"
          },
          "type": {"type": "string"},
          "name": {"type": "string"},
          "enabled": {"type": "boolean"},
          "config": {"type": "object"},
          "language": {
            "type": "string",
            "enum": ["javascript", "go", "tengo", "starlark"]
          },
          "outputs": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["name"],
              "properties": {
                "name": {"type": "string"}
              }
            }
          }
        }
      }
    },
    "wires": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["from", "to"],
        "properties": {
          "from": {"type": "string"},
          "to": {"type": "string"},
          "output": {"type": "string"},
          "input": {"type": "string"},
          "buffer_size": {
            "type": "integer",
            "minimum": 1
          }
        }
      }
    },
    "modules": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "required": ["path"],
        "properties": {
          "path": {"type": "string"},
          "version": {"type": "string"},
          "params": {"type": "object"}
        }
      }
    }
  }
}
```

---

## 6. Example Flows

### 6.1 Simple Timer Flow

```yaml
version: "0.1"
metadata:
  name: "Simple Timer"
  description: "Emits timestamp every second"
nodes:
  - id: timer
    type: core.inject
    config:
      payload: ""
      interval_ms: 1000
  - id: timestamp
    type: core.script
    language: javascript
    config:
      code: |
        msg.payload = new Date().toISOString();
        return msg;
  - id: output
    type: core.debug
wires:
  - from: timer
    to: timestamp
  - from: timestamp
    to: output
```

### 6.2 HTTP to MQTT Bridge

```yaml
version: "0.1"
metadata:
  name: "HTTP to MQTT Bridge"
  description: "Fetch data from API and publish to MQTT"
environment:
  API_URL: "https://api.example.com/data"
  MQTT_BROKER: "mqtt://localhost:1883"
  MQTT_TOPIC: "data/incoming"
nodes:
  - id: poller
    type: core.inject
    config:
      payload: null
      interval_ms: 5000
  - id: fetch
    type: core.http
    config:
      url: "+{API_URL}"
      method: "GET"
    outputs:
      - name: success
      - name: error
  - id: publish
    type: protocol.mqtt
    config:
      broker: "+{MQTT_BROKER}"
      topic: "+{MQTT_TOPIC}"
      qos: 1
  - id: log_error
    type: core.debug
    config:
      level: "error"
wires:
  - from: poller
    to: fetch
  - from: fetch
    output: success
    to: publish
  - from: fetch
    output: error
    to: log_error
```

### 6.3 Modbus to Database Flow

```yaml
version: "0.1"
metadata:
  name: "Modbus Data Logger"
  description: "Read Modbus registers and insert into database"
environment:
  MODBUS_HOST: "192.168.1.100:502"
  DB_URL: "postgres://user:pass@localhost/iot"
runtime:
  log_level: "debug"
  buffer_size: 50
nodes:
  - id: modbus_poll
    type: protocol.modbus
    name: "Temperature Sensor"
    config:
      address: "+{MODBUS_HOST}"
      slave_id: 1
      function: "read_holding_registers"
      start_address: 0
      count: 10
      poll_interval_ms: 1000
  - id: transform
    type: core.script
    language: javascript
    config:
      code: |
        // registers is array of uint16
        const temp = msg.payload.registers[0] / 10.0;
        msg.payload = {
          timestamp: new Date().toISOString(),
          sensor_id: "temp-01",
          value: temp,
          unit: "celsius"
        };
        return msg;
  - id: filter
    type: core.script
    language: javascript
    config:
      code: |
        // Only store if temp > 25C
        if (msg.payload.value > 25) {
          return msg;
        }
        return null;  // Drop message
  - id: db_insert
    type: core.database
    config:
      driver: "postgres"
      dsn: "+{DB_URL}"
      query: |
        INSERT INTO sensor_readings (timestamp, sensor_id, value, unit)
        VALUES (+1, +2, +3, +4)
      params:
        - "+.timestamp"
        - "+.sensor_id"
        - "+.value"
        - "+.unit"
  - id: debug
    type: core.debug
wires:
  - from: modbus_poll
    to: transform
  - from: transform
    to: filter
  - from: filter
    to: db_insert
  - from: filter
    to: debug
```

### 6.4 Complex Routing with Multi-Outputs

```yaml
version: "0.1"
metadata:
  name: "Smart Router"
  description: "Route messages based on conditions"
nodes:
  - id: input
    type: core.inject
    config:
      payload: {"temperature": 30, "humidity": 65}
      interval_ms: 2000
  - id: router
    type: core.script
    language: javascript
    config:
      code: |
        const temp = msg.payload.temperature;
        if (temp > 80) {
          return {output: "critical", msg: msg};
        } else if (temp > 50) {
          return {output: "warning", msg: msg};
        } else {
          return {output: "normal", msg: msg};
        }
    outputs:
      - name: critical
      - name: warning
      - name: normal
  - id: alert
    type: core.http
    config:
      url: "https://alerts.example.com/critical"
      method: "POST"
  - id: log_warning
    type: core.debug
    config:
      level: "warn"
  - id: log_normal
    type: core.debug
    config:
      level: "info"
wires:
  - from: input
    to: router
  - from: router
    output: critical
    to: alert
  - from: router
    output: warning
    to: log_warning
  - from: router
    output: normal
    to: log_normal
```

### 6.5 WASI Node Example

```yaml
version: "0.1"
metadata:
  name: "WebAssembly Image Processing"
  description: "Process images with WASI module"
nodes:
  - id: image_source
    type: core.file
    config:
      path: "/data/images"
      watch: true
  - id: process
    type: wasi.custom
    config:
      module_path: "./wasm/image_processor.wasm"
      function: "process_image"
      memory_limit_mb: 128
  - id: save
    type: core.file
    config:
      path: "/data/processed"
      mode: "write"
wires:
  - from: image_source
    to: process
  - from: process
    to: save
```

---

## 7. Best Practices

### 7.1 Flow Organization

**Practice 7.1.1: Logical Grouping**
- Group related nodes conceptually using comments
- Use meaningful node IDs that reflect purpose

```yaml
nodes:
  # Data ingestion
  - id: modbus_sensor_1
  - id: modbus_sensor_2

  # Transformation
  - id: normalize
  - id: aggregate

  # Output
  - id: mqtt_publish
  - id: db_insert
```

**Practice 7.1.2: Consistent Naming**
- Use `snake_case` for node IDs
- Prefix nodes by function: `fetch_`, `transform_`, `publish_`
- Use descriptive names, not `node1`, `node2`

**Practice 7.1.3: Configuration Comments**
```yaml
nodes:
  - id: api_fetch
    type: core.http
    config:
      url: "https://api.example.com"
      # Increased timeout for slow API
      timeout_ms: 10000
```

### 7.2 Environment Variable Usage

**Practice 7.2.1: Externalize Secrets**
- Never hardcode passwords, tokens, or API keys
- Use environment variables or secret management

```yaml
# BAD
config:
  password: "my_secret_password"

# GOOD
environment:
  DB_PASSWORD: "+{DB_PASSWORD}"  # From system environment
config:
  password: "+{DB_PASSWORD}"
```

**Practice 7.2.2: Provide Defaults**
```yaml
environment:
  LOG_LEVEL: "info"
  TIMEOUT_MS: "5000"
```

### 7.3 Error Handling

**Practice 7.3.1: Always Handle Errors**
- Wire error outputs to logging or alerting nodes

```yaml
outputs:
  - name: success
  - name: error
wires:
  - from: http_request
    output: error
    to: error_logger
```

**Practice 7.3.2: Dead Letter Queues**
- Send failed messages to a dead-letter node for investigation

```yaml
  - id: dead_letter
    type: core.file
    config:
      path: "/var/log/vibeflow/dead-letters"
```

### 7.4 Performance

**Practice 7.4.1: Buffer Sizing**
- Increase buffer size for high-volume connections

```yaml
wires:
  - from: high_frequency_source
    to: slow_processor
    buffer_size: 1000
```

**Practice 7.4.2: Parallelization**
- Use multiple instances of nodes for parallel processing

```yaml
nodes:
  - id: worker_1
    type: core.http
  - id: worker_2
    type: core.http
  - id: worker_3
    type: core.http
wires:
  - from: load_balancer
    to: worker_1
  - from: load_balancer
    to: worker_2
  - from: load_balancer
    to: worker_3
```

### 7.5 Testing

**Practice 7.5.1: Use Inject Nodes for Testing**
```yaml
  - id: test_inject
    type: core.inject
    enabled: false  # Disable in production
    config:
      payload: {"test": true}
```

**Practice 7.5.2: Debug Nodes**
```yaml
  - id: checkpoint_debug
    type: core.debug
    name: "Checkpoint: After Transform"
```

---

## 8. Schema Versioning

### 8.1 Versioning Strategy

**Version Format:** `MAJOR.MINOR`

- **MAJOR**: Incompatible schema changes (breaking)
- **MINOR**: Backward-compatible additions (non-breaking)

### 8.2 Version History

| Version | Date | Changes |
|---------|------|---------|
| 0.1 | 2025-12-10 | Initial schema |

### 8.3 Compatibility Rules

**Rule 8.3.1: Runtime Support**
- Runtime must declare supported schema versions
- Runtime MUST reject flows with unsupported versions
- Error: `unsupported flow schema version: "0.5" (runtime supports: "0.1")`

**Rule 8.3.2: Forward Compatibility**
- Minor version increases are backward-compatible
- Runtime v0.2 must load v0.1 flows

**Rule 8.3.3: Migration**
- Breaking changes require migration tools
- Migration path: v0.x -> v1.0

### 8.4 Deprecation Policy

**Process:**
1. **Announce**: Document deprecated fields in changelog
2. **Warn**: Runtime emits warnings for deprecated features
3. **Remove**: Remove in next major version

**Example:**
```yaml
# Deprecated in v0.3, removed in v1.0
nodes:
  - id: old_node
    type: core.legacy_http  # Warning: core.legacy_http is deprecated, use core.http
```

---

## 9. Migration Guide

### 9.1 Future: v0.1 to v0.2

**Not yet applicable.** When v0.2 is released, migration steps will be documented here.

### 9.2 Migration Tools

**Future Consideration:** Provide CLI tool for automated migration:

```bash
vibeflow migrate --from 0.1 --to 0.2 flow.yaml
```

---

## 10. Open Questions

### 10.1 Conditional Wiring

**Question:** Should wires support conditional routing without script nodes?

**Proposal:**
```yaml
wires:
  - from: sensor
    to: alert
    condition: "payload.temperature > 80"
```

**Trade-offs:**
- Simplifies simple routing
- Adds complexity to wire definition
- May be better handled by dedicated router node

### 10.2 Wire Labeling

**Question:** Should wires support labels for documentation?

**Proposal:**
```yaml
wires:
  - from: sensor
    to: processor
    label: "Temperature readings"
```

### 10.3 Include Directive

**Question:** Should flows support includes for reusable snippets?

**Proposal:**
```yaml
nodes:
  !include "./common-nodes.yaml"
```

**Trade-offs:**
- Reduces duplication
- Complicates validation
- May be superseded by modules

### 10.4 Default Values

**Question:** Should node types declare default config values?

**Current:** Nodes handle defaults in code
**Alternative:** Manifest declares defaults, merged at load time

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-10 | Initial | First draft |

---

## References

- [01-overview.md](./01-overview.md) - Project Overview
- [03-node-spec.md](./03-node-spec.md) - Node Interface Specification
- [06-node-manifest.md](./06-node-manifest.md) - Node Manifest Format
- [JSON Schema Specification](https://json-schema.org/)
- [YAML 1.2 Specification](https://yaml.org/spec/1.2/spec.html)
