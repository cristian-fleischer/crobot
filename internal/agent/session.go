package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cristian-fleischer/crobot/internal/version"
)

// acpProtocolVersion is the ACP protocol version we support.
const acpProtocolVersion = 1

// SessionConfig holds the dependencies for creating a new session.
type SessionConfig struct {
	Client    *Client
	FSHandler *FSHandler
	// ModelID, if non-empty, requests a specific model from the agent.
	ModelID string
	// StreamWriter, if non-nil, receives agent streaming output as it arrives
	// via session/update notifications.
	StreamWriter io.Writer
	// ActivityFunc, if non-nil, is called when the agent's activity changes
	// (e.g. "thinking...", "using tool..."). Used for progress display.
	ActivityFunc func(activity string)
}

// SessionResult contains the final output from a session prompt.
type SessionResult struct {
	// FinalText is the accumulated assistant text from session/update
	// notifications during the prompt turn.
	FinalText  string
	StopReason string
}

// Message represents a single message exchanged during a session.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ModelInfo describes a model available from the agent.
type ModelInfo struct {
	ID          string `json:"modelId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Session manages the lifecycle of an ACP session with an agent subprocess.
type Session struct {
	client       *Client
	sessionID    string
	modelID      string // requested model
	fsHandler    *FSHandler
	streamWriter io.Writer
	activityFunc func(activity string)

	// Model metadata from session/new response.
	CurrentModel   string
	AvailableModels []ModelInfo

	// mu protects agentText and stream state during concurrent notification handling.
	mu               sync.Mutex
	agentText        strings.Builder
	lastStreamWasTool bool
	trailingNewlines  int // consecutive trailing newlines in the stream
}

// NewSession creates a new Session with the given configuration.
func NewSession(cfg SessionConfig) *Session {
	s := &Session{
		client:       cfg.Client,
		modelID:      cfg.ModelID,
		fsHandler:    cfg.FSHandler,
		streamWriter: cfg.StreamWriter,
		activityFunc: cfg.ActivityFunc,
	}

	// Register the request handler to route incoming requests.
	s.client.SetRequestHandler(s.handleRequest)

	// Register the notification handler for streaming output.
	s.client.SetNotificationHandler(s.handleNotification)

	return s
}

// Initialize performs the ACP initialize handshake with the agent subprocess.
func (s *Session) Initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": acpProtocolVersion,
		"clientInfo": map[string]string{
			"name":    "crobot",
			"version": version.Version,
		},
		"clientCapabilities": map[string]any{
			"fs": map[string]any{
				"readTextFile": true,
			},
		},
	}

	result, err := s.client.SendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("agent: initialize: %w", err)
	}

	slog.Debug("agent: initialize response", "result", string(result))

	// Parse the server capabilities (we don't need them yet, but validate the
	// response is valid JSON).
	var serverCaps map[string]any
	if err := json.Unmarshal(result, &serverCaps); err != nil {
		return fmt.Errorf("agent: initialize: parsing server capabilities: %w", err)
	}

	return nil
}

// CreateSession explicitly creates a new ACP session. This populates
// CurrentModel and AvailableModels from the agent's response. It is
// called automatically by Prompt if no session exists, but can be called
// earlier when the caller needs model metadata before prompting.
func (s *Session) CreateSession(ctx context.Context) error {
	return s.createSession(ctx)
}

// SetModel changes the requested model ID for subsequent session creation.
func (s *Session) SetModel(modelID string) {
	s.modelID = modelID
}

// Prompt sends a prompt to the agent and collects the response.
// The agent's text output is accumulated from session/update notifications
// during the prompt turn. The final response contains only a stopReason.
func (s *Session) Prompt(ctx context.Context, prompt string) (*SessionResult, error) {
	// Create a new session if we don't have one.
	if s.sessionID == "" {
		if err := s.createSession(ctx); err != nil {
			return nil, err
		}
	}

	// Reset accumulated text for this turn.
	s.mu.Lock()
	s.agentText.Reset()
	s.mu.Unlock()

	// ACP prompt format: prompt is an array of ContentBlock.
	params := map[string]any{
		"sessionId": s.sessionID,
		"prompt": []map[string]string{
			{"type": "text", "text": prompt},
		},
	}

	result, err := s.client.SendRequest(ctx, "session/prompt", params)
	if err != nil {
		return nil, fmt.Errorf("agent: prompt: %w", err)
	}

	slog.Debug("agent: prompt response", "raw", string(result))

	var promptResult struct {
		StopReason string          `json:"stopReason"`
		Content    json.RawMessage `json:"content"`
		Messages   json.RawMessage `json:"messages"`
		Text       string          `json:"text"`
	}
	if err := json.Unmarshal(result, &promptResult); err != nil {
		return nil, fmt.Errorf("agent: prompt: parsing result: %w", err)
	}

	s.mu.Lock()
	finalText := s.agentText.String()
	s.mu.Unlock()

	slog.Debug("agent: accumulated streaming text", "len", len(finalText))

	// If no text was accumulated from streaming notifications, try to
	// extract it from the response itself. Some ACP adapters (e.g. codex-acp)
	// return the full text in the response rather than streaming it.
	if finalText == "" {
		if responseText := extractResponseText(promptResult.Content, promptResult.Messages, promptResult.Text); responseText != "" {
			finalText = responseText
			slog.Debug("agent: extracted text from response", "len", len(finalText))
			// Stream it to the writer so --show-agent-output still works.
			if s.streamWriter != nil {
				_, _ = fmt.Fprint(s.streamWriter, finalText)
			}
		}
	}

	slog.Debug("agent: final text", "len", len(finalText))

	return &SessionResult{
		FinalText:  finalText,
		StopReason: promptResult.StopReason,
	}, nil
}

// Close terminates the ACP session. If the agent doesn't support
// session/stop (it's an optional capability), the error is logged
// and the session is cleaned up anyway. Uses a short timeout since
// session/stop is best-effort.
func (s *Session) Close(_ context.Context) error {
	if s.sessionID == "" {
		return nil
	}

	// Use a short dedicated timeout — don't inherit the caller's context
	// which may have a long agent timeout (e.g. 600s).
	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	params := map[string]any{
		"sessionId": s.sessionID,
	}

	_, err := s.client.SendRequest(stopCtx, "session/stop", params)
	if err != nil {
		// session/stop is optional in ACP — log but don't fail.
		slog.Debug("agent: session/stop not supported or failed", "error", err)
	}

	s.sessionID = ""
	return nil
}

// createSession creates a new ACP session.
func (s *Session) createSession(ctx context.Context) error {
	cwd, _ := os.Getwd()

	params := map[string]any{
		"cwd":        cwd,
		"mcpServers": []any{},
	}
	if s.modelID != "" {
		params["modelId"] = s.modelID
	}

	result, err := s.client.SendRequest(ctx, "session/new", params)
	if err != nil {
		return fmt.Errorf("agent: session/new: %w", err)
	}

	slog.Debug("agent: session/new response", "result", string(result))

	var sessionResp struct {
		SessionID string `json:"sessionId"`
		Models    struct {
			AvailableModels []ModelInfo `json:"availableModels"`
			CurrentModelID  string     `json:"currentModelId"`
		} `json:"models"`
	}
	if err := json.Unmarshal(result, &sessionResp); err != nil {
		return fmt.Errorf("agent: session/new: parsing response: %w", err)
	}

	s.sessionID = sessionResp.SessionID
	s.CurrentModel = sessionResp.Models.CurrentModelID
	s.AvailableModels = sessionResp.Models.AvailableModels

	slog.Debug("agent: session created",
		"session_id", s.sessionID,
		"model", s.CurrentModel,
		"available_models", len(s.AvailableModels),
	)
	return nil
}

// contentBlock is a single ACP content block.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// parseContent extracts content blocks from a json.RawMessage that may be
// either a single ContentBlock object or an array of ContentBlock objects.
func parseContent(raw json.RawMessage) []contentBlock {
	if len(raw) == 0 {
		return nil
	}

	// Try array first (most common from claude-agent-acp).
	var arr []contentBlock
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}

	// Fall back to single object.
	var single contentBlock
	if err := json.Unmarshal(raw, &single); err == nil {
		return []contentBlock{single}
	}

	return nil
}

// handleNotification processes incoming JSON-RPC notifications from the agent.
// session/update notifications contain streaming output — text from
// agent_message_chunk updates is accumulated and, if a StreamWriter is
// configured, written in real-time.
func (s *Session) handleNotification(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}

	// ACP session/update params are: {sessionId, update: {sessionUpdate, content, ...}}
	// The update object may contain additional fields (name, input, output)
	// depending on the sessionUpdate type, so we capture it as raw JSON.
	var envelope struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
		// Flat fallback for non-standard agents that send sessionUpdate at top level.
		SessionUpdate string          `json:"sessionUpdate"`
		Content       json.RawMessage `json:"content"`
		// Legacy fields.
		Delta string `json:"delta"`
		Text  string `json:"text"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		slog.Debug("agent: session/update parse error", "error", err)
		return
	}

	// Parse the update object to extract standard fields.
	var update struct {
		SessionUpdate string          `json:"sessionUpdate"`
		Content       json.RawMessage `json:"content"`
	}
	if len(envelope.Update) > 0 {
		_ = json.Unmarshal(envelope.Update, &update)
	}

	// Resolve the update — prefer the nested ACP format, fall back to flat.
	sessionUpdate := update.SessionUpdate
	rawContent := update.Content
	// rawUpdate is the full update object for extracting tool details.
	rawUpdate := envelope.Update
	if sessionUpdate == "" {
		sessionUpdate = envelope.SessionUpdate
		rawContent = envelope.Content
		rawUpdate = nil
	}

	slog.Debug("agent: session/update", "type", sessionUpdate, "content_len", len(rawContent), "raw_update", string(rawUpdate))

	blocks := parseContent(rawContent)

	var text string
	switch sessionUpdate {
	case "agent_message_chunk":
		for _, b := range blocks {
			if b.Type == "text" {
				text += b.Text
			}
		}
	case "agent_thought_chunk":
		if s.activityFunc != nil {
			s.activityFunc("thinking...")
		}
		// Thought chunks — stream but don't accumulate as final text.
		var thought string
		for _, b := range blocks {
			if b.Type == "text" {
				thought += b.Text
			}
		}
		if thought == "" {
			return
		}
		s.mu.Lock()
		if s.lastStreamWasTool {
			display := strings.TrimLeft(thought, "\n")
			if display == "" {
				s.mu.Unlock()
				return
			}
			s.ensureBlankLine()
			s.lastStreamWasTool = false
			s.writeStream(display)
		} else {
			s.writeStream(thought)
		}
		s.mu.Unlock()
		return
	case "tool_call", "tool_call_update":
		toolName := extractToolName(rawUpdate, rawContent)
		slog.Debug("agent: tool activity", "type", sessionUpdate, "tool", toolName)

		// Activity indicator.
		if s.activityFunc != nil {
			if toolName != "" {
				s.activityFunc("tool: " + toolName)
			} else {
				s.activityFunc("using tool...")
			}
		}

		// Show tool call line when input becomes available.
		input := extractToolInput(rawUpdate, rawContent)
		if input != "" {
			label := toolName
			if label == "" {
				label = "tool call"
			}
			s.mu.Lock()
			if !s.lastStreamWasTool {
				s.ensureBlankLine()
				s.lastStreamWasTool = true
			}
			s.writeStream(fmt.Sprintf("%s │ %s(%s)%s\n", dimStart, label, input, dimEnd))
			s.mu.Unlock()
		}

		// Log tool results at debug level only (shown with -v).
		output := extractToolOutput(rawUpdate, rawContent)
		if output != "" {
			output = strings.TrimSpace(output)
			output = capLines(output, maxToolOutputLines)
			slog.Debug("agent: tool output", "tool", toolName, "output", output)
		}
		return
	case "tool_result":
		// Legacy/generic ACP tool_result — unlikely from Claude Code agents.
		slog.Debug("agent: tool result")
		return
	default:
		// Legacy fallback for non-standard agents.
		switch {
		case envelope.Delta != "":
			text = envelope.Delta
		case envelope.Text != "":
			text = envelope.Text
		}
	}

	if text == "" {
		return
	}

	// Accumulate agent text (unmodified) for the final result.
	s.mu.Lock()
	s.agentText.WriteString(text)

	// Stream to writer — manage tool→text transition.
	if s.lastStreamWasTool {
		// Strip leading newlines from LLM text so we control spacing exactly.
		display := strings.TrimLeft(text, "\n")
		if display == "" {
			// Chunk was pure newlines — keep flag, wait for real content.
			s.mu.Unlock()
			return
		}
		s.ensureBlankLine()
		s.lastStreamWasTool = false
		s.writeStream(display)
	} else {
		s.writeStream(text)
	}
	s.mu.Unlock()
}

