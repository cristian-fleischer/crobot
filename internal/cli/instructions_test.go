package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestInstructionsCmd_OutputsInstructions(t *testing.T) {
	t.Parallel()

	cmd := RootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"review-instructions"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("review-instructions failed: %v", err)
	}

	output := buf.String()

	required := []string{
		"CRoBot Review Instructions",
		"ReviewFinding Schema",
		"crobot export-pr-context",
		"Workflow",
		"Rules",
	}

	for _, s := range required {
		if !strings.Contains(output, s) {
			t.Errorf("output missing expected content: %q", s)
		}
	}
}
