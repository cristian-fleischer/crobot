## CLI Commands Reference

`--workspace` and `--repo` are optional if set in the config file or via
`CROBOT_BITBUCKET_WORKSPACE` / `CROBOT_BITBUCKET_REPO` environment variables.

### Export Context
```bash
# PR context
crobot export-pr-context --pr <number>

# Local changes (no PR needed)
crobot export-pr-context --local
crobot export-pr-context --local --base main
```
Output: JSON with metadata, changed files, and a `diff_dir` pointing to
per-file diffs on disk. Read `.crobot/diffs-<run-id>/.crobot-index.md` for the
file index.

### Get File Snippet
```bash
crobot get-file-snippet --workspace <ws> --repo <repo> \
  --commit <hash> --path <file> --line <n> --context <lines>
```
Use `head_commit` from the exported context. Default context: 5 lines.

### List Bot Comments
```bash
crobot list-bot-comments --pr <number>
```

### List PR Comments
```bash
# All inline comments
crobot list-pr-comments --pr <number>

# Only unresolved comments
crobot list-pr-comments --pr <number> --unresolved
```

### Apply Review Findings
```bash
# Dry run (default — always do this first)
crobot apply-review-findings --pr <number> --input findings.json --dry-run

# Post comments
crobot apply-review-findings --pr <number> --input findings.json --write

# Local mode (validate and render to terminal)
crobot apply-review-findings --local --input findings.json
```
