package external

import (
	"context"
	"fmt"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

// ProxyNode is a node that proxies calls to an external process.
type ProxyNode struct {
	host     *Host
	nodeID   string
	nodeType string
	hasStart bool
	outputs  node.Outputs
}

// NewProxyNode creates a new proxy node.
func NewProxyNode(host *Host, nodeType string, hasStart bool) *ProxyNode {
	return &ProxyNode{
		host:     host,
		nodeType: nodeType,
		hasStart: hasStart,
	}
}

// Init initializes the proxy node.
func (n *ProxyNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.nodeID = cfg.ID
	n.outputs = outputs

	params := &InitParams{
		NodeID:   cfg.ID,
		NodeType: n.nodeType,
		Config:   cfg.Config,
	}

	result, err := n.host.Init(ctx, params)
	if err != nil {
		return fmt.Errorf("external node init failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("external node init error: %s", result.Error)
	}

	return nil
}

// Process processes a message through the external node.
func (n *ProxyNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	params := &ProcessParams{
		NodeID:  n.nodeID,
		Message: msg,
	}

	result, err := n.host.Process(ctx, params)
	if err != nil {
		return fmt.Errorf("external node process failed: %w", err)
	}

	if result.Error != "" {
		return fmt.Errorf("external node process error: %s", result.Error)
	}

	// Emit output messages
	for outputName, msgs := range result.Outputs {
		if n.outputs.Has(outputName) {
			if out, err := n.outputs.Get(outputName); err == nil {
				for _, m := range msgs {
					out.Send(m)
				}
			}
		}
	}

	return nil
}

// Start implements node.Starter for nodes that need it.
func (n *ProxyNode) Start(ctx context.Context) error {
	if !n.hasStart {
		return nil
	}

	params := &StartParams{
		NodeID: n.nodeID,
	}

	result, err := n.host.StartNode(ctx, params)
	if err != nil {
		return fmt.Errorf("external node start failed: %w", err)
	}

	if result.Error != "" {
		return fmt.Errorf("external node start error: %s", result.Error)
	}

	// Emit any initial messages
	for outputName, msgs := range result.Outputs {
		if n.outputs.Has(outputName) {
			if out, err := n.outputs.Get(outputName); err == nil {
				for _, m := range msgs {
					out.Send(m)
				}
			}
		}
	}

	return nil
}

// Stop stops the external node.
func (n *ProxyNode) Stop(ctx context.Context) error {
	params := &StopParams{
		NodeID: n.nodeID,
	}

	result, err := n.host.StopNode(ctx, params)
	if err != nil {
		return fmt.Errorf("external node stop failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("external node stop error: %s", result.Error)
	}

	return nil
}

// Ensure ProxyNode implements the node interfaces
var _ node.Node = (*ProxyNode)(nil)
var _ node.Starter = (*ProxyNode)(nil)
