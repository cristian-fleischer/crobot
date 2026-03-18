# CRoBot

AI-powered code review bot that posts inline comments on pull requests.

CRoBot is a local-first CLI tool written in Go. It fetches PR data, validates
AI-generated review findings against the actual diff, deduplicates against
existing comments, and posts inline review comments. The same binary works on a
developer's machine and in CI pipelines.

## Features

- **One-command review**: `crobot review <pr-url>` spawns an AI agent, feeds it
  the PR diff, and posts inline comments -- fully automated.
- **Three integration modes**: Use CRoBot as an autonomous orchestrator, as an
  MCP tool server for interactive agent sessions, or as a CLI toolkit for
  maximum flexibility (see [Use Cases](#use-cases) below).
- **Platform-agnostic**: Bitbucket Cloud and GitHub supported; GitLab planned.
- **Agent-agnostic**: Works with any AI coding agent (Claude Code, Codex CLI,
  OpenCode, Gemini, Copilot) or directly via AI provider APIs (planned).
- **Safe by default**: Dry-run mode is the default. Use `--write` to post.
- **Smart deduplication**: Fingerprints prevent duplicate comments on re-runs.
- **Diff-aware validation**: Only allows comments on lines actually changed in
  the PR.
- **Customizable review philosophy**: Export, edit, and override the review
  focus to match your project's needs (see [Configuration](#configuration)).
- **Formatted streaming output**: Agent output is rendered as formatted
  markdown in the terminal with a live progress indicator.
- **MCP server**: Expose all tools over the Model Context Protocol for direct
  agent integration via stdio.
- **Local pre-push review**: Review local changes before pushing with
  `crobot review` (no PR required). Diffs all changes (committed, staged, and
  unstaged) against a base branch and renders findings in the terminal.
- **Single binary**: No runtime dependencies.

## Use Cases

CRoBot supports three integration modes, each suited to different workflows:

### Orchestrated (`crobot review`)

CRoBot drives the AI agent end-to-end. One command fetches the PR, spawns the
agent, collects findings, and posts comments. No human interaction required.

```bash
crobot review https://bitbucket.org/team/repo/pull-requests/42 --write
```

**Best for:** CI/CD pipelines, automated review on every PR, hands-off
workflows where reviews should happen without human intervention.

### Interactive (MCP Server)

CRoBot runs as an MCP tool server. An MCP-capable agent (Claude Code, Cursor,
etc.) discovers CRoBot's tools and the human guides the review interactively.

```json
{ "mcpServers": { "crobot": { "command": "crobot", "args": ["serve", "--mcp"] } } }
```

**Best for:** Interactive development sessions where you want to review PRs
conversationally, ask follow-up questions, iterate on findings, or combine code
review with other agent tasks.

### Toolkit (CLI Commands)

The agent (or a human) calls individual CRoBot commands
(`export-pr-context`, `apply-review-findings`, etc.) as discrete steps. Install
a skill to teach the agent the workflow automatically.

```bash
# Install the review skill for your agent
crobot export-skill --agent claude-code

# Then use /review-pr <url> in your agent session
```

**Best for:** Custom agent workflows, agents that use shell commands (via
skills/slash commands), scenarios where you need full control over each step.

## Installation

### Quick Setup (Recommended)

Run the interactive setup wizard from your project directory. It installs the
latest CRoBot binary (if missing), then walks you through platform credentials,
usage modes, agent configuration, and review settings — generating config
files, `.mcp.json`, and printing next steps. Re-runnable; existing values are
loaded as defaults.

```bash
curl -sS https://raw.githubusercontent.com/cristian-fleischer/crobot/master/scripts/setup.sh | sh
```

### From Releases

Download the latest binary from the
[Releases](https://github.com/cristian-fleischer/crobot/releases/latest) page,
or use the CLI:

```bash
# Adjust OS (linux/darwin) and ARCH (amd64/arm64) as needed
OS=linux ARCH=amd64
VERSION=$(curl -sS https://api.github.com/repos/cristian-fleischer/crobot/releases/latest | grep -oP '"tag_name":\s*"v?\K[^"]+')
curl -sL "https://github.com/cristian-fleischer/crobot/releases/latest/download/crobot_${VERSION}_${OS}_${ARCH}.tar.gz" | tar xz
sudo mv crobot /usr/local/bin/
```

### From Source

```bash
go install github.com/cristian-fleischer/crobot/cmd/crobot@latest
```

### Build from Source

```bash
git clone https://github.com/cristian-fleischer/crobot.git
cd crobot
go build -o crobot ./cmd/crobot
```

> **Built with Claude Code.** CRoBot was developed largely through pair
> programming with [Claude Code](https://claude.ai/code), Anthropic's AI
> coding agent. From architecture to implementation to code reviews, Claude
> Code has been an integral collaborator throughout this project. The
> [`.ai/`](.ai/) folder contains planning artifacts (architecture plan, task
> breakdown, implementation prompts) created during the design phase — also in
> collaboration with Claude.

## Table of Contents

- [Features](#features)
- [Use Cases](#use-cases)
- [Installation](#installation)
- [Setting Up Bitbucket Authentication](#setting-up-bitbucket-authentication)
- [Setting Up GitHub Authentication](#setting-up-github-authentication)
- [Quickstart](#quickstart)
- [How It Works](#how-it-works)
  - [Orchestrated Mode](#orchestrated-mode-crobot-review)
  - [MCP Server Mode](#mcp-server-mode-crobot-serve---mcp)
  - [CLI Toolkit Mode](#cli-toolkit-mode-skill--slash-command)
  - [Mode Comparison](#mode-comparison)
- [Commands](#commands)
  - [`export-pr-context`](#export-pr-context)
  - [`get-file-snippet`](#get-file-snippet)
  - [`list-bot-comments`](#list-bot-comments)
  - [`apply-review-findings`](#apply-review-findings)
  - [`review`](#review)
  - [`models`](#models)
  - [`serve`](#serve)
  - [`review-instructions`](#review-instructions)
  - [`export-skill`](#export-skill)
  - [`export-philosophy`](#export-philosophy)
  - [Global Flags](#global-flags)
- [ReviewFinding Schema](#reviewfinding-schema)
- [Configuration](#configuration)
- [CI/CD Integration](#cicd-integration)
- [Agent Integration](#agent-integration)
- [Debugging](#debugging)
- [Development](#development)
- [License](#license)

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

> **Security note:** If you store credentials in the config file, ensure it is
> not committed to version control. For CI environments, prefer environment
> variables and a secrets manager. Add `.crobot.yaml` to your `.gitignore` if
> using per-project config with credentials.

---

## Setting Up GitHub Authentication

CRoBot authenticates with GitHub using a **Personal Access Token (PAT)**.

### Step 1: Create a Personal Access Token

1. Go to [GitHub Settings > Developer settings > Fine-grained tokens](https://github.com/settings/tokens?type=beta)
2. Click **Generate new token**
3. Configure the token:
   - **Name**: Give it a descriptive name (e.g. "CRoBot Code Reviews")
   - **Expiration**: Set an appropriate expiry period
   - **Repository access**: Select the repositories CRoBot should access
4. Under **Repository permissions**, set:
   - **Pull requests**: Read and write (to read PRs and post review comments)
   - **Contents**: Read-only (to fetch file content at specific commits)
5. Click **Generate token**
6. **Copy the generated token immediately** -- it is only shown once

> **Note:** Classic tokens also work. They need the `repo` scope.

### Step 2: Configure CRoBot

**Environment variables (recommended for CI):**

```bash
export CROBOT_GITHUB_TOKEN="ghp_your-token-here"
export CROBOT_GITHUB_OWNER="your-org-or-username"
export CROBOT_GITHUB_REPO="your-repo"
```

**Config file (recommended for local development):**

```yaml
# ~/.config/crobot/config.yaml
platform: github

github:
  owner: your-org-or-username
  repo: your-repo
  token: ghp_your-token-here
```

---

## Quickstart

### 1. Set Up Credentials

Run `./scripts/setup.sh` for guided setup, or follow the
[Bitbucket](#setting-up-bitbucket-authentication) /
[GitHub](#setting-up-github-authentication) authentication steps manually.

### Option A: One-Command Review (Recommended)

Use `crobot review` to run an end-to-end AI-powered review with a single
command. CRoBot spawns an ACP-compatible agent, feeds it the PR diff, collects
findings, and posts inline comments.

The PR can be specified as a positional argument or via `--pr`. When a URL is
provided, the workspace, repo, and PR number are extracted automatically:

```bash
# Using a PR URL as a positional argument (simplest — works with Bitbucket and GitHub)
crobot review https://bitbucket.org/myteam/my-service/pull-requests/42
crobot review https://github.com/my-org/my-service/pull/42

# Using a PR number (requires workspace/repo from config or flags)
crobot review 42

# Post comments with live agent output (markdown formatted)
crobot review https://bitbucket.org/myteam/my-service/pull-requests/42 --write --show-agent-output

# Raw unformatted agent output
crobot review https://bitbucket.org/myteam/my-service/pull-requests/42 --show-agent-output --raw

# --pr flag also works
crobot review --pr 42

# Choose a specific model
crobot review 42 --agent-command "gemini --experimental-acp" --model gemini-2.5-pro

# Interactive model selection
crobot review 42 --model ask

# Zero-config: specify the agent binary directly (no config file needed)
CROBOT_BITBUCKET_USER="you@example.com" CROBOT_BITBUCKET_TOKEN="your-token" \
  crobot review https://bitbucket.org/myteam/my-service/pull-requests/42 --agent-command claude-agent-acp
```

The agent can be configured in your config file (see [Configuration](#configuration))
or specified directly with `--agent-command` for zero-config usage.

### Option B: Manual Workflow

For more control, use the individual CLI commands to export context, generate
findings with your own AI agent, and apply them.

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

## How It Works

CRoBot supports three integration modes. Each divides responsibilities
differently between CRoBot and the AI agent.

### Orchestrated Mode (`crobot review`)

CRoBot drives the entire review pipeline. The agent's only job is to analyze
the code and return structured findings as JSON. CRoBot handles everything
else: fetching PR data, delivering it to the agent, validating findings,
deduplicating against prior comments, and posting.

```
┌─────────────────────────────────────────────────────────────┐
│  crobot review <pr-url> --write                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Step 1  [CRoBot]   Fetch PR context from platform          │
│                      (metadata, changed files, diff hunks)  │
│                                                             │
│  Step 2  [CRoBot]   Build review prompt                     │
│                      (methodology + philosophy + PR data)   │
│                                                             │
│  Step 3  [CRoBot]   Spawn agent subprocess (ACP handshake) │
│                                                             │
│  Step 4  [CRoBot]   Send prompt to agent                    │
│                                                             │
│  Step 5  [Agent]    Read diff from prompt                   │
│                      Read full files from disk for context  │
│                                                             │
│  Step 6  [Agent]    Output JSON array of ReviewFindings     │
│                                                             │
│  Step 7  [CRoBot]   Parse findings from agent response      │
│                                                             │
│  Step 8  [CRoBot]   Review engine:                          │
│                      - Validate findings against diff       │
│                      - Deduplicate against existing comments │
│                      - Enforce max-comments / severity       │
│                                                             │
│  Step 9  [CRoBot]   Post inline comments (or dry-run)       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

The agent receives the full PR context in the prompt and does not need to call
any CRoBot commands or tools. It may optionally call `list_bot_comments` to
check for prior reviews, but must never post findings itself -- CRoBot handles
that.

### MCP Server Mode (`crobot serve --mcp`)

CRoBot runs as a passive tool server. The agent drives the entire workflow by
discovering and calling CRoBot's MCP tools. The full review methodology is
delivered to the agent automatically on connection via the MCP instructions
field -- no separate setup step is needed.

```
┌─────────────────────────────────────────────────────────────┐
│  crobot serve --mcp  (stdio transport)                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Step 1  [CRoBot]   Start MCP server, register 4 tools     │
│                      Deliver review methodology via         │
│                      MCP instructions on connect            │
│                                                             │
│  Step 2  [Agent]    Connect as MCP client                   │
│                      Receive tools + instructions           │
│                                                             │
│  Step 3  [Agent]    Call export_pr_context                   │
│          [CRoBot]   → Fetch from platform, return JSON     │
│                                                             │
│  Step 4  [Agent]    Read full files from disk for context   │
│                                                             │
│  Step 5  [Agent]    Call list_bot_comments                   │
│          [CRoBot]   → Return existing comments             │
│                                                             │
│  Step 6  [Agent]    Formulate findings                      │
│                                                             │
│  Step 7  [Agent]    Call apply_review_findings (dry_run)     │
│          [CRoBot]   → Validate, return results             │
│                                                             │
│  Step 8  [Agent]    Fix rejected findings if needed          │
│                                                             │
│  Step 9  [Agent]    Call apply_review_findings (write)       │
│          [CRoBot]   → Post comments to platform            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

The agent is in control. CRoBot is a tool provider that handles platform API
calls, validation, and deduplication on behalf of the agent.

### CLI Toolkit Mode (skill / slash command)

The agent uses CRoBot's CLI commands via shell access. A skill file bootstraps
the workflow by telling the agent to first load the review methodology via
`crobot review-instructions`, then follow the step-by-step workflow.

```
┌─────────────────────────────────────────────────────────────┐
│  /review-pr <pr-url>  (or manual invocation)                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Step 1  [Skill]    Tell agent to load instructions         │
│                                                             │
│  Step 2  [Agent]    Run: crobot review-instructions         │
│          [CRoBot]   → Print methodology to stdout          │
│                                                             │
│  Step 3  [Agent]    Run: crobot export-pr-context --pr N    │
│          [CRoBot]   → Fetch from platform, print JSON      │
│                                                             │
│  Step 4  [Agent]    Read full files from disk for context   │
│                                                             │
│  Step 5  [Agent]    Run: crobot list-bot-comments --pr N    │
│          [CRoBot]   → Print existing comments              │
│                                                             │
│  Step 6  [Agent]    Formulate findings, save to JSON file   │
│                                                             │
│  Step 7  [Agent]    Run: crobot apply-review-findings       │
│                          --dry-run --input findings.json    │
│          [CRoBot]   → Validate, print results              │
│                                                             │
│  Step 8  [Agent]    Fix rejected findings if needed          │
│                                                             │
│  Step 9  [Agent]    Run: crobot apply-review-findings       │
│                          --write --input findings.json      │
│          [CRoBot]   → Post comments to platform            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

The agent is in control, using shell commands instead of MCP tools. The skill
provides the entry point; `crobot review-instructions` provides the
methodology.

### Mode Comparison

|                          | Orchestrated (`review`) | MCP Server (`serve`)       | CLI Toolkit (skill)        |
|--------------------------|-------------------------|----------------------------|----------------------------|
| **Who orchestrates**     | CRoBot                  | Agent                      | Agent                      |
| **Who fetches PR data**  | CRoBot                  | Agent (via MCP tool)       | Agent (via CLI)            |
| **Who analyzes code**    | Agent                   | Agent                      | Agent                      |
| **Who validates**        | CRoBot                  | Agent (dry-run tool)       | Agent (dry-run CLI)        |
| **Who posts comments**   | CRoBot                  | Agent (write tool)         | Agent (write CLI)          |
| **Agent output**         | JSON text               | MCP tool calls             | Shell commands             |
| **CRoBot's role**        | Orchestrator            | Tool server                | CLI toolkit                |
| **Setup needed**         | Agent config            | `.mcp.json`                | `crobot export-skill`      |
| **Best for**             | CI/CD, automation       | Interactive sessions       | Custom agent workflows     |

> **Warning: Use one mode at a time.** Each mode assumes it owns the review
> workflow. If the agent has access to both the orchestrated mode (via
> `crobot review`) and CRoBot's MCP tools simultaneously, it may attempt to
> post findings through MCP while CRoBot also posts them from the orchestrated
> pipeline -- resulting in duplicate comments or conflicting behavior. When
> using `crobot review`, ensure the CRoBot MCP server is not also configured
> in the agent's MCP settings for the same project.

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

### `review`

Runs an end-to-end AI-powered code review on a pull request or local changes.
CRoBot spawns an ACP-compatible agent subprocess, sends it the diff with review
instructions, collects the agent's findings, validates them against the diff,
deduplicates against existing comments, and posts inline review comments.

When no PR is specified, CRoBot enters **local mode**: it diffs the working tree
(committed, staged, and unstaged changes) against a base branch and renders
findings directly in the terminal. Local mode always runs as dry-run.

```bash
# Review local changes against master (no PR needed)
crobot review

# Review local changes against a different base branch
crobot review --base main

# Using a PR URL (simplest — workspace and repo deduced from the URL)
crobot review https://bitbucket.org/myteam/my-service/pull-requests/42

# Using a PR number (requires workspace/repo from config or flags)
crobot review 42
crobot review --workspace <ws> --repo <repo> 42

# With a specific agent from config
crobot review https://bitbucket.org/myteam/my-service/pull-requests/42 --agent claude

# Post comments with formatted live agent output
crobot review https://bitbucket.org/myteam/my-service/pull-requests/42 --write --show-agent-output

# Raw unformatted output (disable markdown rendering)
crobot review https://bitbucket.org/myteam/my-service/pull-requests/42 --show-agent-output --raw

# Steer the review with additional instructions
crobot review 42 -i "focus on security issues and SQL injection"

# Use an agent command directly (no config file needed)
crobot review 42 --agent-command "gemini --experimental-acp"

# Limit comments
crobot review 42 --write --max-comments 10

# --pr flag also works (equivalent to positional arg)
crobot review --pr 42
```

The PR reference (URL or number) can be passed as a positional argument or via
`--pr`. If both are provided, an error is returned.

| Flag                  | Type   | Required | Default    | Description                                              |
|-----------------------|--------|----------|------------|----------------------------------------------------------|
| (positional)          | string | no*      |            | PR number or URL as the first positional argument        |
| `--pr`                | string | no*      |            | PR number or URL (alternative to positional argument)    |
| `--workspace`         | string | no**     |            | Workspace/organization slug                              |
| `--repo`              | string | no**     |            | Repository slug                                          |
| `--base`              | string | no       | `master`   | Base branch for local review (when no PR is specified)   |
| `--agent`             | string | no       | config     | ACP agent name (from `agent.agents` in config)           |
| `--agent-command`     | string | no       |            | ACP agent command with args, e.g. `"gemini --experimental-acp"` |
| `-m`, `--model`       | string | no       | config     | Model ID to use, or `"ask"` for interactive selection    |
| `--dry-run`           | bool   | no       | `true`     | Validate without posting (default behavior)              |
| `--write`             | bool   | no       | `false`    | Actually post comments to the PR                         |
| `--max-comments`      | int    | no       | config     | Max comments to post (`0` = unlimited)                   |
| `--show-agent-output` | bool   | no       | `false`    | Stream formatted agent output with progress indicator    |
| `--raw`               | bool   | no       | `false`    | Disable markdown formatting and progress indicator       |
| `-i`, `--instructions`| string | no       |            | Additional instructions appended to the review prompt    |

*When omitted, CRoBot enters local mode and reviews local git changes.
**Not required when a URL is used (workspace and repo are extracted from it)
or in local mode. Otherwise required unless set in config file or env vars.

The agent must be configured in your config file under `agent.agents` (see
[Configuration](#configuration)). The agent binary must be installed and
available on your `PATH`.

### `models`

Lists available models from an ACP agent. Useful for discovering which model
IDs to pass via `--model`.

```bash
# List models from a configured agent
crobot models --agent claude

# List models from an agent command
crobot models --agent-command "gemini --experimental-acp"
```

| Flag              | Type   | Required | Default | Description                                              |
|-------------------|--------|----------|---------|----------------------------------------------------------|
| `--agent`         | string | no       | config  | ACP agent name (from config)                             |
| `--agent-command` | string | no       |         | ACP agent command with args                              |

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

The server exposes the following tools: `export_pr_context`,
`get_file_snippet`, `list_bot_comments`, `export_local_context`, and
`apply_review_findings`.

### `review-instructions`

Prints the CRoBot review methodology, finding schema, workflow, and rules
to stdout. AI agents should read this output before performing a code review.

```bash
crobot review-instructions
```

This is the CLI counterpart to the MCP server's built-in instructions. MCP
agents receive these instructions automatically on connection; CLI agents
should run this command first.

### `export-skill`

Exports the CRoBot review skill for an AI agent. The skill teaches agents the
full code review workflow via a slash command.

```bash
# Print to stdout
crobot export-skill

# Install for a specific agent
crobot export-skill --agent claude-code

# Install globally (home directory)
crobot export-skill --agent claude-code --global
```

| Flag      | Type   | Required | Default | Description                                            |
|-----------|--------|----------|---------|--------------------------------------------------------|
| `--agent` | string | no       |         | Target agent: `claude-code`, `codex`, `opencode`, `generic` |
| `--global`| bool   | no       | `false` | Install to home directory (available across all projects)   |

### `export-philosophy`

Exports the default review philosophy to a file for customization. Override what
CRoBot focuses on during reviews by editing the exported file.

```bash
# Print default philosophy to stdout
crobot export-philosophy

# Save to local project override
crobot export-philosophy --local

# Save to global override
crobot export-philosophy --global
```

| Flag       | Type | Required | Default | Description                                         |
|------------|------|----------|---------|-----------------------------------------------------|
| `--local`  | bool | no       | `false` | Write to `.crobot-philosophy.md` in current directory |
| `--global` | bool | no       | `false` | Write to `~/.config/crobot/review-philosophy.md`     |

Philosophy is resolved in this order (first found wins):
1. `--review-philosophy` flag on `crobot review`
2. `review.philosophy_path` in config file
3. `CROBOT_REVIEW_PHILOSOPHY` env var
4. `.crobot-philosophy.md` in current directory
5. `~/.config/crobot/review-philosophy.md`
6. Built-in default

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
| `suggestion`  | string | no       | Replacement code (valid code only, applied verbatim) |
| `fingerprint` | string | no       | Leave empty for auto-generation                    |

**Severity levels** (highest to lowest):
- `error` -- bugs, security vulnerabilities, crashes
- `warning` -- code smells, performance issues, potential bugs
- `info` -- style suggestions, documentation improvements

---

## Configuration

> **Security Warning:** CRoBot loads `.crobot.yaml` from the **current working
> directory**. A malicious repository could include a `.crobot.yaml` that
> configures an arbitrary agent command, which `crobot review` would then
> execute. **Do not run CRoBot in untrusted or unreviewed repositories.**
> Always inspect `.crobot.yaml` before running CRoBot in a new project.

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

  # Credentials can be set here, via env vars, or CLI flags.
  # For CI, prefer environment variables (CROBOT_BITBUCKET_USER, CROBOT_BITBUCKET_TOKEN).
  user: you@example.com
  token: your-api-token

# GitHub-specific settings (used when platform: github).
github:
  # Repository owner (user or organization).
  owner: my-org

  # Default repository name.
  repo: my-service

  # GitHub personal access token (fine-grained or classic).
  token: ghp_your-token-here

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

# Agent runner settings (for `crobot review` command).
agent:
  # Default agent to use when --agent is not specified.
  default: claude

  # Default model ID to request from the agent (optional).
  # Can also be set via CROBOT_MODEL env var or --model flag.
  # model: gemini-2.5-pro

  # Named agent configurations. Each entry defines an ACP-compatible agent
  # subprocess that CRoBot can spawn.
  #
  # Most agents don't speak ACP natively. Use an ACP adapter:
  #   npm install -g @zed-industries/claude-agent-acp
  agents:
    claude:
      command: claude-agent-acp
      args: []

  # Overall timeout in seconds for the agent subprocess. Default: 300 (5 min).
  timeout: 600

# AI provider settings (Phase 5 - not yet implemented).
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
| `CROBOT_GITHUB_OWNER`          | GitHub repository owner (user or org)            |              |
| `CROBOT_GITHUB_REPO`           | GitHub repository name                           |              |
| `CROBOT_GITHUB_TOKEN`          | GitHub personal access token                     |              |
| `CROBOT_MAX_COMMENTS`          | Max comments per run                             | `25`         |
| `CROBOT_DRY_RUN`               | Default dry-run mode (`true`, `1`, `yes`)        | `true`       |
| `CROBOT_AGENT`                 | Default agent name for `crobot review`           |              |
| `CROBOT_MODEL`                 | Default model ID to request from the agent       |              |
| `CROBOT_AI_PROVIDER`           | Default AI provider (Phase 5)                    |              |
| `CROBOT_ANTHROPIC_API_KEY`     | Anthropic API key (Phase 5)                      |              |
| `CROBOT_OPENAI_API_KEY`        | OpenAI API key (Phase 5)                         |              |
| `CROBOT_GOOGLE_API_KEY`        | Google API key (Phase 5)                         |              |
| `CROBOT_OPENROUTER_API_KEY`    | OpenRouter API key (Phase 5)                     |              |

### Recommended Setup

**For local development** -- use a global config file with defaults:

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

### Using `crobot review` (Recommended)

The simplest CI integration uses `crobot review` to handle the entire flow:

**Bitbucket Pipelines:**

```yaml
- step:
    name: AI Code Review
    script:
      - export CROBOT_BITBUCKET_USER=$BITBUCKET_USER
      - export CROBOT_BITBUCKET_TOKEN=$BITBUCKET_TOKEN
      - export CROBOT_BITBUCKET_WORKSPACE=$BITBUCKET_WORKSPACE
      - export CROBOT_BITBUCKET_REPO=$BITBUCKET_REPO_SLUG
      - crobot review $BITBUCKET_PR_ID --agent claude --write
```

**GitHub Actions:**

```yaml
- name: AI Code Review
  env:
    CROBOT_BITBUCKET_USER: ${{ secrets.BITBUCKET_USER }}
    CROBOT_BITBUCKET_TOKEN: ${{ secrets.BITBUCKET_TOKEN }}
    CROBOT_BITBUCKET_WORKSPACE: myteam
    CROBOT_BITBUCKET_REPO: my-service
  run: crobot review ${{ github.event.pull_request.number }} --agent claude --write
```

### Using Individual Commands

For more control, use the step-by-step commands:

**Bitbucket Pipelines:**

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

**GitHub Actions:**

```yaml
- name: AI Code Review
  env:
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

### ACP Orchestrator (`crobot review`)

The `review` command uses the Agent Client Protocol (ACP) to spawn any
compatible agent as a subprocess, send it the PR diff and review instructions,
and collect structured findings. The agent runs in a sandboxed, read-only mode:
it can read files from the repository at the PR's head commit but cannot write
files or run terminal commands.

Most AI coding agents don't speak ACP natively. Use an ACP adapter such as
[claude-agent-acp](https://github.com/zed-industries/claude-agent-acp):

```bash
npm install -g @zed-industries/claude-agent-acp
```

Configure agents in your config file:

```yaml
agent:
  default: claude
  agents:
    claude:
      command: claude-agent-acp
      args: []
  timeout: 600
```

Then run:

```bash
crobot review https://bitbucket.org/myteam/my-service/pull-requests/42 --write
```

### MCP Server

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

The MCP server delivers the full review methodology (finding schema, workflow,
severity guidelines, rules) to the agent automatically on connection via the
MCP instructions field.

### CLI Agents (Skill / Slash Command)

For agents using CLI commands, CRoBot includes a built-in review skill that you
can install with a single command. The skill teaches the agent the full review
workflow: load instructions, perform the review (with multi-agent parallelism
when possible), and post findings.

```bash
# Install for Claude Code (current project)
crobot export-skill --agent claude-code

# Install for Claude Code (globally, all projects)
crobot export-skill --agent claude-code --global

# Install for other agents
crobot export-skill --agent codex
crobot export-skill --agent opencode
crobot export-skill --agent generic

# Print skill to stdout (inspect or pipe)
crobot export-skill
```

| Agent          | Install Path                                    |
|----------------|-------------------------------------------------|
| `claude-code`  | `.claude/skills/review-pr/SKILL.md`             |
| `codex`        | `.codex/skills/review-pr/SKILL.md`              |
| `opencode`     | `.opencode/skills/review-pr/SKILL.md`           |
| `generic`      | `.agents/skills/review-pr.md`                   |

Once installed, use the `/review-pr <pr-url-or-number>` slash command in your
agent session. The skill instructs the agent to:

1. Run `crobot review-instructions` to load the review methodology
2. Perform the review (spawning specialist sub-agents when possible)
3. Dry-run findings with `crobot apply-review-findings --dry-run`, then post
   with `--write`

The underlying review instructions are also available directly:

```bash
crobot review-instructions   # agent reads this output, then follows it
```

---

## Debugging

Use the `--verbose` (`-v`) flag to enable debug logging. CRoBot writes debug
output to stderr, so it won't interfere with JSON output on stdout.

```bash
# Verbose output for any command
crobot -v export-pr-context --pr 42
crobot -v review https://github.com/org/repo/pull/42 --write

# Capture debug logs to a file while keeping normal output
crobot -v review 42 --write 2> debug.log

# Capture everything (stdout + stderr) for a bug report
crobot -v review 42 --show-agent-output 2>&1 | tee review-debug.log
```

Debug logging includes:
- Config resolution (which config files loaded, platform selected)
- HTTP requests to the platform API (URLs, status codes, retries)
- Rate limit handling and backoff timing
- Agent subprocess lifecycle (spawn, initialize, prompt, shutdown)
- Finding validation and deduplication decisions
- Comment posting results

When reporting issues, include the full verbose output:

```bash
crobot -v <your-command> 2>&1 | tee debug.log
# Attach debug.log to your issue at https://github.com/cristian-fleischer/crobot/issues
```

> **Note:** Verbose output may contain workspace/repo names and PR numbers but
> never prints credentials (tokens or passwords).

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

### Releasing

Releases are automated via [GoReleaser](https://goreleaser.com/) and GitHub
Actions. Pushing a version tag triggers a workflow that runs tests, cross-compiles
binaries for all platforms, and publishes a GitHub release.

```bash
# 1. Bump the version in internal/version/version.go
# 2. Commit the change
git add internal/version/version.go
git commit -m "chore: bump version to 0.3.15-alpha"

# 3. Tag and push
git tag v0.3.15-alpha
git push origin master v0.3.15-alpha
```

The release workflow builds binaries for:

| OS      | Architectures  | Format |
|---------|---------------|--------|
| Linux   | amd64, arm64  | tar.gz |
| macOS   | amd64, arm64  | tar.gz |
| Windows | amd64, arm64  | zip    |

Releases appear at
[github.com/cristian-fleischer/crobot/releases](https://github.com/cristian-fleischer/crobot/releases)
with checksums for verification.

> **Note:** The version is defined in source (`internal/version/version.go`).
> Do not override it with `-ldflags` during builds.

## License

MIT
