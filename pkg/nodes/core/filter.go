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
	node.RegisterWithInfo("core.filter", func() node.Node { return &FilterNode{} }, node.TypeInfo{
		Type:        "core.filter",
		Name:        "Filter",
		Description: "Filters messages based on a condition",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "default", Description: "Message to filter"}},
		Outputs:     []node.PortInfo{{Name: "default", Description: "Passed messages"}},
		Config: []node.ConfigSpec{
			{Name: "condition", Type: "string", Default: "true", Description: "Expression that returns boolean (e.g., payload > 50)", Format: "expression"},
			{Name: "pass_empty", Type: "bool", Default: false, Description: "Pass messages with empty/nil payload"},
		},
	})
}

// FilterNode filters messages based on a condition.
type FilterNode struct {
	id        string
	condition string // Expression that returns boolean
	compiled  *vm.Program
	passEmpty bool // Pass messages with empty/nil payload
	output    node.Output
}

func (n *FilterNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.condition = cfg.GetString("condition", "true")
	n.passEmpty = cfg.GetBool("pass_empty", false)

	// Compile the condition with expr
	program, err := expr.Compile(n.condition, expr.AsBool())
	if err != nil {
		return fmt.Errorf("filter node %s: compile error: %w", n.id, err)
	}
	n.compiled = program

	// Get output handle
	if outputs.Has("default") {
		n.output, err = outputs.Get("default")
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *FilterNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	if n.output == nil {
		return nil
	}

	// Handle empty payload
	if msg.Payload == nil {
		if n.passEmpty {
			n.output.Send(msg)
		}
		return nil
	}

	// Build environment for expression evaluation (includes flow/node/global context)
	env := node.BuildExprEnvWithContext(ctx, msg)

	// Run the compiled expression
	result, err := expr.Run(n.compiled, env)
	if err != nil {
		// Drop message on error
		return nil
	}

	// Pass message if condition is true
	if pass, ok := result.(bool); ok && pass {
		n.output.Send(msg)
	}

	return nil
}

func (n *FilterNode) Stop(ctx context.Context) error {
	return nil
}
