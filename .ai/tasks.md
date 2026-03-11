# CRoBot - Task List

> Status: `[ ]` = pending, `[~]` = in progress, `[x]` = done, `[-]` = skipped

> **Testing Rule**: Every task that produces or modifies Go code MUST include
> corresponding tests. Implementing agents MUST run `go test ./...` after every
> significant change and before marking any task complete. Broken tests are
> blocking — fix them before moving on. The test suite is the primary validation
> mechanism for all work. See the "Testing Philosophy" section in `plan.md` for
> full requirements.

---

## Phase 1: CLI + Bitbucket Cloud + Review Engine (MVP)

### P1.1 Project Scaffolding

- [ ] P1.1.1 Initialize Go module (`go mod init github.com/dizzyc/crobot`)
- [ ] P1.1.2 Create project directory structure (`cmd/crobot/`, `internal/`)
- [ ] P1.1.3 Add cobra dependency, create root command skeleton (`internal/cli/root.go`)
- [ ] P1.1.4 Add global flags: `--version`, `--help`, `--verbose`, `--output-format`
- [ ] P1.1.5 Set up structured logging (stderr, with `--verbose` flag)
- [ ] P1.1.6 Create `cmd/crobot/main.go` entry point

### P1.2 Configuration

- [ ] P1.2.1 Implement config loading (`internal/config/config.go`)
  - [ ] Support `~/.config/crobot/config.yaml` and `.crobot.yaml` in repo root
  - [ ] Layer: defaults < config file < env vars < CLI flags
- [ ] P1.2.2 Define config struct matching YAML schema in plan
- [ ] P1.2.3 Wire env vars (`CROBOT_PLATFORM`, `CROBOT_BITBUCKET_USER`, `CROBOT_BITBUCKET_TOKEN`, etc.)
- [ ] P1.2.4 Unit tests for config loading and layering

### P1.3 Platform Abstraction Layer

- [ ] P1.3.1 Define `Platform` interface (`internal/platform/platform.go`)
- [ ] P1.3.2 Define shared types (`internal/platform/types.go`)
  - [ ] `PRRequest`, `PRContext`, `ChangedFile`, `DiffHunk`
  - [ ] `ReviewFinding`, `InlineComment`, `Comment`
  - [ ] `FileRequest`
- [ ] P1.3.3 Implement platform factory (`internal/platform/factory.go`)
- [ ] P1.3.4 Add JSON validation for `ReviewFinding` input
- [ ] P1.3.5 Unit tests for shared types (serialization, validation)

### P1.4 Bitbucket Cloud Implementation

- [ ] P1.4.1 HTTP client with auth (`internal/platform/bitbucket/client.go`)
  - [ ] Basic HTTP client with auth header (user + token from env)
  - [ ] Pagination helper (Bitbucket `next` URL pattern)
  - [ ] Rate limit handling and retries with backoff
  - [ ] Error mapping (Bitbucket errors -> structured Go errors)
- [ ] P1.4.2 `GetPRContext` (`internal/platform/bitbucket/pr.go`)
  - [ ] Fetch PR metadata and normalize to `PRContext`
  - [ ] Fetch diffstat for list of changed files
  - [ ] Fetch raw diff and parse into `DiffHunk` slices
  - [ ] Unit tests with recorded HTTP responses
- [ ] P1.4.3 Diff parsing (`internal/platform/bitbucket/diff.go`)
  - [ ] Parse unified diff format into `DiffHunk` structs
  - [ ] Handle renames, binary files, new/deleted files
  - [ ] Unit tests with sample diffs
- [ ] P1.4.4 `GetFileContent` (`internal/platform/bitbucket/file.go`)
  - [ ] Fetch file at specific commit via `/src/{commit}/{path}`
  - [ ] Handle binary files and missing files gracefully
  - [ ] Unit tests
- [ ] P1.4.5 `ListBotComments` (`internal/platform/bitbucket/comments.go`)
  - [ ] Fetch all PR comments with pagination
  - [ ] Filter to bot comments (by fingerprint marker)
  - [ ] Extract fingerprint from comment body
  - [ ] Unit tests
- [ ] P1.4.6 `CreateInlineComment` (`internal/platform/bitbucket/comments.go`)
  - [ ] Build Bitbucket payload with `inline.to` / `inline.from`
  - [ ] Embed fingerprint as HTML comment in body
  - [ ] Unit tests
- [ ] P1.4.7 `DeleteComment` (`internal/platform/bitbucket/comments.go`)
  - [ ] Delete by comment ID
  - [ ] Unit tests

### P1.5 Review Engine

- [ ] P1.5.1 Finding validation (`internal/review/validate.go`)
  - [ ] Check path exists in PR changed files
  - [ ] Check line falls within a diff hunk range
  - [ ] Check side is valid ("new" or "old")
  - [ ] Filter by severity threshold
  - [ ] Return validated + rejected findings with reasons
  - [ ] Unit tests
