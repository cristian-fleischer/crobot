---
name: review-pr
description: Run an AI-powered code review on a pull request or local changes using CRoBot
argument-hint: [pr-url-or-number]
license: MIT
---

# CRoBot Code Review

You are performing a code review using CRoBot. **Do NOT use `crobot review`** —
that command is for non-interactive automated pipelines.

## Step 1 — Load instructions and follow the workflow

Run this command and read the output carefully:

```bash
crobot review-instructions
```

It contains the complete workflow (numbered steps with exact commands), the
ReviewFinding JSON schema, review rules, and review philosophy. **Follow the
workflow from the instructions exactly.**

## Step 2 — Determine PR or local mode

The workflow starts with exporting context. Use `$ARGUMENTS` to determine mode:

- **`$ARGUMENTS` is a PR URL or number** → use `--pr $ARGUMENTS` on export and apply commands
- **`$ARGUMENTS` is empty** → use `--local` on export and apply commands
