package jj

import (
	"strings"
)

// Push pushes the given bookmark to origin.
// Uses --allow-new so newly created bookmarks don't require a separate tracking step.
func Push(bookmark string) error {
	_, err := Run("git", "push", "--bookmark", bookmark, "--allow-new")
	return err
}

// ForcePush pushes the given bookmark to origin after a rebase.
// jj's safety model tracks the last-known remote state, so after jj git fetch
// + jj rebase the push proceeds without an explicit force flag.
func ForcePush(bookmark string) error {
	_, err := Run("git", "push", "--bookmark", bookmark, "--allow-new")
	return err
}

// Fetch fetches from origin, updating remote tracking bookmarks.
func Fetch() error {
	_, err := Run("git", "fetch")
	return err
}

// Rebase runs jj rebase -s <bookmark> -d <destination>.
// This rebases the bookmark and all its descendants onto destination,
// leaving any commits below the bookmark (e.g. a merged auth commit)
// untouched. Use -s (source) rather than -b (branch) so that already-merged
// commits in the base are not dragged along.
func Rebase(bookmark, destination string) error {
	_, err := Run("rebase", "-s", bookmark, "-d", destination)
	return err
}

// BookmarkInfo holds info about a local bookmark and its remote counterpart.
type BookmarkInfo struct {
	Name         string
	LocalCommit  string
	RemoteCommit string // empty if not pushed or tracking info unavailable
}

// ListBookmarks returns commit IDs for a set of bookmark names,
// comparing local vs remote (origin) state.
func ListBookmarks(names []string) ([]BookmarkInfo, error) {
	result := make([]BookmarkInfo, 0, len(names))
	for _, name := range names {
		local, err := CommitID(name)
		if err != nil {
			local = "(unknown)"
		}
		remote, err := CommitID(name + "@origin")
		if err != nil {
			remote = "(not pushed)"
		}
		result = append(result, BookmarkInfo{
			Name:         name,
			LocalCommit:  local,
			RemoteCommit: remote,
		})
	}
	return result, nil
}

// RepoRoot returns the root directory of the current jj repo
// by running jj workspace root.
func RepoRoot() (string, error) {
	out, err := Run("workspace", "root")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
