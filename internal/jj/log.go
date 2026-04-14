package jj

import (
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
// change_id | commit_id | bookmarks (comma-sep) | description
// description is last so SplitN(4) is safe even if it contains pipes.
const logTemplate = `change_id.short() ++ "|" ++ commit_id.short() ++ "|" ++ local_bookmarks.join(",") ++ "|" ++ description.first_line() ++ "\n"`

// Log runs jj log with the given revset and returns parsed entries.
// Results are returned in newest-first order (as jj outputs them).
func Log(revset string) ([]LogEntry, error) {
	out, err := Run("log", "--no-graph", "-r", revset, "-T", logTemplate)
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
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
