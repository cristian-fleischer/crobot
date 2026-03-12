// Package config provides centralized, layered configuration for crobot.
//
// Configuration is resolved in order: defaults < config file < env vars < CLI flags.
// The Load function handles the first three layers; CLI flag overrides are applied
// by the caller after Load returns.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for crobot.
type Config struct {
	// Platform is the SCM platform to use (bitbucket, github, gitlab).
	Platform string `yaml:"platform"`

	// Bitbucket holds Bitbucket-specific settings.
	Bitbucket BitbucketConfig `yaml:"bitbucket"`

	// Review holds code-review behaviour settings.
	Review ReviewConfig `yaml:"review"`

	// Agent holds agent-runner settings (Phase 3).
	Agent AgentConfig `yaml:"agent"`

	// AI holds AI provider settings (Phase 4).
	AI AIConfig `yaml:"ai"`
}

// BitbucketConfig holds Bitbucket-specific connection settings.
type BitbucketConfig struct {
	// Workspace is the Bitbucket workspace/team slug.
	Workspace string `yaml:"workspace"`

	// Repo is the default Bitbucket repository slug.
	Repo string `yaml:"repo"`

	// User is the Bitbucket username or email for API authentication.
	User string `yaml:"user"`

	// Token is the Bitbucket API token for authentication.
	Token string `yaml:"token"`
}

// ReviewConfig holds settings that control code-review behaviour.
type ReviewConfig struct {
	// MaxComments is the maximum number of review comments per run.
	MaxComments int `yaml:"max_comments"`

	// DryRun controls whether comments are actually posted.
	DryRun bool `yaml:"dry_run"`

	// BotLabel is the label used to identify bot-generated comments.
	BotLabel string `yaml:"bot_label"`

	// SeverityThreshold is the minimum severity level to report (e.g. "warning", "error").
	SeverityThreshold string `yaml:"severity_threshold"`
}

// AgentConfig holds agent-runner settings (Phase 3).
type AgentConfig struct {
	// Default is the name of the default agent to use.
	Default string `yaml:"default"`

	// Agents maps agent names to their definitions.
	Agents map[string]AgentDef `yaml:"agents"`

	// Timeout is the maximum agent execution time in seconds.
	Timeout int `yaml:"timeout"`
}

// AgentDef defines a single agent's command and arguments.
type AgentDef struct {
	// Command is the executable to run.
	Command string `yaml:"command"`

	// Args is the list of arguments passed to the command.
	Args []string `yaml:"args"`
}

// AIConfig holds AI provider settings (Phase 4).
type AIConfig struct {
	// DefaultProvider is the name of the default AI provider.
	DefaultProvider string `yaml:"default_provider"`

	// Providers maps provider names to their definitions.
	Providers map[string]ProviderDef `yaml:"providers"`

	// MaxTokens is the maximum number of tokens for AI responses.
	MaxTokens int `yaml:"max_tokens"`

	// Temperature controls randomness in AI responses (0.0–1.0).
	Temperature float64 `yaml:"temperature"`
}

// ProviderDef defines a single AI provider's model and credentials.
type ProviderDef struct {
	// Model is the model identifier (e.g. "claude-sonnet-4-20250514").
	Model string `yaml:"model"`

	// APIKey is the API key for this provider. It is populated from environment
	// variables only (yaml:"-" prevents accidental serialization of secrets).
	APIKey string `yaml:"-"`
}

// EnvLookupFunc is the signature for an environment variable lookup function.
// It mirrors os.LookupEnv.
type EnvLookupFunc func(key string) (string, bool)

// Defaults returns a Config populated with the built-in default values.
func Defaults() Config {
	return Config{
		Platform: "bitbucket",
		Review: ReviewConfig{
			MaxComments:       25,
			DryRun:            true,
			BotLabel:          "crobot",
			SeverityThreshold: "warning",
		},
	}
}

// Load resolves configuration by layering defaults, config files, and
// environment variables. Config files are read from globalPath and localPath;
// missing files are silently ignored. Environment variables are looked up
// using the provided lookupEnv function.
//
// The returned Config is ready for CLI flag overrides to be applied on top.
func Load(globalPath, localPath string, lookupEnv EnvLookupFunc) (Config, error) {
	cfg := Defaults()

	// Layer config files: global first, then local (local wins).
	for _, path := range []string{globalPath, localPath} {
		if path == "" {
			continue
		}
		if err := loadFile(path, &cfg); err != nil {
			return Config{}, fmt.Errorf("loading config %s: %w", path, err)
		}
	}

	// Layer environment variables on top.
	applyEnv(&cfg, lookupEnv)

	return cfg, nil
}

// LoadDefault is a convenience wrapper around Load that uses the standard
// config file locations (~/.config/crobot/config.yaml and .crobot.yaml)
// and os.LookupEnv.
func LoadDefault() (Config, error) {
	globalPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		globalPath = filepath.Join(home, ".config", "crobot", "config.yaml")
	}
	return Load(globalPath, ".crobot.yaml", os.LookupEnv)
}

// loadFile reads a YAML config file at path and merges it into cfg.
// If the file does not exist, it is silently ignored.
func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("reading file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}
	return nil
}

// applyEnv overlays environment variables onto cfg using the provided lookup function.
func applyEnv(cfg *Config, lookupEnv EnvLookupFunc) {
	if lookupEnv == nil {
		return
	}

	if v, ok := lookupEnv("CROBOT_PLATFORM"); ok {
		cfg.Platform = v
	}
	if v, ok := lookupEnv("CROBOT_BITBUCKET_WORKSPACE"); ok {
		cfg.Bitbucket.Workspace = v
	}
	if v, ok := lookupEnv("CROBOT_BITBUCKET_REPO"); ok {
		cfg.Bitbucket.Repo = v
	}
	if v, ok := lookupEnv("CROBOT_BITBUCKET_USER"); ok {
		cfg.Bitbucket.User = v
	}
	if v, ok := lookupEnv("CROBOT_BITBUCKET_TOKEN"); ok {
		cfg.Bitbucket.Token = v
	}
	if v, ok := lookupEnv("CROBOT_MAX_COMMENTS"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Review.MaxComments = n
		}
	}
	if v, ok := lookupEnv("CROBOT_DRY_RUN"); ok {
		cfg.Review.DryRun = parseBool(v)
	}

	// Phase 3 env vars.
	if v, ok := lookupEnv("CROBOT_AGENT"); ok {
		cfg.Agent.Default = v
	}

	// Phase 4 env vars.
	if v, ok := lookupEnv("CROBOT_AI_PROVIDER"); ok {
		cfg.AI.DefaultProvider = v
	}

	// API key env vars (Phase 4).
	apiKeyEnvVars := map[string]string{
		"CROBOT_ANTHROPIC_API_KEY":  "anthropic",
		"CROBOT_OPENAI_API_KEY":     "openai",
		"CROBOT_GOOGLE_API_KEY":     "google",
		"CROBOT_OPENROUTER_API_KEY": "openrouter",
	}
	for envVar, providerName := range apiKeyEnvVars {
		if v, ok := lookupEnv(envVar); ok {
			if cfg.AI.Providers == nil {
				cfg.AI.Providers = make(map[string]ProviderDef)
			}
			p := cfg.AI.Providers[providerName]
			p.APIKey = v
			cfg.AI.Providers[providerName] = p
		}
	}
}

// parseBool interprets common boolean string representations.
// It returns true for "true", "1", "yes" (case-insensitive), false otherwise.
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}