// extractResponseText tries to extract text from a prompt response that
// embeds the result directly (instead of streaming via notifications).
// It handles several common response formats from ACP adapters.
func extractResponseText(content, messages json.RawMessage, text string) string {
	// Direct text field.
	if text != "" {
		return text
	}

	// content: array of ContentBlock or single ContentBlock.
	if len(content) > 0 {
		blocks := parseContent(content)
		var sb strings.Builder
		for _, b := range blocks {
			if b.Type == "text" {
				sb.WriteString(b.Text)
			}
		}
		if sb.Len() > 0 {
			return sb.String()
		}
	}

	// messages: array of message objects, each with content blocks.
	if len(messages) > 0 {
		var msgs []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
			Text    string          `json:"text"`
		}
		if json.Unmarshal(messages, &msgs) == nil {
			var sb strings.Builder
			for _, m := range msgs {
				if m.Text != "" {
					sb.WriteString(m.Text)
					continue
				}
				blocks := parseContent(m.Content)
				for _, b := range blocks {
					if b.Type == "text" {
						sb.WriteString(b.Text)
					}
				}
			}
			if sb.Len() > 0 {
				return sb.String()
			}
		}
	}

	return ""
}

// extractToolName pulls the tool name from the update object.
// Claude Code agents put it at _meta.claudeCode.toolName; the title field
// is a human-readable fallback. Generic ACP agents may use top-level name/tool.
func extractToolName(rawUpdate, rawContent json.RawMessage) string {
	if len(rawUpdate) > 0 {
		var u struct {
			Meta struct {
				ClaudeCode struct {
					ToolName string `json:"toolName"`
				} `json:"claudeCode"`
			} `json:"_meta"`
			Title string `json:"title"`
			Name  string `json:"name"`
			Tool  string `json:"tool"`
		}
		if json.Unmarshal(rawUpdate, &u) == nil {
			if u.Meta.ClaudeCode.ToolName != "" {
				return u.Meta.ClaudeCode.ToolName
			}
			if u.Name != "" {
				return u.Name
			}
			if u.Tool != "" {
				return u.Tool
			}
			if u.Title != "" {
				return u.Title
			}
		}
	}

	return ""
}

