---
name: review-pr
description: Run an AI-powered code review on a pull request or local changes using CRoBot
argument-hint: [pr-url-or-number]
license: MIT
---

# CRoBot Code Review

You are performing a code review using CRoBot. CRoBot handles data fetching,
finding validation, deduplication, and comment posting. You do the actual review.

**Do NOT use `crobot review`.** That command is for non-interactive automated
pipelines. Follow the procedure below.

## Procedure

Follow these steps **in order**. Do not skip steps.

### Step 1 — Load review instructions

Run this command and read the output carefully. It contains the ReviewFinding
JSON schema, severity guidelines, rules, review philosophy, and the detailed
workflow with all CRoBot commands.

```bash
crobot review-instructions
```

### Step 2 — Export context

**If `$ARGUMENTS` is a PR URL or number:**
```bash
crobot export-pr-context --pr $ARGUMENTS
```

**If `$ARGUMENTS` is empty (local review):**
```bash
crobot export-pr-context --local
```

This writes per-file diffs to `.crobot/diffs-<run-id>/` and returns JSON with a
`diff_dir` field. Read the diff index and individual file diffs from that directory.

### Step 3 — Review and post

Follow the workflow from Step 1 to review the code and produce findings.
The workflow covers: understanding the changes, reviewing with optional
sub-agents, formulating findings, and validating/posting them via CRoBot.
