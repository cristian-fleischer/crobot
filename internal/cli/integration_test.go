package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristian-fleischer/crobot/internal/platform"
	"github.com/cristian-fleischer/crobot/internal/platform/bitbucket"
	"github.com/cristian-fleischer/crobot/internal/review"
)

// TestEndToEnd_ExportThenDryRunApply exercises the full CLI workflow:
//  1. Export PR context using a mock Bitbucket server
//  2. Create review findings targeting files/lines from the exported context
//  3. Apply those findings in dry-run mode
//
// This verifies the entire pipeline: HTTP client → platform → review engine.
func TestEndToEnd_ExportThenDryRunApply(t *testing.T) {
	t.Parallel()

	prMeta := map[string]any{
		"id":          100,
		"title":       "Add logging module",
		"description": "Introduces structured logging",
		"state":       "OPEN",
		"author":      map[string]any{"display_name": "alice"},
		"source": map[string]any{
			"branch": map[string]any{"name": "feat/logging"},
			"commit": map[string]any{"hash": "head111"},
		},
		"destination": map[string]any{
			"branch": map[string]any{"name": "main"},
			"commit": map[string]any{"hash": "base222"},
		},
	}

	diffstat := map[string]any{
		"values": []map[string]any{
			{
				"new":    map[string]any{"path": "pkg/log/logger.go", "type": "commit_file"},
				"old":    map[string]any{"path": "", "type": ""},
				"status": "added",
			},
			{
				"new":    map[string]any{"path": "main.go", "type": "commit_file"},
				"old":    map[string]any{"path": "main.go", "type": "commit_file"},
				"status": "modified",
			},
		},
	}

	diff := `diff --git a/pkg/log/logger.go b/pkg/log/logger.go
new file mode 100644
--- /dev/null
+++ b/pkg/log/logger.go
@@ -0,0 +1,15 @@
+package log
+
+import "fmt"
+
+type Logger struct{}
+
+func New() *Logger {
+	return &Logger{}
+}
+
+func (l *Logger) Info(msg string) {
+	fmt.Println(msg)
+}
+
+func (l *Logger) Error(msg string) {
+	fmt.Println("ERROR: " + msg)
+}
diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,8 @@
 package main
 
+import "my/pkg/log"
+
 func main() {
+	logger := log.New()
+	logger.Info("started")
 }
`

	commentsResp := map[string]any{
		"values": []any{},
	}

	var postedComments []map[string]any

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/acme/svc/pullrequests/100": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, prMeta)
		},
		"/2.0/repositories/acme/svc/pullrequests/100/diffstat": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, diffstat)
		},
		"/2.0/repositories/acme/svc/pullrequests/100/diff": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			fmt.Fprint(w, diff)
		},
		"/2.0/repositories/acme/svc/pullrequests/100/comments": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				jsonResponse(w, 200, commentsResp)
				return
			}
			if r.Method == http.MethodPost {
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				postedComments = append(postedComments, body)
				jsonResponse(w, 201, map[string]any{
					"id":      len(postedComments),
					"content": body["content"],
				})
				return
			}
		},
	})
	defer srv.Close()

	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "bot",
		Token:   "secret",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	ctx := t.Context()

	// Step 1: Export PR context.
	prCtx, err := client.GetPRContext(ctx, platform.PRRequest{
		Workspace: "acme",
		Repo:      "svc",
		PRNumber:  100,
	})
	if err != nil {
		t.Fatalf("GetPRContext: %v", err)
	}

	// Verify export data.
	if prCtx.ID != 100 {
		t.Errorf("PR ID = %d, want 100", prCtx.ID)
	}
	if prCtx.Title != "Add logging module" {
		t.Errorf("Title = %q, want %q", prCtx.Title, "Add logging module")
	}
	if len(prCtx.Files) != 2 {
		t.Fatalf("Files count = %d, want 2", len(prCtx.Files))
	}
	if prCtx.Files[0].Status != "added" {
		t.Errorf("Files[0].Status = %q, want %q", prCtx.Files[0].Status, "added")
	}
	if len(prCtx.DiffHunks) < 2 {
		t.Fatalf("DiffHunks count = %d, want >= 2", len(prCtx.DiffHunks))
	}

	// Verify PRContext serializes to JSON correctly.
	prCtxJSON, err := json.Marshal(prCtx)
	if err != nil {
		t.Fatalf("marshal PRContext: %v", err)
	}
	var roundTrip platform.PRContext
	if err := json.Unmarshal(prCtxJSON, &roundTrip); err != nil {
		t.Fatalf("unmarshal PRContext: %v", err)
	}
	if roundTrip.ID != prCtx.ID {
		t.Errorf("roundtrip ID mismatch: %d vs %d", roundTrip.ID, prCtx.ID)
	}

	// Step 2: Create findings targeting the exported context.
	findings := []platform.ReviewFinding{
		{
			Path:     "pkg/log/logger.go",
			Line:     3,
			Side:     "new",
			Severity: "warning",
			Category: "style",
			Message:  "Consider using log/slog instead of fmt for logging",
		},
		{
			Path:     "main.go",
			Line:     3,
			Side:     "new",
			Severity: "info",
			Category: "best-practice",
			Message:  "Consider using a dependency injection framework",
		},
		{
			Path:     "nonexistent.go",
			Line:     1,
			Side:     "new",
			Severity: "error",
			Category: "bug",
			Message:  "This should be rejected (file not in PR)",
		},
	}

	// Step 3: Apply findings in dry-run mode.
	engine := review.NewEngine(client, review.EngineConfig{
		MaxComments:       25,
		DryRun:            true,
		BotLabel:          "crobot",
		SeverityThreshold: "info",
	})

	result, err := engine.Run(ctx, platform.PRRequest{
		Workspace: "acme",
		Repo:      "svc",
		PRNumber:  100,
	}, findings)
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	// Verify results.
	if result.Summary.Total != 3 {
		t.Errorf("total = %d, want 3", result.Summary.Total)
	}
	// 2 valid findings should be "posted" in dry-run mode.
	if result.Summary.Posted != 2 {
		t.Errorf("posted = %d, want 2", result.Summary.Posted)
	}
	// 1 finding should be skipped (nonexistent file).
	if result.Summary.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Summary.Skipped)
	}
	if result.Summary.Failed != 0 {
		t.Errorf("failed = %d, want 0", result.Summary.Failed)
	}

	// All posted comments should have dry-run ID.
	for i, p := range result.Posted {
		if p.CommentID != "dry-run" {
			t.Errorf("posted[%d].CommentID = %q, want %q", i, p.CommentID, "dry-run")
		}
	}

	// No comments should have been actually posted to the server.
	if len(postedComments) != 0 {
		t.Errorf("server received %d POST requests, want 0 in dry-run", len(postedComments))
	}

	// Verify result serializes to JSON correctly.
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(resultJSON), "dry-run") {
		t.Error("result JSON does not contain 'dry-run'")
	}
}

