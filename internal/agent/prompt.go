// Package agent provides the ACP orchestrator components for interacting with
// AI agents to perform code reviews.
package agent

import (
	"fmt"
	"strings"

	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/prompt"
)

// BuildSystemPrompt returns the system-level instructions for the reviewing
// agent. It uses the core review methodology (philosophy, schema, rules) but
// replaces the MCP/CLI workflow with a self-contained ACP workflow since the
// PR data is already provided in the prompt.
func BuildSystemPrompt() string {
	var b strings.Builder

	b.WriteString(prompt.CoreInstructions())

	b.WriteString("\n\n## Workflow\n\n")
	b.WriteString("The PR metadata, changed files, and full diff are provided below.\n")
	b.WriteString("You do NOT need to fetch any data — everything you need is in this prompt.\n\n")
	b.WriteString("1. Read the PR title and description to understand the intent.\n")
	b.WriteString("2. Analyze each changed file's diff hunks.\n")
	b.WriteString("3. If you have filesystem access, read full files for additional context.\n")
	b.WriteString("4. Formulate findings with specific messages and remediation code.\n")
	b.WriteString("5. Output your findings as described below.\n")

	b.WriteString("\n\n## Output Format\n\n")
	b.WriteString("You MUST output your review findings as a JSON array of ReviewFinding objects.\n")
	b.WriteString("Output ONLY the JSON array — no surrounding text, no markdown fences.\n")
	b.WriteString("If you find no issues, output an empty array: []\n")

	return b.String()
}

// BuildReviewPrompt formats the PR context into a review prompt for the agent.
// It includes PR metadata, changed files, and the full diff organized by file.
func BuildReviewPrompt(prCtx *platform.PRContext, ref *platform.PRRequest) string {
	var b strings.Builder

	b.WriteString("# Pull Request Review\n\n")

	// PR metadata
	b.WriteString("## PR Metadata\n\n")
	if ref != nil {
		b.WriteString(fmt.Sprintf("- **Workspace**: %s\n", ref.Workspace))
		b.WriteString(fmt.Sprintf("- **Repository**: %s\n", ref.Repo))
		b.WriteString(fmt.Sprintf("- **PR Number**: %d\n", ref.PRNumber))
	}
	b.WriteString(fmt.Sprintf("- **Title**: %s\n", prCtx.Title))
	b.WriteString(fmt.Sprintf("- **Author**: %s\n", prCtx.Author))
	b.WriteString(fmt.Sprintf("- **Source Branch**: %s\n", prCtx.SourceBranch))
	b.WriteString(fmt.Sprintf("- **Target Branch**: %s\n", prCtx.TargetBranch))
	b.WriteString(fmt.Sprintf("- **State**: %s\n", prCtx.State))
	if prCtx.Description != "" {
		b.WriteString(fmt.Sprintf("\n### Description\n\n%s\n", prCtx.Description))
	}

	// Changed files
	b.WriteString("\n## Changed Files\n\n")
	if len(prCtx.Files) == 0 {
		b.WriteString("No files changed.\n")
	} else {
		for _, f := range prCtx.Files {
			status := f.Status
			line := fmt.Sprintf("- `%s` (%s)", f.Path, status)
			if f.OldPath != "" && f.OldPath != f.Path {
				line += fmt.Sprintf(" (renamed from `%s`)", f.OldPath)
			}
			b.WriteString(line + "\n")
		}
	}

	// Diff hunks organized by file
	b.WriteString("\n## Diff\n\n")
	if len(prCtx.DiffHunks) == 0 {
		b.WriteString("No diff hunks available.\n")
	} else {
		hunksByFile := groupHunksByFile(prCtx.DiffHunks)
		for _, file := range orderedFiles(prCtx.DiffHunks) {
			b.WriteString(fmt.Sprintf("### %s\n\n", file))
			for _, hunk := range hunksByFile[file] {
				b.WriteString(fmt.Sprintf("```diff\n@@ -%d,%d +%d,%d @@\n",
					hunk.OldStart, hunk.OldLines,
					hunk.NewStart, hunk.NewLines))
				b.WriteString(hunk.Body)
				if !strings.HasSuffix(hunk.Body, "\n") {
					b.WriteString("\n")
				}
				b.WriteString("```\n\n")
			}
		}
	}

	// Instructions for the review
	b.WriteString("## Instructions\n\n")
	b.WriteString("Review the diff above and output your findings as a JSON array of ReviewFinding objects.\n")
	b.WriteString("Only comment on lines that appear within the diff hunks.\n")
	b.WriteString("Leave the `fingerprint` field empty.\n")
	b.WriteString("If no issues are found, output: []\n")

	return b.String()
}

// BuildFullPrompt combines the system prompt and review prompt into a single
// string. This is used when the ACP protocol does not support separate system
// prompts. The optional PRRequest provides workspace, repo, and PR number metadata.
func BuildFullPrompt(prCtx *platform.PRContext, ref *platform.PRRequest) string {
	return BuildSystemPrompt() + "\n---\n\n" + BuildReviewPrompt(prCtx, ref)
}

// groupHunksByFile groups diff hunks by their file path.
func groupHunksByFile(hunks []platform.DiffHunk) map[string][]platform.DiffHunk {
	m := make(map[string][]platform.DiffHunk)
	for _, h := range hunks {
		m[h.Path] = append(m[h.Path], h)
	}
	return m
}

// orderedFiles returns unique file paths from hunks in the order they first
// appear, preserving the original diff ordering.
func orderedFiles(hunks []platform.DiffHunk) []string {
	seen := make(map[string]bool)
	var files []string
	for _, h := range hunks {
		if !seen[h.Path] {
			seen[h.Path] = true
			files = append(files, h.Path)
		}
	}
	return files
}
