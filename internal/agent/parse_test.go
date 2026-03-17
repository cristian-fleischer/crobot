package agent

import (
	"testing"
)

func TestExtractFindings(t *testing.T) {
	t.Parallel()

	validFinding := `{"path":"src/auth.go","line":10,"side":"new","severity":"warning","category":"security","message":"Token not validated","fingerprint":""}`

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:  "raw JSON array with one finding",
			input: `[` + validFinding + `]`,
			want:  1,
		},
		{
			name:  "raw JSON empty array",
			input: `[]`,
			want:  0,
		},
		{
			name:  "markdown fenced json",
			input: "```json\n[" + validFinding + "]\n```",
			want:  1,
		},
		{
			name:  "markdown fenced without json tag",
			input: "```\n[" + validFinding + "]\n```",
			want:  1,
		},
		{
			name:  "text with embedded JSON",
			input: "Here are my findings:\n```json\n[" + validFinding + "]\n```\nLet me explain the issue.",
			want:  1,
		},
		{
			name: "multiple findings",
			input: `[
				{"path":"a.go","line":1,"side":"new","severity":"error","category":"bug","message":"null deref","fingerprint":""},
				{"path":"b.go","line":5,"side":"new","severity":"info","category":"style","message":"unused var","fingerprint":""}
			]`,
			want: 2,
		},
		{
			name:  "bare array in text without fences",
			input: "I found the following issues:\n[" + validFinding + "]\nThat's all.",
			want:  1,
		},
		{
			name:  "empty array in fences",
			input: "No issues found:\n```json\n[]\n```",
			want:  0,
		},
		{
			name:    "no JSON at all",
			input:   "I found no issues with this code. Everything looks good.",
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			input:   `[{"path": "broken}]`,
			wantErr: true,
		},
		{
			name:    "empty output",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   \n\t  ",
			wantErr: true,
		},
		{
			name:    "invalid JSON in fences",
			input:   "```json\n{not valid json}\n```",
			wantErr: true,
		},
		{
			name: "multiple fences picks valid one",
			input: "Here's some data:\n```json\n{\"not\": \"an array\"}\n```\n\nFindings:\n```json\n[" +
				validFinding + "]\n```",
			want: 1,
		},
		{
			name:  "raw array with whitespace padding",
			input: "  \n\n  [" + validFinding + "]  \n\n  ",
			want:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			findings, err := ExtractFindings(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractFindings() expected error, got %d findings", len(findings))
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractFindings() unexpected error: %v", err)
			}
			if len(findings) != tt.want {
				t.Errorf("ExtractFindings() got %d findings, want %d", len(findings), tt.want)
			}
		})
	}
}
