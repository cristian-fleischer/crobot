## Multi-Agent Review

If you can spawn sub-agents, distributing the review to specialist agents
produces higher-quality results than a single pass. This is optional but
suggested when your environment supports it.

### Recommended Team Structure

Create one specialist agent per review concern. Each specialist reviews the
entire diff through its specific lens:

- **Security specialist** — injection vectors, auth issues, credential
  exposure, insecure defaults, input validation at trust boundaries
- **Architecture specialist** — pattern violations, misplaced responsibilities,
  tight coupling, broken abstractions, API contract consistency
- **Logic & correctness specialist** — bugs, off-by-one errors, nil/null
  dereferences, race conditions, edge cases, error handling gaps
- **Performance specialist** — hot-path complexity, unbounded allocations,
  N+1 queries, missing indexes, unnecessary work in loops
- **Data integrity specialist** — missing transactions, partial writes,
  inconsistent state, schema mismatches

Include other agents if there are other review concerns that need to be addressed.

You (the lead agent) should:

1. **Distribute the review** — give each specialist the diff context and its
   focus area. All specialists share the same ReviewFinding schema and rules.
2. **Oversee and merge** — collect findings from all specialists, deduplicate,
   resolve conflicts, and remove low-quality items.
3. **Add high-level findings** — if you notice cross-cutting concerns that no
   specialist caught (e.g., the PR's overall approach is flawed, or test
   coverage is missing), add your own findings.
4. **Produce the final output** — a single consolidated JSON array of
   ReviewFinding objects.
