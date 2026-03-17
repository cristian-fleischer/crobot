// Package agent implements the ACP (Agent Client Protocol) JSON-RPC 2.0
// client for communicating with agent subprocesses over stdio.
package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// JSON-RPC 2.0 message types.

// Request is a JSON-RPC 2.0 request message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response message.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Notification is a JSON-RPC 2.0 notification (no ID, no response expected).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface for RPCError.
func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// rawMessage is used to determine the type of an incoming JSON-RPC message.
type rawMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// ClientConfig holds the configuration for spawning an agent subprocess.
type ClientConfig struct {
	// Command is the executable to run.
	Command string
	// Args are the command-line arguments.
	Args []string
	// Dir is the working directory for the subprocess.
	Dir string
	// Env contains additional environment variables for the subprocess.
	Env []string
	// Timeout is the overall timeout for the subprocess lifecycle.
	Timeout time.Duration
	// Stderr, if non-nil, receives the subprocess's stderr output.
	// This is useful for showing the agent's progress or debug output.
	Stderr io.Writer
}

// closeTimeout is the maximum time Close() waits for the subprocess to exit
// before killing it. This is deliberately short and independent of the
// lifecycle timeout (cfg.Timeout).
const closeTimeout = 3 * time.Second

// Client manages a JSON-RPC 2.0 connection to an agent subprocess over stdio.
type Client struct {
	cfg    ClientConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	nextID   atomic.Int64
	pending  map[int]chan *Response
	mu       sync.Mutex
	handler       func(ctx context.Context, method string, params json.RawMessage) (any, error)
	notifyHandler func(method string, params json.RawMessage)
	startCtx      context.Context
	done     chan struct{}
	readErr  error
	writeMu  sync.Mutex
	started  bool
	closed   bool
}

// NewClient creates a new Client with the given configuration. Call Start to
// spawn the subprocess.
func NewClient(cfg ClientConfig) *Client {
	return &Client{
		cfg:     cfg,
		pending: make(map[int]chan *Response),
		done:    make(chan struct{}),
	}
}

// Start spawns the agent subprocess and begins reading from its stdout.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("agent: client already started")
	}
	c.started = true
	c.mu.Unlock()

	// Use exec.Command (not CommandContext) so we manage the process
	// lifecycle ourselves in Close(). CommandContext can interact poorly
	// with our own timeout/kill logic and with cmd.Wait() draining I/O.
	cmd := exec.Command(c.cfg.Command, c.cfg.Args...)
	if c.cfg.Dir != "" {
		cmd.Dir = c.cfg.Dir
	}
	if len(c.cfg.Env) > 0 {
		cmd.Env = append(cmd.Environ(), c.cfg.Env...)
	}
	if c.cfg.Stderr != nil {
		cmd.Stderr = c.cfg.Stderr
	} else {
		// Explicitly discard stderr. If left nil, Go inherits the parent's
		// stderr fd, and cmd.Wait() blocks until ALL holders of that fd
		// (including child processes) close it — causing hangs on shutdown.
		cmd.Stderr = io.Discard
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("agent: creating stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("agent: creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("agent: starting subprocess: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.startCtx = ctx

	go c.readLoop()

	return nil
}

// SetRequestHandler registers a handler for incoming JSON-RPC requests from the
// subprocess. The handler is called for each incoming request and its return
// value is sent back as the response.
func (c *Client) SetRequestHandler(handler func(ctx context.Context, method string, params json.RawMessage) (any, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// SetNotificationHandler registers a handler for incoming JSON-RPC
// notifications from the subprocess (e.g. session/update streaming events).
func (c *Client) SetNotificationHandler(handler func(method string, params json.RawMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifyHandler = handler
}

// SendRequest sends a JSON-RPC 2.0 request and waits for the response.
func (c *Client) SendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := int(c.nextID.Add(1))

	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("agent: marshaling params: %w", err)
		}
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}

	ch := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	slog.Debug("agent: sending request", "id", id, "method", method)
	if err := c.writeMessage(req); err != nil {
		return nil, fmt.Errorf("agent: sending request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("agent: request %q: %w", method, ctx.Err())
	case <-c.done:
		return nil, fmt.Errorf("agent: connection closed while waiting for response to %q", method)
	case resp := <-ch:
		if resp == nil {
			return nil, fmt.Errorf("agent: connection closed while waiting for response to %q", method)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("agent: request %q: %w", method, resp.Error)
		}
		return resp.Result, nil
	}
}

// SendNotification sends a JSON-RPC 2.0 notification (no response expected).
func (c *Client) SendNotification(ctx context.Context, method string, params any) error {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("agent: marshaling params: %w", err)
		}
	}

	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
	}

	if err := c.writeMessage(notif); err != nil {
		return fmt.Errorf("agent: sending notification: %w", err)
	}
	return nil
}

