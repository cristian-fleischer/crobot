## Multi-Agent Review (Recommended)

If you have the ability to spawn a team of agents (preferred) or parallel sub-agents
or background agents, use them to parallelize this review. A team-based review produces
higher-quality results than a single pass.

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

1. **Distribute the review** — give each specialist the PR diff and its focus
   area. All specialists share the same ReviewFinding schema and rules.
2. **Oversee and merge** — collect findings from all specialists, deduplicate,
   resolve conflicts, and remove low-quality items.
3. **Add high-level findings** — if you notice cross-cutting concerns that no
   specialist caught (e.g., the PR's overall approach is flawed, or test
   coverage is missing), add your own findings.
4. **Produce the final output** — a single consolidated JSON array of
   ReviewFinding objects.

If you cannot spawn sub-agents, perform the review yourself following the
standard single-agent workflow below.
