package stack

import (
	"strings"
	"testing"

	"github.com/rodrifs/jjstack/internal/config"
)

func makeState(bases ...string) *config.State {
	s := &config.State{}
	for i, base := range bases {
		st := s.NewStack(base)
		st.ID = strings.Repeat(string(rune('a'+i)), 8) // "aaaaaaaa", "bbbbbbbb", ...
	}
	return s
}

func TestResolveNoStacks(t *testing.T) {
	s := &config.State{}
	_, err := Resolve(s, "")
	if err == nil {
		t.Fatal("expected error for empty state")
	}
	if !strings.Contains(err.Error(), "no stacks tracked") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveSingleStack(t *testing.T) {
	s := makeState("main")
	got, err := Resolve(s, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != s.Stacks[0] {
		t.Error("expected the single stack to be returned")
	}
}

func TestResolveByIDExact(t *testing.T) {
	s := makeState("main", "dev")
	want := s.Stacks[1]
	got, err := Resolve(s, want.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected stack %q, got %q", want.ID, got.ID)
	}
}

func TestResolveByIDPrefix(t *testing.T) {
	s := makeState("main", "dev")
	want := s.Stacks[0]
	got, err := Resolve(s, want.ID[:4]) // first 4 chars
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected stack %q, got %q", want.ID, got.ID)
	}
}

func TestResolveByIDNotFound(t *testing.T) {
	s := makeState("main")
	_, err := Resolve(s, "zzzzzzzz")
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
	if !strings.Contains(err.Error(), "no stack with id") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveMultipleStacksAmbiguousError(t *testing.T) {
	// With multiple stacks and no --stack flag, Resolve tries jj inference
	// which will fail in a test environment (no jj subprocess). The error
	// should fall through to the ambiguous message listing all stacks.
	s := makeState("main", "dev")
	s.Stacks[0].Order = []string{"feat-a1", "feat-a2"}
	s.Stacks[1].Order = []string{"feat-b1"}

	_, err := Resolve(s, "")
	if err == nil {
		t.Fatal("expected error when multiple stacks and cannot infer")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "multiple stacks tracked") {
		t.Errorf("expected ambiguous error, got: %v", err)
	}
	// Error should list both stack IDs.
	if !strings.Contains(errMsg, s.Stacks[0].ID) || !strings.Contains(errMsg, s.Stacks[1].ID) {
		t.Errorf("error should list stack IDs, got: %v", err)
	}
}

func TestListStacks(t *testing.T) {
	s := makeState("main", "dev")
	s.Stacks[0].Order = []string{"feat-a1", "feat-a2"}
	s.Stacks[1].Order = []string{"feat-b1"}

	out := listStacks(s)
	if !strings.Contains(out, "feat-a1") || !strings.Contains(out, "feat-a2") {
		t.Errorf("listStacks missing stack A bookmarks: %s", out)
	}
	if !strings.Contains(out, "feat-b1") {
		t.Errorf("listStacks missing stack B bookmarks: %s", out)
	}
	if !strings.Contains(out, "main") || !strings.Contains(out, "dev") {
		t.Errorf("listStacks missing base branches: %s", out)
	}
}
