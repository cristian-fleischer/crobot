package mcp

import (
	"testing"

	"github.com/cristian-fleischer/crobot/internal/config"
)

func TestNewServer_Success(t *testing.T) {
	t.Parallel()

	mock := &mockPlatform{}
	cfg := config.Defaults()

	srv, err := NewServer(mock, cfg)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer returned nil server")
	}
	if srv.stdioServer == nil {
		t.Fatal("internal stdio server is nil")
	}
}

func TestNewServer_RegistersAllTools(t *testing.T) {
	t.Parallel()

	defs := toolDefinitions()
	expectedTools := map[string]bool{
		"export_pr_context":     false,
		"get_file_snippet":      false,
		"list_bot_comments":     false,
		"export_local_context":  false,
		"apply_review_findings": false,
	}

	for _, td := range defs {
		if _, ok := expectedTools[td.name]; !ok {
			t.Errorf("unexpected tool registered: %q", td.name)
		}
		expectedTools[td.name] = true
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not registered", name)
		}
	}
}
