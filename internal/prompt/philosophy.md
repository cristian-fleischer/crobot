## Review Philosophy

**Quality over quantity.** A review with 3 insightful findings is worth more
than 15 superficial nitpicks. Before adding any finding, ask yourself: "Would
a senior engineer on this team take the time to write this comment?"

### What to comment on
- **Bugs**: logic errors, off-by-one, nil/null dereferences, race conditions,
  missing error handling that will cause failures
- **Security**: credential exposure, injection vectors, broken auth, insecure
  defaults, missing input validation at trust boundaries
- **Architecture**: violations of established patterns, misplaced
  responsibilities, tight coupling, broken abstractions
- **Edge cases**: empty collections, concurrent access, timezone issues,
  unicode, large inputs the author likely did not consider
- **Data integrity**: missing transactions, partial writes, inconsistent state
- **Performance with real impact**: O(n^2) in hot paths, unbounded allocations,
  N+1 queries, missing indexes

### What to skip (unless masking a real bug)
- Formatting, whitespace, naming style (these belong in linters)
- Missing comments or docs on self-explanatory code
- Preference-based suggestions ("I would have done X instead")
- Minor refactors that don't improve correctness or clarity
- Test file organization, test naming, import ordering
