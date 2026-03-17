// Package main implements a mock ACP agent for integration testing.
// It reads JSON-RPC 2.0 messages from stdin and writes responses to stdout.
// Behavior is configurable via environment variables:
//
//	MOCK_DELAY      - delay before responding (e.g. "10s")
//	MOCK_CRASH      - if "true", exit with code 1 after initialize
//	MOCK_BAD_JSON   - if "true", write malformed JSON for session/prompt
//	MOCK_FINDINGS   - custom findings JSON to return (overrides default)
//	MOCK_EMPTY      - if "true", return empty findings array
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

var defaultFindings = `[{"path":"src/main.go","line":12,"side":"new","severity":"warning","category":"style","message":"Consider renaming this variable.","fingerprint":""},{"path":"src/main.go","line":14,"side":"new","severity":"error","category":"bug","message":"Possible nil dereference.","fingerprint":""}]`

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	sessionID := "test-session-001"

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		// Skip notifications (no ID).
		if req.ID == nil {
			continue
		}

		// Apply delay if configured.
		if d := os.Getenv("MOCK_DELAY"); d != "" {
			if dur, err := time.ParseDuration(d); err == nil {
				time.Sleep(dur)
			}
		}

		switch req.Method {
		case "initialize":
			resp := response{
				JSONRPC: "2.0",
				ID:      *req.ID,
				Result: map[string]any{
					"protocolVersion": 1,
					"capabilities": map[string]any{
						"review": true,
					},
				},
			}
			writeResponse(resp)

			// Crash after initialize if configured.
			if os.Getenv("MOCK_CRASH") == "true" {
				os.Exit(1)
			}

		case "session/new":
			resp := response{
				JSONRPC: "2.0",
				ID:      *req.ID,
				Result: map[string]any{
					"sessionId": sessionID,
				},
			}
			writeResponse(resp)

		case "session/prompt":
			if os.Getenv("MOCK_BAD_JSON") == "true" {
				fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","id":%d,"result":{"stopReason":"end_turn"}}`+"\n", *req.ID)
				// Send a malformed chunk before.
				fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","method":"session/update","params":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"not valid json [[["}}}`+"\n")
				continue
			}

			findings := defaultFindings
			if custom := os.Getenv("MOCK_FINDINGS"); custom != "" {
				findings = custom
			}
			if os.Getenv("MOCK_EMPTY") == "true" {
				findings = "[]"
			}

			// Send findings as an agent_message_chunk notification.
			notif := notification{
				JSONRPC: "2.0",
				Method:  "session/update",
				Params: map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content": map[string]string{
						"type": "text",
						"text": findings,
					},
				},
			}
			writeNotification(notif)

			resp := response{
				JSONRPC: "2.0",
				ID:      *req.ID,
				Result: map[string]any{
					"stopReason": "end_turn",
				},
			}
			writeResponse(resp)

		case "session/stop":
			resp := response{
				JSONRPC: "2.0",
				ID:      *req.ID,
				Result:  map[string]any{"status": "stopped"},
			}
			writeResponse(resp)

		default:
			resp := response{
				JSONRPC: "2.0",
				ID:      *req.ID,
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    -32601,
					Message: fmt.Sprintf("method not found: %s", req.Method),
				},
			}
			writeResponse(resp)
		}
	}
}

func writeResponse(resp response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

func writeNotification(notif notification) {
	data, err := json.Marshal(notif)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
}
