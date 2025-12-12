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
		Inputs:      []node.PortInfo{{Name: "input", Description: "Message to template"}},
		Outputs:     []node.PortInfo{{Name: "output", Description: "Templated message"}},
		Config: []node.ConfigSpec{
			{Name: "template", Type: "string", Required: true, Description: "Go template string"},
			{Name: "output", Type: "string", Default: "payload", Description: "Output property path"},
		},
	})
}

// TemplateNode applies Go templates to messages.
type TemplateNode struct {
	id       string
	template *template.Template
	output   string // "payload" or property path
}

func (n *TemplateNode) Init(ctx context.Context, cfg *node.Config) error {
	n.id = cfg.ID
	n.output = cfg.GetString("output", "payload")

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

	return nil
}

func (n *TemplateNode) Process(ctx context.Context, msg *message.Message, emit node.Emitter) error {
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
	if n.output == "payload" {
		out.Payload = result
	} else {
		// Set nested property (simplified - only supports context.X)
		if out.Context == nil {
			out.Context = make(map[string]any)
		}
		out.Context[n.output] = result
	}

	emit.Emit("default", out)
	return nil
}

func (n *TemplateNode) Stop(ctx context.Context) error {
	return nil
}