// TestEndToEnd_WriteMode exercises the full pipeline in write mode, verifying
// that comments are actually posted to the mock server.
func TestEndToEnd_WriteMode(t *testing.T) {
	t.Parallel()

	prMeta := map[string]any{
		"id":          50,
		"title":       "Fix auth bug",
		"description": "",
		"state":       "OPEN",
		"author":      map[string]any{"display_name": "bob"},
		"source": map[string]any{
			"branch": map[string]any{"name": "fix/auth"},
			"commit": map[string]any{"hash": "fix111"},
		},
		"destination": map[string]any{
			"branch": map[string]any{"name": "main"},
			"commit": map[string]any{"hash": "base222"},
		},
	}

	diffstat := map[string]any{
		"values": []map[string]any{
			{
				"new":    map[string]any{"path": "auth.go", "type": "commit_file"},
				"old":    map[string]any{"path": "auth.go", "type": "commit_file"},
				"status": "modified",
			},
		},
	}

	diff := `diff --git a/auth.go b/auth.go
--- a/auth.go
+++ b/auth.go
@@ -1,5 +1,8 @@
 package auth
 
+func Validate(token string) bool {
+	return token != ""
+}
`

	commentsResp := map[string]any{
		"values": []any{},
	}

	var postedComments []map[string]any
	commentID := 0

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/acme/svc/pullrequests/50": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, prMeta)
		},
		"/2.0/repositories/acme/svc/pullrequests/50/diffstat": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, diffstat)
		},
		"/2.0/repositories/acme/svc/pullrequests/50/diff": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			fmt.Fprint(w, diff)
		},
		"/2.0/repositories/acme/svc/pullrequests/50/comments": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				jsonResponse(w, 200, commentsResp)
				return
			}
			if r.Method == http.MethodPost {
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				commentID++
				postedComments = append(postedComments, body)
				jsonResponse(w, 201, map[string]any{
					"id":      commentID,
					"content": body["content"],
				})
				return
			}
		},
	})
	defer srv.Close()

	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "bot",
		Token:   "secret",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	ctx := t.Context()

	findings := []platform.ReviewFinding{
		{
			Path:     "auth.go",
			Line:     3,
			Side:     "new",
			Severity: "error",
			Category: "security",
			Message:  "Token validation is too permissive",
			Suggestion: `func Validate(token string) bool {
	return len(token) >= 32
}`,
		},
		{
			Path:     "auth.go",
			Line:     4,
			Side:     "new",
			Severity: "warning",
			Category: "style",
			Message:  "Consider adding a doc comment",
		},
	}

	engine := review.NewEngine(client, review.EngineConfig{
		MaxComments:       25,
		DryRun:            false, // Write mode!
		BotLabel:          "crobot",
		SeverityThreshold: "warning",
	})

	result, err := engine.Run(ctx, platform.PRRequest{
		Workspace: "acme",
		Repo:      "svc",
		PRNumber:  50,
	}, findings)
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	// Both findings should be posted.
	if result.Summary.Total != 2 {
		t.Errorf("total = %d, want 2", result.Summary.Total)
	}
	if result.Summary.Posted != 2 {
		t.Errorf("posted = %d, want 2", result.Summary.Posted)
	}
	if result.Summary.Skipped != 0 {
		t.Errorf("skipped = %d, want 0", result.Summary.Skipped)
	}

	// Comments should have real IDs (not "dry-run").
	for i, p := range result.Posted {
		if p.CommentID == "dry-run" {
			t.Errorf("posted[%d].CommentID = dry-run, expected real ID", i)
		}
	}

	// Server should have received 2 POST requests.
	if len(postedComments) != 2 {
		t.Fatalf("server received %d POST requests, want 2", len(postedComments))
	}

	// Verify the first posted comment contains the fingerprint marker.
	firstComment, ok := postedComments[0]["content"].(map[string]any)
	if !ok {
		t.Fatal("first comment has no content field")
	}
	rawBody, ok := firstComment["raw"].(string)
	if !ok {
		t.Fatal("first comment content has no raw field")
	}
	if !strings.Contains(rawBody, "[//]: # \"crobot:fp=") {
		t.Error("posted comment does not contain fingerprint marker")
	}
	if !strings.Contains(rawBody, "[//]: # \"crobot:bot=crobot\"") {
		t.Error("posted comment does not contain bot label marker")
	}
}

