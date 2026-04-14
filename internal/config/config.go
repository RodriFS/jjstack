package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rodrifs/jjstack/internal/jj"
)

const stateDir = ".jjstack"
const stateFile = "state.json"
const gitignoreFile = ".gitignore"

// BookmarkState tracks GitHub PR info for a single bookmark.
type BookmarkState struct {
	PR    int    `json:"pr"`
	PRURL string `json:"pr_url"`
}

// StackState holds all data for a single tracked stack.
type StackState struct {
	ID        string                   `json:"id"`
	Base      string                   `json:"base"`
	Order     []string                 `json:"order"`     // bookmark names, bottom-up
	Bookmarks map[string]BookmarkState `json:"bookmarks"` // bookmark name → PR info
}

// State is the full persisted state for jjstack in a repo.
type State struct {
	Stacks []*StackState `json:"stacks"`
}

// NewStack creates a new StackState with a random ID and appends it to s.
func (s *State) NewStack(base string) *StackState {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	st := &StackState{
		ID:        hex.EncodeToString(b),
		Base:      base,
		Bookmarks: make(map[string]BookmarkState),
	}
	s.Stacks = append(s.Stacks, st)
	return st
}

// FindStackByID returns the first stack whose ID starts with the given prefix.
func (s *State) FindStackByID(id string) *StackState {
	for _, st := range s.Stacks {
		if strings.HasPrefix(st.ID, id) {
			return st
		}
	}
	return nil
}

// FindStackByBookmark returns the stack that tracks the given bookmark, or nil.
func (s *State) FindStackByBookmark(bookmark string) *StackState {
	for _, st := range s.Stacks {
		if _, ok := st.Bookmarks[bookmark]; ok {
			return st
		}
	}
	return nil
}

// RemoveStack deletes the stack with the given ID from s.
func (s *State) RemoveStack(id string) {
	filtered := s.Stacks[:0]
	for _, st := range s.Stacks {
		if st.ID != id {
			filtered = append(filtered, st)
		}
	}
	s.Stacks = filtered
}

// legacyState is the old single-stack format, kept for migration only.
type legacyState struct {
	BaseBranch string                   `json:"base_branch"`
	StackOrder []string                 `json:"stack_order"`
	Bookmarks  map[string]BookmarkState `json:"bookmarks"`
}

// Load reads the state file from the repo root. Returns an empty State if none exists.
// Automatically migrates the old single-stack format to the new multi-stack format.
func Load() (*State, error) {
	root, err := jj.RepoRoot()
	if err != nil {
		return nil, fmt.Errorf("not inside a jj repository: %w", err)
	}
	path := filepath.Join(root, stateDir, stateFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}

	// Try the current multi-stack format first.
	var s State
	if err := json.Unmarshal(data, &s); err == nil && s.Stacks != nil {
		for _, st := range s.Stacks {
			if st.Bookmarks == nil {
				st.Bookmarks = make(map[string]BookmarkState)
			}
		}
		return &s, nil
	}

	// Fall back to the legacy single-stack format and migrate it.
	var legacy legacyState
	if err := json.Unmarshal(data, &legacy); err == nil && legacy.Bookmarks != nil {
		s = State{}
		if len(legacy.Bookmarks) > 0 {
			b := make([]byte, 4)
			_, _ = rand.Read(b)
			st := &StackState{
				ID:        hex.EncodeToString(b),
				Base:      legacy.BaseBranch,
				Order:     legacy.StackOrder,
				Bookmarks: legacy.Bookmarks,
			}
			s.Stacks = []*StackState{st}
		}
		return &s, nil
	}

	return &State{}, nil
}

// Save writes the state file to the repo root, creating the directory if needed.
func Save(s *State) error {
	root, err := jj.RepoRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, stateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ensureGitignore(dir)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, stateFile), append(data, '\n'), 0o644)
}

// ensureGitignore writes a .gitignore in the state dir so it isn't committed.
func ensureGitignore(dir string) {
	path := filepath.Join(dir, gitignoreFile)
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.WriteFile(path, []byte("*\n"), 0o644)
}
