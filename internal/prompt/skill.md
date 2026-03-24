---
name: review-pr
description: Run an AI-powered code review on a pull request or local changes using CRoBot
argument-hint: [pr-url-or-number]
---

# CRoBot Code Review

You are performing a code review using CRoBot, a CLI tool that handles PR data
fetching, finding validation, deduplication, and comment posting.

This skill supports two modes:
- **PR review** (primary): Review a pull request by URL or number.
- **Local review**: Review uncommitted local changes before pushing. Invoked when
  no PR argument is provided.

## Step 1: Load Review Instructions

IMPORTANT: You MUST run the following command and read its output carefully
before proceeding. It contains the review methodology, required JSON schema,
severity guidelines, workflow steps, and rules.

```bash
crobot review-instructions
```

## Step 2: Perform the Review

Follow the workflow from the instructions above to review the code changes.

If `$ARGUMENTS` is provided, review that PR. If no argument is provided, review
local changes (committed, staged, and unstaged) against the base branch.

If you have the ability to spawn a team of agents (preferred) or parallel sub-agents or background agents, use them to
parallelize the review — one specialist per concern (security, architecture,
logic, performance, data integrity, and so on). Collect and merge their findings into a
single consolidated output.

## Step 3: Post Findings

Always dry-run first to validate, then post with `--write` if the dry-run
passes. For local reviews, findings are rendered to the terminal only (no PR to
post to).
