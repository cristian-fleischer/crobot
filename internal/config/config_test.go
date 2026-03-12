package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

// testdataPath returns the absolute path to a file in the testdata directory.
func testdataPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

// noEnv is a lookup function that always returns ("", false).
func noEnv(_ string) (string, bool) { return "", false }

// envMap returns a lookup function backed by the provided map.
func envMap(m map[string]string) EnvLookupFunc {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

func TestDefaults(t *testing.T) {
	t.Parallel()

	cfg := Defaults()

	if cfg.Platform != "bitbucket" {
		t.Errorf("Platform = %q, want %q", cfg.Platform, "bitbucket")
	}
	if cfg.Review.MaxComments != 25 {
		t.Errorf("MaxComments = %d, want %d", cfg.Review.MaxComments, 25)
	}
	if cfg.Review.DryRun != true {
		t.Errorf("DryRun = %v, want true", cfg.Review.DryRun)
	}
	if cfg.Review.BotLabel != "crobot" {
		t.Errorf("BotLabel = %q, want %q", cfg.Review.BotLabel, "crobot")
	}
	if cfg.Review.SeverityThreshold != "warning" {
		t.Errorf("SeverityThreshold = %q, want %q", cfg.Review.SeverityThreshold, "warning")
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		globalPath string
		localPath  string
		env        map[string]string
		check      func(t *testing.T, cfg Config)
		wantErr    bool
	}{
		{
			name:       "defaults only with no files and no env",
			globalPath: "",
			localPath:  "",
			env:        nil,
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.Platform != "bitbucket" {
					t.Errorf("Platform = %q, want %q", cfg.Platform, "bitbucket")
				}
				if cfg.Review.MaxComments != 25 {
					t.Errorf("MaxComments = %d, want %d", cfg.Review.MaxComments, 25)
				}
				if cfg.Review.DryRun != true {
					t.Errorf("DryRun = %v, want true", cfg.Review.DryRun)
				}
				if cfg.Review.BotLabel != "crobot" {
					t.Errorf("BotLabel = %q, want %q", cfg.Review.BotLabel, "crobot")
				}
				if cfg.Review.SeverityThreshold != "warning" {
					t.Errorf("SeverityThreshold = %q, want %q", cfg.Review.SeverityThreshold, "warning")
				}
			},
		},
		{
			name:       "YAML file overrides defaults",
			globalPath: "full.yaml",
			localPath:  "",
			env:        nil,
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.Platform != "github" {
					t.Errorf("Platform = %q, want %q", cfg.Platform, "github")
				}
				if cfg.Bitbucket.Workspace != "testteam" {
					t.Errorf("Workspace = %q, want %q", cfg.Bitbucket.Workspace, "testteam")
				}
				if cfg.Bitbucket.Repo != "test-repo" {
					t.Errorf("Repo = %q, want %q", cfg.Bitbucket.Repo, "test-repo")
				}
				if cfg.Review.MaxComments != 50 {
					t.Errorf("MaxComments = %d, want %d", cfg.Review.MaxComments, 50)
				}
				if cfg.Review.DryRun != false {
					t.Errorf("DryRun = %v, want false", cfg.Review.DryRun)
				}
				if cfg.Review.BotLabel != "mybot" {
					t.Errorf("BotLabel = %q, want %q", cfg.Review.BotLabel, "mybot")
				}
				if cfg.Review.SeverityThreshold != "error" {
					t.Errorf("SeverityThreshold = %q, want %q", cfg.Review.SeverityThreshold, "error")
				}
			},
		},
		{
			name:       "env var overlay overrides file values",
			globalPath: "full.yaml",
			localPath:  "",
			env: map[string]string{
				"CROBOT_PLATFORM":     "gitlab",
				"CROBOT_MAX_COMMENTS": "100",
				"CROBOT_DRY_RUN":      "true",
			},
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.Platform != "gitlab" {
					t.Errorf("Platform = %q, want %q", cfg.Platform, "gitlab")
				}
				if cfg.Review.MaxComments != 100 {
					t.Errorf("MaxComments = %d, want %d", cfg.Review.MaxComments, 100)
				}
				if cfg.Review.DryRun != true {
					t.Errorf("DryRun = %v, want true", cfg.Review.DryRun)
				}
				// File values that env didn't override should persist.
				if cfg.Review.BotLabel != "mybot" {
					t.Errorf("BotLabel = %q, want %q", cfg.Review.BotLabel, "mybot")
				}
			},
		},
		{
			name:       "layering: defaults < global < local",
			globalPath: "global.yaml",
			localPath:  "local.yaml",
			env:        nil,
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				// Platform from global, not overridden by local.
				if cfg.Platform != "gitlab" {
					t.Errorf("Platform = %q, want %q", cfg.Platform, "gitlab")
				}
				// MaxComments: default=25, global=30, local=15 → local wins.
				if cfg.Review.MaxComments != 15 {
					t.Errorf("MaxComments = %d, want %d", cfg.Review.MaxComments, 15)
				}
				// BotLabel: default=crobot, global=globalbot, local=localbot → local wins.
				if cfg.Review.BotLabel != "localbot" {
					t.Errorf("BotLabel = %q, want %q", cfg.Review.BotLabel, "localbot")
				}
				// DryRun: only in defaults → default true.
				if cfg.Review.DryRun != true {
					t.Errorf("DryRun = %v, want true", cfg.Review.DryRun)
				}
				// SeverityThreshold: only in defaults → default "warning".
				if cfg.Review.SeverityThreshold != "warning" {
					t.Errorf("SeverityThreshold = %q, want %q", cfg.Review.SeverityThreshold, "warning")
				}
			},
		},
		{
			name:       "layering: defaults < global < local < env",
			globalPath: "global.yaml",
			localPath:  "local.yaml",
			env: map[string]string{
				"CROBOT_PLATFORM":     "bitbucket",
				"CROBOT_MAX_COMMENTS": "99",
			},
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				// Env overrides global's gitlab.
				if cfg.Platform != "bitbucket" {
					t.Errorf("Platform = %q, want %q", cfg.Platform, "bitbucket")
				}
				// Env overrides local's 15.
				if cfg.Review.MaxComments != 99 {
					t.Errorf("MaxComments = %d, want %d", cfg.Review.MaxComments, 99)
				}
				// BotLabel still from local.
				if cfg.Review.BotLabel != "localbot" {
					t.Errorf("BotLabel = %q, want %q", cfg.Review.BotLabel, "localbot")
				}
			},
		},
		{
			name:       "missing config file is not an error",
			globalPath: "/nonexistent/path/config.yaml",
			localPath:  "/also/nonexistent/.crobot.yaml",
			env:        nil,
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				// Should get pure defaults.
				if cfg.Platform != "bitbucket" {
					t.Errorf("Platform = %q, want %q", cfg.Platform, "bitbucket")
				}
				if cfg.Review.MaxComments != 25 {
					t.Errorf("MaxComments = %d, want %d", cfg.Review.MaxComments, 25)
				}
			},
		},
		{
			name:       "invalid YAML returns error",
			globalPath: "invalid.yaml",
			localPath:  "",
			env:        nil,
			wantErr:    true,
		},
		{
			name:       "partial config only sets specified fields",
			globalPath: "partial.yaml",
			localPath:  "",
			env:        nil,
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				// Overridden by file.
				if cfg.Review.MaxComments != 10 {
					t.Errorf("MaxComments = %d, want %d", cfg.Review.MaxComments, 10)
				}
				// Defaults preserved.
				if cfg.Platform != "bitbucket" {
					t.Errorf("Platform = %q, want %q", cfg.Platform, "bitbucket")
				}
				if cfg.Review.DryRun != true {
					t.Errorf("DryRun = %v, want true", cfg.Review.DryRun)
				}
				if cfg.Review.BotLabel != "crobot" {
					t.Errorf("BotLabel = %q, want %q", cfg.Review.BotLabel, "crobot")
				}
				if cfg.Review.SeverityThreshold != "warning" {
					t.Errorf("SeverityThreshold = %q, want %q", cfg.Review.SeverityThreshold, "warning")
				}
			},
		},
		{
			name:       "complete YAML with Phase 3/4 fields",
			globalPath: "complete.yaml",
			localPath:  "",
			env:        nil,
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.Bitbucket.Workspace != "myteam" {
					t.Errorf("Workspace = %q, want %q", cfg.Bitbucket.Workspace, "myteam")
				}
				if cfg.Bitbucket.Repo != "my-service" {
					t.Errorf("Repo = %q, want %q", cfg.Bitbucket.Repo, "my-service")
				}
				if cfg.Agent.Default != "claude" {
					t.Errorf("Agent.Default = %q, want %q", cfg.Agent.Default, "claude")
				}
				if cfg.Agent.Timeout != 300 {
					t.Errorf("Agent.Timeout = %d, want %d", cfg.Agent.Timeout, 300)
				}
				agent, ok := cfg.Agent.Agents["claude"]
				if !ok {
					t.Fatal("Agent.Agents missing 'claude' entry")
				}
				if agent.Command != "claude" {
					t.Errorf("Agent.Command = %q, want %q", agent.Command, "claude")
				}
				if len(agent.Args) != 2 || agent.Args[0] != "--model" || agent.Args[1] != "sonnet-4" {
					t.Errorf("Agent.Args = %v, want [--model sonnet-4]", agent.Args)
				}
				if cfg.AI.DefaultProvider != "anthropic" {
					t.Errorf("AI.DefaultProvider = %q, want %q", cfg.AI.DefaultProvider, "anthropic")
				}
				if cfg.AI.MaxTokens != 8192 {
					t.Errorf("AI.MaxTokens = %d, want %d", cfg.AI.MaxTokens, 8192)
				}
				if cfg.AI.Temperature != 0.2 {
					t.Errorf("AI.Temperature = %f, want %f", cfg.AI.Temperature, 0.2)
				}
				prov, ok := cfg.AI.Providers["anthropic"]
				if !ok {
					t.Fatal("AI.Providers missing 'anthropic' entry")
				}
				if prov.Model != "claude-sonnet-4-20250514" {
					t.Errorf("Provider.Model = %q, want %q", prov.Model, "claude-sonnet-4-20250514")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Resolve testdata paths for non-empty, non-absolute paths.
			globalPath := tt.globalPath
			if globalPath != "" && !filepath.IsAbs(globalPath) {
				globalPath = testdataPath(t, globalPath)
			}
			localPath := tt.localPath
			if localPath != "" && !filepath.IsAbs(localPath) {
				localPath = testdataPath(t, localPath)
			}

			lookup := noEnv
			if tt.env != nil {
				lookup = envMap(tt.env)
			}

			cfg, err := Load(globalPath, localPath, lookup)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestEnvVarParsing(t *testing.T) {
	t.Parallel()

	t.Run("bool parsing", func(t *testing.T) {
		t.Parallel()

		boolTests := []struct {
			input string
			want  bool
		}{
			{"true", true},
			{"True", true},
			{"TRUE", true},
			{"1", true},
			{"yes", true},
			{"Yes", true},
			{"false", false},
			{"False", false},
			{"0", false},
			{"no", false},
			{"", false},
			{"garbage", false},
		}

		for _, bt := range boolTests {
			t.Run(bt.input, func(t *testing.T) {
				t.Parallel()

				env := map[string]string{"CROBOT_DRY_RUN": bt.input}
				cfg, err := Load("", "", envMap(env))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.Review.DryRun != bt.want {
					t.Errorf("DryRun for %q = %v, want %v", bt.input, cfg.Review.DryRun, bt.want)
				}
			})
		}
	})

	t.Run("int parsing", func(t *testing.T) {
		t.Parallel()

		intTests := []struct {
			input string
			want  int
		}{
			{"0", 0},
			{"1", 1},
			{"100", 100},
			{"notanumber", 25}, // invalid → keeps default
		}

		for _, it := range intTests {
			t.Run(it.input, func(t *testing.T) {
				t.Parallel()

				env := map[string]string{"CROBOT_MAX_COMMENTS": it.input}
				cfg, err := Load("", "", envMap(env))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.Review.MaxComments != it.want {
					t.Errorf("MaxComments for %q = %d, want %d", it.input, cfg.Review.MaxComments, it.want)
				}
			})
		}
	})

	t.Run("bitbucket credentials from env", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"CROBOT_BITBUCKET_USER":  "alice@example.com",
			"CROBOT_BITBUCKET_TOKEN": "secret-token-123",
		}
		cfg, err := Load("", "", envMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Bitbucket.User != "alice@example.com" {
			t.Errorf("User = %q, want %q", cfg.Bitbucket.User, "alice@example.com")
		}
		if cfg.Bitbucket.Token != "secret-token-123" {
			t.Errorf("Token = %q, want %q", cfg.Bitbucket.Token, "secret-token-123")
		}
	})

	t.Run("bitbucket workspace and repo from env", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"CROBOT_BITBUCKET_WORKSPACE": "env-workspace",
			"CROBOT_BITBUCKET_REPO":      "env-repo",
		}
		cfg, err := Load("", "", envMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Bitbucket.Workspace != "env-workspace" {
			t.Errorf("Workspace = %q, want %q", cfg.Bitbucket.Workspace, "env-workspace")
		}
		if cfg.Bitbucket.Repo != "env-repo" {
			t.Errorf("Repo = %q, want %q", cfg.Bitbucket.Repo, "env-repo")
		}
	})

	t.Run("bitbucket workspace and repo env override file", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"CROBOT_BITBUCKET_WORKSPACE": "override-ws",
			"CROBOT_BITBUCKET_REPO":      "override-repo",
		}
		cfg, err := Load(testdataPath(t, "full.yaml"), "", envMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Bitbucket.Workspace != "override-ws" {
			t.Errorf("Workspace = %q, want %q", cfg.Bitbucket.Workspace, "override-ws")
		}
		if cfg.Bitbucket.Repo != "override-repo" {
			t.Errorf("Repo = %q, want %q", cfg.Bitbucket.Repo, "override-repo")
		}
	})

	t.Run("agent and provider from env", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"CROBOT_AGENT":       "codex",
			"CROBOT_AI_PROVIDER": "openai",
		}
		cfg, err := Load("", "", envMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Agent.Default != "codex" {
			t.Errorf("Agent.Default = %q, want %q", cfg.Agent.Default, "codex")
		}
		if cfg.AI.DefaultProvider != "openai" {
			t.Errorf("AI.DefaultProvider = %q, want %q", cfg.AI.DefaultProvider, "openai")
		}
	})

	t.Run("API keys from env", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"CROBOT_ANTHROPIC_API_KEY":  "sk-ant-123",
			"CROBOT_OPENAI_API_KEY":     "sk-oai-456",
			"CROBOT_GOOGLE_API_KEY":     "goog-789",
			"CROBOT_OPENROUTER_API_KEY": "or-abc",
		}
		cfg, err := Load("", "", envMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.AI.Providers["anthropic"].APIKey != "sk-ant-123" {
			t.Errorf("anthropic APIKey = %q, want %q", cfg.AI.Providers["anthropic"].APIKey, "sk-ant-123")
		}
		if cfg.AI.Providers["openai"].APIKey != "sk-oai-456" {
			t.Errorf("openai APIKey = %q, want %q", cfg.AI.Providers["openai"].APIKey, "sk-oai-456")
		}
		if cfg.AI.Providers["google"].APIKey != "goog-789" {
			t.Errorf("google APIKey = %q, want %q", cfg.AI.Providers["google"].APIKey, "goog-789")
		}
		if cfg.AI.Providers["openrouter"].APIKey != "or-abc" {
			t.Errorf("openrouter APIKey = %q, want %q", cfg.AI.Providers["openrouter"].APIKey, "or-abc")
		}
	})
}

func TestLoadNilEnvLookup(t *testing.T) {
	t.Parallel()

	cfg, err := Load("", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should get pure defaults without panic.
	if cfg.Platform != "bitbucket" {
		t.Errorf("Platform = %q, want %q", cfg.Platform, "bitbucket")
	}
}
