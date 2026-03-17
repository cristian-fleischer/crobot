package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n    int
		want string
	}{
		{0, "0B"},
		{1, "1B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{102400, "100.0KB"},
		{1048576, "1.0MB"},
		{1572864, "1.5MB"},
		{10485760, "10.0MB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := formatBytes(tt.n); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestProgressWriter_NonTerminal_Passthrough(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	// Use /dev/null as the "terminal" fd — it's not a terminal, so
	// progressWriter should fall back to plain pass-through.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	pw := newProgressWriter(&buf, devNull, 100, "test-agent")
	defer pw.Finish()

	data := []byte("hello world")
	n, err := pw.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}
	if buf.String() != "hello world" {
		t.Errorf("got %q, want %q", buf.String(), "hello world")
	}
}

func TestProgressWriter_SetActivity(t *testing.T) {
	t.Parallel()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	pw := newProgressWriter(&bytes.Buffer{}, devNull, 0, "test")
	defer pw.Finish()

	pw.SetActivity("thinking")
	pw.mu.Lock()
	got := pw.activity
	pw.mu.Unlock()

	if got != "thinking" {
		t.Errorf("activity = %q, want %q", got, "thinking")
	}
}

func TestProgressWriter_SetModel(t *testing.T) {
	t.Parallel()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	pw := newProgressWriter(&bytes.Buffer{}, devNull, 0, "test")
	defer pw.Finish()

	pw.SetModel("claude-sonnet-4-20250514")
	pw.mu.Lock()
	got := pw.model
	pw.mu.Unlock()

	if got != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want %q", got, "claude-sonnet-4-20250514")
	}
}

func TestProgressWriter_NonTerminal_NoByteCounting(t *testing.T) {
	t.Parallel()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	pw := newProgressWriter(&bytes.Buffer{}, devNull, 0, "test")
	defer pw.Finish()

	pw.Write([]byte("abc"))
	pw.Write([]byte("defgh"))

	// In non-terminal mode, bytes are not tracked (no status bar to display them).
	pw.mu.Lock()
	got := pw.outputBytes
	pw.mu.Unlock()

	if got != 0 {
		t.Errorf("outputBytes = %d, want 0 (non-terminal skips counting)", got)
	}
}

// TestProgressWriter_FinishDoubleClose verifies that calling Finish() twice
// does not panic (SF-6 fix: closeOnce guards).
func TestProgressWriter_FinishDoubleClose(t *testing.T) {
	t.Parallel()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	pw := newProgressWriter(&bytes.Buffer{}, devNull, 100, "test-agent")

	// Should not panic on first or second call.
	pw.Finish()
	pw.Finish() // second call — must not panic
}

// TestProgressWriter_FinishSingle verifies normal single Finish() works.
func TestProgressWriter_FinishSingle(t *testing.T) {
	t.Parallel()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	pw := newProgressWriter(&bytes.Buffer{}, devNull, 50, "agent")
	pw.Write([]byte("some output"))
	pw.Finish() // must not panic or error
}
