# CRoBot - Code Review Bot

## Overview

CRoBot is a local-first CLI tool written in Go that enables AI-powered automated
code reviews on pull requests, posting real inline review comments.

The tool is built in five progressive phases:

1. **Phase 1 (MVP)**: CLI commands + Bitbucket Cloud + review engine. AI coding
   agents (Claude Code, Codex CLI, OpenCode, etc.) invoke CRoBot via shell
   commands and exchange JSON.
2. **Phase 2 (MCP)**: CRoBot runs as an MCP server (local stdio), exposing its
   commands as tools that MCP-capable agents discover automatically.
3. **Phase 3 (ACP)**: CRoBot becomes the single entry point. It acts as an ACP
   client, spawning and orchestrating an ACP-compatible coding agent subprocess
   to perform the review end-to-end.
4. **Phase 4 (Native Agent SDKs)**: For agents with rich proprietary SDKs that
   go beyond ACP (Claude Code Agent SDK, OpenAI Agents SDK), CRoBot uses
   dedicated adapters that unlock advanced features like tool injection,
   streaming, permission control, and hooks.
5. **Phase 5 (Direct AI Providers)**: CRoBot calls AI provider APIs directly
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
    Phase 5          Phase 4              Phase 3               Phase 1
+--------------+ +--------------+  +---------------+      +--------------+
| Direct AI    | | Native Agent |  | ACP Client    |      | External     |
| Provider     | | SDK Adapters |  | (CRoBot       |      | AI Agent     |
| (Anthropic,  | | (Claude SDK, |  |  spawns agent |      | (Claude Code,|
|  OpenAI ...) | |  Codex SDK)  |  |  subprocess)  |      |  Codex, ...) |
+------+-------+ +------+-------+  +-------+-------+      +------+-------+
       |                |                  |                     |
       v                v                  v                     v
      +------------------------------------------------------------+
      |                       Analysis Layer                       |
      |           internal/analysis/                               |
      |                                                            |
      |           Prompt construction, finding extraction,         |
      |           AI provider abstraction                          |
      +----------------------------------+-------------------------+
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
 | Cloud     | |         | | (future)|
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

# --- Native Agent SDKs (Phase 4) ---
agent_sdk:
  default: claude-sdk
  adapters:
    claude-sdk:
      # Uses Claude Code Agent SDK (subprocess protocol)
      model: claude-sonnet-4-20250514
      permission_mode: reject_edits   # read-only for code review
      max_turns: 50
      allowed_tools: [Read, Glob, Grep]
      # api_key from env: CROBOT_ANTHROPIC_API_KEY
    codex-sdk:
      # Uses OpenAI Agents SDK (subprocess protocol)
      model: gpt-4.1
      # api_key from env: CROBOT_OPENAI_API_KEY
  timeout: 300                  # seconds, default 5 minutes

# --- AI Provider (Phase 5: Direct API) ---
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
| `CROBOT_AGENT_SDK` | Default native SDK adapter (Phase 4) |
| `CROBOT_AI_PROVIDER` | Default AI provider (Phase 5) |
| `CROBOT_ANTHROPIC_API_KEY` | Anthropic API key (Phase 4/5) |
| `CROBOT_OPENAI_API_KEY` | OpenAI API key (Phase 4/5) |
| `CROBOT_GOOGLE_API_KEY` | Google AI API key (Phase 5) |
| `CROBOT_OPENROUTER_API_KEY` | OpenRouter API key (Phase 5) |

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

### Local (Pre-Push) Review (P3.8)

CRoBot supports reviewing local git changes before pushing — no PR or platform
credentials needed. When `crobot review` is invoked without a PR, it enters
local mode:

- **Source**: `git diff <merge-base>` against a base branch (default: `master`,
  configurable via `--base`), capturing committed + staged + unstaged changes.
- **Provider**: `internal/platform/local/provider.go` implements `Platform`
  using local git commands. No-op for list/create/delete comments.
