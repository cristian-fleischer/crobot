---
name: review-pr
description: Run an AI-powered code review on a pull request using CRoBot
argument-hint: <pr-url-or-number>
---

# CRoBot Code Review

You are performing a code review using CRoBot, a CLI tool that handles PR data
fetching, finding validation, deduplication, and comment posting.

## Step 1: Load Review Instructions

IMPORTANT: You MUST run the following command and read its output carefully
before proceeding. It contains the review methodology, required JSON schema,
severity guidelines, workflow steps, and rules.

```bash
crobot review-instructions
```

## Step 2: Perform the Review

Follow the workflow from the instructions above to review PR $ARGUMENTS.

If you have the ability to spawn a team of agents (preferred) or parallel sub-agents or background agents, use them to
parallelize the review — one specialist per concern (security, architecture,
logic, performance, data integrity, and so on). Collect and merge their findings into a
single consolidated output.

## Step 3: Post Findings

Always dry-run first to validate, then post with `--write` if the dry-run
passes.
