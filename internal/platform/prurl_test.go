package platform

import (
	"testing"
)

func TestParsePRURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		want      *PRRequest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "bitbucket standard",
			url:  "https://bitbucket.org/smartbridge/staffcloud-app/pull-requests/8314",
			want: &PRRequest{Workspace: "smartbridge", Repo: "staffcloud-app", PRNumber: 8314},
		},
		{
			name: "bitbucket trailing slash",
			url:  "https://bitbucket.org/myteam/my-repo/pull-requests/42/",
			want: &PRRequest{Workspace: "myteam", Repo: "my-repo", PRNumber: 42},
		},
		{
			name: "bitbucket with extra path segments",
			url:  "https://bitbucket.org/myteam/my-repo/pull-requests/42/diff",
			want: &PRRequest{Workspace: "myteam", Repo: "my-repo", PRNumber: 42},
		},
		{
			name: "bitbucket with query params",
			url:  "https://bitbucket.org/team/repo/pull-requests/7?tab=commits",
			want: &PRRequest{Workspace: "team", Repo: "repo", PRNumber: 7},
		},
		{
			name: "github standard",
			url:  "https://github.com/owner/repo/pull/123",
			want: &PRRequest{Workspace: "owner", Repo: "repo", PRNumber: 123},
		},
		{
			name: "github trailing slash",
			url:  "https://github.com/owner/repo/pull/42/",
			want: &PRRequest{Workspace: "owner", Repo: "repo", PRNumber: 42},
		},
		{
			name: "github with extra path segments",
			url:  "https://github.com/owner/repo/pull/42/files",
			want: &PRRequest{Workspace: "owner", Repo: "repo", PRNumber: 42},
		},
		{
			name: "github with fragment",
			url:  "https://github.com/owner/repo/pull/42#discussion_r12345",
			want: &PRRequest{Workspace: "owner", Repo: "repo", PRNumber: 42},
		},
		{
			name:      "github wrong path format",
			url:       "https://github.com/owner/repo/issues/42",
			wantErr:   true,
			errSubstr: "invalid GitHub PR URL",
		},
		{
			name:      "github too few segments",
			url:       "https://github.com/owner",
			wantErr:   true,
			errSubstr: "invalid GitHub PR URL",
		},
		{
			name:      "github non-numeric PR",
			url:       "https://github.com/owner/repo/pull/abc",
			wantErr:   true,
			errSubstr: "not a valid PR number",
		},
		{
			name:      "github zero PR",
			url:       "https://github.com/owner/repo/pull/0",
			wantErr:   true,
			errSubstr: "not a valid PR number",
		},
		{
			name:      "github negative PR",
			url:       "https://github.com/owner/repo/pull/-1",
			wantErr:   true,
			errSubstr: "not a valid PR number",
		},
		{
			name: "github large PR number",
			url:  "https://github.com/torvalds/linux/pull/99999",
			want: &PRRequest{Workspace: "torvalds", Repo: "linux", PRNumber: 99999},
		},
		{
			name: "github with query params",
			url:  "https://github.com/org/repo/pull/7?diff=split",
			want: &PRRequest{Workspace: "org", Repo: "repo", PRNumber: 7},
		},
		{
			name:      "unsupported host",
			url:       "https://gitlab.com/owner/repo/merge_requests/123",
			wantErr:   true,
			errSubstr: "unsupported PR URL host",
		},
		{
			name:      "bitbucket wrong path format",
			url:       "https://bitbucket.org/myteam/my-repo/commits/abc123",
			wantErr:   true,
			errSubstr: "invalid Bitbucket PR URL",
		},
		{
			name:      "bitbucket too few segments",
			url:       "https://bitbucket.org/myteam",
			wantErr:   true,
			errSubstr: "invalid Bitbucket PR URL",
		},
		{
			name:      "bitbucket non-numeric PR",
			url:       "https://bitbucket.org/myteam/my-repo/pull-requests/abc",
			wantErr:   true,
			errSubstr: "not a valid PR number",
		},
		{
			name:      "bitbucket zero PR",
			url:       "https://bitbucket.org/myteam/my-repo/pull-requests/0",
			wantErr:   true,
			errSubstr: "not a valid PR number",
		},
		{
			name:      "not a URL",
			url:       "not-a-url",
			wantErr:   true,
			errSubstr: "unsupported PR URL host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParsePRURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !containsStr(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Workspace != tt.want.Workspace {
				t.Errorf("Workspace = %q, want %q", got.Workspace, tt.want.Workspace)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.want.Repo)
			}
			if got.PRNumber != tt.want.PRNumber {
				t.Errorf("PRNumber = %d, want %d", got.PRNumber, tt.want.PRNumber)
			}
		})
	}
}

func TestIsPRURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"https://bitbucket.org/team/repo/pull-requests/42", true},
		{"http://bitbucket.org/team/repo/pull-requests/42", true},
		{"https://github.com/owner/repo/pull/123", true},
		{"http://github.com/owner/repo/pull/123", true},
		{"42", false},
		{"bitbucket.org/team/repo/pull-requests/42", false},
		{"github.com/owner/repo/pull/123", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := IsPRURL(tt.input); got != tt.want {
				t.Errorf("IsPRURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
