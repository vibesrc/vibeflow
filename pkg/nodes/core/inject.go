// Package core provides the core built-in nodes.
package core

import (
	"context"
	"time"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
	node.RegisterWithInfo("core.inject", func() node.Node { return &InjectNode{} }, node.TypeInfo{
		Type:        "core.inject",
		Name:        "Inject",
		Description: "Injects messages on a timer or at startup",
		Category:    "core",
		Inputs:      []node.PortInfo{}, // No inputs - inject is a source node
		Outputs:     []node.PortInfo{{Name: "default", Description: "Injected message"}},
		Config: []node.ConfigSpec{
			{Name: "payload_type", Type: "string", Default: "json", Options: []any{"json", "expr"}, Description: "Payload type: static JSON or expression"},
			{Name: "payload", Type: "object", Format: "code", Language: "json", Description: "Static JSON payload", ShowWhen: &node.ShowWhen{Field: "payload_type", Eq: "json"}},
			{Name: "payload_expr", Type: "string", Format: "expression", Description: "Expression for dynamic payload (e.g., now(), flow.get(\"counter\"))", ShowWhen: &node.ShowWhen{Field: "payload_type", Eq: "expr"}},
			{Name: "on_start", Type: "bool", Default: true, Description: "Inject on flow start"},
			{Name: "repeat", Type: "bool", Default: true, Description: "Repeat at interval"},
			{Name: "interval_ms", Type: "int", Default: 1000, Description: "Interval in milliseconds", ShowWhen: &node.ShowWhen{Field: "repeat", Eq: true}},
		},
		HasStart: true,
	})
}

// InjectNode injects messages on a timer or at startup.
type InjectNode struct {
	id          string
	payloadType string // "json" or "expr"
	payload     any    // static payload (for json type)
	payloadExpr string // expression (for expr type)
	intervalMS  int
	onStart     bool
	repeat      bool

	output node.Output
	ticker *time.Ticker
	stopCh chan struct{}
}

func (n *InjectNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.payloadType = cfg.GetString("payload_type", "json")
	n.payload = cfg.Get("payload")
	if n.payload == nil {
		n.payload = map[string]any{}
	}
	n.payloadExpr = cfg.GetString("payload_expr", "")
	n.intervalMS = cfg.GetInt("interval_ms", 0)
	n.onStart = cfg.GetBool("on_start", true)
	n.repeat = cfg.GetBool("repeat", true)
	n.stopCh = make(chan struct{})

	// Get output handle - inject is a source, output may not be wired
	if outputs.Has("default") {
		var err error
		n.output, err = outputs.Get("default")
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *InjectNode) Start(ctx context.Context) error {
	if n.output == nil {
		return nil // Not wired, nothing to do
	}

	// Inject on start if configured
	if n.onStart {
		n.sendMessage(ctx, nil)
	}

	// Start timer if interval is set
	if n.intervalMS > 0 && n.repeat {
		n.ticker = time.NewTicker(time.Duration(n.intervalMS) * time.Millisecond)
		go n.tickLoop(ctx)
	}

	return nil
}

func (n *InjectNode) tickLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-n.stopCh:
			return
		case <-n.ticker.C:
			if n.output != nil {
				n.sendMessage(ctx, nil)
			}
		}
	}
}

func (n *InjectNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	if n.output == nil {
		return nil
	}
	// Inject nodes can also be triggered by incoming messages
	n.sendMessage(ctx, msg)
	return nil
}

// sendMessage builds and sends an inject message.
// triggeredBy is optional - set when triggered by an incoming message.
func (n *InjectNode) sendMessage(ctx context.Context, triggeredBy *message.Message) {
	payload := n.buildPayload(ctx)
	msg := message.New(payload)
	msg.SetMeta("source", n.id)
	msg.SetTimestamp()
	if triggeredBy != nil {
		msg.SetMeta("triggered_by", triggeredBy.ID)
	}
	n.output.Send(msg)
}

// buildPayload returns the payload based on payload_type.
func (n *InjectNode) buildPayload(ctx context.Context) any {
	if n.payloadType == "expr" && n.payloadExpr != "" {
		// Evaluate expression with context (for flow.get, node.get, global.get, now(), etc.)
		// Use an empty message as base since inject doesn't have an incoming message
		emptyMsg := message.New(nil)
		result, err := node.EvalExprWithContext(ctx, n.payloadExpr, emptyMsg)
		if err != nil {
			// Return the error as the payload so user can debug
			return map[string]any{"error": err.Error(), "expression": n.payloadExpr}
		}
		return result
	}
	// Default: static JSON payload
	return n.payload
}

func (n *InjectNode) Stop(ctx context.Context) error {
	close(n.stopCh)
	if n.ticker != nil {
		n.ticker.Stop()
	}
	return nil
}