- [ ] P1.5.2 Deduplication (`internal/review/dedupe.go`)
  - [ ] Generate fingerprint if not provided by agent
  - [ ] Compare against existing bot comment fingerprints
  - [ ] Return new/duplicate/updated finding lists
  - [ ] Unit tests
- [ ] P1.5.3 Comment rendering (`internal/review/render.go`)
  - [ ] Format severity badge + category header
  - [ ] Render message body as markdown
  - [ ] Render optional code suggestion in fenced block
  - [ ] Append hidden fingerprint HTML comment
  - [ ] Golden file tests
- [ ] P1.5.4 Review orchestrator (`internal/review/engine.go`)
  - [ ] Full flow: load findings -> validate -> dedupe -> render -> post
  - [ ] Dry-run mode (validate + render, return plan without posting)
  - [ ] Write mode (post comments, return results with IDs)
  - [ ] Max comments cap with configurable limit
  - [ ] Summary output (posted / skipped / failed counts)
  - [ ] Unit tests

### P1.6 CLI Commands

- [ ] P1.6.1 `export-pr-context` command (`internal/cli/export.go`)
  - [ ] Wire flags: `--workspace`, `--repo`, `--pr`
  - [ ] Call platform, output `PRContext` JSON to stdout
  - [ ] Integration test with mock server
- [ ] P1.6.2 `get-file-snippet` command (`internal/cli/snippet.go`)
  - [ ] Wire flags: `--commit`, `--path`, `--line`, `--context`
  - [ ] Fetch file, extract line range, output JSON
  - [ ] Integration test
- [ ] P1.6.3 `list-bot-comments` command (`internal/cli/comments.go`)
  - [ ] Wire flags, call platform, output `Comment[]` JSON
  - [ ] Integration test
- [ ] P1.6.4 `apply-review-findings` command (`internal/cli/apply.go`)
  - [ ] Wire flags: `--input`, `--dry-run`, `--write`, `--max-comments`
  - [ ] Read findings from file or stdin (`--input -`)
  - [ ] Call review engine, output result JSON
  - [ ] Integration test

### P1.7 Testing & Quality

- [ ] P1.7.1 Set up test infrastructure
  - [ ] HTTP response recorder / mock server for Bitbucket API
  - [ ] Test fixture directory (`testdata/`) with sample diffs, PRs, comments
  - [ ] Golden file tests for comment rendering
  - [ ] Table-driven test helpers for validation logic
- [ ] P1.7.2 Unit test coverage for all packages
  - [ ] `internal/config/` — config loading, layering, defaults
  - [ ] `internal/platform/` — type serialization, validation, factory
  - [ ] `internal/platform/bitbucket/` — client, PR fetching, diff parsing, comments
  - [ ] `internal/review/` — validation, deduplication, rendering, engine orchestration
- [ ] P1.7.3 Integration tests
  - [ ] End-to-end: export -> apply dry-run with recorded HTTP responses
  - [ ] End-to-end: apply --write with mock Bitbucket server
  - [ ] CLI command tests (flag parsing, output format, error handling)
- [ ] P1.7.4 CI pipeline
  - [ ] GitHub Actions workflow: lint (`golangci-lint`), test (`go test ./...`), build
  - [ ] Require all tests to pass before merge
  - [ ] Release workflow: goreleaser for multi-platform binaries

### P1.8 Agent Instruction Files

- [ ] P1.8.1 Write canonical instructions (`.ai/agent-instructions.md`)
  - [ ] Document all crobot commands with flags and examples
  - [ ] Define `ReviewFinding` JSON schema with field descriptions
  - [ ] Write step-by-step review workflow
  - [ ] Define agent rules (dry-run first, max comments, no direct API calls)
- [ ] P1.8.2 Create `CLAUDE.md` (Claude Code instruction file)
- [ ] P1.8.3 Create `OPENCODE.md` (OpenCode instruction file)
- [ ] P1.8.4 Create `AGENTS.md` (Codex CLI / Copilot CLI instruction file)
- [ ] P1.8.5 Write README with quickstart and usage examples
- [ ] P1.8.6 Add `--help` text for every command with examples

---

## Phase 2: MCP Server

> Tests: All MCP server code must have unit tests for tool routing, schema
> validation, and error mapping. Integration tests must verify full round-trip
> tool calls over stdio.

### P2.1 MCP Server Core

- [ ] P2.1.1 Set up MCP server skeleton (`internal/mcp/server.go`)
  - [ ] stdio transport (JSON-RPC 2.0 over stdin/stdout)
  - [ ] Server capability negotiation
- [ ] P2.1.2 Define MCP tool schemas (`internal/mcp/tools.go`)
  - [ ] `export_pr_context` tool definition
  - [ ] `get_file_snippet` tool definition
  - [ ] `list_bot_comments` tool definition
  - [ ] `apply_review_findings` tool definition
- [ ] P2.1.3 Implement tool call handler (`internal/mcp/handler.go`)
  - [ ] Route MCP tool calls to existing internal functions
  - [ ] Error mapping (internal errors -> MCP error responses)

### P2.2 CLI Integration

