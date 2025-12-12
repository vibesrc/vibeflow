// Example external node plugin that echoes messages.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bherbruck/vibeflow/pkg/external"
	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/sdk"
)

func main() {
	plugin := sdk.NewPlugin("echo-plugin", "1.0.0", "Example echo plugin")

	// Register the echo node
	plugin.RegisterNode(external.NodeTypeInfo{
		Type:        "example.echo",
		Name:        "Echo",
		Description: "Echoes messages with optional prefix/suffix",
		Category:    "example",
		Inputs:      []external.PortInfo{{Name: "default", Description: "Input messages"}},
		Outputs:     []external.PortInfo{{Name: "default", Description: "Echoed messages"}},
		Config: []external.ConfigSpec{
			{Name: "prefix", Type: "string", Default: "", Description: "Prefix to add to payload"},
			{Name: "suffix", Type: "string", Default: "", Description: "Suffix to add to payload"},
			{Name: "uppercase", Type: "bool", Default: false, Description: "Convert to uppercase"},
		},
	}, func() sdk.NodeHandler { return &EchoNode{} })

	// Register a counter node (demonstrates stateful nodes)
	plugin.RegisterNode(external.NodeTypeInfo{
		Type:        "example.counter",
		Name:        "Counter",
		Description: "Counts messages processed",
		Category:    "example",
		Inputs:      []external.PortInfo{{Name: "default", Description: "Input messages"}},
		Outputs:     []external.PortInfo{{Name: "default", Description: "Messages with count"}},
		Config: []external.ConfigSpec{
			{Name: "start", Type: "int", Default: 0, Description: "Starting count"},
		},
	}, func() sdk.NodeHandler { return &CounterNode{} })

	if err := plugin.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "plugin error: %v\n", err)
		os.Exit(1)
	}
}

// EchoNode echoes messages with optional transformations.
type EchoNode struct {
	prefix    string
	suffix    string
	uppercase bool
}

func (n *EchoNode) Init(ctx context.Context, nodeID string, config map[string]any) error {
	if prefix, ok := config["prefix"].(string); ok {
		n.prefix = prefix
	}
	if suffix, ok := config["suffix"].(string); ok {
		n.suffix = suffix
	}
	if uppercase, ok := config["uppercase"].(bool); ok {
		n.uppercase = uppercase
	}
	return nil
}

func (n *EchoNode) Process(ctx context.Context, msg *message.Message) (map[string][]*message.Message, error) {
	out := msg.Clone()

	// Transform string payloads
	if str, ok := msg.Payload.(string); ok {
		result := n.prefix + str + n.suffix
		if n.uppercase {
			result = strings.ToUpper(result)
		}
		out.Payload = result
	}

	return map[string][]*message.Message{
		"default": {out},
	}, nil
}

func (n *EchoNode) Stop(ctx context.Context) error {
	return nil
}

// CounterNode counts messages.
type CounterNode struct {
	count int
}

func (n *CounterNode) Init(ctx context.Context, nodeID string, config map[string]any) error {
	if start, ok := config["start"].(float64); ok {
		n.count = int(start)
	}
	return nil
}

func (n *CounterNode) Process(ctx context.Context, msg *message.Message) (map[string][]*message.Message, error) {
	n.count++

	out := msg.Clone()

	// Add count to payload
	if payload, ok := out.Payload.(map[string]any); ok {
		payload["count"] = n.count
	} else {
		out.Payload = map[string]any{
			"original": out.Payload,
			"count":    n.count,
		}
	}

	return map[string][]*message.Message{
		"default": {out},
	}, nil
}

func (n *CounterNode) Stop(ctx context.Context) error {
	return nil
}
