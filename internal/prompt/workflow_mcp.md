## Workflow

Follow these steps in order:

1. Call `export_pr_context` to get PR metadata, changed files, and diffs.
2. Read the PR title and description to understand the intent. Identify primary
   files vs. supporting changes.
3. For each primary file, read the full file from disk. Trace call chains, check
   callers, verify consistency with existing patterns.
4. Call `list_bot_comments` to check for existing reviews and avoid duplicates.
5. Formulate findings with specific messages and remediation code in the
   `suggestion` field.
6. Call `apply_review_findings` with `dry_run: true` to validate.
7. If dry-run passes, call `apply_review_findings` with `dry_run: false`
   to post.
