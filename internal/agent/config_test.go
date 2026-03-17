package agent

import (
	"testing"
	"time"

	"github.com/cristian-fleischer/crobot/internal/config"
)

func TestResolveAgentConfig_NamedAgent(t *testing.T) {
	cfg := config.Config{
		Agent: config.AgentConfig{
			Agents: map[string]config.AgentDef{
				"claude": {Command: "claude", Args: []string{"--model", "opus"}},
			},
		},
	}

	got, err := ResolveAgentConfig(cfg, "claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "claude" {
		t.Errorf("Name = %q, want %q", got.Name, "claude")
	}
	if got.Command != "claude" {
		t.Errorf("Command = %q, want %q", got.Command, "claude")
	}
	if len(got.Args) != 2 || got.Args[0] != "--model" || got.Args[1] != "opus" {
		t.Errorf("Args = %v, want [--model opus]", got.Args)
	}
	if got.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", got.Timeout, DefaultTimeout)
	}
}

func TestResolveAgentConfig_DefaultAgent(t *testing.T) {
	cfg := config.Config{
		Agent: config.AgentConfig{
			Default: "gemini",
			Agents: map[string]config.AgentDef{
				"gemini": {Command: "gemini-cli", Args: []string{"review"}},
			},
		},
	}

	got, err := ResolveAgentConfig(cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "gemini" {
		t.Errorf("Name = %q, want %q", got.Name, "gemini")
	}
	if got.Command != "gemini-cli" {
		t.Errorf("Command = %q, want %q", got.Command, "gemini-cli")
	}
}

func TestResolveAgentConfig_AgentNotFound(t *testing.T) {
	cfg := config.Config{
		Agent: config.AgentConfig{
			Agents: map[string]config.AgentDef{
				"claude": {Command: "claude"},
			},
		},
	}

	_, err := ResolveAgentConfig(cfg, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := `agent "nonexistent" not found in configuration`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestResolveAgentConfig_CustomTimeout(t *testing.T) {
	cfg := config.Config{
		Agent: config.AgentConfig{
			Timeout: 60,
			Agents: map[string]config.AgentDef{
				"fast": {Command: "fast-agent"},
			},
		},
	}

	got, err := ResolveAgentConfig(cfg, "fast")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", got.Timeout, 60*time.Second)
	}
}

func TestResolveAgentConfig_EmptyConfig(t *testing.T) {
	cfg := config.Config{}

	_, err := ResolveAgentConfig(cfg, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "no agent specified and no default agent configured"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestResolveAgentConfig_EmptyNameNoDefault(t *testing.T) {
	cfg := config.Config{
		Agent: config.AgentConfig{
			Agents: map[string]config.AgentDef{
				"claude": {Command: "claude"},
			},
		},
	}

	_, err := ResolveAgentConfig(cfg, "")
	if err == nil {
		t.Fatal("expected error for empty name with no default")
	}
}
