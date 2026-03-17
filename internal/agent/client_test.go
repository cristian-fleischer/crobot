package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestHelperEcho is a test helper subprocess that reads JSON-RPC requests from
// stdin and echoes the method back as the result.
func TestHelperEcho(t *testing.T) {
	if os.Getenv("CROBOT_TEST_HELPER") != "echo" {
		return
	}

	// This runs as a subprocess — read requests, write responses.
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for dec.More() {
		var msg rawMessage
		if err := dec.Decode(&msg); err != nil {
			os.Exit(1)
		}

		if msg.ID != nil && msg.Method != "" {
			// It's a request — echo the method back.
			result, _ := json.Marshal(map[string]string{"echo": msg.Method})
			resp := Response{
				JSONRPC: "2.0",
				ID:      *msg.ID,
				Result:  result,
			}
			if err := enc.Encode(resp); err != nil {
				os.Exit(1)
			}
		}

		if msg.ID == nil && msg.Method != "" {
			// It's a notification — ignore.
		}
	}
}

// TestHelperError is a test helper that always returns an RPC error.
func TestHelperError(t *testing.T) {
	if os.Getenv("CROBOT_TEST_HELPER") != "error" {
		return
	}

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for dec.More() {
		var msg rawMessage
		if err := dec.Decode(&msg); err != nil {
			os.Exit(1)
		}

		if msg.ID != nil {
			resp := Response{
				JSONRPC: "2.0",
				ID:      *msg.ID,
				Error: &RPCError{
					Code:    -32000,
					Message: "test error",
				},
			}
			if err := enc.Encode(resp); err != nil {
				os.Exit(1)
			}
		}
	}
}

// TestHelperServerRequest is a test helper that sends a request TO the client
// and prints the result to stderr for verification.
func TestHelperServerRequest(t *testing.T) {
	if os.Getenv("CROBOT_TEST_HELPER") != "server_request" {
		return
	}

	enc := json.NewEncoder(os.Stdout)
	dec := json.NewDecoder(os.Stdin)

	// Send a request to the client.
	id := 1
	params, _ := json.Marshal(map[string]string{"file": "test.go"})
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "fs/read",
		Params:  params,
	}
	if err := enc.Encode(req); err != nil {
		os.Exit(1)
	}

	// Read the response from the client.
	var resp rawMessage
	if err := dec.Decode(&resp); err != nil {
		os.Exit(1)
	}

	// Echo the result back as a notification so the test can verify.
	if resp.Result != nil {
		result, _ := json.Marshal(map[string]string{"got_result": string(resp.Result)})
		notif := Notification{
			JSONRPC: "2.0",
			Method:  "result_received",
			Params:  result,
		}
		_ = enc.Encode(notif)
	}

	// Keep reading until stdin closes.
	for dec.More() {
		var discard rawMessage
		_ = dec.Decode(&discard)
	}
}

// helperCommand returns the path to the test binary and args to invoke the
// named test helper.
func helperCommand(helperName string) (string, []string) {
	// Re-exec the test binary with the helper env var set.
	return os.Args[0], []string{"-test.run=^TestHelper", "-test.v"}
}

func TestSendRequest(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("echo")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=echo"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	result, err := client.SendRequest(ctx, "test/ping", nil)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	var out map[string]string
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if out["echo"] != "test/ping" {
		t.Errorf("expected echo=test/ping, got %q", out["echo"])
	}
}

func TestSendRequestWithParams(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("echo")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=echo"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	params := map[string]string{"key": "value"}
	result, err := client.SendRequest(ctx, "test/echo", params)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	var out map[string]string
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if out["echo"] != "test/echo" {
		t.Errorf("expected echo=test/echo, got %q", out["echo"])
	}
}

func TestSendNotification(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("echo")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=echo"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	// Notifications should succeed without error.
	err := client.SendNotification(ctx, "test/notify", map[string]string{"msg": "hello"})
	if err != nil {
		t.Fatalf("SendNotification: %v", err)
	}
}

