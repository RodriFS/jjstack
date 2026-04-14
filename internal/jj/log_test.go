package jj

import (
	"testing"
)

func TestParseLog(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []LogEntry
	}{
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:  "single entry no bookmark",
			input: "abc123|def456||first line of description",
			want: []LogEntry{
				{ChangeID: "abc123", CommitID: "def456", Bookmarks: nil, Description: "first line of description"},
			},
		},
		{
			name:  "single entry with bookmark",
			input: "abc123|def456|main|some commit",
			want: []LogEntry{
				{ChangeID: "abc123", CommitID: "def456", Bookmarks: []string{"main"}, Description: "some commit"},
			},
		},
		{
			name:  "strips trailing asterisk from bookmark",
			input: "abc123|def456|feature-x*|unpushed commit",
			want: []LogEntry{
				{ChangeID: "abc123", CommitID: "def456", Bookmarks: []string{"feature-x"}, Description: "unpushed commit"},
			},
		},
		{
			name:  "multiple bookmarks on one commit",
			input: "abc123|def456|main,feature-x|tagged commit",
			want: []LogEntry{
				{ChangeID: "abc123", CommitID: "def456", Bookmarks: []string{"main", "feature-x"}, Description: "tagged commit"},
			},
		},
		{
			name:  "multiple bookmarks with asterisk",
			input: "abc123|def456|feature-x*,feature-y|mixed push state",
			want: []LogEntry{
				{ChangeID: "abc123", CommitID: "def456", Bookmarks: []string{"feature-x", "feature-y"}, Description: "mixed push state"},
			},
		},
		{
			name: "multiple entries",
			input: "aaa|111|feat-top*|top commit\nbbb|222|feat-bot|bottom commit\n",
			want: []LogEntry{
				{ChangeID: "aaa", CommitID: "111", Bookmarks: []string{"feat-top"}, Description: "top commit"},
				{ChangeID: "bbb", CommitID: "222", Bookmarks: []string{"feat-bot"}, Description: "bottom commit"},
			},
		},
		{
			name:  "description with pipe character",
			input: "abc|def|mybranch|fix: handle a|b edge case",
			want: []LogEntry{
				{ChangeID: "abc", CommitID: "def", Bookmarks: []string{"mybranch"}, Description: "fix: handle a|b edge case"},
			},
		},
		{
			name:  "blank lines are skipped",
			input: "abc|def|main|commit\n\n   \n",
			want: []LogEntry{
				{ChangeID: "abc", CommitID: "def", Bookmarks: []string{"main"}, Description: "commit"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLog(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d\ngot:  %+v\nwant: %+v", len(got), len(tt.want), got, tt.want)
			}
			for i, g := range got {
				w := tt.want[i]
				if g.ChangeID != w.ChangeID || g.CommitID != w.CommitID || g.Description != w.Description {
					t.Errorf("entry %d: got %+v, want %+v", i, g, w)
				}
				if len(g.Bookmarks) != len(w.Bookmarks) {
					t.Errorf("entry %d bookmarks: got %v, want %v", i, g.Bookmarks, w.Bookmarks)
					continue
				}
				for j, b := range g.Bookmarks {
					if b != w.Bookmarks[j] {
						t.Errorf("entry %d bookmark %d: got %q, want %q", i, j, b, w.Bookmarks[j])
					}
				}
			}
		})
	}
}
