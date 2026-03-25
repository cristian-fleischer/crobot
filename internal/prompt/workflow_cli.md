## Workflow

**IMPORTANT: Do NOT use `crobot review`.** That command is for non-interactive
automated pipelines. You must follow the steps below using `export-pr-context`
and `apply-review-findings`.

Follow these steps in order:

### 1. Export context

**For a PR:**
```bash
crobot export-pr-context --pr <number>
```

**For local changes (no PR):**
```bash
crobot export-pr-context --local
crobot export-pr-context --local --base main   # if base branch is not master
```

Both modes write per-file diffs to `.crobot/diffs-<run-id>/` and return JSON
with a `diff_dir` field. Read `.crobot/diffs-<run-id>/.crobot-index.md` for the
file index, then read individual file diffs as needed.

### 2. Understand the changes

- Read the **title and description** (from PR metadata or git log) to understand
  the author's intent.
- Identify **primary files** (business logic) vs. **supporting files** (tests,
  config, docs).
- For each primary changed file, **read the full file from disk**. Trace call
  chains, check callers, verify consistency with existing patterns.

### 3. Check for existing comments (PR mode only)

```bash
crobot list-bot-comments --pr <number>
```

Skip this step for local reviews.

### 4. Review the code

Review all changes through these lenses: security, correctness, architecture,
performance, data integrity, error handling.

**If you can spawn sub-agents**, distribute the review to specialist agents in
parallel — this produces higher-quality results than a single pass:

- **Security** — injection, auth, credential exposure, input validation
- **Logic & correctness** — bugs, null dereferences, edge cases, error handling
- **Architecture** — pattern violations, coupling, misplaced responsibilities
- **Performance** — hot-path complexity, N+1 queries, unbounded allocations
- **Data integrity** — missing transactions, partial writes, inconsistent state

Give each sub-agent the diff context from Step 1, the ReviewFinding schema, their
focus area, and filesystem access for full file reads. Collect all findings,
deduplicate, resolve conflicts, and remove low-quality items. Add any
cross-cutting findings the specialists missed.

If you cannot spawn sub-agents, review all concerns yourself in a single pass.

### 5. Formulate findings

Produce a JSON array of `ReviewFinding` objects using the schema above.
Save it to a temporary file (e.g., `/tmp/review-findings.json`).

- Findings must reference lines **within the diff** (not unchanged code).
- Use `"side": "new"` for added/modified lines, `"old"` for deleted lines.
- Include a `suggestion` with **valid code only** (applied verbatim).
- Leave `fingerprint` empty (CRoBot auto-generates).
- Quality over quantity — zero findings is valid if the code is sound.

### 6. Validate and post (PR mode) or present (local mode)

**For a PR** — always dry-run first:
```bash
crobot apply-review-findings --pr <number> --input /tmp/review-findings.json --dry-run
```

If validation fails, fix findings and re-run. When dry-run passes:
```bash
crobot apply-review-findings --pr <number> --input /tmp/review-findings.json --write
```

**For local changes** — there is no PR to post to. Present the findings
directly to the user, grouped by severity (errors first).