// Close gracefully shuts down the subprocess. It closes stdin, waits briefly
// for the process to exit, and kills it if necessary.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Close stdin to signal the subprocess to exit.
	if c.stdin != nil {
		c.stdin.Close()
	}

	// If Start() was never called (or failed), there is no read loop or
	// subprocess to wait for.
	if !c.started || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	// Wait for the process to exit with a short, fixed timeout.
	// This covers both the read loop finishing and the process exiting.
	waitDone := make(chan error, 1)
	go func() {
		<-c.done // wait for read loop
		waitDone <- c.cmd.Wait()
	}()

	select {
	case err := <-waitDone:
		if err != nil {
			return fmt.Errorf("agent: subprocess exited: %w", err)
		}
		return nil
	case <-time.After(closeTimeout):
		_ = c.cmd.Process.Kill()
		<-waitDone
		return fmt.Errorf("agent: subprocess killed after timeout")
	}
}

// writeMessage serializes msg as JSON and writes it as a newline-delimited
// message to the subprocess stdin.
func (c *Client) writeMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if _, err := c.stdin.Write(data); err != nil {
		return err
	}
	return nil
}

// readLoop reads newline-delimited JSON messages from the subprocess stdout
// and routes them to the appropriate handler.
func (c *Client) readLoop() {
	defer close(c.done)

	scanner := bufio.NewScanner(c.stdout)
	// Allow up to 1MB messages.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg rawMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			slog.Debug("agent: readLoop: skipping non-JSON line", "error", err)
			continue
		}

		c.routeMessage(msg)
	}

	c.readErr = scanner.Err()

	// Wake up any pending requests.
	c.mu.Lock()
	for _, ch := range c.pending {
		close(ch)
	}
	c.mu.Unlock()
}

// routeMessage dispatches an incoming message based on its structure.
func (c *Client) routeMessage(msg rawMessage) {
	// Response: has an ID but no Method.
	if msg.ID != nil && msg.Method == "" {
		slog.Debug("agent: received response", "id", *msg.ID, "has_error", msg.Error != nil)
		c.mu.Lock()
		ch, ok := c.pending[*msg.ID]
		c.mu.Unlock()
		if ok {
			ch <- &Response{
				JSONRPC: msg.JSONRPC,
				ID:      *msg.ID,
				Result:  msg.Result,
				Error:   msg.Error,
			}
		}
		return
	}

	// Request from subprocess: has an ID and a Method.
	if msg.ID != nil && msg.Method != "" {
		slog.Debug("agent: received request from agent", "id", *msg.ID, "method", msg.Method)
		c.mu.Lock()
		handler := c.handler
		c.mu.Unlock()

		go func() {
			id := *msg.ID
			if handler == nil {
				resp := Response{
					JSONRPC: "2.0",
					ID:      id,
					Error: &RPCError{
						Code:    -32601,
						Message: "method not found",
					},
				}
				if err := c.writeMessage(resp); err != nil {
					slog.Debug("agent: failed to send response", "id", id, "error", err)
				}
				return
			}

			handlerCtx := context.Background()
			if c.startCtx != nil {
				handlerCtx = c.startCtx
			}
			result, err := handler(handlerCtx, msg.Method, msg.Params)
			if err != nil {
				resp := Response{
					JSONRPC: "2.0",
					ID:      id,
					Error: &RPCError{
						Code:    -32000,
						Message: err.Error(),
					},
				}
				if err := c.writeMessage(resp); err != nil {
					slog.Debug("agent: failed to send response", "id", id, "error", err)
				}
				return
			}

			resultJSON, merr := json.Marshal(result)
			if merr != nil {
				resp := Response{
					JSONRPC: "2.0",
					ID:      id,
					Error: &RPCError{
						Code:    -32603,
						Message: "internal error: " + merr.Error(),
					},
				}
				if err := c.writeMessage(resp); err != nil {
					slog.Debug("agent: failed to send response", "id", id, "error", err)
				}
				return
			}

			resp := Response{
				JSONRPC: "2.0",
				ID:      id,
				Result:  resultJSON,
			}
			if err := c.writeMessage(resp); err != nil {
				slog.Debug("agent: failed to send response", "id", id, "error", err)
			}
		}()
		return
	}

	// Notification: has a Method but no ID.
	if msg.Method != "" {
		slog.Debug("agent: received notification", "method", msg.Method, "params_len", len(msg.Params))
	}
	c.mu.Lock()
	nh := c.notifyHandler
	c.mu.Unlock()
	if nh != nil {
		nh(msg.Method, msg.Params)
	}
}
