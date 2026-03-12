# CRoBot - Code Review Bot

## Overview

CRoBot is a local-first CLI tool written in Go that enables AI-powered automated
code reviews on pull requests, posting real inline review comments.

The tool is built in four progressive phases:

1. **Phase 1 (MVP)**: CLI commands + Bitbucket Cloud + review engine. AI coding
   agents (Claude Code, Codex CLI, OpenCode, etc.) invoke CRoBot via shell
   commands and exchange JSON.
2. **Phase 2 (MCP)**: CRoBot runs as an MCP server (local stdio), exposing its
   commands as tools that MCP-capable agents discover automatically.
3. **Phase 3 (ACP)**: CRoBot becomes the single entry point. It acts as an ACP
   client, spawning and orchestrating an ACP-compatible coding agent subprocess
   to perform the review end-to-end.
4. **Phase 4 (Direct AI Providers)**: CRoBot calls AI provider APIs directly
   (Anthropic, OpenAI, Google, OpenRouter, etc.), removing the need for an
   external coding agent entirely.

### Design Principles

- **Modular & phased**: each phase adds a layer without rewriting previous work.
- **Agent-agnostic**: any CLI-capable agent can use it (P1); any MCP agent (P2);
  any ACP agent (P3); or no agent at all (P4).
- **Platform-agnostic**: a `Platform` interface abstracts Bitbucket, GitHub,
  GitLab, etc.
- **Safe by default**: the model outputs structured findings; CRoBot controls
  what gets written. Dry-run by default; `--write` required to post.
- **Local-first, CI-ready**: same binary works on a developer's machine and in
  a pipeline.
- **Single binary**: Go compiles to one executable; no runtime dependencies.

### Testing Philosophy

An extensive, well-maintained test suite is a **first-class deliverable** — not
an afterthought. Every package must have meaningful test coverage that serves as
both a correctness guarantee and a living specification. The test suite must be
the primary way any implementing agent (human or AI) validates their work.

**Requirements:**

1. **Every package gets tests**. No exceptions. Unit tests for pure logic,
   integration tests for I/O boundaries (HTTP clients, filesystem, subprocess).
2. **Tests must be runnable with a single command**: `go test ./...` from the
   project root. No external services, no manual setup, no API keys needed for
   the default test run.
3. **Test-first validation**: Implementing agents MUST run the full test suite
   (`go test ./...`) after every significant change and before considering any
   task complete. Broken tests are blocking — they must be fixed before moving
   on.
4. **Regression safety**: The test suite must be comprehensive enough that any
   refactoring or addition that breaks existing behavior is caught immediately.
   If a bug is found, a failing test must be written first, then the fix applied.
5. **Test infrastructure**: Use HTTP response recorders / mock servers for
   external API calls (Bitbucket, AI providers). Use golden files for comment
   rendering. Use table-driven tests for validation logic. Store test fixtures
   in `testdata/` directories alongside the packages that use them.
6. **No test pollution**: Tests must not depend on execution order, global state,
   or environment variables (use test helpers to inject config). Tests must be
   parallelizable where possible (`t.Parallel()`).
7. **Integration tests with build tags**: Tests that require real API access or
   external dependencies use `//go:build integration` and are excluded from
   the default `go test` run.

---

## Architecture

The architecture is layered so that each phase extends the system without
modifying lower layers.

