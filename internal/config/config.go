package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

// State is the full persisted state for jjstack in a repo.
type State struct {
	BaseBranch string                   `json:"base_branch"`
	StackOrder []string                 `json:"stack_order"` // bookmark names bottom-up
	Bookmarks  map[string]BookmarkState `json:"bookmarks"`
}

// Load reads the state file from the repo root. Returns a default State if none exists.
func Load() (*State, error) {
	root, err := jj.RepoRoot()
	if err != nil {
		return nil, fmt.Errorf("not inside a jj repository: %w", err)
	}
	path := filepath.Join(root, stateDir, stateFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{
			BaseBranch: "main",
			Bookmarks:  make(map[string]BookmarkState),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("corrupt state file %s: %w", path, err)
	}
	if s.Bookmarks == nil {
		s.Bookmarks = make(map[string]BookmarkState)
	}
	return &s, nil
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
