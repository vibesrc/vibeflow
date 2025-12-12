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
		Inputs:      []node.PortInfo{{Name: "input", Description: "Trigger injection"}},
		Outputs:     []node.PortInfo{{Name: "output", Description: "Injected message"}},
		Config: []node.ConfigSpec{
			{Name: "payload", Type: "object", Description: "Payload to inject"},
			{Name: "interval_ms", Type: "int", Default: 0, Description: "Interval in milliseconds (0 = no repeat)"},
			{Name: "on_start", Type: "bool", Default: true, Description: "Inject on flow start"},
			{Name: "repeat", Type: "bool", Default: true, Description: "Repeat at interval"},
		},
		HasStart: true,
	})
}

// InjectNode injects messages on a timer or at startup.
type InjectNode struct {
	id         string
	payload    any
	intervalMS int
	onStart    bool
	repeat     bool

	ticker *time.Ticker
	stopCh chan struct{}
}

func (n *InjectNode) Init(ctx context.Context, cfg *node.Config) error {
	n.id = cfg.ID
	n.payload = cfg.Get("payload")
	if n.payload == nil {
		n.payload = map[string]any{}
	}
	n.intervalMS = cfg.GetInt("interval_ms", 0)
	n.onStart = cfg.GetBool("on_start", true)
	n.repeat = cfg.GetBool("repeat", true)
	n.stopCh = make(chan struct{})
	return nil
}

func (n *InjectNode) Start(ctx context.Context, emit node.Emitter) error {
	// Inject on start if configured
	if n.onStart {
		msg := message.New(n.payload)
		msg.SetMeta("source", n.id)
		msg.SetTimestamp()
		emit.Emit("default", msg)
	}

	// Start timer if interval is set
	if n.intervalMS > 0 && n.repeat {
		n.ticker = time.NewTicker(time.Duration(n.intervalMS) * time.Millisecond)
		go n.tickLoop(ctx, emit)
	}

	return nil
}

func (n *InjectNode) tickLoop(ctx context.Context, emit node.Emitter) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-n.stopCh:
			return
		case <-n.ticker.C:
			msg := message.New(n.payload)
			msg.SetMeta("source", n.id)
			msg.SetTimestamp()
			emit.Emit("default", msg)
		}
	}
}

func (n *InjectNode) Process(ctx context.Context, msg *message.Message, emit node.Emitter) error {
	// Inject nodes can also be triggered by incoming messages
	newMsg := message.New(n.payload)
	newMsg.SetMeta("source", n.id)
	newMsg.SetMeta("triggered_by", msg.ID)
	newMsg.SetTimestamp()
	emit.Emit("default", newMsg)
	return nil
}

func (n *InjectNode) Stop(ctx context.Context) error {
	close(n.stopCh)
	if n.ticker != nil {
		n.ticker.Stop()
	}
	return nil
}