```
                      Phase 4                Phase 3               Phase 1
                 +--------------+      +--------------+      +--------------+
                 | Direct AI    |      | ACP Client   |      | External     |
                 | Provider     |      | (CRoBot      |      | AI Agent     |
                 | (Anthropic,  |      |  spawns agent |      | (Claude Code,|
                 |  OpenAI ...) |      |  subprocess)  |      |  Codex, ...) |
                 +------+-------+      +------+-------+      +------+-------+
                        |                     |                      |
                        v                     v                      v
               +--------------------------------------------------+
               |              Analysis Layer                       |
               |  internal/analysis/                               |
               |                                                   |
               |  Prompt construction, finding extraction,         |
               |  AI provider abstraction                          |
               +-------------------------+------------------------+
                                         |
                    +--------------------+--------------------+
                    |                                         |
                    v                                         v
+--------------------------------------+    +----------------------------------+
|          Review Engine               |    |       MCP Server (Phase 2)       |
|  internal/review/                    |    |  internal/mcp/                   |
|                                      |    |                                  |
|  validate -> dedupe -> render -> post|    |  Exposes CLI commands as MCP     |
+------------------+-------------------+    |  tools over stdio JSON-RPC       |
                   |                        +----------------------------------+
                   v
+--------------------------------------+
|       Platform Interface             |
|  internal/platform/                  |
|                                      |
|  Platform interface + shared types   |
+------+----------+----------+--------+
       |          |          |
       v          v          v
 +-----------+ +---------+ +---------+
 | Bitbucket | | GitHub  | | GitLab  |
 | Cloud     | | (future)| | (future)|
 +-----------+ +---------+ +---------+

+--------------------------------------+
|       Configuration                  |
|  internal/config/                    |
|                                      |
|  YAML config file + env vars + flags |
+--------------------------------------+
```

---

## Configuration

Configuration is centralized and layered: **defaults < config file < env vars <
CLI flags**. A single config file drives all phases.

```yaml
# ~/.config/crobot/config.yaml (global)
# .crobot.yaml (repo root, per-project override)

# --- Platform ---
platform: bitbucket

bitbucket:
  workspace: myteam
  # user and token come from env vars (CROBOT_BITBUCKET_USER, CROBOT_BITBUCKET_TOKEN)

# github:                    # future
#   owner: myorg

# --- Review ---
review:
  max_comments: 25
  dry_run: true
  bot_label: crobot
  severity_threshold: warning   # skip "info" findings

# --- Agent (Phase 3: ACP) ---
agent:
  default: claude
  agents:
    claude:
      command: claude
      args: ["--model", "sonnet-4"]
    codex:
      command: codex
      args: []
    gemini:
      command: gemini
      args: []
    opencode:
      command: opencode
      args: []
  timeout: 300                  # seconds, default 5 minutes

# --- AI Provider (Phase 4: Direct API) ---
ai:
  default_provider: anthropic
  providers:
    anthropic:
      model: claude-sonnet-4-20250514
      # api_key from env: CROBOT_ANTHROPIC_API_KEY
    openai:
      model: gpt-4.1
      # api_key from env: CROBOT_OPENAI_API_KEY
    google:
      model: gemini-2.5-pro
      # api_key from env: CROBOT_GOOGLE_API_KEY
    openrouter:
      model: anthropic/claude-sonnet-4
      # api_key from env: CROBOT_OPENROUTER_API_KEY
    # Additional providers follow the same pattern
  max_tokens: 8192
  temperature: 0.2
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CROBOT_PLATFORM` | Default platform (bitbucket, github, gitlab) |
| `CROBOT_BITBUCKET_USER` | Bitbucket username/email |
| `CROBOT_BITBUCKET_TOKEN` | Bitbucket API token |
| `CROBOT_MAX_COMMENTS` | Max comments per run |
| `CROBOT_DRY_RUN` | Default dry-run mode (true/false) |
| `CROBOT_AGENT` | Default ACP agent name (Phase 3) |
| `CROBOT_AI_PROVIDER` | Default AI provider (Phase 4) |
| `CROBOT_ANTHROPIC_API_KEY` | Anthropic API key (Phase 4) |
| `CROBOT_OPENAI_API_KEY` | OpenAI API key (Phase 4) |
| `CROBOT_GOOGLE_API_KEY` | Google AI API key (Phase 4) |
| `CROBOT_OPENROUTER_API_KEY` | OpenRouter API key (Phase 4) |

---

## Platform Abstraction

The core abstraction is a `Platform` interface that any git hosting provider must
implement. This layer is shared across all phases.

