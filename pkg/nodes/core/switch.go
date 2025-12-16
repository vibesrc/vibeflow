package core

import (
	"context"
	"fmt"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

func init() {
	node.RegisterWithInfo("core.switch", func() node.Node { return &SwitchNode{} }, node.TypeInfo{
		Type:        "core.switch",
		Name:        "Switch",
		Description: "Routes messages to different outputs based on conditions",
		Category:    "core",
		Inputs:  []node.PortInfo{{Name: "default", Description: "Message to route"}},
		Outputs: []node.PortInfo{{Name: "default", Description: "Default output (no match)"}},
		Config: []node.ConfigSpec{
			{Name: "property", Type: "string", Default: "payload", Description: "Property to evaluate (dot notation)"},
			{
				Name:        "rules",
				Type:        "array",
				Required:    true,
				Description: "Routing rules",
				Items: []node.ConfigSpec{
					{Name: "condition", Type: "string", Required: true, Description: "Expression (e.g., value > 50)", Format: "expression"},
					{Name: "output", Type: "string", Required: true, Description: "Output port name (e.g., out0, out1)", Format: "expression"},
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
	checkAll   bool        // Check all rules vs stop at first match
	defaultOut node.Output // Default output if no match
	outputs    node.Outputs
}

type switchRule struct {
	condition string // Expression
	output    node.Output
	compiled  *vm.Program
}

func (n *SwitchNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.property = cfg.GetString("property", "payload")
	n.checkAll = cfg.GetBool("check_all", false)
	n.outputs = outputs

	// Get default output if configured and wired
	defaultOutName := cfg.GetString("default", "")
	if defaultOutName != "" && outputs.Has(defaultOutName) {
		var err error
		n.defaultOut, err = outputs.Get(defaultOutName)
		if err != nil {
			return err
		}
	}

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
		outputName, _ := ruleMap["output"].(string)

		if condition == "" {
			return fmt.Errorf("switch node %s: rule %d missing condition", n.id, i)
		}
		if outputName == "" {
			outputName = fmt.Sprintf("out%d", i)
		}

		// Compile the condition with expr
		program, err := expr.Compile(condition, expr.AsBool())
		if err != nil {
			return fmt.Errorf("switch node %s: rule %d compile error: %w", n.id, i, err)
		}

		// Get output handle if wired
		var output node.Output
		if outputs.Has(outputName) {
			output, err = outputs.Get(outputName)
			if err != nil {
				return err
			}
		}

		n.rules = append(n.rules, switchRule{
			condition: condition,
			output:    output,
			compiled:  program,
		})
	}

	return nil
}

func (n *SwitchNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	// Get the property value to evaluate using expr
	value, _ := node.EvalExprWithContext(ctx, n.property, msg)

	// Build environment for expression evaluation (includes flow/node/global context)
	env := node.BuildExprEnvWithContext(ctx, msg)
	env["value"] = value

	matched := false
	for _, rule := range n.rules {
		result, err := expr.Run(rule.compiled, env)
		if err != nil {
			continue // Skip rules that error
		}

		if pass, ok := result.(bool); ok && pass {
			matched = true
			if rule.output != nil {
				rule.output.Send(msg.Clone())
			}
			if !n.checkAll {
				return nil
			}
		}
	}

	// Send to default output if no match and default is wired
	if !matched && n.defaultOut != nil {
		n.defaultOut.Send(msg.Clone())
	}

	return nil
}

func (n *SwitchNode) Stop(ctx context.Context) error {
	return nil
}
