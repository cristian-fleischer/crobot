# CRoBot — Implementation Handoff Prompt

You are tasked with building **CRoBot**, a local-first CLI tool written in Go
that enables AI-powered automated code reviews on pull requests. The full
architecture, design decisions, API details, type definitions, and project
structure have already been planned. Your job is to implement it.

---

## Source of Truth

All requirements, architecture, interfaces, types, configuration schemas, and
API details are defined in these two files. Read them completely before writing
any code:

1. **`.ai/plan.md`** — Full architecture and design specification. Contains:
   the 4-phase overview, design principles, testing philosophy, architecture
   diagram, configuration schema (YAML + env vars), `Platform` interface and
   all shared Go types (`PRRequest`, `PRContext`, `ChangedFile`, `DiffHunk`,
   `ReviewFinding`, `InlineComment`, `Comment`), Bitbucket Cloud API endpoints
   and payload formats, review engine design (validation, deduplication,
   rendering, rate limiting), Phase 1 CLI commands with usage examples, Phase 2
   MCP server design, Phase 3 ACP orchestrator design, Phase 4 AI provider
   interface, project directory structure, and the `review` command flow.

2. **`.ai/tasks.md`** — Task breakdown organized by phase (P1–P4) with 2-level
   subtasks. This is your implementation checklist. Update task statuses as you
   work (`[ ]` → `[~]` → `[x]`).

**Do not deviate from the plan without explicit justification.** If you
encounter a design decision not covered by the plan, make a reasonable choice
consistent with the stated design principles and document it as a comment in
the relevant code.

---

## Scope

**Implement Phase 1 (MVP) completely.** This includes:

- Project scaffolding (Go module, directory structure, cobra CLI skeleton)
- Configuration system (YAML config loading, env vars, flag layering)
- Platform abstraction layer (interface, shared types, factory)
- Bitbucket Cloud implementation (HTTP client, auth, all Platform methods)
- Review engine (validation, deduplication, rendering, orchestration)
- CLI commands (`export-pr-context`, `get-file-snippet`, `list-bot-comments`,
  `apply-review-findings`)
- Test infrastructure and comprehensive test suite
- Agent instruction files (`CLAUDE.md`, `OPENCODE.md`, `AGENTS.md`,
  `.ai/agent-instructions.md`)
- README with quickstart and usage examples

Phases 2–4 are **out of scope** for this implementation round. However, all
Phase 1 code must be structured so that Phases 2–4 can be added without
rewriting existing code (this is a core design principle — verify it).

---

## How to Work

### Structured Execution

Work through `.ai/tasks.md` in order, section by section (P1.1 → P1.2 → ...
→ P1.8). Within each section, complete tasks sequentially — each builds on the
previous. Update `.ai/tasks.md` as you go: mark tasks `[~]` when you start
them and `[x]` when complete.

### Parallel Sub-Agents

Where tasks are independent, spawn sub-agents to work in parallel. Good
candidates for parallelization:

- **P1.3 (Platform abstraction)** and **P1.2 (Configuration)** can be built in
  parallel after P1.1 scaffolding is done.
- **P1.5 (Review engine)** can be built in parallel with **P1.4 (Bitbucket
  implementation)** since they share types but not code.
- **P1.8 (Agent instruction files)** can be written in parallel with P1.7
  (testing & quality) since instruction files are documentation, not code.
- Within **P1.4**, the diff parser (P1.4.3) and file content retrieval (P1.4.4)
  are independent of each other.

**Always** keep one sub-agent dedicated to code review (see below).

### Mandatory Code Review Loop

After completing each task section (P1.1, P1.2, P1.3, etc.), run a dedicated
**code review sub-agent** that performs a thorough review of the code just
written. This is not optional — it is a required step in the workflow.

The review sub-agent must check:

1. **Correctness against requirements**: Does the code match what `.ai/plan.md`
   specifies? Are all interface methods implemented? Do types match the
   definitions? Are all flags, env vars, and config fields wired correctly?

2. **Test quality**: Tests must verify actual functionality, not just exercise
   code paths for coverage metrics. Specifically:
   - Do tests assert meaningful behavior and edge cases?
   - Do tests verify error conditions and boundary cases?
   - Are table-driven tests used where appropriate?
   - Would a subtle bug (e.g., off-by-one in line range validation, wrong
     diff side mapping, missing pagination) actually be caught by these tests?
   - Are tests independent and parallelizable (`t.Parallel()`)?
   - Do mock servers / recorded responses reflect realistic Bitbucket API
     responses, not just minimal stubs that happen to make tests pass?
   - **Red flag**: If every test passes trivially or tests mirror the
     implementation logic rather than testing observable behavior, the tests
     are inadequate.

