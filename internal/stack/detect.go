package stack

import (
	"fmt"
	"strings"

	"github.com/rodrifs/jjstack/internal/jj"
)

// Detect builds a Stack from the given target bookmark down to base.
// It uses the revset `base::target ~ base` to find all commits between them,
// then filters to only commits that carry at least one local bookmark.
// Returns entries in bottom-up order (closest to base first).
func Detect(target, base string) (Stack, error) {
	// Get commits between base and target that have bookmarks.
	// We exclude base itself since it's not part of the stack.
	revset := fmt.Sprintf("%s::%s ~ %s", base, target, base)
	entries, err := jj.Log(revset)
	if err != nil {
		return nil, fmt.Errorf("detecting stack from %q to %q: %w", target, base, err)
	}

	// Filter to only commits that carry at least one local bookmark.
	var bookmarked []jj.LogEntry
	for _, e := range entries {
		if len(e.Bookmarks) > 0 {
			bookmarked = append(bookmarked, e)
		}
	}

	if len(bookmarked) == 0 {
		return nil, fmt.Errorf("no bookmarked commits found between %q and %q", base, target)
	}

	// jj log returns newest-first; reverse for bottom-up order.
	for i, j := 0, len(bookmarked)-1; i < j; i, j = i+1, j-1 {
		bookmarked[i], bookmarked[j] = bookmarked[j], bookmarked[i]
	}

	// Build stack entries, setting parent bookmark for each.
	stack := make(Stack, len(bookmarked))
	for i, e := range bookmarked {
		parent := base
		if i > 0 {
			parent = stack[i-1].Bookmark
		}
		stack[i] = Entry{
			Bookmark:       e.Bookmarks[0], // use first bookmark if a commit has multiple
			ChangeID:       e.ChangeID,
			CommitID:       e.CommitID,
			Description:    e.Description,
			ParentBookmark: parent,
		}
	}

	return stack, nil
}

// DetectForImport is like Detect but also considers remote-only bookmarks.
// This allows importing stacks where local bookmarks haven't been created yet.
// The target may include a remote suffix (e.g. "mybookmark@origin"); it will
// be stripped for the purposes of bookmark name lookups and PR head matching.
func DetectForImport(target, base string) (Stack, error) {
	// Strip @remote suffix so the revset resolves correctly even when the
	// caller passes a remote-qualified name.
	canonicalTarget := strings.SplitN(target, "@", 2)[0]

	// Try the target as given, then with @origin appended, then with the
	// canonical name (stripped of any @remote suffix). This lets callers pass
	// "mybookmark", "mybookmark@origin", or any remote-qualified form.
	candidates := []string{target}
	if canonicalTarget == target {
		candidates = append(candidates, target+"@origin")
	} else {
		candidates = append(candidates, canonicalTarget)
	}

	var entries []jj.LogEntry
	var lastErr error
	for _, candidate := range candidates {
		revset := fmt.Sprintf("%s::%s ~ %s", base, candidate, base)
		var err error
		entries, err = jj.LogAll(revset)
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("detecting stack from %q to %q: %w", canonicalTarget, base, lastErr)
	}

	var bookmarked []jj.LogEntry
	for _, e := range entries {
		if len(e.Bookmarks) > 0 {
			bookmarked = append(bookmarked, e)
		}
	}
	if len(bookmarked) == 0 {
		return nil, fmt.Errorf("no bookmarked commits found between %q and %q", base, canonicalTarget)
	}

	for i, j := 0, len(bookmarked)-1; i < j; i, j = i+1, j-1 {
		bookmarked[i], bookmarked[j] = bookmarked[j], bookmarked[i]
	}

	stack := make(Stack, len(bookmarked))
	for i, e := range bookmarked {
		parent := base
		if i > 0 {
			parent = stack[i-1].Bookmark
		}
		stack[i] = Entry{
			Bookmark:       e.Bookmarks[0],
			ChangeID:       e.ChangeID,
			CommitID:       e.CommitID,
			Description:    e.Description,
			ParentBookmark: parent,
		}
	}
	return stack, nil
}
