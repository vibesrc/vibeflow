package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
	node.RegisterWithInfo("core.change", func() node.Node { return &ChangeNode{} }, node.TypeInfo{
		Type:        "core.change",
		Name:        "Change",
		Description: "Modifies message properties",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "default", Description: "Message to modify"}},
		Outputs:     []node.PortInfo{{Name: "default", Description: "Modified message"}},
		Config: []node.ConfigSpec{
			{
				Name:        "rules",
				Type:        "array",
				Required:    true,
				Description: "Change rules",
				Items: []node.ConfigSpec{
					{Name: "action", Type: "string", Required: true, Description: "Action type", Options: []any{"set", "delete", "move", "copy"}},
					{Name: "property", Type: "string", Required: true, Description: "Target property (e.g., payload.data)"},
					{Name: "value", Type: "string", Description: "Value to set (for 'set' action)"},
					{Name: "from", Type: "string", Description: "Source property (for 'move' and 'copy' actions)"},
				},
			},
		},
	})
}

// ChangeNode modifies message properties.
type ChangeNode struct {
	id     string
	rules  []changeRule
	output node.Output
}

type changeRule struct {
	action   string // "set", "delete", "move", "copy"
	property string // Target property (dot notation)
	value    any    // Value for set operations
	from     string // Source property for move/copy
}

func (n *ChangeNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID

	rulesRaw, ok := cfg.Get("rules").([]any)
	if !ok || len(rulesRaw) == 0 {
		return fmt.Errorf("change node %s: missing or invalid 'rules' config", n.id)
	}

	for i, r := range rulesRaw {
		ruleMap, ok := r.(map[string]any)
		if !ok {
			return fmt.Errorf("change node %s: rule %d is not a map", n.id, i)
		}

		action, _ := ruleMap["action"].(string)
		if action == "" {
			action = "set"
		}

		property, _ := ruleMap["property"].(string)
		if property == "" {
			return fmt.Errorf("change node %s: rule %d missing property", n.id, i)
		}

		rule := changeRule{
			action:   action,
			property: property,
			value:    ruleMap["value"],
			from:     "", // Default empty
		}

		if from, ok := ruleMap["from"].(string); ok {
			rule.from = from
		}

		n.rules = append(n.rules, rule)
	}

	if outputs.Has("default") {
		var err error
		n.output, err = outputs.Get("default")
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *ChangeNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	if n.output == nil {
		return nil
	}

	// Clone the message to avoid modifying the original
	out := msg.Clone()

	for _, rule := range n.rules {
		switch rule.action {
		case "set":
			n.setProperty(out, rule.property, rule.value)
		case "delete":
			n.deleteProperty(out, rule.property)
		case "move":
			if rule.from != "" {
				val := n.getProperty(out, rule.from)
				n.setProperty(out, rule.property, val)
				n.deleteProperty(out, rule.from)
			}
		case "copy":
			if rule.from != "" {
				val := n.getProperty(out, rule.from)
				n.setProperty(out, rule.property, val)
			}
		}
	}

	n.output.Send(out)
	return nil
}

func (n *ChangeNode) setProperty(msg *message.Message, path string, value any) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return
	}

	// Handle top-level properties
	if len(parts) == 1 {
		switch parts[0] {
		case "payload":
			msg.Payload = value
		}
		return
	}

	// Handle nested properties
	var target map[string]any
	switch parts[0] {
	case "payload":
		if m, ok := msg.Payload.(map[string]any); ok {
			target = m
		} else {
			// Convert payload to map if needed
			target = make(map[string]any)
			msg.Payload = target
		}
	case "context":
		if msg.Context == nil {
			msg.Context = make(map[string]any)
		}
		target = msg.Context
	case "metadata":
		// Metadata is map[string]string, handle specially
		if len(parts) == 2 {
			if s, ok := value.(string); ok {
				msg.SetMeta(parts[1], s)
			}
		}
		return
	default:
		return
	}

	// Navigate to the parent of the target property
	for i := 1; i < len(parts)-1; i++ {
		if next, ok := target[parts[i]].(map[string]any); ok {
			target = next
		} else {
			// Create nested map
			newMap := make(map[string]any)
			target[parts[i]] = newMap
			target = newMap
		}
	}

	// Set the final property
	target[parts[len(parts)-1]] = value
}

func (n *ChangeNode) getProperty(msg *message.Message, path string) any {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}

	var current any
	switch parts[0] {
	case "payload":
		current = msg.Payload
	case "metadata":
		if len(parts) == 2 {
			return msg.GetMeta(parts[1])
		}
		return msg.Metadata
	case "context":
		current = msg.Context
	default:
		return nil
	}

	for _, part := range parts[1:] {
		if m, ok := current.(map[string]any); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

func (n *ChangeNode) deleteProperty(msg *message.Message, path string) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return
	}

	// Can't delete top-level payload/context/metadata
	if len(parts) == 1 {
		return
	}

	var target map[string]any
	switch parts[0] {
	case "payload":
		if m, ok := msg.Payload.(map[string]any); ok {
			target = m
		}
	case "context":
		target = msg.Context
	case "metadata":
		if len(parts) == 2 {
			delete(msg.Metadata, parts[1])
		}
		return
	default:
		return
	}

	if target == nil {
		return
	}

	// Navigate to parent
	for i := 1; i < len(parts)-1; i++ {
		if next, ok := target[parts[i]].(map[string]any); ok {
			target = next
		} else {
			return
		}
	}

	delete(target, parts[len(parts)-1])
}

func (n *ChangeNode) Stop(ctx context.Context) error {
	return nil
}
