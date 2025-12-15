// Package flow defines the flow structure and YAML parsing.
package flow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Flow represents a complete flow definition.
type Flow struct {
	Version     string            `yaml:"version"`
	Metadata    Metadata          `yaml:"metadata,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Nodes       []NodeDef         `yaml:"nodes"`
	Wires       []Wire            `yaml:"wires"`
}

// Metadata contains flow metadata.
type Metadata struct {
	Name        string   `yaml:"name,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

// NodeDef defines a node in the flow.
type NodeDef struct {
	ID       string         `yaml:"id"`
	Type     string         `yaml:"type"`
	Name     string         `yaml:"name,omitempty"`
	Config   map[string]any `yaml:"config,omitempty"`
	Language string         `yaml:"language,omitempty"` // For script nodes
	Enabled  *bool          `yaml:"enabled,omitempty"`
}

// IsEnabled returns whether the node is enabled (default true).
func (n *NodeDef) IsEnabled() bool {
	if n.Enabled == nil {
		return true
	}
	return *n.Enabled
}

// Wire defines a connection between nodes.
type Wire struct {
	From   string `yaml:"from"`
	To     string `yaml:"to"`
	Output string `yaml:"output"` // Required: explicit output port name
	Input  string `yaml:"input"`  // Required: explicit input port name
}

// LoadFromFile loads a flow from a YAML file.
func LoadFromFile(path string) (*Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file: %w", err)
	}

	return Parse(data)
}

// Parse parses a flow from YAML bytes.
func Parse(data []byte) (*Flow, error) {
	var flow Flow
	if err := yaml.Unmarshal(data, &flow); err != nil {
		return nil, fmt.Errorf("failed to parse flow YAML: %w", err)
	}

	if err := flow.Validate(); err != nil {
		return nil, err
	}

	return &flow, nil
}

// Validate checks the flow for errors.
func (f *Flow) Validate() error {
	if len(f.Nodes) == 0 {
		return fmt.Errorf("flow has no nodes")
	}

	// Build node ID set
	nodeIDs := make(map[string]bool)
	for _, n := range f.Nodes {
		if n.ID == "" {
			return fmt.Errorf("node missing id")
		}
		if n.Type == "" {
			return fmt.Errorf("node %q missing type", n.ID)
		}
		if nodeIDs[n.ID] {
			return fmt.Errorf("duplicate node id: %s", n.ID)
		}
		nodeIDs[n.ID] = true
	}

	// Validate wires and check for duplicates
	seenWires := make(map[string]bool)
	for _, w := range f.Wires {
		if w.From == "" {
			return fmt.Errorf("wire missing 'from'")
		}
		if w.To == "" {
			return fmt.Errorf("wire missing 'to'")
		}
		if w.Output == "" {
			return fmt.Errorf("wire from %s missing 'output' port", w.From)
		}
		if w.Input == "" {
			return fmt.Errorf("wire to %s missing 'input' port", w.To)
		}
		if !nodeIDs[w.From] {
			return fmt.Errorf("wire references unknown node: %s", w.From)
		}
		if !nodeIDs[w.To] {
			return fmt.Errorf("wire references unknown node: %s", w.To)
		}

		// Check for duplicate wires - use exact port names (no normalization)
		wireKey := fmt.Sprintf("%s:%s->%s:%s", w.From, w.Output, w.To, w.Input)
		if seenWires[wireKey] {
			return fmt.Errorf("duplicate wire: %s:%s -> %s:%s", w.From, w.Output, w.To, w.Input)
		}
		seenWires[wireKey] = true
	}

	return nil
}

// GetNode returns a node definition by ID.
func (f *Flow) GetNode(id string) *NodeDef {
	for i := range f.Nodes {
		if f.Nodes[i].ID == id {
			return &f.Nodes[i]
		}
	}
	return nil
}

// GetWiresFrom returns all wires originating from a node.
func (f *Flow) GetWiresFrom(nodeID string) []Wire {
	var wires []Wire
	for _, w := range f.Wires {
		if w.From == nodeID {
			wires = append(wires, w)
		}
	}
	return wires
}

// GetWiresTo returns all wires going to a node.
func (f *Flow) GetWiresTo(nodeID string) []Wire {
	var wires []Wire
	for _, w := range f.Wires {
		if w.To == nodeID {
			wires = append(wires, w)
		}
	}
	return wires
}
