package core

import (
	"context"
	"fmt"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
	"github.com/dop251/goja"
)

const defaultScriptCode = "return msg"

func init() {
	node.RegisterWithInfo("core.script", func() node.Node { return &ScriptNode{} }, node.TypeInfo{
		Type:        "core.script",
		Name:        "Script",
		Description: "Executes JavaScript code using goja",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "default", Description: "Message to process"}},
		Outputs:     []node.PortInfo{{Name: "default", Description: "Processed message"}},
		Config: []node.ConfigSpec{
			{Name: "code", Type: "string", Default: defaultScriptCode, Description: "JavaScript code to execute. The code runs inside a function with 'msg' as the argument. Use 'return' to output a message.", Format: "code", Language: "javascript"},
			{Name: "stateful", Type: "bool", Default: false, Description: "Reuse VM across invocations"},
		},
	})
}

// ScriptNode executes JavaScript code using goja.
type ScriptNode struct {
	id       string
	code     string
	vm       *goja.Runtime
	script   *goja.Program
	stateful bool // If true, reuse VM across invocations
	outputs  node.Outputs
	output   node.Output // default output
}

func (n *ScriptNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.code = cfg.GetString("code", defaultScriptCode)
	n.stateful = cfg.GetBool("stateful", false)
	n.outputs = outputs

	// Wrap the user code in a function
	wrappedCode := fmt.Sprintf(`
(function(msg) {
%s
})
`, n.code)

	// Compile the script
	program, err := goja.Compile(n.id, wrappedCode, false)
	if err != nil {
		return fmt.Errorf("script node %s: compilation error: %w", n.id, err)
	}
	n.script = program

	// For stateful scripts, create a persistent VM
	if n.stateful {
		n.vm = goja.New()
		n.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	}

	// Get default output if wired
	if outputs.Has("default") {
		n.output, err = outputs.Get("default")
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *ScriptNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	// Use persistent VM for stateful scripts, fresh VM otherwise
	var vm *goja.Runtime
	if n.stateful && n.vm != nil {
		vm = n.vm
	} else {
		vm = goja.New()
		vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	}

	// Run the compiled script to get the function
	val, err := vm.RunProgram(n.script)
	if err != nil {
		return fmt.Errorf("script node %s: runtime error: %w", n.id, err)
	}

	// Get the function
	fn, ok := goja.AssertFunction(val)
	if !ok {
		return fmt.Errorf("script node %s: code did not return a function", n.id)
	}

	// Convert message to JS object
	msgObj := vm.ToValue(map[string]any{
		"id":       msg.ID,
		"payload":  msg.Payload,
		"metadata": msg.Metadata,
		"context":  msg.Context,
	})

	// Call the function
	result, err := fn(goja.Undefined(), msgObj)
	if err != nil {
		return fmt.Errorf("script node %s: execution error: %w", n.id, err)
	}

	// Handle result
	if goja.IsNull(result) || goja.IsUndefined(result) {
		// Drop message
		return nil
	}

	// Convert result back to Go
	exported := result.Export()
	if exported == nil {
		return nil
	}

	switch v := exported.(type) {
	case map[string]any:
		// Check if it's a {output: string, msg: object} format
		if outputName, ok := v["output"].(string); ok {
			if msgData, ok := v["msg"].(map[string]any); ok {
				outMsg := n.mapToMessage(msgData, msg)
				// Try to get the named output
				if n.outputs.Has(outputName) {
					if out, err := n.outputs.Get(outputName); err == nil {
						out.Send(outMsg)
					}
				}
				return nil
			}
		}
		// Otherwise treat as the new message
		outMsg := n.mapToMessage(v, msg)
		if n.output != nil {
			n.output.Send(outMsg)
		}

	default:
		// Use result as payload
		outMsg := msg.Clone()
		outMsg.Payload = exported
		if n.output != nil {
			n.output.Send(outMsg)
		}
	}

	return nil
}

func (n *ScriptNode) mapToMessage(data map[string]any, original *message.Message) *message.Message {
	msg := original.Clone()

	if payload, ok := data["payload"]; ok {
		msg.Payload = payload
	}

	if metadata, ok := data["metadata"].(map[string]any); ok {
		for k, v := range metadata {
			if s, ok := v.(string); ok {
				msg.SetMeta(k, s)
			}
		}
	}

	if ctx, ok := data["context"].(map[string]any); ok {
		msg.Context = ctx
	}

	return msg
}

func (n *ScriptNode) Stop(ctx context.Context) error {
	return nil
}
