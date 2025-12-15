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
		},
	})
}

// ScriptNode executes JavaScript code using goja.
type ScriptNode struct {
	id      string
	vm      *goja.Runtime
	fn      goja.Callable // Cached function reference
	outputs node.Outputs
	output  node.Output // default output
}

func (n *ScriptNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.outputs = outputs

	code := cfg.GetString("code", defaultScriptCode)

	// Wrap the user code in a function
	wrappedCode := fmt.Sprintf(`
(function(msg) {
%s
})
`, code)

	// Compile the script
	program, err := goja.Compile(n.id, wrappedCode, false)
	if err != nil {
		return fmt.Errorf("script node %s: compilation error: %w", n.id, err)
	}

	// Create VM and cache the function
	n.vm = goja.New()
	n.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	val, err := n.vm.RunProgram(program)
	if err != nil {
		return fmt.Errorf("script node %s: runtime error: %w", n.id, err)
	}
	fn, ok := goja.AssertFunction(val)
	if !ok {
		return fmt.Errorf("script node %s: code did not return a function", n.id)
	}
	n.fn = fn

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
	// Update context accessors (need fresh Go context each call)
	n.setupContextAccessors(ctx, n.vm)

	// Convert message to JS object
	msgObj := n.vm.ToValue(map[string]any{
		"id":       msg.ID,
		"payload":  msg.Payload,
		"metadata": msg.Metadata,
		"context":  msg.Context,
	})

	// Call the cached function
	result, err := n.fn(goja.Undefined(), msgObj)
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

// setupContextAccessors adds flow, node, and global context objects to the VM.
// Each object has get(key), set(key, value), delete(key), and keys() methods.
func (n *ScriptNode) setupContextAccessors(ctx context.Context, vm *goja.Runtime) {
	// Helper to create a context accessor object
	createAccessor := func(accessor node.ContextAccessor) map[string]any {
		return map[string]any{
			"get": func(key string) any {
				if accessor == nil {
					return nil
				}
				val, _ := accessor.Get(key)
				return val
			},
			"set": func(key string, value any) {
				if accessor != nil {
					accessor.Set(key, value)
				}
			},
			"delete": func(key string) {
				if accessor != nil {
					accessor.Delete(key)
				}
			},
			"keys": func() []string {
				if accessor == nil {
					return nil
				}
				return accessor.Keys()
			},
		}
	}

	// Add flow, node, and global context objects
	vm.Set("flow", createAccessor(node.GetFlowContext(ctx)))
	vm.Set("node", createAccessor(node.GetNodeContext(ctx)))
	vm.Set("global", createAccessor(node.GetGlobalContext(ctx)))
}

func (n *ScriptNode) Stop(ctx context.Context) error {
	return nil
}
