package core

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/bherbruck/vibeflow/pkg/message"
	"github.com/bherbruck/vibeflow/pkg/node"
)

func init() {
	node.RegisterWithInfo("core.delay", func() node.Node { return &DelayNode{} }, node.TypeInfo{
		Type:        "core.delay",
		Name:        "Delay",
		Description: "Delays messages by a specified duration",
		Category:    "core",
		Inputs:      []node.PortInfo{{Name: "default", Description: "Message to delay"}},
		Outputs:     []node.PortInfo{{Name: "default", Description: "Delayed message"}},
		Config: []node.ConfigSpec{
			{Name: "delay_ms", Type: "int", Default: 1000, Description: "Delay in milliseconds"},
			{Name: "randomize", Type: "bool", Default: false, Description: "Add random variation"},
			{Name: "max_delay_ms", Type: "int", Description: "Max delay when randomizing"},
			{Name: "rate_limit", Type: "int", Default: 0, Description: "Messages per second (0 = disabled)"},
			{Name: "drop", Type: "bool", Default: false, Description: "Drop messages if rate limited"},
		},
	})
}

// DelayNode delays messages by a specified duration.
type DelayNode struct {
	id         string
	delayMS    int
	randomize  bool  // Add random variation
	maxDelayMS int   // Max delay when randomizing
	rateLimit  int   // Messages per second (0 = disabled)
	drop       bool  // Drop messages if rate limited

	output   node.Output
	mu       sync.Mutex
	wg       sync.WaitGroup
	stopCh   chan struct{}
	lastSent time.Time
}

func (n *DelayNode) Init(ctx context.Context, cfg *node.Config, inputs node.Inputs, outputs node.Outputs) error {
	n.id = cfg.ID
	n.delayMS = cfg.GetInt("delay_ms", 1000)
	n.randomize = cfg.GetBool("randomize", false)
	n.maxDelayMS = cfg.GetInt("max_delay_ms", n.delayMS*2)
	n.rateLimit = cfg.GetInt("rate_limit", 0)
	n.drop = cfg.GetBool("drop", false)
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

func (n *DelayNode) Process(ctx context.Context, msg *message.Message, inputPort string) error {
	if n.output == nil {
		return nil
	}

	// Check rate limit
	if n.rateLimit > 0 {
		n.mu.Lock()
		minInterval := time.Second / time.Duration(n.rateLimit)
		elapsed := time.Since(n.lastSent)
		if elapsed < minInterval {
			if n.drop {
				n.mu.Unlock()
				return nil // Drop the message
			}
			// Wait for rate limit
			time.Sleep(minInterval - elapsed)
		}
		n.lastSent = time.Now()
		n.mu.Unlock()
	}

	// Calculate delay
	delay := time.Duration(n.delayMS) * time.Millisecond
	if n.randomize {
		minDelay := n.delayMS
		maxDelay := n.maxDelayMS
		if maxDelay <= minDelay {
			maxDelay = minDelay + 1
		}
		randomDelay := minDelay + rand.Intn(maxDelay-minDelay)
		delay = time.Duration(randomDelay) * time.Millisecond
	}

	// Schedule delayed emit
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()

		select {
		case <-ctx.Done():
			return
		case <-n.stopCh:
			return
		case <-time.After(delay):
			n.output.Send(msg.Clone())
		}
	}()

	return nil
}

func (n *DelayNode) Stop(ctx context.Context) error {
	close(n.stopCh)
	n.wg.Wait()
	return nil
}