// TestEndToEnd_MaxCommentsCap verifies that the engine caps comments at MaxComments.
func TestEndToEnd_MaxCommentsCap(t *testing.T) {
	t.Parallel()

	prMeta := map[string]any{
		"id":          10,
		"title":       "Many changes",
		"description": "",
		"state":       "OPEN",
		"author":      map[string]any{"display_name": "charlie"},
		"source": map[string]any{
			"branch": map[string]any{"name": "feat/bulk"},
			"commit": map[string]any{"hash": "bulk111"},
		},
		"destination": map[string]any{
			"branch": map[string]any{"name": "main"},
			"commit": map[string]any{"hash": "base222"},
		},
	}

	diffstat := map[string]any{
		"values": []map[string]any{
			{
				"new":    map[string]any{"path": "file.go", "type": "commit_file"},
				"old":    map[string]any{"path": "file.go", "type": "commit_file"},
				"status": "modified",
			},
		},
	}

	// Create a diff with many lines.
	var diffBuilder strings.Builder
	diffBuilder.WriteString("diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,50 @@\n package main\n")
	for i := 2; i <= 50; i++ {
		fmt.Fprintf(&diffBuilder, "+line%d\n", i)
	}

	commentsResp := map[string]any{"values": []any{}}

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/acme/svc/pullrequests/10": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, prMeta)
		},
		"/2.0/repositories/acme/svc/pullrequests/10/diffstat": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, diffstat)
		},
		"/2.0/repositories/acme/svc/pullrequests/10/diff": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			fmt.Fprint(w, diffBuilder.String())
		},
		"/2.0/repositories/acme/svc/pullrequests/10/comments": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, commentsResp)
		},
	})
	defer srv.Close()

	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "bot",
		Token:   "secret",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	// Create 10 findings, cap at 3.
	var findings []platform.ReviewFinding
	for i := 0; i < 10; i++ {
		findings = append(findings, platform.ReviewFinding{
			Path:     "file.go",
			Line:     i + 2,
			Side:     "new",
			Severity: "warning",
			Category: "style",
			Message:  fmt.Sprintf("Finding %d", i+1),
		})
	}

	engine := review.NewEngine(client, review.EngineConfig{
		MaxComments:       3,
		DryRun:            true,
		BotLabel:          "crobot",
		SeverityThreshold: "warning",
	})

	result, err := engine.Run(t.Context(), platform.PRRequest{
		Workspace: "acme",
		Repo:      "svc",
		PRNumber:  10,
	}, findings)
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	if result.Summary.Total != 10 {
		t.Errorf("total = %d, want 10", result.Summary.Total)
	}
	if result.Summary.Posted != 3 {
		t.Errorf("posted = %d, want 3", result.Summary.Posted)
	}
	if !result.Summary.MaxCapped {
		t.Error("expected MaxCapped to be true")
	}
	// 7 should be skipped due to max cap.
	cappedCount := 0
	for _, s := range result.Skipped {
		if strings.Contains(s.Reason, "max comments limit") {
			cappedCount++
		}
	}
	if cappedCount != 7 {
		t.Errorf("max-capped skipped = %d, want 7", cappedCount)
	}
}

