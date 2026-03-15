package mcp

import (
	"encoding/json"
	"testing"
)

func TestToolDefinitions_Count(t *testing.T) {
	t.Parallel()

	defs := toolDefinitions()
	if len(defs) != 4 {
		t.Errorf("expected 4 tool definitions, got %d", len(defs))
	}
}

func TestToolDefinitions_Names(t *testing.T) {
	t.Parallel()

	expectedNames := []string{
		"export_pr_context",
		"get_file_snippet",
		"list_bot_comments",
		"apply_review_findings",
	}

	defs := toolDefinitions()
	for i, expected := range expectedNames {
		if i >= len(defs) {
			t.Fatalf("missing tool definition at index %d", i)
		}
		if defs[i].name != expected {
			t.Errorf("tool[%d].name = %q, want %q", i, defs[i].name, expected)
		}
		if defs[i].tool.Name != expected {
			t.Errorf("tool[%d].tool.Name = %q, want %q", i, defs[i].tool.Name, expected)
		}
	}
}

func TestToolDefinitions_HaveDescriptions(t *testing.T) {
	t.Parallel()

	for _, td := range toolDefinitions() {
		if td.tool.Description == "" {
			t.Errorf("tool %q has no description", td.name)
		}
	}
}

func TestToolDefinitions_SerializeToJSON(t *testing.T) {
	t.Parallel()

	for _, td := range toolDefinitions() {
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(td.tool)
			if err != nil {
				t.Fatalf("failed to marshal tool %q: %v", td.name, err)
			}
			if len(data) == 0 {
				t.Errorf("tool %q serialized to empty JSON", td.name)
			}

			// Verify it contains the tool name.
			var parsed map[string]any
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("failed to unmarshal tool JSON: %v", err)
			}
			if parsed["name"] != td.name {
				t.Errorf("serialized name = %v, want %q", parsed["name"], td.name)
			}
		})
	}
}

func TestToolDefinitions_InputSchemaHasRequiredFields(t *testing.T) {
	t.Parallel()

	// Expected required fields per tool.
	expectedRequired := map[string][]string{
		"export_pr_context":     {"workspace", "repo", "pr"},
		"get_file_snippet":      {"workspace", "repo", "commit", "path", "line"},
		"list_bot_comments":     {"workspace", "repo", "pr"},
		"apply_review_findings": {"workspace", "repo", "pr", "findings"},
	}

	for _, td := range toolDefinitions() {
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()

			expected, ok := expectedRequired[td.name]
			if !ok {
				t.Fatalf("no expected required fields for %q", td.name)
			}

			required := td.tool.InputSchema.Required
			requiredSet := make(map[string]bool)
			for _, r := range required {
				requiredSet[r] = true
			}

			for _, field := range expected {
				if !requiredSet[field] {
					t.Errorf("tool %q: field %q should be required", td.name, field)
				}
			}
		})
	}
}
