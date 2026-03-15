package review_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/review"
)

// mockPlatform is a test double for platform.Platform.
type mockPlatform struct {
	mu sync.Mutex

	prContext    *platform.PRContext
	prContextErr error

	botComments    []platform.Comment
	botCommentsErr error

	postedComments []platform.InlineComment
	postErr        error
	postResult     func(c platform.InlineComment) *platform.Comment
}

func (m *mockPlatform) GetPRContext(_ context.Context, _ platform.PRRequest) (*platform.PRContext, error) {
	return m.prContext, m.prContextErr
}

func (m *mockPlatform) GetFileContent(_ context.Context, _ platform.FileRequest) ([]byte, error) {
	return nil, nil
}

func (m *mockPlatform) ListBotComments(_ context.Context, _ platform.PRRequest) ([]platform.Comment, error) {
	return m.botComments, m.botCommentsErr
}

func (m *mockPlatform) CreateInlineComment(_ context.Context, _ platform.PRRequest, comment platform.InlineComment) (*platform.Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.postErr != nil {
		return nil, m.postErr
	}

	m.postedComments = append(m.postedComments, comment)

	if m.postResult != nil {
		return m.postResult(comment), nil
	}

	return &platform.Comment{
		ID:   fmt.Sprintf("comment-%d", len(m.postedComments)),
		Path: comment.Path,
		Line: comment.Line,
		Body: comment.Body,
	}, nil
}

func (m *mockPlatform) DeleteComment(_ context.Context, _ platform.PRRequest, _ string) error {
	return nil
}

func newMockPlatform() *mockPlatform {
	return &mockPlatform{
		prContext: testPRContext(),
	}
}

func defaultReq() platform.PRRequest {
	return platform.PRRequest{
		Workspace: "team",
		Repo:      "repo",
		PRNumber:  1,
	}
}

func defaultFindings() []platform.ReviewFinding {
	return []platform.ReviewFinding{
		{
			Path: "src/main.go", Line: 12, Side: "new",
			Severity: "warning", Category: "style",
			Message: "Consider renaming.", Fingerprint: "fp-1",
		},
		{
			Path: "src/main.go", Line: 14, Side: "new",
			Severity: "error", Category: "bug",
			Message: "Possible nil deref.", Fingerprint: "fp-2",
		},
	}
}

func TestEngine_DryRun(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       25,
		DryRun:            true,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	result, err := engine.Run(context.Background(), defaultReq(), defaultFindings())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// In dry-run mode, no actual comments are posted.
	if len(mock.postedComments) != 0 {
		t.Errorf("expected 0 posted comments in dry-run, got %d", len(mock.postedComments))
	}

	// But result should show them as "posted" with dry-run ID.
	if result.Summary.Posted != 2 {
		t.Errorf("expected 2 posted in summary, got %d", result.Summary.Posted)
	}
	for _, p := range result.Posted {
		if p.CommentID != "dry-run" {
			t.Errorf("expected comment ID 'dry-run', got %q", p.CommentID)
		}
	}
}

func TestEngine_WriteMode(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       25,
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	result, err := engine.Run(context.Background(), defaultReq(), defaultFindings())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Comments should actually be posted.
	if len(mock.postedComments) != 2 {
		t.Errorf("expected 2 posted comments, got %d", len(mock.postedComments))
	}

	if result.Summary.Posted != 2 {
		t.Errorf("expected 2 posted in summary, got %d", result.Summary.Posted)
	}
	if result.Summary.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Summary.Total)
	}
}

