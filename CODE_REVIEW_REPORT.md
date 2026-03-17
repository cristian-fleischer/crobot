# CRoBot Pre-Release Code Review Report

**Date:** 2026-03-17
**Version:** 0.3.13-alpha (commit a89a64a, branch master)
**Codebase:** ~15,500 lines Go across 68 files
**Status:** All tests pass. `go vet` clean. Zero compilation warnings.
**Review Team:** Security Engineer, Go Language Expert, Software Architect,
QA/Testing Expert, Readability Expert, Master Reviewer (Go expert & pragmatic
team lead)

---

## Executive Summary

CRoBot is a well-architected Go CLI application that demonstrates many
production-quality patterns: clean interface abstractions, composable review
pipelines, embedded prompts, layered configuration, and strong test coverage.
It is suitable as a Go learning example with targeted improvements.

This report consolidates findings from six specialist reviewers into a
prioritized action list. Findings are classified as:

- **P0 (Must Fix)** -- Blocks open-source release. Security or correctness issues.
- **P1 (Should Fix)** -- Significant quality issues. Affects perception as exemplary Go code.
- **P2 (Nice to Have)** -- Polish items that improve learning value.

**Summary counts:** 7 P0, 12 P1, 12 P2

---

## Table of Contents

- [P0: Must Fix Before Release](#p0-must-fix-before-release)
- [P1: Should Fix](#p1-should-fix)
- [P2: Nice to Have](#p2-nice-to-have)
- [What the Codebase Does Well](#what-the-codebase-does-well)
- [Test Coverage Summary](#test-coverage-summary)
- [Appendix: File-Level Notes](#appendix-file-level-notes)

---

## P0: Must Fix Before Release

### P0-1: Agent Permission Auto-Approve Fallback is Dangerous

**File:** `internal/agent/session.go:560-563`
**Category:** Security
**Reviewer:** Security

The `handlePermission` method falls back to approving the **first available
option** when no "allow" or "always_allow" option is found. A malicious or
misbehaving agent could request destructive permissions (file writes, shell
access) and they would be silently approved.

```go
// CURRENT (dangerous fallback)
if optionID == "" && len(req.Options) > 0 {
    optionID = req.Options[0].OptionID
    fallback = true
}
```

**Fix:** Remove the fallback. If no allow/always_allow option exists, cancel
the permission request:

```go
if optionID == "" {
    slog.Warn("agent: no allow option found, cancelling permission request")
    return map[string]any{
        "outcome": map[string]string{"outcome": "cancelled"},
    }, nil
}
```

---

### P0-2: URL Path Injection in Bitbucket API Calls

**Files:** `internal/platform/bitbucket/pr.go:60`, `file.go:19-20`,
`comments.go:52-53,120-121,178-179`
**Category:** Security
**Reviewer:** Security

Workspace, repo, commit, path, and comment ID values are interpolated directly
into URL paths via `fmt.Sprintf` without URL-encoding. The MCP server accepts
these values from untrusted AI agent input. A crafted `repo` value like
`../../other-repo` or a `path` containing `?` or `#` could alter the API
request target.

```go
// CURRENT
path := fmt.Sprintf("/2.0/repositories/%s/%s/src/%s/%s",
    workspace, opts.Repo, opts.Commit, opts.Path)
```

**Fix:** Use `url.PathEscape()` for each interpolated segment:

```go
path := fmt.Sprintf("/2.0/repositories/%s/%s/src/%s/%s",
    url.PathEscape(workspace),
    url.PathEscape(opts.Repo),
    url.PathEscape(opts.Commit),
    url.PathEscape(opts.Path))
```

Alternatively, add input validation (alphanumeric + hyphens for workspace/repo,
hex for commit hashes) at the Platform interface boundary.

---

### P0-3: Unsanitized Commit Hash in `git show` Argument

**File:** `internal/agent/fs.go:66`
**Category:** Security
**Reviewer:** Security

The `headCommit` field is used directly in a `git show` argument without
validation. While `exec.Command` prevents shell injection, a value containing
`:` or other git revision specifiers could reference unexpected objects.

```go
ref := fmt.Sprintf("%s:%s", h.headCommit, p.Path)
```

**Fix:** Validate commit hash format in `NewFSHandler`:

```go
var hexHashRe = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

func NewFSHandler(headCommit, repoDir string) *FSHandler {
    if !hexHashRe.MatchString(headCommit) {
        // return error -- invalid commit hash
    }
    // ...
}
```

---

### P0-4: README Config Example Shows Credentials That Won't Load

**File:** `README.md:126-135`, `internal/config/config.go:50-55`
**Category:** Security / Documentation
**Reviewer:** Master Reviewer

The README shows this config example:

```yaml
bitbucket:
  user: you@example.com
  token: your-api-token
```

But `User` and `Token` have `yaml:"-"` tags, meaning they are **never loaded
from YAML files**. This means:

1. Users following the README will think their credentials are loaded but they
   won't be -- leading to confusing auth failures.
2. Users may create config files containing plaintext credentials that serve
   no purpose.

**Fix:** Update the README config example to show credentials via env vars only.
Remove `user` and `token` from the YAML example. Add a clear note:

```yaml
bitbucket:
  workspace: myteam
  repo: my-service
  # Credentials are loaded from environment variables only:
  #   CROBOT_BITBUCKET_USER, CROBOT_BITBUCKET_TOKEN
  # This prevents accidental credential leakage via config files.
```

---

### P0-5: `defer f.Close()` After `io.ReadAll` -- Wrong Defer Placement

**File:** `internal/cli/apply.go:68-73`
**Category:** Bug / Idiomatic Go
**Reviewer:** Go Standards

The file is opened, fully read, and then `Close` is deferred. The defer should
be placed immediately after a successful `Open`, before any operations on the
file handle.

```go
// CURRENT
f, openErr := os.Open(input)
if openErr != nil {
    return fmt.Errorf("reading input: %w", openErr)
}
findingsData, err = io.ReadAll(io.LimitReader(f, maxInputSize))
defer f.Close()  // TOO LATE
```

**Fix:**
```go
f, openErr := os.Open(input)
if openErr != nil {
    return fmt.Errorf("reading input: %w", openErr)
}
defer f.Close()
findingsData, err = io.ReadAll(io.LimitReader(f, maxInputSize))
```

---

### P0-6: Supply-Chain Risk: Local `.crobot.yaml` Can Execute Arbitrary Commands

**Files:** `internal/config/config.go:172`, `internal/agent/config.go:31-34`
**Category:** Security
**Reviewer:** Security

`LoadDefault()` loads `.crobot.yaml` from the current working directory. If a
user clones a repository that includes a `.crobot.yaml` with a malicious
`agent.agents.*.command`, running `crobot review` executes arbitrary code.

The `.gitignore` excludes `.crobot.yaml` for *this* repository, but does not
protect users who run CRoBot in other repositories that ship a malicious config.

**Fix (choose one or combine):**
1. **Document prominently** in the README: "CRoBot loads `.crobot.yaml` from
   the current directory. Do not run CRoBot in untrusted repositories."
2. **Restrict agent definitions** to only the global config
   (`~/.config/crobot/config.yaml`). Local config can override workspace/repo
   but not commands.
3. **Warn on first use**: Print a warning when loading agent definitions from
   a local config file.

---

### P0-7: `go 1.25.0` in go.mod -- Potentially Unreleased Go Version

**File:** `go.mod:4`
**Category:** Build / Compatibility
**Reviewer:** Master Reviewer

The go.mod specifies `go 1.25.0`. Users who `go install` will fail if they
don't have Go 1.25+. Verify this is the minimum Go version actually required
by the codebase.

**Fix:** Set to the minimum Go version actually required. If the `for range`
integer syntax (Go 1.22+) is used, `go 1.22` is the floor. Test with that
version and update accordingly.

---

## P1: Should Fix

### P1-1: `cli/helpers.go` Directly Imports `platform/bitbucket` -- Breaks Factory Pattern

**File:** `internal/cli/helpers.go:8`
**Category:** Architecture
**Reviewer:** Architecture

The `buildPlatform` function imports `internal/platform/bitbucket` directly and
constructs a `*bitbucket.Config` inside a `switch` statement. This defeats the
factory pattern and `init()` registration. Adding a new platform requires
modifying the CLI package.

**Fix:** Have each platform package provide a constructor that accepts
`config.Config` directly, or move the config mapping into platform
registration. The CLI should call `platform.NewPlatform(cfg.Platform, cfg)`
with no switch.

---

### P1-2: Duplicated Agent Config Resolution Logic

**Files:** `internal/cli/review.go:112-128`, `internal/cli/models.go:38-55`
**Category:** DRY / Maintainability
**Reviewer:** Architecture

The `--agent-command` parsing logic (splitting strings, constructing
`AgentRunConfig`, setting timeout) is duplicated verbatim in both files.

**Fix:** Extract into a shared helper:
```go
func resolveAgentFromFlags(cfg config.Config, agentName, agentCommand string) (*agent.AgentRunConfig, error)
```

---

### P1-3: `review.go` RunE is a 240-Line Monolith

**File:** `internal/cli/review.go:66-305`
**Category:** Readability / Testability
**Reviewer:** Architecture + Readability

The `RunE` closure handles 12 sequential steps in one function. It is the
largest function in the codebase and has zero test coverage. For a learning
codebase, this is the hardest function to understand.

**Fix:** Extract a `runReview(opts ReviewOpts) error` function that handles the
business logic, separate from cobra flag wiring. The 12 steps naturally group
into 3-4 phases: resolve inputs, run agent, apply findings.

---

### P1-4: `types.go` Mixes Five Responsibilities

**File:** `internal/platform/types.go`
**Category:** Cohesion
**Reviewer:** Architecture

This 158-line file contains DTOs, domain types, validation, parsing, and
fingerprint extraction.

**Fix:** Split into:
- `types.go` -- pure DTOs (PRRequest, FileRequest, PRContext, etc.)
- `finding.go` -- ReviewFinding with Validate and ParseFindings
- `fingerprint.go` -- ExtractFingerprint and the regex

---

### P1-5: Redundant `GetPRContext` API Call in Review Flow

**Files:** `internal/cli/review.go:135`, `internal/review/engine.go:92`
**Category:** Performance / Architecture
**Reviewer:** Architecture

The `review` command calls `plat.GetPRContext()` to build the agent prompt,
then `engine.Run()` calls it again internally. This doubles the API calls.

**Fix:** Add an `Engine.RunWithContext(ctx, req, prCtx, findings)` method that
accepts a pre-fetched PRContext.

---

### P1-6: Error Messages May Leak API Details to MCP Agents

**Files:** `internal/mcp/handler.go:65,99,169,191`,
`internal/platform/bitbucket/client.go:252-265`
**Category:** Security / Information Disclosure
**Reviewer:** Security

MCP handlers return raw error chains to AI agents via
`mcp.NewToolResultError(fmt.Sprintf("...: %v", err))`. Bitbucket API errors
can include internal details (emails, paths). The `mapHTTPError` function
includes up to 512 bytes of raw response body.

**Fix:** Return sanitized, user-facing error messages to MCP tool callers.
Parse Bitbucket error JSON for just the `error.message` field.

---

### P1-7: Name Stuttering in Agent Package

**Files:**
- `internal/agent/config.go:14` -- `agent.AgentRunConfig`
- `internal/agent/session.go:42` -- `agent.AgentMessage`

**Category:** Idiomatic Go
**Reviewer:** Go Standards

Go convention says package names should not be repeated in type names.

**Fix:** Rename to `agent.RunConfig` and `agent.Message`. Update all callers.

---

### P1-8: Missing `Category` Validation in `ReviewFinding.Validate()`

**File:** `internal/platform/types.go:89-109`
**Category:** Correctness
**Reviewer:** Master Reviewer

`Validate()` checks Path, Line, Side, Severity, SeverityScore, and Message but
does not check Category, which is documented as required in the schema.

**Fix:** Add category validation:
```go
if f.Category == "" {
    return fmt.Errorf("review finding: %w", ErrEmptyCategory)
}
```

---

### P1-9: Workspace Fallback Logic is Scattered

**Files:** `internal/platform/bitbucket/pr.go:56`, `file.go:14`,
`comments.go:47,119,173`
**Category:** DRY
**Reviewer:** Architecture

Every Bitbucket client method starts with
`workspace := opts.Workspace; if workspace == "" { workspace = c.workspace }`.
Repeated 5 times.

**Fix:** Add a private `(c *Client) resolveWorkspace(ws string) string` helper,
or resolve workspace once in the CLI and make it required in PRRequest.

---

### P1-10: `--output-format` Flag is Declared But Never Used

**File:** `internal/cli/root.go:39`
**Category:** Dead Code
**Reviewer:** Readability

The `outputFormat` variable is declared and registered as a persistent flag but
never referenced by any subcommand.

**Fix:** Remove it, or implement format switching.

---

### P1-11: Duplicate HTTP Retry Logic

**File:** `internal/platform/bitbucket/client.go:94-242`
**Category:** DRY / Maintainability
**Reviewer:** Readability

`do()`, `doRaw()`, and `doURL()` each implement their own retry loop with
identical backoff logic (~130 lines of duplication).

**Fix:** Extract a generic retry helper.

---

### P1-12: `review` Command Has Zero Test Coverage

**File:** `internal/cli/review.go` (entire file)
**Category:** Test Coverage
**Reviewer:** QA/Testing

The most complex and important code path in the application (~240 lines) has
zero test coverage. The overall `cli` package coverage is only ~34%.

**Fix:** Extract the orchestration logic into a testable function with
injectable dependencies, then write tests covering the happy path and key
error branches.

---

## P2: Nice to Have

### P2-1: No Interface for `agent.Client` (Testability)

**File:** `internal/agent/client.go`, `session.go`
**Category:** Testability
**Reviewer:** Architecture

`Session` depends on a concrete `*Client`. Testing requires spawning a real
subprocess. A small `AgentTransport` interface would allow in-process test
doubles.

### P2-2: Hardcoded ANSI Escape Sequences in `progress.go`

**File:** `internal/cli/progress.go`
**Category:** Portability
**Reviewer:** Readability

ANSI sequences are hardcoded throughout. On Windows without Windows Terminal or
in piped environments, these produce garbled output.

### P2-3: Package Doc Comment on Wrong File

**File:** `internal/platform/types.go:2` vs `platform.go`
**Category:** Go Doc
**Reviewer:** Go Standards

Package doc is on `types.go` but Go convention places it on the primary file.

### P2-4: Debug Logs May Expose Sensitive Data

**File:** `internal/agent/session.go:112,166,251`
**Category:** Security
**Reviewer:** Security

`slog.Debug` calls log raw JSON-RPC responses, which could contain API keys
when verbose mode is enabled.

### P2-5: Use `map[string]struct{}` for Set Types

**Files:** `internal/platform/types.go:74,80`, `internal/review/validate.go:34`
**Category:** Idiomatic Go
**Reviewer:** Go Standards

Use `map[string]struct{}` instead of `map[string]bool` for set membership.

### P2-6: `mdterm` Dependency is a Pseudo-Version

**File:** `go.mod:18`
**Category:** Dependencies
**Reviewer:** Security

`github.com/mkozhukh/mdterm v0.0.0-20250811...` has no tagged release.

### P2-7: `extractBareArray` Is O(n^2) in Worst Case

**File:** `internal/agent/parse.go:55-96`
**Category:** Performance
**Reviewer:** Go Standards

Scanner iterates from each `[` and scans forward. After a failed parse,
advance `i` past the scanned region.

### P2-8: `RenderComment` and `Engine.Run` Generate Fingerprint Redundantly

**Files:** `internal/review/render.go:81-83`, `engine.go:149-152`
**Category:** DRY
**Reviewer:** Master Reviewer

`DedupeFindings` already ensures every finding has a fingerprint. The fallback
in both `RenderComment` and `Engine.Run` is dead code.

### P2-9: Prompt Injection Surface in Review Prompt

**File:** `internal/agent/prompt.go:41-106`
**Category:** Security (inherent limitation)
**Reviewer:** Security

PR metadata is interpolated into the agent prompt. Document as known limitation.

### P2-10: `doRaw` Missing Body Size Limit for Non-Error Responses

**File:** `internal/platform/bitbucket/client.go:146-185`
**Category:** Robustness
**Reviewer:** Security

`doRaw` returns raw responses without enforcing body size limits on success.

### P2-11: Dead `AIConfig`/`ProviderDef` Types (Phase 4/5)

**File:** `internal/config/config.go:97-120`
**Category:** Dead Code
**Reviewer:** Readability

Phase 4/5 config types are defined and wired up but unused by any code path.
Add comments noting they are placeholders, or remove until needed.

### P2-12: Consider Adding `.golangci.yml` Configuration

**File:** (project root)
**Category:** Tooling
**Reviewer:** Master Reviewer

The README mentions `golangci-lint run` but there's no config file. For a
learning project, a linter config demonstrates best practices.

---

## What the Codebase Does Well

These patterns are production-quality and worth highlighting as learning
examples:

### Architecture
1. **Platform Abstraction (Textbook DIP)** -- `platform.Platform` is minimal
   (5 methods), platform-neutral, defined where consumed. Adding GitHub is a
   self-contained new package.
2. **Self-Registering Factory** -- `Register`/`NewPlatform` with
   mutex-protected registry follows the `database/sql` driver pattern.
3. **Review Engine Pipeline** -- Four stages (validate, dedupe, render, post)
   decomposed into pure, independently testable functions.
4. **Layered Configuration** -- Four-layer precedence with injectable
   `EnvLookupFunc` for testing. Secrets use `yaml:"-"`.
5. **Embedded Prompts via `go:embed`** -- Review instructions as `.md` files,
   compiled into the binary. Three composition functions for MCP/CLI/ACP modes.
6. **MCP Server as Thin Adapter** -- Zero business logic. Pure dispatch to the
   same packages the CLI uses. Behavioral consistency guaranteed.
7. **Agent Client/Session Separation** -- `Client` handles JSON-RPC transport;
   `Session` handles ACP semantics. Each testable independently.

### Security
8. **SSRF Protection** -- `doURL` validates pagination URLs match base host.
9. **Path Traversal Defense** -- FSHandler validates against `../` traversal.
10. **Credential Serialization Prevention** -- `yaml:"-"` on secret fields.
11. **Dry-Run Default** -- Must explicitly `--write` to post comments.
12. **Input Size Limits** -- `LimitReader` on API responses and file input.
13. **Subprocess Sandboxing** -- FSHandler rejects write/terminal operations.

### Code Quality
14. **Consistent Error Wrapping** -- `fmt.Errorf("context: %w", err)` throughout.
15. **Modern Go Features** -- `for range` integers, `go:embed`, `slog`, proper
    `context.Context` threading.
16. **Minimal Dependencies** -- Only 3 direct deps (mcp-go, cobra, yaml.v3).
17. **Clean CLI Structure** -- Each command in its own file, self-contained
    `newXxxCmd()` functions with examples in help text.

### Testing
18. **Table-Driven Tests** -- `validate_test.go` is exemplary.
19. **HTTP Test Servers** -- `client_test.go` uses `httptest.Server` properly.
20. **Integration Tests** -- `agent/integration_test.go` with real subprocess.
21. **Config Layering Tests** -- Full precedence coverage with testdata fixtures.

---

## Test Coverage Summary

| Package | Approximate Coverage | Assessment |
|---------|---------------------|------------|
| `internal/review` | ~97% | Excellent |
| `internal/config` | ~91% | Excellent |
| `internal/agent` | ~89% | Good |
| `internal/mcp` | ~84% | Good |
| `internal/platform/bitbucket` | ~84% | Good |
| `internal/prompt` | ~67% | Moderate |
| `internal/platform` | ~65% | Moderate |
| **`internal/cli`** | **~34%** | **Critical gap** |

### Key Coverage Gaps
1. `review.go` RunE handler -- zero coverage on the most complex code path.
2. `models.go` -- zero coverage.
3. `ExtractSnippet()` in `internal/platform` -- no dedicated tests for edge cases.

---

## Appendix: File-Level Notes

| File | Lines | Issues |
|------|-------|--------|
| `cmd/crobot/main.go` | 17 | Clean |
| `internal/cli/root.go` | 59 | P1-10: unused flag |
| `internal/cli/helpers.go` | 37 | P1-1: direct bitbucket import |
| `internal/cli/review.go` | 402 | P1-3, P1-2, P1-5, P1-12 |
| `internal/cli/models.go` | 116 | P1-2: duplicated agent config |
| `internal/cli/apply.go` | 148 | P0-5: wrong defer placement |
| `internal/cli/progress.go` | 237 | P2-2: hardcoded ANSI |
| `internal/cli/export.go` | 70 | Clean |
| `internal/cli/comments.go` | 70 | Clean |
| `internal/cli/snippet.go` | 91 | Clean |
| `internal/cli/serve.go` | 53 | Clean |
| `internal/cli/instructions.go` | 22 | Clean |
| `internal/config/config.go` | 266 | P0-4: README mismatch |
| `internal/platform/platform.go` | 24 | P2-3: missing package doc |
| `internal/platform/types.go` | 157 | P1-4, P1-8, P2-5 |
| `internal/platform/factory.go` | 41 | Exemplary |
| `internal/platform/errors.go` | 31 | Clean |
| `internal/platform/snippet.go` | 51 | Clean |
| `internal/platform/prurl.go` | 54 | Clean |
| `internal/platform/bitbucket/client.go` | 275 | P0-2, P1-11, P2-10 |
| `internal/platform/bitbucket/pr.go` | 180 | P0-2, P1-9 |
| `internal/platform/bitbucket/comments.go` | 187 | P0-2, P1-9 |
| `internal/platform/bitbucket/diff.go` | 183 | Clean -- well-tested parser |
| `internal/platform/bitbucket/file.go` | 34 | P0-2 |
| `internal/mcp/server.go` | 55 | Clean |
| `internal/mcp/handler.go` | 200 | P1-6 |
| `internal/mcp/tools.go` | 64 | Clean |
| `internal/review/engine.go` | 184 | P1-5, P2-8 |
| `internal/review/validate.go` | 117 | P2-5 |
| `internal/review/dedupe.go` | 55 | Clean |
| `internal/review/render.go` | 92 | P2-8 |
| `internal/agent/client.go` | 458 | P2-1 |
| `internal/agent/session.go` | 588 | P0-1, P2-4 |
| `internal/agent/config.go` | 48 | P1-7 |
| `internal/agent/fs.go` | 78 | P0-3 |
| `internal/agent/parse.go` | 96 | P2-7 |
| `internal/agent/prompt.go` | 136 | P2-9 |
| `internal/prompt/prompt.go` | 44 | Elegant go:embed pattern |
| `internal/version/version.go` | 5 | Clean |
| `go.mod` | 25 | P0-7, P2-6 |
| `README.md` | 799 | P0-4: credential docs mismatch |
| `.gitignore` | 8 | Adequate |

---

## Recommended Action Plan

### Phase 1: Security Hardening (P0s -- do first)
1. Fix permission auto-approve fallback (P0-1)
2. Add URL path escaping in Bitbucket client (P0-2)
3. Validate commit hash in FSHandler (P0-3)
4. Fix README credential documentation (P0-4)
5. Fix defer placement in apply.go (P0-5)
6. Document/mitigate local config command execution risk (P0-6)
7. Verify go.mod version (P0-7)

### Phase 2: Structural Improvements (P1s)
8. Decouple CLI from platform/bitbucket (P1-1)
9. Extract shared agent config resolver (P1-2)
10. Break up review.go RunE (P1-3) + add tests (P1-12)
11. Split types.go (P1-4)
12. Add RunWithContext to Engine (P1-5)
13. Sanitize MCP error messages (P1-6)
14. Fix name stuttering (P1-7)
15. Add category validation (P1-8)
16. Consolidate workspace resolution (P1-9)
17. Remove dead output-format flag (P1-10)
18. Extract retry helper (P1-11)

### Phase 3: Polish (P2s)
19. Address P2 items based on time and priority.

---

*Report generated by the CRoBot Review Team -- 2026-03-17*
*Reviewed from six perspectives: Security, Architecture, Go Standards,
Readability, Test Coverage, and Master Review.*
