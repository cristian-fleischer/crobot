## Workflow

Follow these steps in order:

1. Run `crobot export-pr-context` to get PR metadata, changed files, and diffs.
2. Read the PR title and description to understand the intent. Identify primary
   files vs. supporting changes.
3. For each primary file, read the full file from disk. Trace call chains, check
   callers, verify consistency with existing patterns.
4. Run `crobot list-bot-comments` to check for existing reviews and avoid
   duplicates.
5. Formulate findings with specific messages and remediation code in the
   `suggestion` field. Save as JSON.
6. Run `crobot apply-review-findings --dry-run` to validate.
7. If dry-run passes, run `crobot apply-review-findings --write` to post.