func TestEngine_MaxCommentsCap(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       1,
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	findings := defaultFindings()
	result, err := engine.Run(context.Background(), defaultReq(), findings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.postedComments) != 1 {
		t.Errorf("expected 1 posted comment (capped), got %d", len(mock.postedComments))
	}
	if result.Summary.Posted != 1 {
		t.Errorf("expected 1 posted in summary, got %d", result.Summary.Posted)
	}
	if !result.Summary.MaxCapped {
		t.Error("expected MaxCapped to be true")
	}
	// The capped finding should be in skipped.
	foundCappedSkip := false
	for _, s := range result.Skipped {
		if s.Reason == "max comments limit reached" {
			foundCappedSkip = true
			break
		}
	}
	if !foundCappedSkip {
		t.Error("expected a skipped finding with 'max comments limit reached' reason")
	}
}

func TestEngine_AllFindingsRejected(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       25,
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "error", // will reject all warnings
	})

	findings := []platform.ReviewFinding{
		{
			Path: "src/main.go", Line: 12, Side: "new",
			Severity: "warning", Category: "style",
			Message: "Style issue.", Fingerprint: "fp-1",
		},
		{
			Path: "src/main.go", Line: 14, Side: "new",
			Severity: "info", Category: "docs",
			Message: "Docs note.", Fingerprint: "fp-2",
		},
	}

	result, err := engine.Run(context.Background(), defaultReq(), findings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.postedComments) != 0 {
		t.Errorf("expected 0 posted comments, got %d", len(mock.postedComments))
	}
	if result.Summary.Posted != 0 {
		t.Errorf("expected 0 posted in summary, got %d", result.Summary.Posted)
	}
	if result.Summary.Skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", result.Summary.Skipped)
	}
}

func TestEngine_WithDuplicates(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	mock.botComments = []platform.Comment{
		{ID: "existing-1", Body: "text [//]: # \"crobot:fp=fp-1\"", IsBot: true},
	}

	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       25,
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	result, err := engine.Run(context.Background(), defaultReq(), defaultFindings())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// fp-1 is a duplicate, fp-2 should be posted.
	if len(mock.postedComments) != 1 {
		t.Errorf("expected 1 posted comment, got %d", len(mock.postedComments))
	}
	if result.Summary.Duplicate != 1 {
		t.Errorf("expected 1 duplicate, got %d", result.Summary.Duplicate)
	}
	if result.Summary.Posted != 1 {
		t.Errorf("expected 1 posted, got %d", result.Summary.Posted)
	}
}

func TestEngine_PostError(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	mock.postErr = fmt.Errorf("API rate limited")

	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       25,
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	result, err := engine.Run(context.Background(), defaultReq(), defaultFindings())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary.Failed != 2 {
		t.Errorf("expected 2 failed, got %d", result.Summary.Failed)
	}
	if result.Summary.Posted != 0 {
		t.Errorf("expected 0 posted, got %d", result.Summary.Posted)
	}
}

func TestEngine_GetPRContextError(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	mock.prContextErr = fmt.Errorf("not found")

	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       25,
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	_, err := engine.Run(context.Background(), defaultReq(), defaultFindings())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEngine_ListBotCommentsError(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	mock.botCommentsErr = fmt.Errorf("API error")

	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       25,
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	_, err := engine.Run(context.Background(), defaultReq(), defaultFindings())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEngine_EmptyFindings(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       25,
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	result, err := engine.Run(context.Background(), defaultReq(), []platform.ReviewFinding{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary.Total != 0 {
		t.Errorf("expected total 0, got %d", result.Summary.Total)
	}
	if result.Summary.Posted != 0 {
		t.Errorf("expected 0 posted, got %d", result.Summary.Posted)
	}
}

func TestEngine_MaxCommentsZeroMeansUnlimited(t *testing.T) {
	t.Parallel()

	mock := newMockPlatform()
	engine := review.NewEngine(mock, review.EngineConfig{
		MaxComments:       0, // no cap
		DryRun:            false,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	result, err := engine.Run(context.Background(), defaultReq(), defaultFindings())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.postedComments) != 2 {
		t.Errorf("expected 2 posted comments with no cap, got %d", len(mock.postedComments))
	}
	if result.Summary.MaxCapped {
		t.Error("expected MaxCapped to be false with no cap")
	}
}