3. **Go standards and idioms**: The code must follow generally accepted Go
   conventions:
   - Effective Go (https://go.dev/doc/effective_go) and Go Code Review
     Comments (https://go.dev/wiki/CodeReviewComments)
   - Error handling: no swallowed errors, use `fmt.Errorf` with `%w` for
     wrapping, check errors immediately
   - Naming: `MixedCaps`, not underscores; short variable names in small
     scopes; acronyms as consistent case (`URL`, `HTTP`, `PR`, not `Url`)
   - Package design: small, focused packages; no circular dependencies; avoid
     package-level state
   - Interfaces: accept interfaces, return structs; keep interfaces small;
     define interfaces where they are consumed, not where they are implemented
   - Context: pass `context.Context` as first parameter; respect cancellation
   - Concurrency: no goroutine leaks; use `sync.WaitGroup` or `errgroup`
     properly; protect shared state
   - Struct initialization: use named fields, not positional
   - Comments: exported symbols must have doc comments starting with the
     symbol name
   - No `init()` functions unless absolutely necessary
   - Use `io.Reader`/`io.Writer` for I/O abstraction
   - Prefer standard library over external dependencies when reasonable

4. **Architecture consistency**: Does the code maintain clean layer separation?
   Could Phases 2–4 be added without modifying this code? Are there any
   hardcoded assumptions that would break extensibility?

**The review loop**: After the review sub-agent produces comments, the
implementing agent must address every pertinent comment by modifying the code.
Then the review sub-agent reviews again. This loop continues until the review
produces no meaningful findings. Only then move to the next task section.

A "meaningful finding" is one that identifies an actual deficiency — a bug, a
missing requirement, a test gap, an idiom violation, or an architecture
concern. Stylistic nitpicks or subjective preferences that don't affect
correctness or maintainability do not count.

---

## Testing Requirements

Read the "Testing Philosophy" section in `.ai/plan.md` carefully. Key rules:

- **Every package gets tests.** No exceptions.
- **`go test ./...` must pass** after every task section. Run it. If it fails,
  fix it before moving on.
- **No external dependencies for default test run.** Use HTTP mock servers,
  recorded responses, golden files, and test fixtures — never real API calls.
- **Store test fixtures** in `testdata/` directories alongside the packages.
- **Golden file tests** for comment rendering (store expected output, compare).
- **Table-driven tests** for validation, deduplication, and config layering.
- **Integration tests** behind `//go:build integration` build tags for anything
  that would hit real external services.

---

## Go Module and Dependencies

- Module path: use an appropriate module path (check if one exists in `go.mod`,
  otherwise use `github.com/dizzyc/crobot` or similar).
- Use `cobra` for CLI framework.
- Use `gopkg.in/yaml.v3` for YAML parsing (or `github.com/goccy/go-yaml`).
- Prefer the Go standard library for HTTP, JSON, testing, etc.
- Minimize external dependencies. Every dependency must justify its inclusion.

---

## Output Expectations

When you are done with Phase 1, the following must be true:

1. `go build ./...` compiles with zero errors.
2. `go test ./...` passes with zero failures.
3. `go vet ./...` reports no issues.
4. Every task in P1.1 through P1.8 in `.ai/tasks.md` is marked `[x]`.
5. The binary can be invoked: `go run ./cmd/crobot --help` shows all commands.
6. A dry-run review works end-to-end with mock/recorded data (no real
   Bitbucket API needed).
7. All exported symbols have doc comments.
8. README.md exists with quickstart instructions.
9. Agent instruction files exist and are consistent with each other.

---

## What NOT to Do

- Do not implement Phases 2, 3, or 4. Only Phase 1.
- Do not add GitHub or GitLab platform adapters. Bitbucket Cloud only.
- Do not add dependencies you don't need. No frameworks for things the
  standard library handles.
- Do not write tests that just assert "no error" without checking the actual
  output. Tests must verify behavior.
- Do not skip the code review loop. It is mandatory after each task section.
- Do not mark a task complete until tests pass and the review loop is clean.
- Do not modify `.ai/plan.md`. It is the specification, not a living document
  for this implementation round.
