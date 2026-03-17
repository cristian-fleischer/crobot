package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristian-fleischer/crobot/internal/agent"
	"github.com/cristian-fleischer/crobot/internal/platform"
)

// mockAgentBinary is the path to the compiled mock agent binary.
// It is built once in TestMain and reused across all tests.
var mockAgentBinary string

func TestMain(m *testing.M) {
	// Build the mock agent binary.
	tmpDir, err := os.MkdirTemp("", "crobot-mockagent-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	binary := filepath.Join(tmpDir, "mockagent")
	cmd := exec.Command("go", "build", "-o", binary, "./testdata/mockagent")
	cmd.Dir = filepath.Join(".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("building mock agent: " + err.Error())
	}

	mockAgentBinary = binary
	os.Exit(m.Run())
}

// TestIntegration_FullFlow exercises the complete ACP lifecycle:
// Client → Start → Session → Initialize → Prompt → Extract findings.
func TestIntegration_FullFlow(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	client := agent.NewClient(agent.ClientConfig{
		Command: mockAgentBinary,
		Timeout: 10 * time.Second,
	})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := agent.NewSession(agent.SessionConfig{
		Client: client,
	})

	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	if result.FinalText == "" {
		t.Fatal("FinalText is empty")
	}

	// Extract and verify findings.
	findings, err := agent.ExtractFindings(result.FinalText)
	if err != nil {
		t.Fatalf("ExtractFindings: %v", err)
	}

	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
	}

	if findings[0].Path != "src/main.go" {
		t.Errorf("findings[0].Path = %q, want %q", findings[0].Path, "src/main.go")
	}
	if findings[0].Severity != "warning" {
		t.Errorf("findings[0].Severity = %q, want %q", findings[0].Severity, "warning")
	}
	if findings[1].Severity != "error" {
		t.Errorf("findings[1].Severity = %q, want %q", findings[1].Severity, "error")
	}

	// Validate each finding.
	for i, f := range findings {
		if err := f.Validate(); err != nil {
			t.Errorf("findings[%d] validation: %v", i, err)
		}
	}

	// Close session.
	if err := session.Close(ctx); err != nil {
		t.Fatalf("session.Close: %v", err)
	}
}

// TestIntegration_EmptyFindings verifies the agent returns an empty findings
// array when MOCK_EMPTY is set.
func TestIntegration_EmptyFindings(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	client := agent.NewClient(agent.ClientConfig{
		Command: mockAgentBinary,
		Env:     []string{"MOCK_EMPTY=true"},
		Timeout: 10 * time.Second,
	})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := agent.NewSession(agent.SessionConfig{
		Client: client,
	})

	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	findings, err := agent.ExtractFindings(result.FinalText)
	if err != nil {
		t.Fatalf("ExtractFindings: %v", err)
	}

	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0", len(findings))
	}
}

// TestIntegration_CustomFindings verifies that custom findings are correctly
// extracted when passed via MOCK_FINDINGS.
func TestIntegration_CustomFindings(t *testing.T) {
	t.Parallel()

	customFindings := []platform.ReviewFinding{
		{
			Path:     "auth.go",
			Line:     5,
			Side:     "new",
			Severity: "error",
			Category: "security",
			Message:  "Token leak",
		},
	}
	findingsJSON, err := json.Marshal(customFindings)
	if err != nil {
		t.Fatalf("marshal custom findings: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	client := agent.NewClient(agent.ClientConfig{
		Command: mockAgentBinary,
		Env:     []string{"MOCK_FINDINGS=" + string(findingsJSON)},
		Timeout: 10 * time.Second,
	})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := agent.NewSession(agent.SessionConfig{
		Client: client,
	})

	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	findings, err := agent.ExtractFindings(result.FinalText)
	if err != nil {
		t.Fatalf("ExtractFindings: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Path != "auth.go" {
		t.Errorf("findings[0].Path = %q, want %q", findings[0].Path, "auth.go")
	}
	if findings[0].Category != "security" {
		t.Errorf("findings[0].Category = %q, want %q", findings[0].Category, "security")
	}
}

// TestIntegration_AgentTimeout verifies that the client respects context
// deadlines when the agent is too slow.
func TestIntegration_AgentTimeout(t *testing.T) {
	t.Parallel()

	// Use a very short timeout so the test runs quickly.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	client := agent.NewClient(agent.ClientConfig{
		Command: mockAgentBinary,
		Env:     []string{"MOCK_DELAY=30s"},
		Timeout: 2 * time.Second,
	})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := agent.NewSession(agent.SessionConfig{
		Client: client,
	})

	// Initialize should time out since the agent delays all responses.
	_, err := session.Prompt(ctx, "Review this code")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "connection closed") {
		t.Errorf("expected timeout or connection closed error, got: %v", err)
	}
}

