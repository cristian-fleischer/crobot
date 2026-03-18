package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestHelperSession is a test helper subprocess that simulates an ACP agent
// with session lifecycle support.
func TestHelperSession(t *testing.T) {
	if os.Getenv("CROBOT_TEST_HELPER") != "session" {
		return
	}

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for dec.More() {
		var msg rawMessage
		if err := dec.Decode(&msg); err != nil {
			os.Exit(1)
		}

		if msg.ID == nil {
			continue
		}

		id := *msg.ID
		var resp Response

		switch msg.Method {
		case "initialize":
			result, _ := json.Marshal(map[string]any{
				"protocolVersion": 1,
				"capabilities":   map[string]any{},
			})
			resp = Response{JSONRPC: "2.0", ID: id, Result: result}

		case "session/new":
			result, _ := json.Marshal(map[string]any{
				"sessionId": "test-session-123",
				"models": map[string]any{
					"currentModelId": "test-model",
					"availableModels": []map[string]any{
						{"modelId": "test-model", "name": "Test Model", "description": "Default test model"},
						{"modelId": "test-model-2", "name": "Test Model 2"},
					},
				},
			})
			resp = Response{JSONRPC: "2.0", ID: id, Result: result}

		case "session/prompt":
			// Send agent_message_chunk notification before the response.
			chunk, _ := json.Marshal(map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content": map[string]string{
					"type": "text",
					"text": "This is a review finding.",
				},
			})
			notif := Notification{
				JSONRPC: "2.0",
				Method:  "session/update",
				Params:  chunk,
			}
			_ = enc.Encode(notif)

			result, _ := json.Marshal(map[string]string{
				"stopReason": "end_turn",
			})
			resp = Response{JSONRPC: "2.0", ID: id, Result: result}

		case "session/stop":
			result, _ := json.Marshal(map[string]string{"status": "stopped"})
			resp = Response{JSONRPC: "2.0", ID: id, Result: result}

		default:
			resp = Response{
				JSONRPC: "2.0",
				ID:      id,
				Error:   &RPCError{Code: -32601, Message: "method not found"},
			}
		}

		if err := enc.Encode(resp); err != nil {
			os.Exit(1)
		}
	}
}

// TestHelperSessionStreaming simulates an ACP agent that sends session/update
// notifications before the final prompt response.
func TestHelperSessionStreaming(t *testing.T) {
	if os.Getenv("CROBOT_TEST_HELPER") != "session_streaming" {
		return
	}

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for dec.More() {
		var msg rawMessage
		if err := dec.Decode(&msg); err != nil {
			os.Exit(1)
		}

		if msg.ID == nil {
			continue
		}

		id := *msg.ID
		var resp Response

		switch msg.Method {
		case "initialize":
			result, _ := json.Marshal(map[string]any{
				"protocolVersion": 1,
				"capabilities":   map[string]any{},
			})
			resp = Response{JSONRPC: "2.0", ID: id, Result: result}

		case "session/new":
			result, _ := json.Marshal(map[string]any{
				"sessionId": "stream-session",
			})
			resp = Response{JSONRPC: "2.0", ID: id, Result: result}

		case "session/prompt":
			// Send streaming agent_message_chunk notifications.
			chunks := []string{"Analyzing ", "the code", " changes..."}
			for _, text := range chunks {
				params, _ := json.Marshal(map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content": map[string]string{
						"type": "text",
						"text": text,
					},
				})
				notif := Notification{
					JSONRPC: "2.0",
					Method:  "session/update",
					Params:  params,
				}
				_ = enc.Encode(notif)
			}

			// Send a thought chunk (should be streamed but not accumulated).
			thoughtParams, _ := json.Marshal(map[string]any{
				"sessionUpdate": "agent_thought_chunk",
				"content": map[string]string{
					"type": "text",
					"text": "[thinking...]",
				},
			})
			thoughtNotif := Notification{
				JSONRPC: "2.0",
				Method:  "session/update",
				Params:  thoughtParams,
			}
			_ = enc.Encode(thoughtNotif)

			// Final response.
			result, _ := json.Marshal(map[string]string{
				"stopReason": "end_turn",
			})
			resp = Response{JSONRPC: "2.0", ID: id, Result: result}

		case "session/stop":
			result, _ := json.Marshal(map[string]string{"status": "stopped"})
			resp = Response{JSONRPC: "2.0", ID: id, Result: result}

		default:
			resp = Response{
				JSONRPC: "2.0",
				ID:      id,
				Error:   &RPCError{Code: -32601, Message: "method not found"},
			}
		}

		if err := enc.Encode(resp); err != nil {
			os.Exit(1)
		}
	}
}