- **Rendering**: `internal/cli/render.go` provides enhanced findings display
  with ANSI-colored diff context snippets around each finding.
- **Dry-run only**: Local mode never posts comments (no PR to post to).
- **MCP support**: `export_local_context` tool lets MCP-connected agents
  review local changes.

---

## Phase 4: Native Agent SDK Adapters

For agents with rich proprietary SDKs that offer capabilities beyond what ACP
exposes, CRoBot uses dedicated adapters. This unlocks advanced features like
custom tool injection, fine-grained permission control, typed streaming, and
lifecycle hooks — features that ACP's generic protocol cannot express.

```
crobot review --pr 42 --sdk claude          # Claude Code Agent SDK
crobot review --pr 42 --sdk codex           # OpenAI Agents SDK
```

### Why Not Just ACP?

ACP is a lowest-common-denominator protocol. It standardizes basic agent
communication (spawn, prompt, respond) but cannot express SDK-specific features:

| Feature | ACP | Claude Agent SDK | OpenAI Agents SDK |
|---------|-----|-----------------|-------------------|
| Spawn subprocess | Yes | Yes | Yes |
| Send prompt | Yes | Yes | Yes |
| Stream response | Yes | Yes (typed events) | Yes |
| Inject custom tools | No | **Yes** (in-process MCP) | **Yes** (tool defs) |
| Permission control | Basic | **Full** (per-tool allow/deny) | Partial |
| Hooks / interceptors | No | **Yes** (PreToolUse, etc.) | **Yes** (guardrails) |
| Session forking | No | **Yes** | No |
| Multi-turn context | Basic | **Full** (stateful client) | **Yes** (Runner) |
| Agent handoffs | No | No | **Yes** |
| Tracing / observability | No | Basic | **Yes** (built-in) |

By using native SDKs, CRoBot can:
1. **Inject CRoBot's own tools** directly into the agent's tool set (e.g.,
   `get-file-snippet`, `list-bot-comments`) so the agent can call them
   natively during its reasoning loop — no shell-out needed.
2. **Enforce read-only mode** at the tool level (allow `Read`, `Glob`, `Grep`;
   deny `Write`, `Edit`, `Bash`).
3. **Hook into the agent loop** to monitor progress, enforce constraints, or
   inject additional context mid-review.
4. **Stream typed events** for real-time progress reporting.

### Architecture

```go
// AgentSDKAdapter abstracts over different agent SDKs.
type AgentSDKAdapter interface {
    // Name returns the adapter identifier (e.g., "claude-sdk", "codex-sdk").
    Name() string

    // Review performs a complete code review using the native SDK.
    // It receives the PR context and review configuration, and returns findings.
    Review(ctx context.Context, opts SDKReviewRequest) (*SDKReviewResult, error)

    // Capabilities returns what this adapter supports beyond basic review.
    Capabilities() AdapterCapabilities
}

type SDKReviewRequest struct {
    PRContext     *PRContext
    SystemPrompt  string
    ReviewPrompt  string
    MaxTurns      int
    AllowedTools  []string     // tools the agent may use
    CustomTools   []ToolDef    // CRoBot tools injected into agent
    Timeout       time.Duration
}

type SDKReviewResult struct {
    Findings  []ReviewFinding
    Events    []ReviewEvent    // streaming events for progress tracking
    Usage     UsageStats       // tokens, cost, duration
}

type AdapterCapabilities struct {
    SupportsToolInjection  bool
    SupportsPermissions    bool
    SupportsHooks          bool
    SupportsStreaming       bool
    SupportsMultiTurn      bool
    SupportsHandoffs       bool
}
```

### Claude Code Agent SDK Adapter

The Claude Code Agent SDK (`@anthropic-ai/claude-code`) provides the richest
programmatic interface of any coding agent. CRoBot wraps it via a Go subprocess
that invokes the SDK.

**How it works:**
1. CRoBot spawns a small Node.js/Python bridge process that imports the Claude
   Agent SDK.
