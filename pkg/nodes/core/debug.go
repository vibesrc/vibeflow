package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
	node.RegisterWithInfo("core.debug", func() node.Node { return &DebugNode{} }, node.TypeInfo{
		Type:        "core.debug",
		Name:        "Debug",
		Description: "Logs messages for debugging",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "default", Description: "Message to debug"}},
		Outputs:     []node.PortInfo{{Name: "default", Description: "Pass-through message"}},
		Config: []node.ConfigSpec{
			{Name: "complete", Type: "bool", Default: false, Description: "Log complete message (vs just payload)"},
			{Name: "to_stdout", Type: "bool", Default: true, Description: "Output to stdout/console"},
			{Name: "to_sidebar", Type: "bool", Default: true, Description: "Output to debug sidebar in UI"},
		},
	})
}

// DebugNode logs messages for debugging.
type DebugNode struct {
	id        string
	name      string
	complete  bool // Log complete message vs just payload
	toStdout  bool // Output to stdout
	toSidebar bool // Output to debug sidebar
	output    node.Output
}

func (n *DebugNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.name = cfg.Name
	if n.name == "" {
		n.name = cfg.ID
	}
	n.complete = cfg.GetBool("complete", false)
	n.toStdout = cfg.GetBool("to_stdout", true)
	n.toSidebar = cfg.GetBool("to_sidebar", true)

	// Output is optional - debug is often a terminal node
	if outputs.Has("default") {
		var err error
		n.output, err = outputs.Get("default")
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *DebugNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	// Determine what to output
	var payload any
	if n.complete {
		payload = msg
	} else {
		payload = msg.Payload
	}

	// Format payload as string for stdout
	var payloadStr string
	switch v := payload.(type) {
	case string:
		payloadStr = v
	case []byte:
		payloadStr = string(v)
	default:
		data, _ := json.Marshal(v)
		payloadStr = string(data)
	}

	// Output to stdout if enabled
	if n.toStdout {
		fmt.Printf("[%s] %s\n", n.name, payloadStr)
	}

	// Output to sidebar via DebugEmitter if enabled
	if n.toSidebar {
		if de := node.GetDebugEmitter(ctx); de != nil {
			// Use msg metadata topic if available
			topic := msg.GetMeta("topic")
			de.Debug(topic, payload)
		}
	}

	// Pass message through if output is wired
	if n.output != nil {
		n.output.Send(msg)
	}
	return nil
}

func (n *DebugNode) Stop(ctx context.Context) error {
	return nil
}
