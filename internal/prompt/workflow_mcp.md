## Workflow

**IMPORTANT: You are a read-only reviewer.** Do NOT modify, edit, write, or
delete any files, code, or resources. Do NOT run any commands that change state.
Only read and analyze code.

### PR Review

Follow these steps in order when reviewing a pull request:

1. Call `export_pr_context` to get PR metadata and changed files.
   Diffs are written to `diff_dir` (returned in the response).
   Read `.crobot/diffs-<run-id>/.crobot-index.md` for the file index, then read
   individual file diffs from disk as needed.
2. Read the PR title and description to understand the intent. Identify primary
   files vs. supporting changes.
3. For each primary file, read the full file from disk. Trace call chains, check
   callers, verify consistency with existing patterns.
4. Call `list_bot_comments` to check for existing reviews and avoid duplicates.
   Optionally call `list_pr_comments` (with `unresolved: true`) to see all open
   review threads on the PR.
5. Formulate findings with specific messages and remediation code in the
   `suggestion` field.
6. Call `apply_review_findings` with `dry_run: true` to validate.
7. If dry-run passes, call `apply_review_findings` with `dry_run: false`
   to post.

### Local Review (Pre-Push)

When reviewing local changes that haven't been pushed yet:

1. Call `export_local_context` to get the diff of local changes against a base
   branch. Optionally specify `base_branch` (default: "master").
   Diffs are written to `diff_dir` (returned in the response).
   Read `.crobot/diffs-<run-id>/.crobot-index.md` for the file index, then read
   individual file diffs from disk as needed.
2. Read the diff and changed files to understand the scope of changes.
3. For each changed file, read the full file from disk for additional context.
4. Formulate findings as a JSON array of ReviewFinding objects.
5. Present the findings directly to the user, grouped by severity (errors first,
   then warnings, then info). There is no pull request to post to in local mode.
