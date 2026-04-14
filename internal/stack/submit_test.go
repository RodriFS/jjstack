package stack

import (
	"strings"
	"testing"

	"github.com/rodrifs/jjstack/internal/github"
)

func TestMergeBody(t *testing.T) {
	section := stackSentinelStart + "\nStacked PRs\n- url1\n" + stackSentinelEnd

	tests := []struct {
		name     string
		existing string
		section  string
		want     string
	}{
		{
			name:     "empty existing body",
			existing: "",
			section:  section,
			want:     section,
		},
		{
			name:     "existing user content, no sentinel",
			existing: "My PR description.",
			section:  section,
			want:     "My PR description.\n\n" + section,
		},
		{
			name:     "existing user content with trailing newlines",
			existing: "My PR description.\n\n",
			section:  section,
			want:     "My PR description.\n\n" + section,
		},
		{
			name:     "replaces existing sentinel block",
			existing: "User content.\n\n" + stackSentinelStart + "\nold section\n" + stackSentinelEnd,
			section:  section,
			want:     "User content.\n\n" + section,
		},
		{
			name:     "replaces sentinel block with no user content above",
			existing: stackSentinelStart + "\nold section\n" + stackSentinelEnd,
			section:  section,
			want:     "\n\n" + section,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeBody(tt.existing, tt.section)
			if got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestBuildStackSection(t *testing.T) {
	s := Stack{
		{Bookmark: "feat-a", PR: &github.PR{URL: "https://github.com/org/repo/pull/1"}},
		{Bookmark: "feat-b", PR: &github.PR{URL: "https://github.com/org/repo/pull/2"}},
		{Bookmark: "feat-c", PR: &github.PR{URL: "https://github.com/org/repo/pull/3"}},
	}

	t.Run("current is bottom entry", func(t *testing.T) {
		section := buildStackSection(s, 0)
		if !strings.Contains(section, stackSentinelStart) || !strings.Contains(section, stackSentinelEnd) {
			t.Error("section missing sentinels")
		}
		// feat-c is top (listed first), feat-a is bottom (listed last, and is current)
		lines := strings.Split(section, "\n")
		var prLines []string
		for _, l := range lines {
			if strings.HasPrefix(l, "- ") {
				prLines = append(prLines, l)
			}
		}
		if len(prLines) != 3 {
			t.Fatalf("expected 3 PR lines, got %d: %v", len(prLines), prLines)
		}
		if !strings.HasPrefix(prLines[2], "- -> ") {
			t.Errorf("expected current marker on last line, got: %q", prLines[2])
		}
	})

	t.Run("current is top entry", func(t *testing.T) {
		section := buildStackSection(s, 2)
		lines := strings.Split(section, "\n")
		var prLines []string
		for _, l := range lines {
			if strings.HasPrefix(l, "- ") {
				prLines = append(prLines, l)
			}
		}
		if !strings.HasPrefix(prLines[0], "- -> ") {
			t.Errorf("expected current marker on first line, got: %q", prLines[0])
		}
	})

	t.Run("nil PR shows pending", func(t *testing.T) {
		withNil := Stack{
			{Bookmark: "feat-a", PR: nil},
			{Bookmark: "feat-b", PR: &github.PR{URL: "https://github.com/org/repo/pull/2"}},
		}
		section := buildStackSection(withNil, 0)
		if !strings.Contains(section, "(pending)") {
			t.Errorf("expected (pending) for nil PR, got:\n%s", section)
		}
	})
}