2. The bridge receives the review request (PR context, prompt, tool defs) as
   JSON over stdin.
3. The bridge creates a `ClaudeSDKClient` session with:
   - Custom system prompt (CRoBot review instructions)
   - `permission_mode: "rejectEdits"` (read-only)
   - `allowed_tools: ["Read", "Glob", "Grep"]`
   - CRoBot tools injected as in-process MCP tools via `create_sdk_mcp_server()`
4. The bridge streams typed events (assistant messages, tool calls, results)
   back to CRoBot over stdout.
5. CRoBot parses the final output for `ReviewFinding[]` JSON.

**Key advantages over ACP:**
- Agent can natively call `get-file-snippet` and `list-bot-comments` as MCP
  tools during its reasoning — deeper context retrieval without extra prompting.
- Fine-grained tool permissions prevent any writes.
- Hooks can intercept and log every tool call for auditability.
- Typed streaming events enable real-time progress bars and cost tracking.

### OpenAI Agents SDK Adapter

The OpenAI Agents SDK (`openai-agents`) provides agent orchestration with
tool definitions, guardrails, handoffs, and tracing.

**How it works:**
1. CRoBot spawns a Python bridge process that imports the Agents SDK.
2. The bridge creates an `Agent` with:
   - Custom instructions (CRoBot review prompt)
   - Tool definitions (CRoBot tools as SDK tool functions)
   - Output type schema (`ReviewFinding[]`)
   - Optional guardrails (e.g., reject findings outside diff)
3. The bridge runs `Runner.run()` with the PR context as input.
4. Results (including structured `ReviewFinding[]` via output type) are
   returned to CRoBot over stdout.

**Key advantages over ACP:**
- Structured output types ensure the agent returns valid `ReviewFinding[]`
  without manual JSON parsing.
- Guardrails can validate findings in-loop before the agent finalizes.
- Built-in tracing provides detailed execution logs.

### Bridge Process Design

Both adapters use a **bridge process** pattern: a small script (Node.js for
Claude SDK, Python for OpenAI SDK) that CRoBot spawns as a subprocess. The
bridge:
- Receives configuration as JSON on stdin
- Imports and uses the native SDK
- Streams events back as newline-delimited JSON on stdout
- Exits when the review is complete

Bridge scripts are bundled with the CRoBot binary (embedded via `go:embed`)
or installed alongside it.

```
CRoBot (Go) ──stdin──► Bridge (Node.js/Python) ──SDK──► Agent
             ◄─stdout──                         ◄──────
```

### Configuration

```yaml
agent_sdk:
  default: claude-sdk
  adapters:
    claude-sdk:
      model: claude-sonnet-4-20250514
      permission_mode: reject_edits
      max_turns: 50
      allowed_tools: [Read, Glob, Grep]
      custom_tools: true          # inject CRoBot tools into agent
      hooks:
        pre_tool_use: log         # log all tool calls
    codex-sdk:
      model: gpt-4.1
      output_type: structured     # use SDK structured output
      guardrails: true
  timeout: 300
```

---

## Phase 5: Direct AI Provider APIs

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