// extractToolInput pulls the tool input/arguments from the update object.
// Claude Code agents use rawInput; generic ACP agents may use input.
// Returns a compact summary string, or "" if no meaningful input found.
func extractToolInput(rawUpdate, rawContent json.RawMessage) string {
	if len(rawUpdate) > 0 {
		var u struct {
			RawInput json.RawMessage `json:"rawInput"`
			Input    json.RawMessage `json:"input"`
		}
		if json.Unmarshal(rawUpdate, &u) == nil {
			// Prefer rawInput (Claude Code), fall back to input (generic ACP).
			for _, raw := range []json.RawMessage{u.RawInput, u.Input} {
				if len(raw) > 0 && string(raw) != "{}" && string(raw) != "null" {
					return summarizeJSON(raw)
				}
			}
		}
	}

	return ""
}

// extractToolOutput pulls tool result text from the update object.
// Claude Code agents use rawOutput (string or JSON); generic agents may use output.
func extractToolOutput(rawUpdate, rawContent json.RawMessage) string {
	if len(rawUpdate) > 0 {
		var u struct {
			RawOutput json.RawMessage `json:"rawOutput"`
			Output    json.RawMessage `json:"output"`
			Status    string          `json:"status"`
		}
		if json.Unmarshal(rawUpdate, &u) == nil {
			// Only extract output when the tool is done.
			if u.Status != "completed" && u.Status != "failed" {
				return ""
			}
			// Prefer rawOutput (Claude Code), fall back to output (generic).
			for _, raw := range []json.RawMessage{u.RawOutput, u.Output} {
				if len(raw) == 0 || string(raw) == "null" {
					continue
				}
				// rawOutput can be a JSON string or structured object.
				var s string
				if json.Unmarshal(raw, &s) == nil {
					return s
				}
				return string(raw)
			}
		}
	}

	return ""
}