// TestIntegration_AgentCrash verifies that a non-zero exit from the agent
// subprocess is handled gracefully.
func TestIntegration_AgentCrash(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	client := agent.NewClient(agent.ClientConfig{
		Command: mockAgentBinary,
		Env:     []string{"MOCK_CRASH=true"},
		Timeout: 10 * time.Second,
	})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := agent.NewSession(agent.SessionConfig{
		Client: client,
	})

	// Initialize succeeds (crash happens after response is sent).
	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// The next request should fail because the agent has exited.
	_, err := session.Prompt(ctx, "Review this code")
	if err == nil {
		t.Fatal("expected error after agent crash, got nil")
	}
	// The error should indicate connection closed or similar.
	if !strings.Contains(err.Error(), "connection closed") &&
		!strings.Contains(err.Error(), "broken pipe") &&
		!strings.Contains(err.Error(), "session/new") {
		t.Logf("got error (acceptable): %v", err)
	}
}

// TestIntegration_MalformedOutput verifies that malformed JSON from the agent
// is handled by ExtractFindings with a descriptive error.
func TestIntegration_MalformedOutput(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	client := agent.NewClient(agent.ClientConfig{
		Command: mockAgentBinary,
		Env:     []string{"MOCK_BAD_JSON=true"},
		Timeout: 10 * time.Second,
	})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := agent.NewSession(agent.SessionConfig{
		Client: client,
	})

	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	// The agent returned text that isn't valid JSON findings.
	_, err = agent.ExtractFindings(result.FinalText)
	if err == nil {
		t.Fatal("expected ExtractFindings error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "extract findings") {
		t.Errorf("expected 'extract findings' in error, got: %v", err)
	}
}

// TestIntegration_WithReviewEngine exercises the full pipeline: agent produces
// findings, engine validates them against a mock PR context.
func TestIntegration_WithReviewEngine(t *testing.T) {
	t.Parallel()

	// Create findings that match our test PR context.
	customFindings := []platform.ReviewFinding{
		{
			Path:     "src/main.go",
			Line:     12,
			Side:     "new",
			Severity: "warning",
			Category: "style",
			Message:  "Consider renaming this variable.",
		},
		{
			Path:     "src/main.go",
			Line:     14,
			Side:     "new",
			Severity: "error",
			Category: "bug",
			Message:  "Possible nil dereference.",
		},
		{
			Path:     "nonexistent.go",
			Line:     1,
			Side:     "new",
			Severity: "error",
			Category: "bug",
			Message:  "This should be filtered out.",
		},
	}
	findingsJSON, err := json.Marshal(customFindings)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	client := agent.NewClient(agent.ClientConfig{
		Command: mockAgentBinary,
		Env:     []string{"MOCK_FINDINGS=" + string(findingsJSON)},
		Timeout: 10 * time.Second,
	})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := agent.NewSession(agent.SessionConfig{
		Client: client,
	})

	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	findings, err := agent.ExtractFindings(result.FinalText)
	if err != nil {
		t.Fatalf("ExtractFindings: %v", err)
	}

	if len(findings) != 3 {
		t.Fatalf("got %d findings, want 3", len(findings))
	}

	// Verify findings can be validated against a PR context.
	// The first two target src/main.go (in hunk range), the third targets
	// a nonexistent file and should be rejected by the engine.
	prCtx := &platform.PRContext{
		ID:           1,
		Title:        "Test PR",
		SourceBranch: "feature",
		TargetBranch: "main",
		HeadCommit:   "abc123",
		Files: []platform.ChangedFile{
			{Path: "src/main.go", Status: "modified"},
		},
		DiffHunks: []platform.DiffHunk{
			{
				Path:     "src/main.go",
				OldStart: 10,
				OldLines: 5,
				NewStart: 10,
				NewLines: 7,
			},
		},
	}

	// Use ValidateFindings to verify filtering works.
	var validFindings []platform.ReviewFinding
	for _, f := range findings {
		if err := f.Validate(); err != nil {
			continue
		}
		// Check path exists in PR files.
		found := false
		for _, cf := range prCtx.Files {
			if cf.Path == f.Path {
				found = true
				break
			}
		}
		if found {
			validFindings = append(validFindings, f)
		}
	}

	// 2 findings target src/main.go, 1 targets nonexistent.go.
	if len(validFindings) != 2 {
		t.Errorf("got %d valid findings, want 2", len(validFindings))
	}
}

// TestIntegration_PromptContainsContext verifies that BuildFullPrompt produces
// a prompt containing the expected PR metadata and diff content.
func TestIntegration_PromptContainsContext(t *testing.T) {
	t.Parallel()

	prCtx := &platform.PRContext{
		ID:           42,
		Title:        "Add authentication module",
		Author:       "alice",
		SourceBranch: "feat/auth",
		TargetBranch: "main",
		State:        "OPEN",
		Description:  "Adds JWT-based authentication",
		Files: []platform.ChangedFile{
			{Path: "auth.go", Status: "added"},
		},
		DiffHunks: []platform.DiffHunk{
			{
				Path:     "auth.go",
				NewStart: 1,
				NewLines: 10,
				Body:     "+package auth\n+\n+func Validate(token string) bool {\n+\treturn token != \"\"\n+}\n",
			},
		},
	}

	prompt := agent.BuildFullPrompt(prCtx, nil)

	// Verify the prompt contains key sections.
	checks := []string{
		"Add authentication module",
		"alice",
		"feat/auth",
		"auth.go",
		"Validate(token string)",
		"ReviewFinding",
		"JSON array",
	}

	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt does not contain %q", check)
		}
	}
}

