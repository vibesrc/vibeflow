package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
	node.RegisterWithInfo("core.join", func() node.Node { return &JoinNode{} }, node.TypeInfo{
		Type:        "core.join",
		Name:        "Join",
		Description: "Combines multiple messages into one",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "default", Description: "Messages to join"}},
		Outputs:     []node.PortInfo{{Name: "default", Description: "Joined message"}},
		Config: []node.ConfigSpec{
			{Name: "mode", Type: "string", Default: "auto", Description: "Join mode", Options: []any{"auto", "count", "timeout"}},
			{Name: "count", Type: "int", Default: 0, Description: "Number of messages to join", ShowWhen: &node.ShowWhen{Field: "mode", Eq: "count"}},
			{Name: "timeout_ms", Type: "int", Default: 5000, Description: "Timeout for collecting messages"},
			{Name: "join_as", Type: "string", Default: "array", Description: "Output format", Options: []any{"array", "string", "object"}},
			{Name: "delimiter", Type: "string", Default: "\n", Description: "Delimiter for string joining", ShowWhen: &node.ShowWhen{Field: "join_as", Eq: "string"}},
			{Name: "key", Type: "string", Description: "Property to use as key", Format: "expression", ShowWhen: &node.ShowWhen{Field: "join_as", Eq: "object"}},
			{Name: "value", Type: "string", Description: "Property to use as value (empty = whole payload)", Format: "expression", ShowWhen: &node.ShowWhen{Field: "join_as", Eq: "object"}},
		},
		HasStart: true,
	})
}

// JoinNode combines multiple messages into one.
type JoinNode struct {
	id        string
	mode      string // "auto" (use split metadata), "count", "timeout"
	count     int    // Number of messages to join
	timeoutMS int    // Timeout for collecting messages
	joinAs    string // "array", "string", "object"
	delimiter string // For string joining
	key       string // Property to use as key for object mode
	value     string // Property to use as value for object mode (empty = whole payload)

	output  node.Output
	mu      sync.Mutex
	groups  map[string]*joinGroup
	stopCh  chan struct{}
	started bool
}

type joinGroup struct {
	messages []*message.Message
	expected int
	created  time.Time
}

func (n *JoinNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.mode = cfg.GetString("mode", "auto")
	n.count = cfg.GetInt("count", 0)
	n.timeoutMS = cfg.GetInt("timeout_ms", 5000)
	n.joinAs = cfg.GetString("join_as", "array")
	n.delimiter = cfg.GetString("delimiter", "\n")
	n.key = cfg.GetString("key", "")
	n.value = cfg.GetString("value", "")
	n.groups = make(map[string]*joinGroup)
	n.stopCh = make(chan struct{})

	if outputs.Has("default") {
		var err error
		n.output, err = outputs.Get("default")
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *JoinNode) Start(ctx context.Context) error {
	n.started = true
	// Start timeout checker
	go n.timeoutChecker(ctx)
	return nil
}

func (n *JoinNode) timeoutChecker(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(n.timeoutMS/2) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-n.stopCh:
			return
		case <-ticker.C:
			n.checkTimeouts(ctx)
		}
	}
}

func (n *JoinNode) checkTimeouts(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()

	timeout := time.Duration(n.timeoutMS) * time.Millisecond
	now := time.Now()

	for groupID, group := range n.groups {
		if now.Sub(group.created) > timeout && len(group.messages) > 0 {
			n.emitGroup(ctx, groupID, group)
			delete(n.groups, groupID)
		}
	}
}

func (n *JoinNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	groupID := n.getGroupID(msg)
	expected := n.getExpected(msg)

	// Get or create group
	group, ok := n.groups[groupID]
	if !ok {
		group = &joinGroup{
			messages: make([]*message.Message, 0),
			expected: expected,
			created:  time.Now(),
		}
		n.groups[groupID] = group
	}

	// Add message to group
	group.messages = append(group.messages, msg.Clone())

	// Check if complete
	complete := false
	switch n.mode {
	case "auto":
		complete = group.expected > 0 && len(group.messages) >= group.expected
	case "count":
		complete = n.count > 0 && len(group.messages) >= n.count
	case "timeout":
		// Let timeout checker handle it
		complete = false
	}

	if complete {
		n.emitGroup(ctx, groupID, group)
		delete(n.groups, groupID)
	}

	return nil
}

func (n *JoinNode) getGroupID(msg *message.Message) string {
	// Try to get split_id from metadata
	if splitID := msg.GetMeta("split_id"); splitID != "" {
		return splitID
	}
	// Use a default group
	return "_default"
}

func (n *JoinNode) getExpected(msg *message.Message) int {
	if total := msg.GetMeta("split_total"); total != "" {
		if count, err := strconv.Atoi(total); err == nil {
			return count
		}
	}
	return n.count
}

func (n *JoinNode) emitGroup(ctx context.Context, _ string, group *joinGroup) {
	if len(group.messages) == 0 || n.output == nil {
		return
	}

	// Sort by split_index if available
	sortedMsgs := n.sortByIndex(group.messages)

	// Create output message from first message
	out := sortedMsgs[0].Clone()

	switch n.joinAs {
	case "array":
		payloads := make([]any, len(sortedMsgs))
		for i, m := range sortedMsgs {
			payloads[i] = m.Payload
		}
		out.Payload = payloads

	case "string":
		var parts []string
		for _, m := range sortedMsgs {
			if s, ok := m.Payload.(string); ok {
				parts = append(parts, s)
			} else {
				parts = append(parts, fmt.Sprintf("%v", m.Payload))
			}
		}
		out.Payload = strings.Join(parts, n.delimiter)

	case "object":
		obj := make(map[string]any)
		for _, m := range sortedMsgs {
			key := m.GetMeta("split_key")
			if key == "" && n.key != "" {
				// Extract key using expr (with context for flow/node/global access)
				if k := node.EvalExprStringWithContext(ctx, n.key, m); k != "" {
					key = k
				}
			}
			if key == "" {
				key = fmt.Sprintf("item_%s", m.ID[:8])
			}
			// Get value - either specific property or whole payload
			var val any = m.Payload
			if n.value != "" {
				if v, err := node.EvalExprWithContext(ctx, n.value, m); err == nil && v != nil {
					val = v
				}
			}
			obj[key] = val
		}
		out.Payload = obj
	}

	// Clear split metadata
	delete(out.Metadata, "split_id")
	delete(out.Metadata, "split_index")
	delete(out.Metadata, "split_total")
	delete(out.Metadata, "split_key")

	out.SetMeta("joined_count", fmt.Sprintf("%d", len(sortedMsgs)))

	n.output.Send(out)
}

func (n *JoinNode) sortByIndex(msgs []*message.Message) []*message.Message {
	// Simple insertion sort by split_index
	sorted := make([]*message.Message, len(msgs))
	copy(sorted, msgs)

	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		keyIdx := n.getIndex(key)
		j := i - 1

		for j >= 0 && n.getIndex(sorted[j]) > keyIdx {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	return sorted
}

func (n *JoinNode) getIndex(msg *message.Message) int {
	if idx := msg.GetMeta("split_index"); idx != "" {
		if i, err := strconv.Atoi(idx); err == nil {
			return i
		}
	}
	return 0
}

func (n *JoinNode) Stop(ctx context.Context) error {
	close(n.stopCh)
	return nil
}

// Ensure JoinNode implements Starter
var _ node.Starter = (*JoinNode)(nil)
