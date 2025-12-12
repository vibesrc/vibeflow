# Vibeflow Specification

**Version:** 0.1
**Last Updated:** 2025-12-10
**Status:** Draft

---

## Overview

Vibeflow is a high-performance, low-code flow-based programming engine built in Go. It enables developers and operators to build automation workflows, protocol integrations, and data transformation pipelines through declarative YAML configurations.

---

## Specification Documents

| Document | Description |
|----------|-------------|
| [01-overview.md](./01-overview.md) | Project vision, goals, architecture overview, and terminology |
| [02-flow-format.md](./02-flow-format.md) | YAML flow definition schema and validation rules |
| [03-node-spec.md](./03-node-spec.md) | Node interface, lifecycle, and implementation contracts |
| [04-message-model.md](./04-message-model.md) | Message structure, zero-copy semantics, and routing |
| [05-runtime.md](./05-runtime.md) | Runtime architecture, scheduling, and execution model |
| [06-node-manifest.md](./06-node-manifest.md) | Node packaging, distribution, and GitHub release model |
| [07-node-sdk.md](./07-node-sdk.md) | Node SDK, JSON-RPC protocol, and development tools |

---

## Key Design Decisions

### Node Types

| Type | Execution | Distribution | Use Case |
|------|-----------|--------------|----------|
| **Built-in (core)** | In-process | Compiled into runtime | Essential nodes (inject, debug, switch) |
| **Built-in (contrib)** | In-process | Compiled into runtime | Common protocols (HTTP, MQTT) |
| **External** | Subprocess + JSON-RPC | GitHub releases | Custom protocols, hardware access |
| **Script** | Embedded interpreter | Inline in flow YAML | Simple transformations |
| **Module** | Sub-flow expansion | Flow YAML file | Reusable patterns |

### External Node Architecture

External nodes run as separate processes communicating via JSON-RPC over stdin/stdout:

```
+-------------------+          stdin/stdout         +-------------------+
|    Vibeflow       |  <=========================>  |   External Node   |
|    Runtime        |         JSON-RPC 2.0          |   (Go binary)     |
+-------------------+                               +-------------------+
```

This enables:
- Full OS access (TCP, UDP, serial, GPIO, SPI, I2C)
- Crash isolation (node crash doesn't kill runtime)
- Distribution via GitHub releases
- Any Go library can be used

### Distribution Model

Users deploy Vibeflow via Docker:

```bash
docker run -v ./config:/config ghcr.io/bherbruck/vibeflow
```

External nodes are downloaded from GitHub releases on startup:

```yaml
# vibeflow.yaml
nodes:
  - github.com/vibeflow/nodes-contrib/modbus-tcp@v1.2.0
  - github.com/company/custom-gpio@v0.5.0

flows:
  - /config/flows/main.yaml
```

---

## Goals

- **High performance**: Near wire-speed automation through zero-copy message handling
- **Low-code usability**: Build flows visually or through simple YAML configuration
- **Modular extensibility**: Native Go nodes, script nodes, external nodes via SDK
- **Portability**: Run on edge devices, servers, and cloud environments
- **Robust ecosystem**: Enable third-party node development with minimal friction
- **Deterministic configuration**: YAML as canonical flow definition format

## Non-Goals

- Serve as a drop-in clone of Node-RED
- Embed heavyweight runtimes such as V8
- Act as a full ETL or workflow orchestration platform
- Replace full-blown application frameworks

---

## Quick Links

- **Getting Started**: Deploy with Docker, define flows in YAML
- **Building Nodes**: Use the [Node SDK](./07-node-sdk.md) to create external nodes
- **Flow Format**: See [02-flow-format.md](./02-flow-format.md) for YAML schema
- **Built-in Nodes**: See [06-node-manifest.md](./06-node-manifest.md) Section 5

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 0.1 | 2025-12-10 | Initial specification |
