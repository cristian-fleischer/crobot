# Pre-Production Code Review Report

**Feature**: Local (Pre-Push) Review Support (P3.8)
**Version**: 0.3.27-alpha
**Date**: 2026-03-18
**Scope**: 14 modified files, +280/-78 lines
**Review Panel**: 5 specialist agents (Security, Logic, Architecture, Docs, Tests)

---

## Executive Summary

The implementation is **ship-ready** with no critical bugs or security
vulnerabilities. All existing PR review functionality is preserved. The panel
identified 3 items worth fixing before push, several improvements to track,
and confirmed the overall design is sound.

---

## Consolidated Findings

### Must Fix (recommended before push)

| # | Source | Severity | Finding | File |
|---|--------|----------|---------|------|
| 1 | Docs | ERROR | README `serve` section claims 4 MCP tools — now 5. `export_local_context` missing from the list. | `README.md` (~line 726) |
| 2 | Docs | WARNING | Skill file references `crobot export-pr-context` with "local mode" — that command has no local mode. Will mislead agents. | `.agents/skills/review-pr.md:17-18` |
| 3 | Docs | WARNING | Feature bullet says "uncommitted changes" but implementation captures committed + staged + unstaged. Misleading. | `README.md:30`, `.agents/skills/review-pr.md` |

### Should Fix (low effort, high polish)

| # | Source | Severity | Finding | File |
|---|--------|----------|---------|------|
| 4 | Security | MEDIUM | `repo_dir` MCP parameter accepts arbitrary paths — any git repo on the filesystem becomes readable. Restrict to CWD or validate. | `handler.go:153-154` |
| 5 | Security | MEDIUM | `base_branch` not protected against `--`-prefixed values (git argument injection). Add `--` separator before user-supplied refs. | `provider.go:41,119` |
| 6 | Logic | EDGE_CASE | `--write` silently overridden in local mode with no user feedback. Log a warning. | `review.go:126` |
| 7 | Logic | LOGIC_ERROR | `extractDiffContext` returns all parsed lines when target not found instead of nil. | `render.go:206-207` |
| 8 | Docs | WARNING | ACP workflow prompt says "PR metadata" even in local mode — inconsistent with the "Local Review Metadata" header from prompt.go. | `workflow_acp.md:3` |

### Track for Later (design improvements)

| # | Source | Severity | Finding |
|---|--------|----------|---------|
| 9 | Arch | DESIGN | Magic `PRNumber == 0` sentinel for local mode — consider explicit `Mode` field on PRRequest |
| 10 | Arch | COUPLING | MCP handler directly imports `localplatform` — bypasses dependency injection pattern |
| 11 | Arch | EXTENSIBILITY | Hardcoded `"master"` default in two places (CLI + MCP) — extract to config or auto-detect |
| 12 | Arch | DESIGN | No-op `ListBotComments` vs erroring `CreateInlineComment` is inconsistent — consider `ErrNotSupported` sentinel |
| 13 | Logic | REGRESSION (minor) | `--show-agent-output` rendering format changed for PR reviews (now includes diff context snippet) |

### Test Gaps

| # | Severity | Missing Test |
|---|----------|--------------|
| 14 | CRITICAL | No tests for `handleExportLocalContext` MCP handler (success, bad repo, bad branch) |
| 15 | CRITICAL | No dispatch routing test for `export_local_context` |
| 16 | HIGH | No test for `BuildReviewPrompt` with `PRNumber: 0` (local mode metadata) |
| 17 | HIGH | `TestReviewCmd_NoPR_EntersLocalMode` is weak — passes by not-failing rather than positive assertion |
| 18 | MEDIUM | No test that `--write` is overridden in local mode |
| 19 | MEDIUM | No direct test for `RenderFindings` top-level function |
| 20 | MEDIUM | `--base` flag propagation to `localplatform.New` untested |
| 21 | LOW | `DeleteComment` error path on local provider untested |

### Positive Findings (things done well)

- **Defense in depth**: Dry-run enforcement via `isDryRun = true` +
  `CreateInlineComment` returns error + MCP `apply_review_findings` requires
  `pr > 0`
- **MCP error sanitization**: `toolError()` logs details, returns generic
  message to client
- **Read-only reviewer prompts**: Both ACP and MCP workflows now clearly state
  agent must not modify anything
- **Good composability**: Both `export_pr_context` and `export_local_context`
  return identical `PRContext` JSON
- **Interface compliance**: `local.Provider` correctly implements all `Platform`
  methods
- **Test isolation**: Local provider tests use `t.TempDir()` with isolated git
  repos
- **No concurrency issues**: Provider fields immutable after construction

---

## Backward Compatibility Matrix

| Code Path | Status | Notes |
|-----------|--------|-------|
| `crobot review <PR>` | Safe | No changes to PR flow |
| `crobot review <PR> --show-agent-output` | Minor change | Output now includes diff context (#13) |
| `crobot review` (new) | New feature | Isolated from PR flow |
| MCP `export_pr_context` | Safe | Untouched |
| MCP `get_file_snippet` | Safe | Untouched |
| MCP `list_bot_comments` | Safe | Untouched |
| MCP `apply_review_findings` | Safe | Untouched |
| MCP `export_local_context` | New tool | No impact on existing tools |
| `agent.BuildReviewPrompt` (PR) | Safe | Falls into `else` branch |
| `agent.BuildReviewPrompt` (nil ref) | Safe | Falls into `else` branch |

---

## Verdict

**PASS — ship with fixes 1-3 (docs errors) and ideally 4-8 (low-effort
hardening).**

The core implementation is correct, well-isolated, and preserves all existing
functionality. The test gaps (#14-21) are real but consistent with the existing
test coverage strategy and don't block a push. The architecture findings (#9-12)
are genuine improvements but are safe to defer.