```go
type Platform interface {
    // GetPRContext returns normalized metadata, changed files, and diff hunks.
    GetPRContext(ctx context.Context, opts PRRequest) (*PRContext, error)

    // GetFileContent returns file content at a specific commit.
    GetFileContent(ctx context.Context, opts FileRequest) ([]byte, error)

    // ListBotComments returns existing comments posted by this bot.
    ListBotComments(ctx context.Context, opts PRRequest) ([]Comment, error)

    // CreateInlineComment posts a single inline comment on a PR.
    CreateInlineComment(ctx context.Context, opts PRRequest, comment InlineComment) (*Comment, error)

    // DeleteComment removes a comment (for cleanup / re-review).
    DeleteComment(ctx context.Context, opts PRRequest, commentID string) error
}
```

### Shared Types

```go
// PRRequest identifies a pull request on any platform.
type PRRequest struct {
    Workspace string // bitbucket workspace / github owner / gitlab namespace
    Repo      string
    PRNumber  int
}

// PRContext is the normalized view of a PR across platforms.
type PRContext struct {
    ID           int
    Title        string
    Description  string
    Author       string
    SourceBranch string
    TargetBranch string
    State        string
    HeadCommit   string
    BaseCommit   string
    Files        []ChangedFile
    DiffHunks    []DiffHunk
}

// ChangedFile represents a single file changed in the PR.
type ChangedFile struct {
    Path    string
    OldPath string // for renames
    Status  string // added, modified, deleted, renamed
}

// DiffHunk represents a contiguous block of changed lines.
type DiffHunk struct {
    Path     string
    OldStart int
    OldLines int
    NewStart int
    NewLines int
    Body     string // raw unified diff text for this hunk
}

// ReviewFinding is the contract between the AI layer and CRoBot.
type ReviewFinding struct {
    Path        string `json:"path"`
    Line        int    `json:"line"`
    Side        string `json:"side"`                  // "new" or "old"
    Severity    string `json:"severity"`              // "info", "warning", "error"
    Category    string `json:"category"`              // e.g. "security", "performance"
    Message     string `json:"message"`
    Suggestion  string `json:"suggestion,omitempty"`   // optional code suggestion
    Fingerprint string `json:"fingerprint"`            // for deduplication
}

// InlineComment is a platform-agnostic inline comment to post.
type InlineComment struct {
    Path        string
    Line        int
    Side        string // "new" or "old"
    Body        string // rendered markdown
    Fingerprint string // hidden in body for dedup
}

// Comment represents an existing comment on a PR.
type Comment struct {
    ID          string
    Path        string
    Line        int
    Body        string
    Author      string
    CreatedAt   string
    IsBot       bool
    Fingerprint string // extracted from body if present
}
```

---

## Bitbucket Cloud Implementation

### Authentication

- **Scoped API Token** (recommended): `CROBOT_BITBUCKET_USER` +
  `CROBOT_BITBUCKET_TOKEN`
- Stored as env vars, never passed through the AI agent.
- Scopes needed: `repository:read`, `pullrequest:read`, `pullrequest:write`

### Key Bitbucket Cloud API Endpoints

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Get PR | GET | `/2.0/repositories/{workspace}/{repo}/pullrequests/{id}` |
| Get PR diff | GET | `/2.0/repositories/{workspace}/{repo}/pullrequests/{id}/diff` |
| Get PR diffstat | GET | `/2.0/repositories/{workspace}/{repo}/pullrequests/{id}/diffstat` |
| List PR comments | GET | `/2.0/repositories/{workspace}/{repo}/pullrequests/{id}/comments` |
| Create PR comment | POST | `/2.0/repositories/{workspace}/{repo}/pullrequests/{id}/comments` |
| Delete PR comment | DELETE | `/2.0/repositories/{workspace}/{repo}/pullrequests/{id}/comments/{comment_id}` |
| Get file content | GET | `/2.0/repositories/{workspace}/{repo}/src/{commit}/{path}` |

### Inline Comment Payload

```json
{
  "content": {
    "raw": "**warning** | security\n\nLogging the raw token can leak credentials.\n\n[//]: # \"crobot:fp=src/auth.ts:new:42:token-log\""
  },
  "inline": {
    "path": "src/auth.ts",
    "to": 42
  }
}
```

- `inline.to` = line in the new version of the file (side = "new")
- `inline.from` = line in the old version (side = "old")
- `[//]: # "crobot:fp=..."` is used for deduplication on reruns.

