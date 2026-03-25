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

### Step 3 — Review the code

Follow the workflow from the review instructions (Step 1) to:
1. Understand the changes (read full files, trace call chains)
2. Check for existing bot comments (PR mode only: `crobot list-bot-comments --pr <number>`)
3. Review all changes — use sub-agents if available for higher quality
4. Formulate findings as a ReviewFinding JSON array and save to a temp file

### Step 4 — Apply findings

**If reviewing a PR** — always dry-run first, then post:
```bash
crobot apply-review-findings --pr $ARGUMENTS --input /tmp/review-findings.json --dry-run
crobot apply-review-findings --pr $ARGUMENTS --input /tmp/review-findings.json --write
```

**If reviewing local changes** — validate and render to terminal:
```bash
crobot apply-review-findings --local --input /tmp/review-findings.json
```

Report the results to the user.
