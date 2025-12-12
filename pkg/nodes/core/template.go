package core

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
	node.RegisterWithInfo("core.template", func() node.Node { return &TemplateNode{} }, node.TypeInfo{
		Type:        "core.template",
		Name:        "Template",
		Description: "Applies Go templates to messages",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "default", Description: "Message to template"}},
		Outputs:     []node.PortInfo{{Name: "default", Description: "Templated message"}},
		Config: []node.ConfigSpec{
			{Name: "template", Type: "string", Required: true, Description: "Go template string"},
			{Name: "output", Type: "string", Default: "payload", Description: "Output property path"},
		},
	})
}

// TemplateNode applies Go templates to messages.
type TemplateNode struct {
	id         string
	template   *template.Template
	outputProp string      // "payload" or property path
	output     node.Output // output port
}

func (n *TemplateNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.outputProp = cfg.GetString("output", "payload")

	tmplStr := cfg.GetString("template", "")
	if tmplStr == "" {
		return fmt.Errorf("template node %s: missing 'template' config", n.id)
	}

	// Parse template
	tmpl, err := template.New(n.id).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("template node %s: parse error: %w", n.id, err)
	}
	n.template = tmpl

	if outputs.Has("default") {
		n.output, err = outputs.Get("default")
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *TemplateNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	if n.output == nil {
		return nil
	}

	// Build template data
	data := map[string]any{
		"id":       msg.ID,
		"payload":  msg.Payload,
		"metadata": msg.Metadata,
		"context":  msg.Context,
	}

	// Execute template
	var buf bytes.Buffer
	if err := n.template.Execute(&buf, data); err != nil {
		return fmt.Errorf("template node %s: execute error: %w", n.id, err)
	}

	result := buf.String()

	// Set output
	out := msg.Clone()
	if n.outputProp == "payload" {
		out.Payload = result
	} else {
		// Set nested property (simplified - only supports context.X)
		if out.Context == nil {
			out.Context = make(map[string]any)
		}
		out.Context[n.outputProp] = result
	}

	n.output.Send(out)
	return nil
}

func (n *TemplateNode) Stop(ctx context.Context) error {
	return nil
}
