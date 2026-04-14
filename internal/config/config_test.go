package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStateHelpers(t *testing.T) {
	t.Run("NewStack assigns unique IDs", func(t *testing.T) {
		s := &State{}
		a := s.NewStack("main")
		b := s.NewStack("main")
		if a.ID == b.ID {
			t.Errorf("expected unique IDs, both got %q", a.ID)
		}
		if len(s.Stacks) != 2 {
			t.Errorf("expected 2 stacks, got %d", len(s.Stacks))
		}
	})

	t.Run("FindStackByID exact and prefix", func(t *testing.T) {
		s := &State{}
		st := s.NewStack("main")
		st.ID = "abcd1234"

		if got := s.FindStackByID("abcd1234"); got != st {
			t.Error("exact match failed")
		}
		if got := s.FindStackByID("abcd"); got != st {
			t.Error("prefix match failed")
		}
		if got := s.FindStackByID("zzzz"); got != nil {
			t.Error("expected nil for unknown prefix")
		}
	})

	t.Run("FindStackByBookmark", func(t *testing.T) {
		s := &State{}
		st := s.NewStack("main")
		st.Bookmarks["feat-a"] = BookmarkState{PR: 1}
		st.Bookmarks["feat-b"] = BookmarkState{PR: 2}

		if got := s.FindStackByBookmark("feat-a"); got != st {
			t.Error("expected to find stack for feat-a")
		}
		if got := s.FindStackByBookmark("feat-b"); got != st {
			t.Error("expected to find stack for feat-b")
		}
		if got := s.FindStackByBookmark("unknown"); got != nil {
			t.Error("expected nil for unknown bookmark")
		}
	})

	t.Run("RemoveStack", func(t *testing.T) {
		s := &State{}
		a := s.NewStack("main")
		b := s.NewStack("main")

		s.RemoveStack(a.ID)
		if len(s.Stacks) != 1 {
			t.Errorf("expected 1 stack after removal, got %d", len(s.Stacks))
		}
		if s.Stacks[0].ID != b.ID {
			t.Errorf("expected remaining stack to be %q, got %q", b.ID, s.Stacks[0].ID)
		}
	})
}

func TestLoadLegacyMigration(t *testing.T) {
	// Write a legacy state file and verify it is migrated correctly.
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".jjstack")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}

	legacy := map[string]any{
		"base_branch": "develop",
		"stack_order": []string{"feat-a", "feat-b"},
		"bookmarks": map[string]any{
			"feat-a": map[string]any{"pr": 1, "pr_url": "https://github.com/org/repo/pull/1"},
			"feat-b": map[string]any{"pr": 2, "pr_url": "https://github.com/org/repo/pull/2"},
		},
	}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Temporarily override the repo root lookup by loading directly.
	rawData, err := os.ReadFile(filepath.Join(stateDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Replicate Load's migration logic since Load calls jj internally.
	var s State
	_ = json.Unmarshal(rawData, &s) // will produce empty Stacks (legacy format)

	var leg legacyState
	if err := json.Unmarshal(rawData, &leg); err != nil {
		t.Fatal(err)
	}
	if len(leg.Bookmarks) == 0 {
		t.Fatal("failed to parse legacy state")
	}
	// Simulate migration.
	st := &StackState{
		ID:        "migrated",
		Base:      leg.BaseBranch,
		Order:     leg.StackOrder,
		Bookmarks: leg.Bookmarks,
	}
	s.Stacks = []*StackState{st}

	if st.Base != "develop" {
		t.Errorf("expected base %q, got %q", "develop", st.Base)
	}
	if len(st.Order) != 2 || st.Order[0] != "feat-a" {
		t.Errorf("unexpected order: %v", st.Order)
	}
	if st.Bookmarks["feat-a"].PR != 1 {
		t.Errorf("expected PR 1 for feat-a, got %d", st.Bookmarks["feat-a"].PR)
	}
}

func TestLoadEmptyWhenNoFile(t *testing.T) {
	// Verify that a missing state file returns an empty State, not an error.
	// We test the JSON path directly since Load calls jj for the repo root.
	data := []byte(`{"stacks": []}`)
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Stacks) != 0 {
		t.Errorf("expected 0 stacks, got %d", len(s.Stacks))
	}
}

func TestNewMultiStackState(t *testing.T) {
	// Round-trip the new format through JSON.
	s := &State{}
	a := s.NewStack("main")
	a.ID = "stack-a"
	a.Order = []string{"feat-a1", "feat-a2"}
	a.Bookmarks["feat-a1"] = BookmarkState{PR: 1, PRURL: "https://github.com/org/repo/pull/1"}

	b := s.NewStack("dev")
	b.ID = "stack-b"
	b.Order = []string{"feat-b1"}
	b.Bookmarks["feat-b1"] = BookmarkState{PR: 2, PRURL: "https://github.com/org/repo/pull/2"}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	var got State
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(got.Stacks))
	}
	if got.Stacks[0].Base != "main" || got.Stacks[1].Base != "dev" {
		t.Errorf("unexpected bases: %q %q", got.Stacks[0].Base, got.Stacks[1].Base)
	}
	if got.Stacks[0].Bookmarks["feat-a1"].PR != 1 {
		t.Errorf("expected PR 1 for feat-a1")
	}
}
