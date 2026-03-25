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

- **Security** — injection vectors, auth issues, credential exposure, insecure
  defaults, input validation at trust boundaries
- **Logic & correctness** — bugs, off-by-one errors, nil/null dereferences,
  race conditions, edge cases, error handling gaps
- **Architecture** — pattern violations, misplaced responsibilities, tight
  coupling, broken abstractions, API contract consistency
- **Performance** — hot-path complexity, unbounded allocations, N+1 queries,
  missing indexes, unnecessary work in loops
- **Data integrity** — missing transactions, partial writes, inconsistent state,
  schema mismatches

Include other specialists if there are other review concerns that need to be
addressed. Give each sub-agent the diff context from Step 1, the ReviewFinding
schema, their focus area, and filesystem access for full file reads.

You (the lead agent) should:
1. Distribute the review to specialists with their focus area.
2. Collect findings, deduplicate, resolve conflicts, remove low-quality items.
3. Add any cross-cutting findings the specialists missed.
4. Produce the final consolidated JSON array of ReviewFinding objects.

If you cannot spawn sub-agents, review all concerns yourself in a single pass.

### 5. Formulate findings

Produce a JSON array of `ReviewFinding` objects using the schema above.
Save it to a temporary file (e.g., `/tmp/review-findings.json`).

- Findings must reference lines **within the diff** (not unchanged code).
- Use `"side": "new"` for added/modified lines, `"old"` for deleted lines.
- Include a `suggestion` with **valid code only** (applied verbatim).
- Leave `fingerprint` empty (CRoBot auto-generates).
- Quality over quantity — zero findings is valid if the code is sound.

### 6. Validate and post (PR mode) or render (local mode)

**For a PR** — always dry-run first:
```bash
crobot apply-review-findings --pr <number> --input /tmp/review-findings.json --dry-run
```

If validation fails, fix findings and re-run. When dry-run passes:
```bash
crobot apply-review-findings --pr <number> --input /tmp/review-findings.json --write
```

**For local changes:**
```bash
crobot apply-review-findings --local --input /tmp/review-findings.json
```

This validates findings against the local diff and renders them in the terminal
with diff context. No PR is needed.
