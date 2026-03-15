## CLI Commands

`--workspace` and `--repo` are optional if set in the config file or via
`CROBOT_BITBUCKET_WORKSPACE` / `CROBOT_BITBUCKET_REPO` environment variables.

### Export PR Context
```bash
crobot export-pr-context --workspace <ws> --repo <repo> --pr <number>
```
Output: JSON with PR metadata, changed files, and diff hunks.

### Get File Snippet
```bash
crobot get-file-snippet --workspace <ws> --repo <repo> \
  --commit <hash> --path <file> --line <n> --context <lines>
```
Use `head_commit` from the PR context. Default context: 5 lines.

### List Bot Comments
```bash
crobot list-bot-comments --workspace <ws> --repo <repo> --pr <number>
```

### Apply Review Findings
```bash
# Dry run (default)
crobot apply-review-findings --workspace <ws> --repo <repo> --pr <number> \
  --input findings.json --dry-run

# Post comments
crobot apply-review-findings --workspace <ws> --repo <repo> --pr <number> \
  --input findings.json --write

# Read from stdin with comment cap
echo '<json>' | crobot apply-review-findings --workspace <ws> --repo <repo> \
  --pr <number> --input - --write --max-comments 10
```
