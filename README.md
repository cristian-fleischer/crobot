# CRoBot

AI-powered code review bot that posts inline comments on pull requests.

CRoBot is a local-first CLI tool written in Go. It fetches PR data, validates
AI-generated review findings against the actual diff, deduplicates against
existing comments, and posts inline review comments. The same binary works on a
developer's machine and in CI pipelines.

## Features

- **Platform-agnostic**: Bitbucket Cloud supported today; GitHub and GitLab
  planned.
- **Agent-agnostic**: Works with any AI coding agent (Claude Code, Codex CLI,
  OpenCode, Copilot) or directly via AI provider APIs (planned).
- **Safe by default**: Dry-run mode is the default. Use `--write` to post.
- **Smart deduplication**: Fingerprints prevent duplicate comments on re-runs.
- **Diff-aware validation**: Only allows comments on lines actually changed in
  the PR.
- **MCP server**: Expose all tools over the Model Context Protocol (MCP) for
  direct agent integration via stdio.
- **Single binary**: No runtime dependencies.

## Installation

### From Source

```bash
go install github.com/cristian-fleischer/crobot/cmd/crobot@latest
```

### From Releases

Download the binary for your platform from the
[Releases](https://github.com/cristian-fleischer/crobot/releases) page.

### Build from Source

```bash
git clone https://github.com/cristian-fleischer/crobot.git
cd crobot
go build -o crobot ./cmd/crobot
```

---

## Setting Up Bitbucket Authentication

CRoBot authenticates with Bitbucket Cloud using an **API token** (formerly
called "app password"). Follow these steps to create one.

### Step 1: Find Your Atlassian Account Email

This is the email address you use to log in to Bitbucket (your Atlassian ID).

1. Go to [Atlassian Account | Email](https://id.atlassian.com/manage-profile/email)

This email is used as the username for API authentication.

> **Note:** For API calls you use your **Atlassian account email** (not your
> Bitbucket display name or username). Alternatively, you can use the static
> username `x-bitbucket-api-token-auth` instead of your email.

### Step 2: Find Your Bitbucket Workspace Slug

The workspace slug is the URL segment that identifies your team or personal
workspace. For example, if your repository URL is
`https://bitbucket.org/myteam/my-repo`, the workspace slug is `myteam`.

1. Go to [Bitbucket](https://bitbucket.org/)
2. Your workspace slug is visible in the URL of any repository:
   `https://bitbucket.org/{workspace}/{repo}`

### Step 3: Create an API Token

1. Go to your Bitbucket profile:
   [Atlassian Account | Security | API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Click **Create API token with scopes**
3. Configure the token:
   - **Name**: Give it a descriptive name (e.g. "CRoBot Code Reviews")
   - **Expiry**: Set an appropriate expiry period
4. Select app: **Bitbucket**
5. Select the required **scopes**:
   - **read:repository:bitbucket** -- required to fetch PR data and file contents
   - **read:pullrequest:bitbucket** -- required to read PR metadata and comments
   - **write:pullrequest:bitbucket** -- required to post review comments
6. Click **Create**
7. **Copy the generated token immediately** -- it is only shown once

### Step 4: Configure CRoBot

You can configure credentials via environment variables or a config file.

**Environment variables (recommended for CI):**

```bash
export CROBOT_BITBUCKET_USER="you@example.com"
export CROBOT_BITBUCKET_TOKEN="your-api-token"
```

**Config file (recommended for local development):**

```yaml
# ~/.config/crobot/config.yaml
platform: bitbucket

bitbucket:
  workspace: myteam
  repo: my-service
  user: you@example.com
  token: your-api-token
```

> **Security note:** If you store the token in a config file, ensure the file
> has restricted permissions (`chmod 600 ~/.config/crobot/config.yaml`). For
> CI environments, always use environment variables or a secrets manager.

---

## Quickstart

### 1. Set Up Credentials

Follow the [Bitbucket Authentication](#setting-up-bitbucket-authentication)
steps above, then either export env vars or create a config file.

### 2. Export PR Context

```bash
crobot export-pr-context --pr 42 > context.json
```

If `workspace` and `repo` are set in your config file or env vars, you don't
need to pass them on every command. Otherwise:

```bash
crobot export-pr-context --workspace myteam --repo my-service --pr 42 > context.json
```

### 3. Generate Findings

Use your preferred AI agent to review the PR context and produce a
`ReviewFinding[]` JSON array. For example, with Claude Code:

```bash
claude -p "Review this PR for bugs and security issues. Output a JSON array \
  of ReviewFinding objects." --input context.json > findings.json
```

### 4. Dry Run (Validate)

```bash
crobot apply-review-findings --pr 42 --input findings.json --dry-run
```

### 5. Post Comments

```bash
crobot apply-review-findings --pr 42 --input findings.json --write
```

---

## Commands

All commands support `--workspace` and `--repo` flags. When omitted, they
fall back to values from the config file or environment variables.

### `export-pr-context`

Fetches PR metadata, changed files, and diff hunks as JSON to stdout.

```bash
crobot export-pr-context --workspace <ws> --repo <repo> --pr <number>
```

| Flag          | Type   | Required | Default | Description                                    |
|---------------|--------|----------|---------|------------------------------------------------|
| `--workspace` | string | no*      |         | Workspace/organization slug                    |
| `--repo`      | string | no*      |         | Repository slug                                |
| `--pr`        | int    | yes      |         | Pull request number                            |

*Required unless set in config file or env vars.

### `get-file-snippet`

Fetches a slice of a file at a specific commit with surrounding context.

```bash
crobot get-file-snippet \
  --workspace <ws> --repo <repo> \
  --commit <hash> --path <file> --line <n> --context <lines>
```

| Flag          | Type   | Required | Default | Description                                    |
|---------------|--------|----------|---------|------------------------------------------------|
| `--workspace` | string | no*      |         | Workspace/organization slug                    |
| `--repo`      | string | no*      |         | Repository slug                                |
| `--commit`    | string | yes      |         | Commit hash                                    |
| `--path`      | string | yes      |         | File path relative to repo root                |
| `--line`      | int    | yes      |         | Center line number                             |
| `--context`   | int    | no       | `5`     | Number of context lines above and below        |

*Required unless set in config file or env vars.

### `list-bot-comments`

Lists existing CRoBot comments on a PR as JSON to stdout.

```bash
crobot list-bot-comments --workspace <ws> --repo <repo> --pr <number>
```

| Flag          | Type   | Required | Default | Description                                    |
|---------------|--------|----------|---------|------------------------------------------------|
| `--workspace` | string | no*      |         | Workspace/organization slug                    |
| `--repo`      | string | no*      |         | Repository slug                                |
| `--pr`        | int    | yes      |         | Pull request number                            |

*Required unless set in config file or env vars.

### `apply-review-findings`

Takes `ReviewFinding[]` JSON and posts them as inline PR comments.

```bash
# Dry run (default)
crobot apply-review-findings \
  --workspace <ws> --repo <repo> --pr <number> \
  --input findings.json --dry-run

# Post comments
crobot apply-review-findings \
  --workspace <ws> --repo <repo> --pr <number> \
  --input findings.json --write

# Read from stdin with comment cap
cat findings.json | crobot apply-review-findings \
  --workspace <ws> --repo <repo> --pr <number> \
  --input - --write --max-comments 10
```

| Flag             | Type   | Required | Default | Description                                    |
|------------------|--------|----------|---------|------------------------------------------------|
| `--workspace`    | string | no*      |         | Workspace/organization slug                    |
| `--repo`         | string | no*      |         | Repository slug                                |
| `--pr`           | int    | yes      |         | Pull request number                            |
| `--input`        | string | yes      |         | Path to findings JSON file (`-` for stdin)     |
| `--dry-run`      | bool   | no       | `true`  | Validate without posting (default behavior)    |
| `--write`        | bool   | no       | `false` | Actually post comments to the PR               |
| `--max-comments` | int    | no       | config  | Max comments to post (`0` = unlimited)         |

*Required unless set in config file or env vars.

### `serve`

Starts CRoBot as an MCP (Model Context Protocol) server over stdio. This
allows MCP-capable agents like Claude Code to call CRoBot tools directly
without shelling out to CLI commands.

```bash
crobot serve --mcp
```

| Flag    | Type | Required | Default | Description                          |
|---------|------|----------|---------|--------------------------------------|
| `--mcp` | bool | yes      | `false` | Start as MCP server over stdio       |

The server exposes the same four tools as the [CLI](#commands): `export_pr_context`,
`get_file_snippet`, `list_bot_comments`, and `apply_review_findings`.

### Global Flags

| Flag              | Type   | Default | Description                          |
|-------------------|--------|---------|--------------------------------------|
| `--verbose`, `-v` | bool   | `false` | Enable debug logging (to stderr)     |
| `--output-format` | string | `json`  | Output format                        |
| `--version`       |        |         | Print version and exit               |

---

## ReviewFinding Schema

```json
[
  {
    "path": "src/auth.ts",
    "line": 42,
    "side": "new",
    "severity": "warning",
    "category": "security",
    "message": "Logging the raw token can leak credentials.",
    "suggestion": "logger.info(\"Token received\", { tokenPrefix: token.slice(0, 4) })",
    "fingerprint": ""
  }
]
```

| Field         | Type   | Required | Description                                       |
|---------------|--------|----------|---------------------------------------------------|
| `path`        | string | yes      | File path relative to repo root                   |
| `line`        | int    | yes      | Line number (> 0)                                 |
| `side`        | string | yes      | `"new"` or `"old"`                                |
| `severity`    | string | yes      | `"info"`, `"warning"`, or `"error"`               |
| `category`    | string | yes      | e.g. `"security"`, `"bug"`, `"performance"`       |
| `message`     | string | yes      | Human-readable explanation                         |
| `suggestion`  | string | no       | Suggested code fix                                 |
| `fingerprint` | string | no       | Leave empty for auto-generation                    |

**Severity levels** (highest to lowest):
- `error` -- bugs, security vulnerabilities, crashes
- `warning` -- code smells, performance issues, potential bugs
- `info` -- style suggestions, documentation improvements

---

## Configuration

CRoBot uses layered configuration. Values are resolved in this order (later
layers override earlier ones):

1. **Built-in defaults**
2. **Global config file** (`~/.config/crobot/config.yaml`)
3. **Local config file** (`.crobot.yaml` in the current directory)
4. **Environment variables**
5. **CLI flags**

### Config File Reference

```yaml
# ~/.config/crobot/config.yaml (global)
# .crobot.yaml (per-repo, in repo root)

# Platform to use. Currently only "bitbucket" is supported.
platform: bitbucket

# Bitbucket-specific settings.
bitbucket:
  # Workspace (team) slug. Avoids passing --workspace on every command.
  workspace: myteam

  # Default repository slug. Avoids passing --repo on every command.
  repo: my-service

  # Atlassian account email for API authentication.
  # Can also be "x-bitbucket-api-token-auth" as a static alternative.
  user: you@example.com

  # Bitbucket API token.
  # Prefer env var CROBOT_BITBUCKET_TOKEN over storing in a file.
  token: your-api-token

# Review behaviour settings.
review:
  # Maximum number of review comments per run. Default: 25.
  max_comments: 25

  # Default dry-run mode. Default: true.
  dry_run: true

  # Label used to identify bot-generated comments. Default: "crobot".
  bot_label: crobot

  # Minimum severity level to report. Default: "warning".
  # Options: "info", "warning", "error".
  severity_threshold: warning

# Agent runner settings (Phase 3 - not yet implemented).
# agent:
#   default: claude
#   agents:
#     claude:
#       command: claude
#       args: ["--model", "sonnet-4"]
#   timeout: 300

# AI provider settings (Phase 4 - not yet implemented).
# ai:
#   default_provider: anthropic
#   providers:
#     anthropic:
#       model: claude-sonnet-4-20250514
#   max_tokens: 8192
#   temperature: 0.2
```

### Environment Variables

Environment variables override config file values.

| Variable                       | Description                                      | Default      |
|--------------------------------|--------------------------------------------------|--------------|
| `CROBOT_PLATFORM`              | Platform to use                                  | `bitbucket`  |
| `CROBOT_BITBUCKET_WORKSPACE`   | Bitbucket workspace/team slug                    |              |
| `CROBOT_BITBUCKET_REPO`        | Bitbucket repository slug                        |              |
| `CROBOT_BITBUCKET_USER`        | Bitbucket username/email for API auth            |              |
| `CROBOT_BITBUCKET_TOKEN`       | Bitbucket API token                              |              |
| `CROBOT_MAX_COMMENTS`          | Max comments per run                             | `25`         |
| `CROBOT_DRY_RUN`               | Default dry-run mode (`true`, `1`, `yes`)        | `true`       |
| `CROBOT_AGENT`                 | Default agent name (Phase 3)                     |              |
| `CROBOT_AI_PROVIDER`           | Default AI provider (Phase 4)                    |              |
| `CROBOT_ANTHROPIC_API_KEY`     | Anthropic API key (Phase 4)                      |              |
| `CROBOT_OPENAI_API_KEY`        | OpenAI API key (Phase 4)                         |              |
| `CROBOT_GOOGLE_API_KEY`        | Google API key (Phase 4)                         |              |
| `CROBOT_OPENROUTER_API_KEY`    | OpenRouter API key (Phase 4)                     |              |

### Recommended Setup

**For local development** -- use a global config file with credentials and
defaults:

```yaml
# ~/.config/crobot/config.yaml
platform: bitbucket

bitbucket:
  workspace: myteam
  user: you@example.com
  token: your-api-token
```

Then add a per-repo `.crobot.yaml` to set the repository:

```yaml
# .crobot.yaml (in your repo root)
bitbucket:
  repo: my-service
```

With this setup, commands simplify to:

```bash
crobot export-pr-context --pr 42
crobot apply-review-findings --pr 42 --input findings.json --write
```

**For CI pipelines** -- use environment variables:

```bash
export CROBOT_BITBUCKET_USER="$BITBUCKET_USER"
export CROBOT_BITBUCKET_TOKEN="$BITBUCKET_TOKEN"
export CROBOT_BITBUCKET_WORKSPACE="$WORKSPACE"
export CROBOT_BITBUCKET_REPO="$REPO"
```

---

## CI/CD Integration

### Bitbucket Pipelines

```yaml
- step:
    name: AI Code Review
    script:
      - export CROBOT_BITBUCKET_USER=$BITBUCKET_USER
      - export CROBOT_BITBUCKET_TOKEN=$BITBUCKET_TOKEN
      - export CROBOT_BITBUCKET_WORKSPACE=$BITBUCKET_WORKSPACE
      - export CROBOT_BITBUCKET_REPO=$BITBUCKET_REPO_SLUG
      - crobot export-pr-context --pr $BITBUCKET_PR_ID > context.json
      - claude -p "Review this PR" --input context.json --output findings.json
      - crobot apply-review-findings --pr $BITBUCKET_PR_ID --input findings.json --write
```

### GitHub Actions

```yaml
- name: AI Code Review
  env:
    CROBOT_PLATFORM: bitbucket  # or github when supported
    CROBOT_BITBUCKET_USER: ${{ secrets.BITBUCKET_USER }}
    CROBOT_BITBUCKET_TOKEN: ${{ secrets.BITBUCKET_TOKEN }}
    CROBOT_BITBUCKET_WORKSPACE: myteam
    CROBOT_BITBUCKET_REPO: my-service
  run: |
    crobot export-pr-context --pr ${{ github.event.pull_request.number }} > context.json
    claude -p "Review this PR" --input context.json --output findings.json
    crobot apply-review-findings --pr ${{ github.event.pull_request.number }} --input findings.json --write
```

---

## Agent Integration

### MCP Server (Recommended)

MCP-capable agents (e.g. Claude Code) can use CRoBot as a native tool server.
Add a `.mcp.json` to your project root:

```json
{
  "mcpServers": {
    "crobot": {
      "command": "crobot",
      "args": ["serve", "--mcp"],
      "env": {
        "CROBOT_PLATFORM": "bitbucket",
        "CROBOT_BITBUCKET_USER": "your-username",
        "CROBOT_BITBUCKET_TOKEN": "your-app-password",
        "CROBOT_BITBUCKET_WORKSPACE": "your-workspace",
        "CROBOT_BITBUCKET_REPO": "your-repo"
      }
    }
  }
}
```

The agent then has direct access to the following tools:

- **`export_pr_context`** -- Fetch PR metadata, changed files, and diff hunks.
- **`get_file_snippet`** -- Retrieve a slice of a file at a specific commit with surrounding context.
- **`list_bot_comments`** -- List existing CRoBot comments on a PR.
- **`apply_review_findings`** -- Validate, deduplicate, and post review findings as inline PR comments.

No shell commands needed. The `apply_review_findings` tool defaults to dry-run
mode and is annotated as destructive, so agents will ask for confirmation
before posting.

### Instruction Files

CRoBot also includes instruction files for agents that don't support MCP:

| Agent               | Instruction File            |
|---------------------|-----------------------------|
| Claude Code         | `CLAUDE.md`                 |
| Codex CLI / Copilot | `AGENTS.md`                 |
| Reference           | `.ai/agent-instructions.md` |

See `.ai/agent-instructions.md` for the full review workflow and rules.

---

## Development

```bash
# Run tests
go test ./...

# Run tests with race detector
go test -race ./...

# Build
go build -o crobot ./cmd/crobot

# Lint (requires golangci-lint)
golangci-lint run
```

## License

MIT
