package bitbucket

import (
	"testing"
)

func TestParseDiff_SingleHunk(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/src/auth.ts b/src/auth.ts
index abc1234..def5678 100644
--- a/src/auth.ts
+++ b/src/auth.ts
@@ -10,5 +10,7 @@ function authenticate() {
   const token = getToken();
+  console.log(token);
+  validateToken(token);
   return token;
 }
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	h := hunks[0]
	if h.Path != "src/auth.ts" {
		t.Errorf("Path: got %q, want %q", h.Path, "src/auth.ts")
	}
	if h.OldStart != 10 {
		t.Errorf("OldStart: got %d, want 10", h.OldStart)
	}
	if h.OldLines != 5 {
		t.Errorf("OldLines: got %d, want 5", h.OldLines)
	}
	if h.NewStart != 10 {
		t.Errorf("NewStart: got %d, want 10", h.NewStart)
	}
	if h.NewLines != 7 {
		t.Errorf("NewLines: got %d, want 7", h.NewLines)
	}
}

func TestParseDiff_MultipleFiles(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/file1.go b/file1.go
index abc..def 100644
--- a/file1.go
+++ b/file1.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 func main() {
 }
diff --git a/file2.go b/file2.go
index ghi..jkl 100644
--- a/file2.go
+++ b/file2.go
@@ -5,2 +5,3 @@ func helper() {
   x := 1
+  y := 2
 }
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	if hunks[0].Path != "file1.go" {
		t.Errorf("hunks[0].Path: got %q, want %q", hunks[0].Path, "file1.go")
	}
	if hunks[1].Path != "file2.go" {
		t.Errorf("hunks[1].Path: got %q, want %q", hunks[1].Path, "file2.go")
	}
}

func TestParseDiff_MultipleHunksSameFile(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/main.go b/main.go
index abc..def 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 func main() {
 }
@@ -20,3 +21,4 @@ func helper() {
   x := 1
+  y := 2
 }
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	if hunks[0].OldStart != 1 {
		t.Errorf("hunks[0].OldStart: got %d, want 1", hunks[0].OldStart)
	}
	if hunks[1].OldStart != 20 {
		t.Errorf("hunks[1].OldStart: got %d, want 20", hunks[1].OldStart)
	}
	if hunks[0].Path != "main.go" {
		t.Errorf("hunks[0].Path: got %q, want %q", hunks[0].Path, "main.go")
	}
	if hunks[1].Path != "main.go" {
		t.Errorf("hunks[1].Path: got %q, want %q", hunks[1].Path, "main.go")
	}
}

func TestParseDiff_NewFile(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,5 @@
+package main
+
+func newFunc() {
+  return
+}
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].Path != "new_file.go" {
		t.Errorf("Path: got %q, want %q", hunks[0].Path, "new_file.go")
	}
	if hunks[0].OldStart != 0 {
		t.Errorf("OldStart: got %d, want 0", hunks[0].OldStart)
	}
	if hunks[0].OldLines != 0 {
		t.Errorf("OldLines: got %d, want 0", hunks[0].OldLines)
	}
	if hunks[0].NewStart != 1 {
		t.Errorf("NewStart: got %d, want 1", hunks[0].NewStart)
	}
	if hunks[0].NewLines != 5 {
		t.Errorf("NewLines: got %d, want 5", hunks[0].NewLines)
	}
}

func TestParseDiff_DeletedFile(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/old_file.go b/old_file.go
deleted file mode 100644
index abc1234..0000000
--- a/old_file.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package main
-
-func oldFunc() {}
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].Path != "old_file.go" {
		t.Errorf("Path: got %q, want %q", hunks[0].Path, "old_file.go")
	}
	if hunks[0].OldStart != 1 {
		t.Errorf("OldStart: got %d, want 1", hunks[0].OldStart)
	}
	if hunks[0].NewLines != 0 {
		t.Errorf("NewLines: got %d, want 0", hunks[0].NewLines)
	}
}