// summarizeJSON returns a compact one-line summary of a JSON input object.
// Shows key=value pairs, truncating long values.
func summarizeJSON(raw json.RawMessage) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		// Not an object — try to unquote if it's a JSON string, otherwise show raw.
		var str string
		if json.Unmarshal(raw, &str) == nil {
			if len(str) > 80 {
				str = str[:80] + "..."
			}
			return str
		}
		s := strings.TrimSpace(string(raw))
		if len(s) > 80 {
			s = s[:80] + "..."
		}
		return s
	}

	var parts []string
	for k, v := range m {
		val := strings.TrimSpace(string(v))
		// Unquote simple strings.
		var str string
		if json.Unmarshal(v, &str) == nil {
			val = str
		}
		if len(val) > 60 {
			val = val[:60] + "..."
		}
		parts = append(parts, k+"="+val)
	}
	return strings.Join(parts, ", ")
}

// maxToolOutputLines is the maximum number of lines shown for tool results.
const maxToolOutputLines = 5

// capLines truncates text to at most n lines, appending "..." if truncated.
func capLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}

// dim wraps text in ANSI dim escape codes.
const dimStart = "\033[2m"
const dimEnd = "\033[0m"

// writeStream writes data to the stream writer and tracks trailing newlines.
// Caller must hold s.mu.
func (s *Session) writeStream(data string) {
	if data == "" || s.streamWriter == nil {
		return
	}
	_, _ = fmt.Fprint(s.streamWriter, data)
	// Count trailing newlines.
	n := 0
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == '\n' {
			n++
		} else {
			break
		}
	}
	if n == len(data) {
		// Data is entirely newlines — add to running count.
		s.trailingNewlines += n
	} else {
		s.trailingNewlines = n
	}
}

