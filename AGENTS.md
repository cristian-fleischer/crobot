# CRoBot - Code Review Bot

This repository contains CRoBot, a local-first CLI tool for AI-powered code
reviews on pull requests. When performing code reviews, use the CRoBot CLI
commands -- never call platform APIs directly.

## Quick Reference

See `.ai/agent-instructions.md` for the full canonical reference. Key points
below.

## Commands

`--workspace` and `--repo` are optional if set in the config file or via
`CROBOT_BITBUCKET_WORKSPACE` / `CROBOT_BITBUCKET_REPO` environment variables.

```bash
# 1. Export PR context (metadata, changed files, diff hunks)
crobot export-pr-context --workspace <ws> --repo <repo> --pr <number>

# 2. Get file snippet at a commit
crobot get-file-snippet --workspace <ws> --repo <repo> \
  --commit <hash> --path <file> --line <n> --context <lines>

# 3. List existing bot comments
crobot list-bot-comments --workspace <ws> --repo <repo> --pr <number>

# 4. Apply findings (dry-run first, then --write)
crobot apply-review-findings --workspace <ws> --repo <repo> --pr <number> \
  --input findings.json --dry-run
crobot apply-review-findings --workspace <ws> --repo <repo> --pr <number> \
  --input findings.json --write
```

## ReviewFinding Schema

```json
[
  {
    "path": "src/auth.ts",
    "line": 42,
    "side": "new",
    "severity": "warning",
    "severity_score": 7,
    "category": "security",
    "criteria": ["Security", "Maintainability"],
    "message": "Logging the raw token can leak credentials.",
    "suggestion": "logger.info(\"Token received\", { tokenPrefix: token.slice(0, 4) })",
    "fingerprint": ""
  }
]
```

Fields: `path` (string, required), `line` (int >0, required), `side` ("new" or
"old", required), `severity` ("info"/"warning"/"error", required),
`severity_score` (int 1-10, optional -- 0 or omit to hide), `category`
(string, required), `criteria` ([]string, optional -- failed quality criteria),
`message` (string, required), `suggestion` (string, optional -- **valid code
only**, no prose; applied verbatim as a code replacement), `fingerprint`
(string, optional -- leave empty for auto-generation).

## Workflow

1. `export-pr-context` to get the PR data
2. Analyze diffs, use `get-file-snippet` for additional context
3. `list-bot-comments` to check for existing reviews
4. `apply-review-findings --dry-run` to validate
5. `apply-review-findings --write` to post

## Rules

- Always dry-run before writing
- Only comment on lines within the PR diff
- Never call platform APIs directly -- use CRoBot commands
- Prioritize high-severity findings; use `--max-comments` to limit
- Leave `fingerprint` empty (CRoBot auto-generates)
- Severity: error (bugs/security) > warning (smells/perf) > info (style/docs)