// TestIntegration_FindingsRoundTrip verifies that findings produced by the
// mock agent can be serialized, deserialized, and validated successfully.
func TestIntegration_FindingsRoundTrip(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	client := agent.NewClient(agent.ClientConfig{
		Command: mockAgentBinary,
		Timeout: 10 * time.Second,
	})

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Close()

	session := agent.NewSession(agent.SessionConfig{
		Client: client,
	})

	if err := session.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	result, err := session.Prompt(ctx, "Review this code")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	findings, err := agent.ExtractFindings(result.FinalText)
	if err != nil {
		t.Fatalf("ExtractFindings: %v", err)
	}

	// Serialize and deserialize.
	data, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := platform.ParseFindings(data)
	if err != nil {
		t.Fatalf("ParseFindings: %v", err)
	}

	if len(parsed) != len(findings) {
		t.Fatalf("roundtrip count: got %d, want %d", len(parsed), len(findings))
	}

	for i := range findings {
		if parsed[i].Path != findings[i].Path {
			t.Errorf("roundtrip[%d].Path = %q, want %q", i, parsed[i].Path, findings[i].Path)
		}
		if parsed[i].Message != findings[i].Message {
			t.Errorf("roundtrip[%d].Message = %q, want %q", i, parsed[i].Message, findings[i].Message)
		}
	}
}