func TestParseDiff_Rename(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/old_name.go b/new_name.go
similarity index 100%
rename from old_name.go
rename to new_name.go
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Pure rename with no content changes — no @@ hunks
	if len(hunks) != 0 {
		t.Errorf("expected 0 hunks for pure rename, got %d", len(hunks))
	}
}

func TestParseDiff_RenameWithChanges(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/old_name.go b/new_name.go
similarity index 80%
rename from old_name.go
rename to new_name.go
index abc..def 100644
--- a/old_name.go
+++ b/new_name.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 func f() {
 }
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].Path != "new_name.go" {
		t.Errorf("Path: got %q, want %q", hunks[0].Path, "new_name.go")
	}
}

func TestParseDiff_BinaryFile(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/image.png b/image.png
index abc..def 100644
Binary files a/image.png and b/image.png differ
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk for binary, got %d", len(hunks))
	}
	if hunks[0].Path != "image.png" {
		t.Errorf("Path: got %q, want %q", hunks[0].Path, "image.png")
	}
	if hunks[0].OldStart != 0 && hunks[0].NewStart != 0 {
		t.Error("binary hunks should have zero line numbers")
	}
}

func TestParseDiff_EmptyDiff(t *testing.T) {
	t.Parallel()

	hunks, err := parseDiff("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 0 {
		t.Errorf("expected 0 hunks, got %d", len(hunks))
	}
}

func TestParseDiff_HunkBodyContent(t *testing.T) {
	t.Parallel()

	raw := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
 package main
-func old() {}
+func new() {}
`

	hunks, err := parseDiff(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	body := hunks[0].Body
	if body == "" {
		t.Fatal("expected non-empty hunk body")
	}
	// Body should start with the @@ header
	if body[:2] != "@@" {
		t.Errorf("body should start with @@, got: %q", body[:10])
	}
	// Body should contain the diff lines
	if !contains(body, "-func old() {}") {
		t.Errorf("body should contain removed line, got: %q", body)
	}
	if !contains(body, "+func new() {}") {
		t.Errorf("body should contain added line, got: %q", body)
	}
}

func TestParseHunkHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		oldStart int
		oldLines int
		newStart int
		newLines int
		wantErr  bool
	}{
		{
			name:     "standard",
			header:   "@@ -10,5 +10,7 @@ function foo()",
			oldStart: 10, oldLines: 5, newStart: 10, newLines: 7,
		},
		{
			name:     "no context",
			header:   "@@ -1,3 +1,3 @@",
			oldStart: 1, oldLines: 3, newStart: 1, newLines: 3,
		},
		{
			name:     "single line",
			header:   "@@ -1 +1 @@",
			oldStart: 1, oldLines: 1, newStart: 1, newLines: 1,
		},
		{
			name:     "new file",
			header:   "@@ -0,0 +1,5 @@",
			oldStart: 0, oldLines: 0, newStart: 1, newLines: 5,
		},
		{
			name:    "not a hunk",
			header:  "not a hunk header",
			wantErr: true,
		},
		{
			name:    "missing closing @@",
			header:  "@@ -1,3 +1,3 ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldStart, oldLines, newStart, newLines, err := parseHunkHeader(tt.header)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if oldStart != tt.oldStart {
				t.Errorf("oldStart: got %d, want %d", oldStart, tt.oldStart)
			}
			if oldLines != tt.oldLines {
				t.Errorf("oldLines: got %d, want %d", oldLines, tt.oldLines)
			}
			if newStart != tt.newStart {
				t.Errorf("newStart: got %d, want %d", newStart, tt.newStart)
			}
			if newLines != tt.newLines {
				t.Errorf("newLines: got %d, want %d", newLines, tt.newLines)
			}
		})
	}
}

func TestParseFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "standard",
			line: "diff --git a/src/auth.ts b/src/auth.ts",
			want: "src/auth.ts",
		},
		{
			name: "nested path",
			line: "diff --git a/pkg/internal/handler.go b/pkg/internal/handler.go",
			want: "pkg/internal/handler.go",
		},
		{
			name: "root file",
			line: "diff --git a/README.md b/README.md",
			want: "README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseFilePath(tt.line)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
