package jj

import (
	"fmt"
	"strings"
)

// LogEntry represents a single commit from jj log output.
type LogEntry struct {
	ChangeID    string
	CommitID    string
	Bookmarks   []string
	Description string
}

// logTemplate outputs pipe-separated fields per line:
// change_id | commit_id | localBookmarks (comma-sep) | description
// description is last so SplitN(4) is safe even if it contains pipes.
const logTemplate = `change_id.short() ++ "|" ++ commit_id.short() ++ "|" ++ local_bookmarks.join(",") ++ "|" ++ description.first_line() ++ "\n"`

// logTemplateAll is like logTemplate but also includes remote bookmark names
// (without the @remote suffix) in a 5th field.
// change_id | commit_id | localBookmarks | remoteBookmarkNames | description
const logTemplateAll = `change_id.short() ++ "|" ++ commit_id.short() ++ "|" ++ local_bookmarks.join(",") ++ "|" ++ remote_bookmarks.map(|b| b.name()).join(",") ++ "|" ++ description.first_line() ++ "\n"`

// Log runs jj log with the given revset and returns parsed entries.
// Only local bookmarks are captured. Results are newest-first.
func Log(revset string) ([]LogEntry, error) {
	out, err := Run("log", "--no-graph", "-r", revset, "-T", logTemplate)
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
}

// LogAll is like Log but captures both local and remote bookmark names,
// deduplicating when both exist for the same commit.
func LogAll(revset string) ([]LogEntry, error) {
	out, err := Run("log", "--no-graph", "-r", revset, "-T", logTemplateAll)
	if err != nil {
		return nil, err
	}
	return parseLogAll(out), nil
}

// CommitID returns the short commit ID of a single revision.
func CommitID(rev string) (string, error) {
	out, err := Run("log", "--no-graph", "-r", rev, "-T", `commit_id.short() ++ "\n"`)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// LogRaw returns the human-readable jj log for display purposes (e.g. dry-run).
func LogRaw(revset string) (string, error) {
	return Run("log", "-r", revset)
}

// AncestorBookmarks returns all local bookmark names that are ancestors of (or
// at) the current working copy revision (@), up to the given depth.
// Used to infer which stack the user is currently working in.
func AncestorBookmarks(depth int) ([]string, error) {
	revset := fmt.Sprintf("ancestors(@, %d) & bookmarks()", depth)
	entries, err := Log(revset)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Bookmarks...)
	}
	return names, nil
}

func parseLog(out string) []LogEntry {
	var entries []LogEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		var bookmarks []string
		if parts[2] != "" {
			for _, b := range strings.Split(parts[2], ",") {
				// jj appends '*' to bookmarks that differ from their remote tracking
				// bookmark (i.e. not yet pushed). Strip it to get the plain name.
				bookmarks = append(bookmarks, strings.TrimRight(b, "*"))
			}
		}
		entries = append(entries, LogEntry{
			ChangeID:    parts[0],
			CommitID:    parts[1],
			Bookmarks:   bookmarks,
			Description: parts[3],
		})
	}
	return entries
}

func parseLogAll(out string) []LogEntry {
	var entries []LogEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		seen := make(map[string]bool)
		var bookmarks []string
		if parts[2] != "" {
			for _, b := range strings.Split(parts[2], ",") {
				b = strings.TrimRight(b, "*")
				if b != "" && !seen[b] {
					seen[b] = true
					bookmarks = append(bookmarks, b)
				}
			}
		}
		// Also add remote bookmark names not already present as local bookmarks.
		if parts[3] != "" {
			for _, b := range strings.Split(parts[3], ",") {
				if b != "" && !seen[b] {
					seen[b] = true
					bookmarks = append(bookmarks, b)
				}
			}
		}
		entries = append(entries, LogEntry{
			ChangeID:    parts[0],
			CommitID:    parts[1],
			Bookmarks:   bookmarks,
			Description: parts[4],
		})
	}
	return entries
}