func TestSessionInitialize(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("session")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=session"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := NewSession(SessionConfig{Client: client})
	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
}

func TestSessionPrompt(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("session")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=session"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := NewSession(SessionConfig{Client: client})
	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	if result.FinalText != "This is a review finding." {
		t.Errorf("unexpected final text: %q", result.FinalText)
	}

	if result.StopReason != "end_turn" {
		t.Errorf("unexpected stop reason: %q", result.StopReason)
	}
}

func TestSessionModelMetadata(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("session")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=session"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := NewSession(SessionConfig{Client: client})
	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := session.CreateSession(ctx); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if session.CurrentModel != "test-model" {
		t.Errorf("CurrentModel = %q, want %q", session.CurrentModel, "test-model")
	}
	if len(session.AvailableModels) != 2 {
		t.Fatalf("AvailableModels count = %d, want 2", len(session.AvailableModels))
	}
	if session.AvailableModels[0].ID != "test-model" {
		t.Errorf("AvailableModels[0].ID = %q, want %q", session.AvailableModels[0].ID, "test-model")
	}
	if session.AvailableModels[1].ID != "test-model-2" {
		t.Errorf("AvailableModels[1].ID = %q, want %q", session.AvailableModels[1].ID, "test-model-2")
	}
}

func TestSessionSetModel(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("session")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=session"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := NewSession(SessionConfig{
		Client:  client,
		ModelID: "custom-model",
	})
	if session.modelID != "custom-model" {
		t.Errorf("modelID = %q, want %q", session.modelID, "custom-model")
	}

	session.SetModel("other-model")
	if session.modelID != "other-model" {
		t.Errorf("modelID after SetModel = %q, want %q", session.modelID, "other-model")
	}
}