---

## Review Engine

The review engine (`internal/review/`) is platform-agnostic and shared across
all phases. It operates on the shared types.

### Validation

- Parse `ReviewFinding[]` from input.
- For each finding, verify `path` exists in `PRContext.Files`.
- Verify `line` falls within a `DiffHunk` range for that path.
- Reject findings outside the diff (Bitbucket will reject these anyway).

### Deduplication

- Fetch existing bot comments via `ListBotComments`.
- Extract fingerprints from existing comments (`[//]: # "crobot:fp=..."`).
- Skip any finding whose fingerprint already exists.

### Comment Rendering

- Format each finding into markdown:
  - severity badge + category
  - message body
  - optional code suggestion in a fenced block
  - hidden fingerprint HTML comment

### Rate Limiting & Safety

- Configurable max comments per run (default: 25).
- Configurable delay between API calls.
- Dry-run mode by default; `--write` required to actually post.

---

## Phase 1: CLI Commands (MVP)

The CLI uses `cobra`. Every command returns JSON to stdout and uses stderr for
logs/progress. AI agents invoke these commands via shell.

### `crobot export-pr-context`

Fetches and outputs the full PR context as JSON.

```
crobot export-pr-context \
  --workspace myteam --repo my-service --pr 42
```

### `crobot get-file-snippet`

Returns a slice of a file at a given commit with surrounding context lines.

```
crobot get-file-snippet \
  --workspace myteam --repo my-service \
  --commit abc123 --path src/auth.ts --line 42 --context 10
```

### `crobot list-bot-comments`

Lists existing bot comments on a PR (for dedup and cleanup).

```
crobot list-bot-comments \
  --workspace myteam --repo my-service --pr 42
```

### `crobot apply-review-findings`

Takes `ReviewFinding[]` JSON and posts them as inline PR comments.

```
# Dry run (default)
crobot apply-review-findings \
  --workspace myteam --repo my-service --pr 42 \
  --input findings.json --dry-run

# Write
crobot apply-review-findings \
  --workspace myteam --repo my-service --pr 42 \
  --input findings.json --write
```

Input: `ReviewFinding[]` JSON from file or stdin (`--input -`).
Output: summary JSON with posted/skipped/failed counts and comment IDs.

### Agent Instruction Files

Each agent reads a different file for project-level instructions:

| Agent | Instruction File | Notes |
|-------|-----------------|-------|
| Claude Code | `CLAUDE.md` | Read automatically |
| OpenCode | `OPENCODE.md` | Read automatically |
| Codex CLI | `AGENTS.md` | OpenAI convention |
| Copilot CLI | `AGENTS.md` | Shares OpenAI convention |
| Generic | `.ai/agent-instructions.md` | Fallback / reference copy |

All files contain the same core: available commands, `ReviewFinding` schema,
review workflow, and rules (dry-run first, max comments, no direct API calls).

### CI/CD Integration

```yaml
# Bitbucket Pipelines example
- step:
    name: AI Code Review
    script:
      - export CROBOT_BITBUCKET_USER=$BITBUCKET_USER
      - export CROBOT_BITBUCKET_TOKEN=$BITBUCKET_TOKEN
      - crobot export-pr-context --workspace $WORKSPACE --repo $REPO --pr $PR_ID > context.json
      - claude -p "Review this PR" --input context.json --output findings.json
      - crobot apply-review-findings --workspace $WORKSPACE --repo $REPO --pr $PR_ID --input findings.json --write
```

---

## Phase 2: MCP Server

CRoBot runs as an MCP (Model Context Protocol) server over local stdio,
exposing its CLI commands as tools that MCP-capable agents discover
automatically.

```jsonc
// .mcp.json (Claude Code) or equivalent agent config
{
  "mcpServers": {
    "crobot": {
      "command": "crobot",
      "args": ["serve", "--mcp"],
      "env": {
        "CROBOT_BITBUCKET_USER": "...",
        "CROBOT_BITBUCKET_TOKEN": "..."
      }
    }
  }
}
```

### MCP Tools Exposed

