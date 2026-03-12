package review

import (
	"context"
	"fmt"

	"github.com/dizzyc/crobot/internal/platform"
)

// Engine orchestrates the full review flow: validate, dedupe, render, and post.
type Engine struct {
	platform platform.Platform
	config   EngineConfig
}

// EngineConfig holds configuration for the review engine.
type EngineConfig struct {
	// MaxComments is the maximum number of comments to post in a single run.
	MaxComments int

	// DryRun controls whether comments are actually posted.
	DryRun bool

	// BotLabel is included in rendered comments for identification.
	BotLabel string

	// SeverityThreshold is the minimum severity level for findings to be
	// posted. Valid values: "info", "warning", "error".
	SeverityThreshold string
}

// ReviewResult is the output of a review run.
type ReviewResult struct {
	Posted  []PostedComment  `json:"posted"`
	Skipped []SkippedComment `json:"skipped"`
	Failed  []FailedComment  `json:"failed"`
	Summary ReviewSummary    `json:"summary"`
}

// PostedComment records a finding that was successfully posted as a comment.
type PostedComment struct {
	Finding   platform.ReviewFinding `json:"finding"`
	CommentID string                 `json:"comment_id"`
}

// SkippedComment records a finding that was not posted, along with the reason.
type SkippedComment struct {
	Finding platform.ReviewFinding `json:"finding"`
	Reason  string                 `json:"reason"`
}

// FailedComment records a finding whose comment post attempt failed.
type FailedComment struct {
	Finding platform.ReviewFinding `json:"finding"`
	Error   string                 `json:"error"`
}

// ReviewSummary provides aggregate counts for the review run.
type ReviewSummary struct {
	Total     int  `json:"total"`
	Posted    int  `json:"posted"`
	Skipped   int  `json:"skipped"`
	Failed    int  `json:"failed"`
	Duplicate int  `json:"duplicate"`
	MaxCapped bool `json:"max_capped"`
}

// NewEngine creates a new review engine with the given platform and config.
func NewEngine(p platform.Platform, cfg EngineConfig) *Engine {
	return &Engine{
		platform: p,
		config:   cfg,
	}
}

// Run executes the full review pipeline:
//  1. Fetch PR context from platform
//  2. Validate findings against PR context
//  3. Fetch existing bot comments for dedup
//  4. Deduplicate findings
//  5. Cap at MaxComments
//  6. Render comment bodies
//  7. If dry-run: return plan without posting
//  8. If write: post each comment, track results
//  9. Return ReviewResult with counts
func (e *Engine) Run(ctx context.Context, req platform.PRRequest, findings []platform.ReviewFinding) (*ReviewResult, error) {
	result := &ReviewResult{}
	result.Summary.Total = len(findings)

	// 1. Fetch PR context.
	prCtx, err := e.platform.GetPRContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fetching PR context: %w", err)
	}

	// 2. Validate findings against PR context.
	validated, rejected := ValidateFindings(findings, prCtx, e.config.SeverityThreshold)
	for _, r := range rejected {
		result.Skipped = append(result.Skipped, SkippedComment{
			Finding: r.Finding,
			Reason:  r.Reason,
		})
	}

	// 3. Fetch existing bot comments for dedup.
	existing, err := e.platform.ListBotComments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("listing bot comments: %w", err)
	}

	// 4. Deduplicate findings.
	newFindings, duplicates := DedupeFindings(validated, existing)
	result.Summary.Duplicate = len(duplicates)
	for _, d := range duplicates {
		result.Skipped = append(result.Skipped, SkippedComment{
			Finding: d,
			Reason:  "duplicate: fingerprint already exists",
		})
	}

	// 5. Cap at MaxComments.
	if e.config.MaxComments > 0 && len(newFindings) > e.config.MaxComments {
		result.Summary.MaxCapped = true
		capped := newFindings[e.config.MaxComments:]
		for _, f := range capped {
			result.Skipped = append(result.Skipped, SkippedComment{
				Finding: f,
				Reason:  "max comments limit reached",
			})
		}
		newFindings = newFindings[:e.config.MaxComments]
	}

	// 6-8. Render and optionally post.
	for _, f := range newFindings {
		body := RenderComment(f, e.config.BotLabel)

		if e.config.DryRun {
			result.Posted = append(result.Posted, PostedComment{
				Finding:   f,
				CommentID: "dry-run",
			})
			continue
		}

		// Ensure fingerprint is set.
		fp := f.Fingerprint
		if fp == "" {
			fp = GenerateFingerprint(&f)
		}

		comment := platform.InlineComment{
			Path:        f.Path,
			Line:        f.Line,
			Side:        f.Side,
			Body:        body,
			Fingerprint: fp,
		}

		posted, err := e.platform.CreateInlineComment(ctx, req, comment)
		if err != nil {
			result.Failed = append(result.Failed, FailedComment{
				Finding: f,
				Error:   err.Error(),
			})
			continue
		}

		result.Posted = append(result.Posted, PostedComment{
			Finding:   f,
			CommentID: posted.ID,
		})
	}

	// 9. Compute summary counts.
	result.Summary.Posted = len(result.Posted)
	result.Summary.Skipped = len(result.Skipped)
	result.Summary.Failed = len(result.Failed)

	return result, nil
}