- [ ] P2.2.1 Add `serve` command (`internal/cli/serve.go`)
  - [ ] `crobot serve --mcp` starts the MCP server
  - [ ] Wire config (platform credentials, review settings)
- [ ] P2.2.2 Create example `.mcp.json` config for Claude Code
- [ ] P2.2.3 Integration test: MCP tool call round-trip

---

## Phase 3: ACP Orchestrator

> Tests: ACP client code must have unit tests for JSON-RPC message
> encoding/decoding, session lifecycle, filesystem capability handling, and
> prompt construction. Integration tests must use a mock agent subprocess to
> verify the full orchestration flow.

### P3.1 ACP Client

- [ ] P3.1.1 JSON-RPC 2.0 client over stdio (`internal/agent/client.go`)
  - [ ] Spawn agent subprocess (configurable command + args)
  - [ ] Read/write JSON-RPC messages over stdin/stdout
  - [ ] Handle notifications and requests
- [ ] P3.1.2 Session lifecycle (`internal/agent/session.go`)
  - [ ] `initialize` handshake
  - [ ] `session/new` to create session
  - [ ] `session/prompt` to send review prompt
  - [ ] `session/update` notification handling (streaming)
  - [ ] Session cleanup and subprocess termination

### P3.2 Filesystem Capability

- [ ] P3.2.1 Read-only filesystem handler (`internal/agent/fs.go`)
  - [ ] `fs/read_text_file`: backed by git checkout at PR head commit
  - [ ] Auto-approve reads, auto-deny writes
  - [ ] No `fs/write_text_file` or `terminal/*`

### P3.3 Prompt & Parsing

- [ ] P3.3.1 Review prompt construction (`internal/agent/prompt.go`)
  - [ ] System prompt with review guidelines
  - [ ] PR context (diff, files, metadata) formatted for agent
  - [ ] Output format instructions (`ReviewFinding[]` JSON)
- [ ] P3.3.2 Finding extraction (`internal/agent/parse.go`)
  - [ ] Parse agent response to extract `ReviewFinding[]` JSON
  - [ ] Handle edge cases (markdown fences, extra text, etc.)
  - [ ] Retry logic on parse failure

### P3.4 CLI Integration

- [ ] P3.4.1 Add `review` command (`internal/cli/review.go`)
  - [ ] `--pr` flag (URL or number)
  - [ ] `--agent` flag (selects ACP agent from config)
  - [ ] `--dry-run` / `--write` flags
  - [ ] Wire full flow: fetch -> spawn agent -> analyze -> review engine -> post
- [ ] P3.4.2 Agent config loading from YAML
- [ ] P3.4.3 Timeout handling (configurable, default 5 min)
- [ ] P3.4.4 Integration tests with mock agent subprocess

---

## Phase 4: Direct AI Provider APIs

> Tests: Each AI provider must have unit tests using recorded HTTP responses
> (no real API calls in default test runs). Shared prompt templates and finding
> parsers must have thorough table-driven tests. Use `//go:build integration`
> for tests that hit real APIs.

### P4.1 AI Provider Interface

- [ ] P4.1.1 Define `AIProvider` interface (`internal/ai/provider.go`)
  - [ ] `Review(ctx, ReviewRequest) ([]ReviewFinding, error)`
  - [ ] `Name() string`
- [ ] P4.1.2 Define `ReviewRequest` struct
- [ ] P4.1.3 Implement provider factory (`internal/ai/factory.go`)
- [ ] P4.1.4 Shared prompt templates (`internal/ai/prompt.go`)

### P4.2 Provider Implementations

- [ ] P4.2.1 Anthropic Claude API (`internal/ai/anthropic/client.go`)
  - [ ] HTTP client with Messages API
  - [ ] Request/response mapping
  - [ ] Parse `ReviewFinding[]` from response
  - [ ] Unit tests
- [ ] P4.2.2 OpenAI GPT API (`internal/ai/openai/client.go`)
  - [ ] HTTP client with Chat Completions API
  - [ ] Request/response mapping
  - [ ] Parse findings
  - [ ] Unit tests
- [ ] P4.2.3 Google Gemini API (`internal/ai/google/client.go`)
  - [ ] HTTP client with Gemini API
  - [ ] Request/response mapping
  - [ ] Parse findings
  - [ ] Unit tests
- [ ] P4.2.4 OpenRouter API (`internal/ai/openrouter/client.go`)
  - [ ] HTTP client (OpenAI-compatible format)
  - [ ] Request/response mapping
  - [ ] Parse findings
  - [ ] Unit tests

### P4.3 CLI Integration

- [ ] P4.3.1 Extend `review` command with `--provider` and `--model` flags
- [ ] P4.3.2 Wire provider selection (config-driven + CLI override)
- [ ] P4.3.3 Integration tests with recorded API responses

---

## Cross-Phase / Future

- [ ] GitHub platform adapter (`internal/platform/github/`)
- [ ] GitLab platform adapter (`internal/platform/gitlab/`)
- [ ] Summary comment with overall review status
- [ ] Webhook listener for automatic PR review triggers
- [ ] Batch review mode (multiple PRs)
