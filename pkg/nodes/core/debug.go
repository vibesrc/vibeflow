package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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
		},
	})
}

// DebugNode logs messages for debugging.
type DebugNode struct {
	id       string
	name     string
	complete bool // Log complete message vs just payload
	logger   *slog.Logger
	output   node.Output
}

func (n *DebugNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.name = cfg.Name
	if n.name == "" {
		n.name = cfg.ID
	}
	n.complete = cfg.GetBool("complete", false)
	n.logger = slog.Default()

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
	if n.complete {
		// Log complete message
		data, _ := json.MarshalIndent(msg, "", "  ")
		n.logger.Info("debug",
			"node", n.name,
			"message", string(data),
		)
	} else {
		// Log just payload
		var payloadStr string
		switch v := msg.Payload.(type) {
		case string:
			payloadStr = v
		case []byte:
			payloadStr = string(v)
		default:
			data, _ := json.Marshal(msg.Payload)
			payloadStr = string(data)
		}
		fmt.Printf("[%s] %s\n", n.name, payloadStr)
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
