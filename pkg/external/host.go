package external

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Host manages an external node process.
type Host struct {
	path   string
	args   []string
	logger *slog.Logger

	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	running bool
	reqID   atomic.Int64

	// Pending requests waiting for responses
	pendingMu sync.Mutex
	pending   map[int]chan *Response
}

// NewHost creates a new external node host.
func NewHost(path string, args ...string) *Host {
	return &Host{
		path:    path,
		args:    args,
		logger:  slog.Default(),
		pending: make(map[int]chan *Response),
	}
}

// Start starts the external process.
func (h *Host) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return fmt.Errorf("host already running")
	}

	h.cmd = exec.CommandContext(ctx, h.path, h.args...)
	h.cmd.Stderr = os.Stderr // Forward stderr for debugging

	stdin, err := h.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	h.stdin = stdin

	stdout, err := h.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	h.stdout = bufio.NewReader(stdout)

	if err := h.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	h.running = true

	// Start response reader
	go h.readResponses(ctx)

	return nil
}

// readResponses reads JSON-RPC responses from stdout.
func (h *Host) readResponses(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := h.stdout.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				h.logger.Error("error reading response", "error", err)
			}
			return
		}

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			h.logger.Error("invalid response JSON", "error", err, "line", string(line))
			continue
		}

		// Find and notify the pending request
		h.pendingMu.Lock()
		ch, ok := h.pending[resp.ID]
		if ok {
			delete(h.pending, resp.ID)
		}
		h.pendingMu.Unlock()

		if ok {
			ch <- &resp
		}
	}
}

// Call sends a request and waits for a response.
func (h *Host) Call(ctx context.Context, method string, params any) (*Response, error) {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return nil, fmt.Errorf("host not running")
	}
	h.mu.Unlock()

	id := int(h.reqID.Add(1))
	req := NewRequest(id, method, params)

	// Create response channel
	respCh := make(chan *Response, 1)
	h.pendingMu.Lock()
	h.pending[id] = respCh
	h.pendingMu.Unlock()

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		h.pendingMu.Lock()
		delete(h.pending, id)
		h.pendingMu.Unlock()
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	h.mu.Lock()
	_, err = h.stdin.Write(append(data, '\n'))
	h.mu.Unlock()

	if err != nil {
		h.pendingMu.Lock()
		delete(h.pending, id)
		h.pendingMu.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		h.pendingMu.Lock()
		delete(h.pending, id)
		h.pendingMu.Unlock()
		return nil, ctx.Err()
	case resp := <-respCh:
		return resp, nil
	}
}

// GetManifest retrieves the manifest from the external process.
func (h *Host) GetManifest(ctx context.Context) (*Manifest, error) {
	resp, err := h.Call(ctx, MethodGetManifest, nil)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	// Convert result to Manifest
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return &manifest, nil
}

// Init initializes a node instance.
func (h *Host) Init(ctx context.Context, params *InitParams) (*InitResult, error) {
	resp, err := h.Call(ctx, MethodInit, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result InitResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Process sends a message to a node for processing.
func (h *Host) Process(ctx context.Context, params *ProcessParams) (*ProcessResult, error) {
	resp, err := h.Call(ctx, MethodProcess, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result ProcessResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// StartNode calls the start method on a node.
func (h *Host) StartNode(ctx context.Context, params *StartParams) (*StartResult, error) {
	resp, err := h.Call(ctx, MethodStart, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result StartResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// StopNode calls the stop method on a node.
func (h *Host) StopNode(ctx context.Context, params *StopParams) (*StopResult, error) {
	resp, err := h.Call(ctx, MethodStop, params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result StopResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Shutdown shuts down the external process gracefully.
func (h *Host) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return nil
	}

	// Send shutdown request
	req := NewRequest(int(h.reqID.Add(1)), MethodShutdown, nil)
	data, _ := json.Marshal(req)
	h.stdin.Write(append(data, '\n'))

	// Close stdin to signal EOF
	h.stdin.Close()

	// Wait for process to exit
	if h.cmd != nil && h.cmd.Process != nil {
		h.cmd.Wait()
	}

	h.running = false
	return nil
}

// Kill forcefully terminates the process.
func (h *Host) Kill() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return nil
	}

	if h.cmd != nil && h.cmd.Process != nil {
		h.cmd.Process.Kill()
	}

	h.running = false
	return nil
}
