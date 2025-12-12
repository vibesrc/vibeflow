package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
	"github.com/dop251/goja"
)

func init() {
	node.RegisterWithInfo("core.switch", func() node.Node { return &SwitchNode{} }, node.TypeInfo{
		Type:        "core.switch",
		Name:        "Switch",
		Description: "Routes messages to different outputs based on conditions",
		Category:    "core",
		Inputs:  []node.PortInfo{{Name: "input", Description: "Message to route"}},
		Outputs: []node.PortInfo{{Name: "default", Description: "Default output (no match)"}},
		Config: []node.ConfigSpec{
			{Name: "property", Type: "string", Default: "payload", Description: "Property to evaluate (dot notation)"},
			{
				Name:        "rules",
				Type:        "array",
				Required:    true,
				Description: "Routing rules",
				Items: []node.ConfigSpec{
					{Name: "condition", Type: "string", Required: true, Description: "JavaScript expression (e.g., value > 50)"},
					{Name: "output", Type: "string", Required: true, Description: "Output port name (e.g., out0, out1)"},
				},
			},
			{Name: "check_all", Type: "bool", Default: false, Description: "Check all rules vs stop at first match"},
			{Name: "default", Type: "string", Description: "Default output port name"},
		},
	})
}

// SwitchNode routes messages to different outputs based on conditions.
type SwitchNode struct {
	id         string
	property   string // Property to evaluate (supports dot notation)
	rules      []switchRule
	checkAll   bool // Check all rules vs stop at first match
	defaultOut string
}

type switchRule struct {
	condition string // JavaScript expression
	output    string // Output port name
	compiled  *goja.Program
}

func (n *SwitchNode) Init(ctx context.Context, cfg *node.Config) error {
	n.id = cfg.ID
	n.property = cfg.GetString("property", "payload")
	n.checkAll = cfg.GetBool("check_all", false)
	n.defaultOut = cfg.GetString("default", "")

	// Parse rules from config
	rulesRaw, ok := cfg.Get("rules").([]any)
	if !ok || len(rulesRaw) == 0 {
		return fmt.Errorf("switch node %s: missing or invalid 'rules' config", n.id)
	}

	for i, r := range rulesRaw {
		ruleMap, ok := r.(map[string]any)
		if !ok {
			return fmt.Errorf("switch node %s: rule %d is not a map", n.id, i)
		}

		condition, _ := ruleMap["condition"].(string)
		output, _ := ruleMap["output"].(string)

		if condition == "" {
			return fmt.Errorf("switch node %s: rule %d missing condition", n.id, i)
		}
		if output == "" {
			output = fmt.Sprintf("out%d", i)
		}

		// Compile the condition
		program, err := goja.Compile(fmt.Sprintf("%s_rule_%d", n.id, i), condition, false)
		if err != nil {
			return fmt.Errorf("switch node %s: rule %d compile error: %w", n.id, i, err)
		}

		n.rules = append(n.rules, switchRule{
			condition: condition,
			output:    output,
			compiled:  program,
		})
	}

	return nil
}

func (n *SwitchNode) Process(ctx context.Context, msg *message.Message, emit node.Emitter) error {
	// Get the property value to evaluate
	value := n.getProperty(msg)

	matched := false
	for _, rule := range n.rules {
		vm := goja.New()
		vm.Set("value", value)
		vm.Set("msg", map[string]any{
			"id":       msg.ID,
			"payload":  msg.Payload,
			"metadata": msg.Metadata,
			"context":  msg.Context,
		})

		result, err := vm.RunProgram(rule.compiled)
		if err != nil {
			continue // Skip rules that error
		}

		if result.ToBoolean() {
			matched = true
			emit.Emit(rule.output, msg.Clone())
			if !n.checkAll {
				return nil
			}
		}
	}

	// Send to default output if no match and default is configured
	if !matched && n.defaultOut != "" {
		emit.Emit(n.defaultOut, msg.Clone())
	}

	return nil
}

func (n *SwitchNode) getProperty(msg *message.Message) any {
	parts := strings.Split(n.property, ".")
	if len(parts) == 0 {
		return nil
	}

	var current any
	switch parts[0] {
	case "payload":
		current = msg.Payload
	case "metadata":
		current = msg.Metadata
	case "context":
		current = msg.Context
	default:
		return nil
	}

	// Navigate nested properties
	for _, part := range parts[1:] {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case map[string]string:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}

func (n *SwitchNode) Stop(ctx context.Context) error {
	return nil
}
