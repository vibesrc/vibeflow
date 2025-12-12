package core

import (
	"context"
	"fmt"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
	"github.com/dop251/goja"
)

func init() {
	node.RegisterWithInfo("core.filter", func() node.Node { return &FilterNode{} }, node.TypeInfo{
		Type:        "core.filter",
		Name:        "Filter",
		Description: "Filters messages based on a JavaScript condition",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "input", Description: "Message to filter"}},
		Outputs:     []node.PortInfo{{Name: "output", Description: "Passed messages"}},
		Config: []node.ConfigSpec{
			{Name: "condition", Type: "string", Default: "true", Description: "JavaScript expression that returns boolean"},
			{Name: "pass_empty", Type: "bool", Default: false, Description: "Pass messages with empty/nil payload"},
		},
	})
}

// FilterNode filters messages based on a condition.
type FilterNode struct {
	id        string
	condition string // JavaScript expression that returns boolean
	compiled  *goja.Program
	passEmpty bool // Pass messages with empty/nil payload
}

func (n *FilterNode) Init(ctx context.Context, cfg *node.Config) error {
	n.id = cfg.ID
	n.condition = cfg.GetString("condition", "true")
	n.passEmpty = cfg.GetBool("pass_empty", false)

	// Compile the condition
	program, err := goja.Compile(n.id, n.condition, false)
	if err != nil {
		return fmt.Errorf("filter node %s: compile error: %w", n.id, err)
	}
	n.compiled = program

	return nil
}

func (n *FilterNode) Process(ctx context.Context, msg *message.Message, emit node.Emitter) error {
	// Handle empty payload
	if msg.Payload == nil {
		if n.passEmpty {
			emit.Emit("default", msg)
		}
		return nil
	}

	// Create VM and set variables
	vm := goja.New()
	vm.Set("msg", map[string]any{
		"id":       msg.ID,
		"payload":  msg.Payload,
		"metadata": msg.Metadata,
		"context":  msg.Context,
	})

	// Also set payload directly for convenience
	vm.Set("payload", msg.Payload)

	// Run the condition
	result, err := vm.RunProgram(n.compiled)
	if err != nil {
		// Log error but don't fail - filter drops message on error
		return nil
	}

	// Pass message if condition is true
	if result.ToBoolean() {
		emit.Emit("default", msg)
	}

	return nil
}

func (n *FilterNode) Stop(ctx context.Context) error {
	return nil
}