Note: Phase 5 shares the same review prompt templates with Phase 4's native SDK
adapters (`internal/analysis/prompt.go`). The difference is that Phase 5 calls
the AI model API directly, while Phase 4 delegates to a full coding agent that
can use tools, browse code, and reason over multiple turns.

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
      review.go                     # review --pr                (P3/P4/P5)

    config/
      config.go                     # unified config loading     (P1)

    platform/
      platform.go                   # Platform interface         (P1)
      types.go                      # shared types               (P1)
      factory.go                    # NewPlatform() factory      (P1)
      diffsize.go                   # diff stats, low-value classification
      diffwriter.go                 # write per-file diffs + index to disk
      bitbucket/
        client.go                   # HTTP client, auth          (P1)
        pr.go                       # GetPRContext               (P1)
        diff.go                     # diff/diffstat parsing      (P1)
        comments.go                 # comment CRUD               (P1)
        file.go                     # file content retrieval     (P1)
      local/
        provider.go                 # local git diff provider    (P3.8)
      github/                       # GitHub adapter
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

    agentsdk/
      adapter.go                    # AgentSDKAdapter interface  (P4)
      factory.go                    # NewAdapter() factory       (P4)
      bridge.go                     # bridge subprocess mgmt     (P4)
      claude/
        adapter.go                  # Claude Agent SDK adapter   (P4)
        bridge.js                   # Node.js bridge script      (P4)
      codex/
        adapter.go                  # OpenAI Agents SDK adapter  (P4)
        bridge.py                   # Python bridge script       (P4)

    ai/
      provider.go                   # AIProvider interface       (P5)
      factory.go                    # NewProvider() factory      (P5)
      prompt.go                     # shared prompt templates    (P5)
      anthropic/
        client.go                   # Anthropic Claude API       (P5)
      openai/
        client.go                   # OpenAI GPT API             (P5)
      google/
        client.go                   # Google Gemini API          (P5)
      openrouter/
        client.go                   # OpenRouter API             (P5)
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

The `crobot review` command is the unified entry point for Phases 3, 4, and 5.
It selects the analysis backend based on flags and config:

```
crobot review --pr 42                         # uses default (config-driven)
crobot review --pr 42 --agent claude          # Phase 3: ACP agent
crobot review --pr 42 --sdk claude            # Phase 4: Claude Agent SDK
crobot review --pr 42 --sdk codex             # Phase 4: OpenAI Agents SDK
crobot review --pr 42 --provider anthropic    # Phase 5: direct API
```

The `review` command orchestrates:

```
1. Resolve PR (parse URL or use --workspace/--repo/--pr flags)
2. Fetch PRContext from platform
3. Write per-file diffs to .crobot/diffs-<run-id>/ with index
4. Analyze (one of):
   a. ACP agent (--agent): spawn subprocess, ACP handshake, prompt with diff dir, parse
   b. Native SDK (--sdk): spawn bridge, use SDK features, stream findings
   c. AI provider (--provider): call API directly, parse response
5. Validate findings against diff
6. Deduplicate against existing bot comments
7. Render comments
8. Post (--dry-run or --write)
9. Clean up diff directory
10. Output summary JSON
```

Steps 4-8 are identical regardless of the analysis backend. The analysis layer
(`internal/agent/`, `internal/agentsdk/`, and `internal/ai/`) is the only part
that differs.

---

## Potential Expansions

Ideas and directions for future development, beyond the core 4-phase roadmap.
These are captured here for reference and prioritization — they are not committed
to any phase yet.

### Multi-Agent Review Panel

Instead of a single agent reviewing, spawn **multiple agents** in parallel, each
with a different focus area or prompt, and synthesize the results:

```
PR Diff ──► Agent A (security focus)    ──► ┐
       ──► Agent B (logic/bugs focus)   ──► ├─► Synthesizer ──► Final Findings
       ──► Agent C (perf/arch focus)    ──► ┘
```

- Each agent reviews with a specialized system prompt and focus area.
- A synthesizer (LLM call or rule-based) merges and deduplicates findings.
- Consensus scoring: if 2/3 agents flag the same issue, confidence increases.
- Reduces false positives and increases coverage across categories.
- Could use different models per agent (e.g., Opus for security, Sonnet for
  style) for cost optimization.

### Multi-Pass Reviews

Instead of a single review pass, run **multiple sequential passes** over the
diff, each refining or deepening the analysis:

1. **Pass 1 — Triage**: Quick scan to identify hotspots and categorize changes
   (new feature, refactor, bugfix, config change, etc.).
2. **Pass 2 — Deep Analysis**: Focused review of hotspot files, pulling in
   additional context via `get-file-snippet` for surrounding code, call sites,
   type definitions, etc.