// TestEndToEnd_DeduplicationFlow verifies that duplicate findings are detected
// when existing bot comments already have matching fingerprints.
func TestEndToEnd_DeduplicationFlow(t *testing.T) {
	t.Parallel()

	prMeta := map[string]any{
		"id":          77,
		"title":       "Dedup test",
		"description": "",
		"state":       "OPEN",
		"author":      map[string]any{"display_name": "eve"},
		"source": map[string]any{
			"branch": map[string]any{"name": "test"},
			"commit": map[string]any{"hash": "test111"},
		},
		"destination": map[string]any{
			"branch": map[string]any{"name": "main"},
			"commit": map[string]any{"hash": "base222"},
		},
	}

	diffstat := map[string]any{
		"values": []map[string]any{
			{
				"new":    map[string]any{"path": "app.go", "type": "commit_file"},
				"old":    map[string]any{"path": "app.go", "type": "commit_file"},
				"status": "modified",
			},
		},
	}

	diff := `diff --git a/app.go b/app.go
--- a/app.go
+++ b/app.go
@@ -1,3 +1,5 @@
 package app
+
+func Run() {}
+func Stop() {}
`

	// Simulate an existing bot comment with a known fingerprint.
	existingFP := "abc123existing"
	commentsResp := map[string]any{
		"values": []map[string]any{
			{
				"id": 999,
				"content": map[string]any{
					"raw": fmt.Sprintf("Old finding\n[//]: # \"crobot:fp=%s\"", existingFP),
				},
				"user":       map[string]any{"display_name": "bot"},
				"inline":     map[string]any{"path": "app.go", "to": 3},
				"created_on": "2025-01-01T00:00:00Z",
			},
		},
	}

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/acme/svc/pullrequests/77": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, prMeta)
		},
		"/2.0/repositories/acme/svc/pullrequests/77/diffstat": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, diffstat)
		},
		"/2.0/repositories/acme/svc/pullrequests/77/diff": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			fmt.Fprint(w, diff)
		},
		"/2.0/repositories/acme/svc/pullrequests/77/comments": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, commentsResp)
		},
	})
	defer srv.Close()

	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "bot",
		Token:   "secret",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	findings := []platform.ReviewFinding{
		{
			Path:        "app.go",
			Line:        3,
			Side:        "new",
			Severity:    "warning",
			Category:    "style",
			Message:     "Old finding (duplicate)",
			Fingerprint: existingFP, // Same fingerprint as existing comment.
		},
		{
			Path:     "app.go",
			Line:     4,
			Side:     "new",
			Severity: "warning",
			Category: "style",
			Message:  "New finding",
		},
	}

	engine := review.NewEngine(client, review.EngineConfig{
		MaxComments:       25,
		DryRun:            true,
		BotLabel:          "crobot",
		SeverityThreshold: "warning",
	})

	result, err := engine.Run(t.Context(), platform.PRRequest{
		Workspace: "acme",
		Repo:      "svc",
		PRNumber:  77,
	}, findings)
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	if result.Summary.Total != 2 {
		t.Errorf("total = %d, want 2", result.Summary.Total)
	}
	if result.Summary.Posted != 1 {
		t.Errorf("posted = %d, want 1", result.Summary.Posted)
	}
	if result.Summary.Duplicate != 1 {
		t.Errorf("duplicate = %d, want 1", result.Summary.Duplicate)
	}
}

