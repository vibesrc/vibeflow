package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
	node.RegisterWithInfo("core.split", func() node.Node { return &SplitNode{} }, node.TypeInfo{
		Type:        "core.split",
		Name:        "Split",
		Description: "Splits a message into multiple messages",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "default", Description: "Message to split"}},
		Outputs:     []node.PortInfo{{Name: "default", Description: "Split messages"}},
		Config: []node.ConfigSpec{
			{Name: "split_on", Type: "string", Default: "array", Description: "Split mode", Options: []any{"array", "string", "object"}},
			{Name: "delimiter", Type: "string", Default: "\n", Description: "Delimiter for string splitting"},
			{Name: "add_parts", Type: "bool", Default: true, Description: "Add split metadata to messages"},
		},
	})
}

// SplitNode splits a message into multiple messages.
type SplitNode struct {
	id        string
	splitOn   string // "array", "string", "object"
	delimiter string // For string splitting
	addParts  bool   // Add parts info to metadata
	output    node.Output
}

func (n *SplitNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.splitOn = cfg.GetString("split_on", "array")
	n.delimiter = cfg.GetString("delimiter", "\n")
	n.addParts = cfg.GetBool("add_parts", true)

	if outputs.Has("default") {
		var err error
		n.output, err = outputs.Get("default")
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *SplitNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	if n.output == nil {
		return nil
	}

	switch n.splitOn {
	case "array":
		return n.splitArray(msg)
	case "string":
		return n.splitString(msg)
	case "object":
		return n.splitObject(msg)
	default:
		return fmt.Errorf("unknown split mode: %s", n.splitOn)
	}
}

func (n *SplitNode) splitArray(msg *message.Message) error {
	arr, ok := msg.Payload.([]any)
	if !ok {
		// Not an array, pass through
		n.output.Send(msg)
		return nil
	}

	total := len(arr)
	for i, item := range arr {
		out := msg.Clone()
		out.Payload = item
		if n.addParts {
			out.SetMeta("split_index", fmt.Sprintf("%d", i))
			out.SetMeta("split_total", fmt.Sprintf("%d", total))
			out.SetMeta("split_id", msg.ID)
		}
		n.output.Send(out)
	}

	return nil
}

func (n *SplitNode) splitString(msg *message.Message) error {
	str, ok := msg.Payload.(string)
	if !ok {
		n.output.Send(msg)
		return nil
	}

	parts := strings.Split(str, n.delimiter)
	total := len(parts)

	for i, part := range parts {
		out := msg.Clone()
		out.Payload = part
		if n.addParts {
			out.SetMeta("split_index", fmt.Sprintf("%d", i))
			out.SetMeta("split_total", fmt.Sprintf("%d", total))
			out.SetMeta("split_id", msg.ID)
		}
		n.output.Send(out)
	}

	return nil
}

func (n *SplitNode) splitObject(msg *message.Message) error {
	obj, ok := msg.Payload.(map[string]any)
	if !ok {
		n.output.Send(msg)
		return nil
	}

	total := len(obj)
	index := 0

	for key, value := range obj {
		out := msg.Clone()
		out.Payload = map[string]any{
			"key":   key,
			"value": value,
		}
		if n.addParts {
			out.SetMeta("split_index", fmt.Sprintf("%d", index))
			out.SetMeta("split_total", fmt.Sprintf("%d", total))
			out.SetMeta("split_id", msg.ID)
			out.SetMeta("split_key", key)
		}
		n.output.Send(out)
		index++
	}

	return nil
}

func (n *SplitNode) Stop(ctx context.Context) error {
	return nil
}