func TestSessionClose(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("session")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=session"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := NewSession(SessionConfig{Client: client})
	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Prompt to create a session.
	if _, err := session.Prompt(ctx, "test"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	if err := session.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSessionCloseWithoutSession(t *testing.T) {
	t.Parallel()

	client := NewClient(ClientConfig{Command: "cat"})
	session := NewSession(SessionConfig{Client: client})

	// Closing without a session should be a no-op.
	if err := session.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSessionPermissionHandler(t *testing.T) {
	t.Parallel()

	session := &Session{}

	tests := []struct {
		name       string
		params     string
		wantSelect bool
	}{
		{
			name: "allow option selected",
			params: `{
				"options": [{"kind": "allow", "name": "Allow", "optionId": "opt-1"}],
				"sessionId": "s1"
			}`,
			wantSelect: true,
		},
		{
			name: "always_allow option selected",
			params: `{
				"options": [{"kind": "always_allow", "name": "Always Allow", "optionId": "opt-2"}],
				"sessionId": "s1"
			}`,
			wantSelect: true,
		},
		{
			name: "no options cancels",
			params: `{
				"options": [],
				"sessionId": "s1"
			}`,
			wantSelect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := session.handlePermission(context.Background(), json.RawMessage(tt.params))
			if err != nil {
				t.Fatalf("handlePermission: %v", err)
			}

			data, _ := json.Marshal(result)
			var resp map[string]any
			if err := json.Unmarshal(data, &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			outcome, ok := resp["outcome"].(map[string]any)
			if !ok {
				t.Fatal("expected 'outcome' map in response")
			}

			if tt.wantSelect {
				if outcome["outcome"] != "selected" {
					t.Errorf("expected outcome=selected, got %v", outcome["outcome"])
				}
			} else {
				if outcome["outcome"] != "cancelled" {
					t.Errorf("expected outcome=cancelled, got %v", outcome["outcome"])
				}
			}
		})
	}
}

func TestSessionStreamingOutput(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("session_streaming")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=session_streaming"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	var buf bytes.Buffer
	session := NewSession(SessionConfig{
		Client:       client,
		StreamWriter: &buf,
	})

	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	// FinalText should contain accumulated agent_message_chunk text only.
	if result.FinalText != "Analyzing the code changes..." {
		t.Errorf("unexpected final text: %q", result.FinalText)
	}

	// The stream buffer should contain both chunks and thought text.
	streamed := buf.String()
	if streamed == "" {
		t.Error("expected streaming output, got empty string")
	}
	if !bytes.Contains([]byte(streamed), []byte("Analyzing ")) {
		t.Errorf("streaming output missing chunk text, got: %q", streamed)
	}
	if !bytes.Contains([]byte(streamed), []byte("the code")) {
		t.Errorf("streaming output missing chunk text, got: %q", streamed)
	}
	// Thought chunks should appear in stream but not in FinalText.
	if !bytes.Contains([]byte(streamed), []byte("[thinking...]")) {
		t.Errorf("streaming output missing thought text, got: %q", streamed)
	}
}

func TestSessionStreamingNilWriter(t *testing.T) {
	t.Parallel()

	cmd, args := helperCommand("session_streaming")
	client := NewClient(ClientConfig{
		Command: cmd,
		Args:    args,
		Env:     []string{"CROBOT_TEST_HELPER=session_streaming"},
	})

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	// No StreamWriter — notifications should be silently ignored without panic.
	session := NewSession(SessionConfig{Client: client})

	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	// FinalText should still accumulate even without StreamWriter.
	if result.FinalText != "Analyzing the code changes..." {
		t.Errorf("unexpected final text: %q", result.FinalText)
	}
}

func TestHandleNotification_VariousFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   string
		wantText string
	}{
		{
			name:     "nested ACP format",
			params:   `{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hello "}}}`,
			wantText: "hello ",
		},
		{
			name:     "flat agent_message_chunk",
			params:   `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"world"}}`,
			wantText: "world",
		},
		{
			name:     "array content blocks",
			params:   `{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}}`,
			wantText: "hello world",
		},
		{
			name:     "legacy delta field",
			params:   `{"delta":"foo"}`,
			wantText: "foo",
		},
		{
			name:     "legacy text field",
			params:   `{"text":"bar"}`,
			wantText: "bar",
		},
		{
			name:     "tool_call_update with input shows call line",
			params:   `{"sessionId":"s1","update":{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call_update","rawInput":{"path":"src/main.go"}}}`,
			wantText: "\n\n\033[2m │ Read(path=src/main.go)\033[0m\n",
		},
		{
			name:     "tool_call pending writes nothing",
			params:   `{"sessionId":"s1","update":{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call","rawInput":{},"status":"pending"}}`,
			wantText: "",
		},
		{
			name:     "tool_call_update completed result only in debug",
			params:   `{"sessionId":"s1","update":{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call_update","status":"completed","rawOutput":"line1\nline2\nline3"}}`,
			wantText: "",
		},
		{
			name:     "usage_update ignored",
			params:   `{"sessionId":"s1","update":{"sessionUpdate":"usage_update"}}`,
			wantText: "",
		},
		{
			name:     "invalid json",
			params:   `{bad json}`,
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			s := &Session{streamWriter: &buf}

			s.handleNotification("session/update", json.RawMessage(tt.params))

			if got := buf.String(); got != tt.wantText {
				t.Errorf("got %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestExtractResponseText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		messages string
		text     string
		want     string
	}{
		{
			name: "direct text field",
			text: "hello world",
			want: "hello world",
		},
		{
			name:    "content single block",
			content: `{"type":"text","text":"from content"}`,
			want:    "from content",
		},
		{
			name:    "content array",
			content: `[{"type":"text","text":"part1 "},{"type":"text","text":"part2"}]`,
			want:    "part1 part2",
		},
		{
			name:     "messages array",
			messages: `[{"role":"assistant","content":[{"type":"text","text":"from messages"}]}]`,
			want:     "from messages",
		},
		{
			name:     "messages with text field",
			messages: `[{"role":"assistant","text":"msg text"}]`,
			want:     "msg text",
		},
		{
			name:    "text takes priority over content",
			text:    "direct",
			content: `{"type":"text","text":"should not use"}`,
			want:    "direct",
		},
		{
			name: "all empty",
			want: "",
		},
		{
			name:    "content with non-text blocks",
			content: `[{"type":"image","text":"ignored"}]`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var content, messages json.RawMessage
			if tt.content != "" {
				content = json.RawMessage(tt.content)
			}
			if tt.messages != "" {
				messages = json.RawMessage(tt.messages)
			}

			got := extractResponseText(content, messages, tt.text)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractToolName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rawUpdate  string
		rawContent string
		want       string
	}{
		{
			name:      "claude code _meta.claudeCode.toolName",
			rawUpdate: `{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call","title":"Read File"}`,
			want:      "Read",
		},
		{
			name:      "fallback to title",
			rawUpdate: `{"sessionUpdate":"tool_call","title":"Read File"}`,
			want:      "Read File",
		},
		{
			name:      "generic ACP name field",
			rawUpdate: `{"sessionUpdate":"tool_call","name":"Write"}`,
			want:      "Write",
		},
		{
			name:      "generic ACP tool field",
			rawUpdate: `{"sessionUpdate":"tool_call","tool":"Grep"}`,
			want:      "Grep",
		},
		{
			name:      "_meta takes priority over name",
			rawUpdate: `{"_meta":{"claudeCode":{"toolName":"Read"}},"name":"SomethingElse"}`,
			want:      "Read",
		},
		{
			name:      "empty update",
			rawUpdate: "",
			want:      "",
		},
		{
			name:      "no name fields at all",
			rawUpdate: `{"sessionUpdate":"tool_call","status":"pending"}`,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var rawUpdate, rawContent json.RawMessage
			if tt.rawUpdate != "" {
				rawUpdate = json.RawMessage(tt.rawUpdate)
			}
			if tt.rawContent != "" {
				rawContent = json.RawMessage(tt.rawContent)
			}

			got := extractToolName(rawUpdate, rawContent)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolCallStreaming(t *testing.T) {
	t.Parallel()

	makeNotification := func(update string) json.RawMessage {
		env := `{"sessionId":"test","update":` + update + `}`
		return json.RawMessage(env)
	}

	t.Run("tool_call_update with input shows call line", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		s := &Session{streamWriter: &buf}
		params := makeNotification(`{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call_update","rawInput":{"file_path":"src/main.go"}}`)
		s.handleNotification("session/update", params)
		got := buf.String()
		if !strings.Contains(got, "│ Read(") {
			t.Errorf("expected tool call line with name and pipe prefix, got %q", got)
		}
		if !strings.Contains(got, "file_path=src/main.go") {
			t.Errorf("expected input in call line, got %q", got)
		}
	})

	t.Run("tool_call with pending status writes nothing", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		s := &Session{streamWriter: &buf}
		params := makeNotification(`{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call","rawInput":{},"status":"pending"}`)
		s.handleNotification("session/update", params)
		if buf.Len() > 0 {
			t.Errorf("pending tool_call should not write, got %q", buf.String())
		}
	})

	t.Run("tool_call_update with completed status writes nothing to stream", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		s := &Session{streamWriter: &buf}
		params := makeNotification(`{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call_update","status":"completed","rawOutput":"file contents here"}`)
		s.handleNotification("session/update", params)
		if buf.Len() > 0 {
			t.Errorf("completed result should not write to stream (debug only), got %q", buf.String())
		}
	})

	t.Run("all output is dimmed with pipe prefix", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		s := &Session{streamWriter: &buf}
		params := makeNotification(`{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call_update","rawInput":{"file_path":"a.go"}}`)
		s.handleNotification("session/update", params)
		got := buf.String()
		if !strings.Contains(got, "\033[2m") || !strings.Contains(got, "\033[0m") {
			t.Errorf("expected ANSI dim codes, got %q", got)
		}
		if !strings.Contains(got, "│") {
			t.Errorf("expected pipe prefix, got %q", got)
		}
	})

	t.Run("blank lines separate text and tool blocks", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		s := &Session{streamWriter: &buf}

		// Send text first.
		textParams := makeNotification(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Hello"}}`)
		s.handleNotification("session/update", textParams)

		// Then a tool call with input — should have double blank line separator.
		toolParams := makeNotification(`{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call_update","rawInput":{"file_path":"a.go"}}`)
		s.handleNotification("session/update", toolParams)

		// Then text again — should have double blank line separator.
		s.handleNotification("session/update", textParams)

		got := buf.String()
		// Text→tool: "Hello" + blank line + tool line
		if !strings.Contains(got, "Hello\n\n") {
			t.Errorf("expected blank line before tool block, got %q", got)
		}
		// Tool→text: tool line\n + blank line + "Hello"
		if !strings.HasSuffix(got, "\n\nHello") {
			t.Errorf("expected blank line after tool block, got %q", got)
		}
	})

	t.Run("LLM newlines after tool block stripped for clean spacing", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		s := &Session{streamWriter: &buf}

		// Tool call.
		toolParams := makeNotification(`{"_meta":{"claudeCode":{"toolName":"Read"}},"sessionUpdate":"tool_call_update","rawInput":{"file_path":"a.go"}}`)
		s.handleNotification("session/update", toolParams)

		// LLM sends \n\n before text (typical paragraph break).
		nlParams := makeNotification(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"\n\n"}}`)
		s.handleNotification("session/update", nlParams)

		// Then actual text.
		textParams := makeNotification(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Next step."}}`)
		s.handleNotification("session/update", textParams)

		got := buf.String()
		// Should have exactly one blank line between tool and text, not three.
		if !strings.HasSuffix(got, "\n\nNext step.") {
			t.Errorf("expected exactly one blank line after tool, got %q", got)
		}
		// Count newlines between tool line end and "Next"
		idx := strings.Index(got, "Next")
		before := got[:idx]
		nlCount := strings.Count(before[strings.LastIndex(before, "\033[0m"):], "\n")
		if nlCount != 2 {
			t.Errorf("expected 2 newlines (=1 blank line) before text, got %d in %q", nlCount, got)
		}
	})
}

func TestCapLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"under limit", "a\nb\nc", 5, "a\nb\nc"},
		{"at limit", "a\nb\nc\nd\ne", 5, "a\nb\nc\nd\ne"},
		{"over limit", "a\nb\nc\nd\ne\nf\ng", 5, "a\nb\nc\nd\ne\n..."},
		{"single line", "hello", 5, "hello"},
		{"empty", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := capLines(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummarizeJSON(t *testing.T) {
	t.Parallel()

	t.Run("simple object", func(t *testing.T) {
		t.Parallel()
		got := summarizeJSON(json.RawMessage(`{"path":"src/main.go","line":42}`))
		// Map ordering is not guaranteed, so check both fields are present.
		if !strings.Contains(got, "path=src/main.go") {
			t.Errorf("expected path=src/main.go, got %q", got)
		}
		if !strings.Contains(got, "line=42") {
			t.Errorf("expected line=42, got %q", got)
		}
	})

	t.Run("long value truncated", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("x", 100)
		got := summarizeJSON(json.RawMessage(`{"val":"` + long + `"}`))
		if !strings.Contains(got, "...") {
			t.Errorf("expected truncation, got %q", got)
		}
	})

	t.Run("non-object", func(t *testing.T) {
		t.Parallel()
		got := summarizeJSON(json.RawMessage(`"just a string"`))
		if got != "just a string" {
			t.Errorf("got %q, want %q", got, "just a string")
		}
	})
}

func TestHandleNotification_IgnoresNonUpdateMethods(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := &Session{streamWriter: &buf}

	s.handleNotification("session/other", json.RawMessage(`{"delta":"should not appear"}`))

	if buf.Len() > 0 {
		t.Errorf("expected empty buffer for non-update method, got %q", buf.String())
	}
}