func TestRPCError(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("error")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=error"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	_, err := client.SendRequest(ctx, "test/fail", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRequestTimeout(t *testing.T) {
	t.Parallel()

	// Use a command that reads stdin but never writes a response.
	client := NewClient(ClientConfig{
		Command: "cat",
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err := client.SendRequest(tctx, "test/hang", nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestSubprocessCrash(t *testing.T) {
	t.Parallel()

	// Use "false" which exits immediately with a non-zero status.
	client := NewClient(ClientConfig{
		Command: "false",
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Give the process time to exit.
	time.Sleep(50 * time.Millisecond)

	// Sending a request after the subprocess crashes should fail.
	_, err := client.SendRequest(ctx, "test/ping", nil)
	if err == nil {
		t.Fatal("expected error after subprocess crash, got nil")
	}
}

func TestConcurrentRequests(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("echo")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=echo"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	const n = 10
	errs := make(chan error, n)

	for i := range n {
		go func(i int) {
			method := fmt.Sprintf("test/concurrent/%d", i)
			result, err := client.SendRequest(ctx, method, nil)
			if err != nil {
				errs <- fmt.Errorf("request %d: %w", i, err)
				return
			}

			var out map[string]string
			if err := json.Unmarshal(result, &out); err != nil {
				errs <- fmt.Errorf("request %d unmarshal: %w", i, err)
				return
			}
			if out["echo"] != method {
				errs <- fmt.Errorf("request %d: expected echo=%q, got %q", i, method, out["echo"])
				return
			}
			errs <- nil
		}(i)
	}

	for range n {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

func TestStartTwice(t *testing.T) {
	t.Parallel()

	client := NewClient(ClientConfig{
		Command: "cat",
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	err := client.Start(ctx)
	if err == nil {
		t.Fatal("expected error on second Start, got nil")
	}
}

func TestCloseIdempotent(t *testing.T) {
	t.Parallel()

	client := NewClient(ClientConfig{
		Command: "cat",
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Close should be safe to call multiple times.
	_ = client.Close()
	_ = client.Close()
}

func TestIncomingRequest(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("server_request")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=server_request"},
	})

	// Register a handler before starting.
	client.SetRequestHandler(func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return map[string]string{"handled": method}, nil
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	// Give time for the subprocess to send its request and get our response.
	time.Sleep(200 * time.Millisecond)
}

func TestRPCErrorInterface(t *testing.T) {
	t.Parallel()

	rpcErr := &RPCError{
		Code:    -32600,
		Message: "invalid request",
	}

	if got := rpcErr.Error(); got != "rpc error -32600: invalid request" {
		t.Errorf("unexpected error string: %q", got)
	}
}

func TestNewClientConfig(t *testing.T) {
	t.Parallel()

	cfg := ClientConfig{
		Command: "echo",
		Args:    []string{"hello"},
		Dir:     "/tmp",
		Env:     []string{"FOO=bar"},
		Timeout: 30 * time.Second,
	}

	client := NewClient(cfg)
	if client.cfg.Command != "echo" {
		t.Errorf("expected command=echo, got %q", client.cfg.Command)
	}
	if client.cfg.Dir != "/tmp" {
		t.Errorf("expected dir=/tmp, got %q", client.cfg.Dir)
	}
}

func TestNotificationHandler(t *testing.T) {
	t.Parallel()

	// Use the server_request helper — it sends a request and then a notification
	// ("result_received") after getting the response.
	cmd, args := helperCommand("server_request")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=server_request"},
	})

	// Track received notifications.
	received := make(chan string, 1)
	client.SetNotificationHandler(func(method string, params json.RawMessage) {
		received <- method
	})

	client.SetRequestHandler(func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return map[string]string{"handled": method}, nil
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	select {
	case method := <-received:
		if method != "result_received" {
			t.Errorf("expected notification method 'result_received', got %q", method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestNotificationHandlerNil(t *testing.T) {
	t.Parallel()

	// Ensure notifications don't panic when no handler is set.
	cmd, args := helperCommand("server_request")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=server_request"},
	})

	// No notification handler set — should not panic.
	client.SetRequestHandler(func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return map[string]string{"handled": method}, nil
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait briefly to let notification be processed without panic.
	time.Sleep(200 * time.Millisecond)
	client.Close()
}

func TestHelperCommandExists(t *testing.T) {
	t.Parallel()

	// Verify that the test binary exists (used as subprocess).
	cmd, _ := helperCommand("echo")
	if _, err := exec.LookPath(cmd); err != nil {
		t.Skipf("test binary not in PATH: %v", err)
	}
}