// TestEndToEnd_SeverityThreshold verifies that findings below the severity
// threshold are filtered out.
func TestEndToEnd_SeverityThreshold(t *testing.T) {
	t.Parallel()

	prMeta := map[string]any{
		"id":          88,
		"title":       "Threshold test",
		"description": "",
		"state":       "OPEN",
		"author":      map[string]any{"display_name": "frank"},
		"source": map[string]any{
			"branch": map[string]any{"name": "test"},
			"commit": map[string]any{"hash": "test888"},
		},
		"destination": map[string]any{
			"branch": map[string]any{"name": "main"},
			"commit": map[string]any{"hash": "base222"},
		},
	}

	diffstat := map[string]any{
		"values": []map[string]any{
			{
				"new":    map[string]any{"path": "svc.go", "type": "commit_file"},
				"old":    map[string]any{"path": "svc.go", "type": "commit_file"},
				"status": "modified",
			},
		},
	}

	diff := `diff --git a/svc.go b/svc.go
--- a/svc.go
+++ b/svc.go
@@ -1,3 +1,6 @@
 package svc
+
+func A() {}
+func B() {}
+func C() {}
`

	commentsResp := map[string]any{"values": []any{}}

	srv := testServer(t, map[string]http.HandlerFunc{
		"/2.0/repositories/acme/svc/pullrequests/88": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, prMeta)
		},
		"/2.0/repositories/acme/svc/pullrequests/88/diffstat": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, diffstat)
		},
		"/2.0/repositories/acme/svc/pullrequests/88/diff": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			fmt.Fprint(w, diff)
		},
		"/2.0/repositories/acme/svc/pullrequests/88/comments": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, commentsResp)
		},
	})
	defer srv.Close()

	client, err := bitbucket.NewClient(&bitbucket.Config{
		User:    "bot",
		Token:   "secret",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	findings := []platform.ReviewFinding{
		{Path: "svc.go", Line: 3, Side: "new", Severity: "info", Category: "style", Message: "info finding"},
		{Path: "svc.go", Line: 4, Side: "new", Severity: "warning", Category: "style", Message: "warning finding"},
		{Path: "svc.go", Line: 5, Side: "new", Severity: "error", Category: "bug", Message: "error finding"},
	}

	// Set threshold to "error" — only "error" severity should pass.
	engine := review.NewEngine(client, review.EngineConfig{
		MaxComments:       25,
		DryRun:            true,
		BotLabel:          "crobot",
		SeverityThreshold: "error",
	})

	result, err := engine.Run(t.Context(), platform.PRRequest{
		Workspace: "acme",
		Repo:      "svc",
		PRNumber:  88,
	}, findings)
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	if result.Summary.Total != 3 {
		t.Errorf("total = %d, want 3", result.Summary.Total)
	}
	if result.Summary.Posted != 1 {
		t.Errorf("posted = %d, want 1 (only error)", result.Summary.Posted)
	}
	if result.Summary.Skipped != 2 {
		t.Errorf("skipped = %d, want 2", result.Summary.Skipped)
	}
}

// TestEndToEnd_FindingsFromFile verifies reading findings from a JSON file and
// applying them.
func TestEndToEnd_FindingsFromFile(t *testing.T) {
	t.Parallel()

	// Write findings JSON to a temp file.
	findings := []platform.ReviewFinding{
		{
			Path:     "handler.go",
			Line:     10,
			Side:     "new",
			Severity: "warning",
			Category: "security",
			Message:  "Missing input validation",
		},
	}

	data, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "findings.json")
	if err := os.WriteFile(inputFile, data, 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	// Read back and parse.
	fileData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	parsed, err := platform.ParseFindings(fileData)
	if err != nil {
		t.Fatalf("ParseFindings: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("got %d findings, want 1", len(parsed))
	}

	// Validate each finding.
	for i, f := range parsed {
		if err := f.Validate(); err != nil {
			t.Errorf("finding[%d] validation failed: %v", i, err)
		}
	}
}
