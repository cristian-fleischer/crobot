package cli

import (
	"strings"
	"testing"
)

func TestServeCmd_Registered(t *testing.T) {
	t.Parallel()

	cmd := RootCmd()
	serveCmd, _, err := cmd.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("serve command not found: %v", err)
	}
	if serveCmd.Use != "serve" {
		t.Errorf("command Use = %q, want %q", serveCmd.Use, "serve")
	}
}

func TestServeCmd_MCPFlagExists(t *testing.T) {
	t.Parallel()

	cmd := newServeCmd()
	flag := cmd.Flags().Lookup("mcp")
	if flag == nil {
		t.Fatal("--mcp flag not found")
	}
	if flag.DefValue != "false" {
		t.Errorf("--mcp default = %q, want %q", flag.DefValue, "false")
	}
}

func TestServeCmd_NoModeError(t *testing.T) {
	t.Parallel()

	cmd := RootCmd()
	cmd.SetArgs([]string{"serve"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no mode specified")
	}
	if !strings.Contains(err.Error(), "specify a server mode") {
		t.Errorf("error %q does not contain expected message", err.Error())
	}
}