3. **Pass 3 — Cross-File Analysis**: Look for issues that span multiple files
   (API contract mismatches, missing error handling propagation, inconsistent
   patterns across the changeset).
4. **Pass 4 — Consolidation**: Deduplicate, prioritize, and refine the final
   findings. Drop low-confidence items, merge related findings.

Each pass builds on the previous one's output. This produces higher-quality
reviews at the cost of more API calls and latency. Could be configurable:
`--review-depth quick|standard|deep` mapping to 1/2/4 passes.

### AST-Aware Diff Parsing

Enhance the diff parser to understand code structure, not just line changes:

- Parse diffs into AST nodes (function added, method signature changed, import
  removed, etc.) rather than raw line ranges.
- Enables smarter context retrieval: automatically fetch the full function body
  when only part of it changed, pull in callers of a modified function, etc.
- Language-aware review: different review rules for different languages.
- Could use tree-sitter for multi-language AST parsing.

### Context Retrieval Enhancement

Improve the context available to the reviewing agent beyond raw diffs:

- **Call graph analysis**: When a function is modified, automatically identify
  and include its callers and callees.
- **Type/interface resolution**: When a type is used in the diff, include its
  definition.
- **Test coverage mapping**: Show which tests cover the changed code.
- **Git blame context**: Who wrote the surrounding code and when, to help
  calibrate review tone.
- **Related PR history**: Previous reviews on the same files, to avoid
  re-raising dismissed findings.

### CRoBot as a DevOps MCP Tool Provider (Idea E)

CRoBot already knows how to talk to Bitbucket/GitHub APIs. Extend it as a
general-purpose **DevOps MCP server** — a toolkit that any AI agent can use to
interact with development infrastructure:

- PR creation/management tools
- Issue tracking integration (Jira, Linear, GitHub Issues)
- CI/CD status checking and trigger tools
- Repository analytics and search
- Branch management
- Release management

This would make CRoBot useful beyond code review — any AI agent connected via
MCP could manage the full development lifecycle.

### Review-as-a-Service (RaaS)

Run CRoBot as a **persistent service** rather than a CLI:

- **Webhook listener**: Auto-trigger reviews on PR creation/update via platform
  webhooks (Bitbucket, GitHub, GitLab).
- **Review queue**: Handle multiple PRs concurrently with configurable
  parallelism and priority.
- **Review memory / calibration**: Track which review comments get resolved vs
  dismissed by developers. Build a feedback loop to adjust prompts, severity
  thresholds, and categories over time. Per-repo "review profile" that adapts.
- **Team configuration**: Different review profiles per repo/team (security-
  focused, perf-focused, strict vs lenient, etc.).
- **Dashboard**: Simple web UI showing review history, stats, false-positive
  rate, cost per review.

### PR Comment Fixer

An agent-driven workflow to **automatically fix** issues raised in PR review
comments:

1. From the PR, identify the source branch.
2. Automatically switch to that branch (or create a git worktree for isolation).
3. Iterate through CR comments, determine which need fixing vs which are
   informational or already addressed.
4. For each fixable comment, apply the fix — one commit per fix for easy
   revert if needed.
5. Reply to / resolve the PR comment after fixing.
6. Push the fix commits to the PR branch.

This closes the loop: CRoBot finds issues, then CRoBot (or a connected agent)
fixes them. Could be triggered via a CLI command (`crobot fix-pr --pr 42`) or
automatically after a review.

### Smart Routing & Cost Optimization

Integrate intelligent model/provider routing (inspired by LiteLLM, Portkey):

- **Rate-limit-aware routing**: Track remaining API quota per provider, route
  to avoid hitting limits.
- **Cost-based routing**: Route simple checks to cheap/fast models, escalate
  complex analysis to capable models.
- **Capability-based routing**: Different models for different review categories
  (security vs style vs logic).
- **Failover**: If one provider errors or rate-limits, transparently retry on
  another.

Could be built into Phase 5 or integrated via a separate project (AgentMux).