// ensureBlankLine adds just enough newlines to produce one blank line in the
// stream. Caller must hold s.mu.
func (s *Session) ensureBlankLine() {
	needed := 2 - s.trailingNewlines // 2 = end current line + 1 blank
	if needed <= 0 {
		return
	}
	s.writeStream(strings.Repeat("\n", needed))
}


// handleRequest routes incoming JSON-RPC requests from the agent subprocess.
func (s *Session) handleRequest(ctx context.Context, method string, params json.RawMessage) (any, error) {
	slog.Debug("agent: handling request from agent", "method", method)

	switch method {
	case "session/request_permission":
		return s.handlePermission(ctx, params)
	case "fs/read_text_file", "fs/write_text_file", "terminal/run":
		if s.fsHandler != nil {
			return s.fsHandler.HandleRequest(ctx, method, params)
		}
		return nil, fmt.Errorf("agent: no filesystem handler configured")
	default:
		slog.Debug("agent: unknown method from agent", "method", method, "params", string(params))
		return nil, fmt.Errorf("agent: unknown method: %s", method)
	}
}

// handlePermission handles permission requests from the agent.
// In review mode, we auto-approve the first "allow" option.
func (s *Session) handlePermission(ctx context.Context, params json.RawMessage) (any, error) {
	var req struct {
		Options []struct {
			Kind     string `json:"kind"`
			Name     string `json:"name"`
			OptionID string `json:"optionId"`
		} `json:"options"`
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("agent: parsing permission request: %w", err)
	}

	slog.Debug("agent: permission request", "raw", string(params))
	for i, opt := range req.Options {
		slog.Debug("agent: permission option", "index", i, "kind", opt.Kind, "name", opt.Name, "optionId", opt.OptionID)
	}

	// Auto-approve: select the first allow-like option.
	// Claude Code sends "allow_always" and "allow_once"; other agents may
	// use "allow" or "always_allow". Accept any of these.
	optionID := ""
	for _, opt := range req.Options {
		switch opt.Kind {
		case "allow_always", "always_allow", "allow_once", "allow":
			optionID = opt.OptionID
			slog.Debug("agent: auto-approving permission", "kind", opt.Kind, "name", opt.Name, "optionId", opt.OptionID)
		}
		if optionID != "" {
			break
		}
	}

	if optionID == "" {
		slog.Warn("agent: no allow/always_allow option found, cancelling permission request", "options_count", len(req.Options))
		return map[string]any{
			"outcome": map[string]string{
				"outcome": "cancelled",
			},
		}, nil
	}

	return map[string]any{
		"outcome": map[string]any{
			"outcome":  "selected",
			"optionId": optionID,
		},
	}, nil
}