| Tool | Maps to CLI Command |
|------|-------------------|
| `export_pr_context` | `crobot export-pr-context` |
| `get_file_snippet` | `crobot get-file-snippet` |
| `list_bot_comments` | `crobot list-bot-comments` |
| `apply_review_findings` | `crobot apply-review-findings` |

The MCP server (`internal/mcp/`) is a thin adapter over the same internal
functions used by the CLI. It adds no new logic; it only translates MCP tool
calls into the same review engine calls.

### MCP Support by Agent

| Agent | MCP Support | Shell Fallback |
|-------|------------|----------------|
| Claude Code | Yes | Yes |
| OpenCode | Yes | Yes |
| Gemini CLI | Yes | Yes |
| Codex CLI | No | Yes (shell only) |
| Copilot CLI | No | Yes (shell only) |

---

## Phase 3: ACP Orchestrator

CRoBot becomes the single entry point for code review. The user runs one
command; CRoBot handles everything internally.

```
crobot review --pr https://bitbucket.org/team/repo/pull-requests/42
crobot review --workspace team --repo repo --pr 42
crobot review --pr 42 --agent codex
```

### How It Works

CRoBot acts as an **ACP (Agent Client Protocol) client**. ACP
(https://agentclientprotocol.com) standardizes communication between clients
and coding agents over JSON-RPC 2.0 via stdio. The agent runs as a subprocess.

```
crobot review --pr <url>
  1. Parse PR URL -> extract workspace/repo/pr
  2. Fetch PRContext from platform (Bitbucket API)
  3. Spawn ACP agent subprocess (e.g. `claude`, `codex`, `gemini`)
  4. ACP handshake: initialize -> session/new
  5. Send session/prompt with PR diff + review instructions
  6. Receive session/update notifications (streaming agent output)
  7. Parse agent output -> ReviewFinding[] JSON
  8. Feed findings into review engine: validate -> dedupe -> render
  9. Post comments via platform (dry-run or --write)
  10. Kill agent subprocess
```

### ACP Client Design

**Transport**: JSON-RPC 2.0 over stdio (agent is a subprocess).

**Client capabilities (read-only filesystem)**:
- `session/request_permission`: auto-approve reads, auto-deny writes
- `fs/read_text_file`: backed by git checkout at PR's head commit, read-only
- NOT implemented: `fs/write_text_file`, `terminal/*`

This gives the agent the ability to browse the codebase beyond the diff
(useful for understanding context) while preventing any modifications.

**Agent selection** is config-driven (see Configuration section above). CLI
override: `--agent <name>`.

**Error handling**:
- Agent process timeout: configurable (default 5 min)
- JSON parse failure: retry once with clarifying prompt, then fail
- Agent crash: clear error, non-zero exit

### ACP-Compatible Agents

Agents that already support ACP (as of early 2026):
- Claude Agent (via Zed SDK adapter)
- Codex CLI (via Zed adapter)
- GitHub Copilot (public preview)
- Gemini CLI
- OpenCode
- Cline, Cursor, Goose, Kiro CLI, and many more

Full list: https://agentclientprotocol.com/get-started/agents

---

## Phase 4: Direct AI Provider APIs

CRoBot calls AI provider APIs directly, removing the need for any external
coding agent. This makes CRoBot fully self-contained.

```
crobot review --pr 42 --provider anthropic
crobot review --pr 42 --provider openai --model gpt-4.1
crobot review --pr 42 --provider openrouter --model anthropic/claude-sonnet-4
```

### AI Provider Interface

```go
// AIProvider sends a code review prompt and returns structured findings.
type AIProvider interface {
    // Review sends the PR context to the AI model and returns findings.
    Review(ctx context.Context, opts ReviewRequest) ([]ReviewFinding, error)

    // Name returns the provider identifier.
    Name() string
}

type ReviewRequest struct {
    PRContext   *PRContext
    Prompt      string      // system + user prompt
    MaxTokens   int
    Temperature float64
}
```

### Provider Implementations

Each provider is a thin HTTP client that:
1. Constructs the API request (model-specific message format)
2. Sends the PR diff + review instructions as a prompt
3. Parses the response to extract `ReviewFinding[]` JSON
4. Returns structured findings to the review engine

Planned providers (in priority order):
- Anthropic (Claude API)
- OpenAI (GPT API)
- Google (Gemini API)
- OpenRouter (multi-model gateway)
- Additional: OpenCode ZEN, z.ai, Minimax, etc.

### Provider Factory

```go
func NewProvider(name string, cfg ProviderConfig) (AIProvider, error)
```

Same factory pattern as `Platform`. New providers are added by implementing
`AIProvider` and registering in the factory.

---

## Project Structure

```
CRoBot/
  cmd/
    crobot/
      main.go                       # entry point

  internal/
    cli/
      root.go                       # cobra root command + global flags
      export.go                     # export-pr-context          (P1)
      snippet.go                    # get-file-snippet           (P1)
      comments.go                   # list-bot-comments          (P1)
      apply.go                      # apply-review-findings      (P1)
      serve.go                      # serve --mcp                (P2)
      review.go                     # review --pr                (P3/P4)

    config/
      config.go                     # unified config loading     (P1)

    platform/
      platform.go                   # Platform interface         (P1)
      types.go                      # shared types               (P1)
      factory.go                    # NewPlatform() factory      (P1)
      bitbucket/
        client.go                   # HTTP client, auth          (P1)
        pr.go                       # GetPRContext               (P1)
        diff.go                     # diff/diffstat parsing      (P1)
        comments.go                 # comment CRUD               (P1)
        file.go                     # file content retrieval     (P1)
      # github/                     # future
      # gitlab/                     # future

    review/
      engine.go                     # orchestrate full flow      (P1)
      validate.go                   # validate findings vs diff  (P1)
      dedupe.go                     # fingerprint dedup          (P1)
      render.go                     # format comment markdown    (P1)

    mcp/
      server.go                     # MCP server (stdio)         (P2)
      tools.go                      # tool definitions           (P2)
      handler.go                    # tool call -> engine bridge (P2)

    agent/
      client.go                     # ACP JSON-RPC client        (P3)
      session.go                    # session lifecycle          (P3)
      fs.go                         # read-only fs handler       (P3)
      prompt.go                     # prompt construction        (P3)
      parse.go                      # extract findings from resp (P3)
      config.go                     # agent config (cmd, args)   (P3)

    ai/
      provider.go                   # AIProvider interface       (P4)
      factory.go                    # NewProvider() factory      (P4)
      prompt.go                     # shared prompt templates    (P4)
      anthropic/
        client.go                   # Anthropic Claude API       (P4)
      openai/
        client.go                   # OpenAI GPT API             (P4)
      google/
        client.go                   # Google Gemini API          (P4)
      openrouter/
        client.go                   # OpenRouter API             (P4)
      # Additional providers follow the same pattern

  CLAUDE.md                         # agent instructions         (P1)
  OPENCODE.md                       # agent instructions         (P1)
  AGENTS.md                         # agent instructions         (P1)

  .ai/
    plan.md                         # this file
    tasks.md                        # task tracking
    agent-instructions.md           # canonical instructions     (P1)

  go.mod
  go.sum
  .goreleaser.yaml
```

---

## How the `review` Command Works Across Phases

The `crobot review` command is the unified entry point for Phase 3 and Phase 4.
It selects the analysis backend based on flags and config:

```
crobot review --pr 42                         # uses default (config-driven)
crobot review --pr 42 --agent claude          # Phase 3: ACP agent
crobot review --pr 42 --provider anthropic    # Phase 4: direct API
```

The `review` command orchestrates:

```
1. Resolve PR (parse URL or use --workspace/--repo/--pr flags)
2. Fetch PRContext from platform
3. Analyze (one of):
   a. ACP agent (--agent): spawn subprocess, ACP handshake, prompt, parse
   b. AI provider (--provider): call API directly, parse response
4. Validate findings against diff
5. Deduplicate against existing bot comments
6. Render comments
7. Post (--dry-run or --write)
8. Output summary JSON
```

Steps 4-8 are identical regardless of the analysis backend. The analysis layer
(`internal/agent/` and `internal/ai/`) is the only part that differs.
