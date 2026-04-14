package stack

import (
	"fmt"
	"strings"

	"github.com/rodrifs/jjstack/internal/config"
	"github.com/rodrifs/jjstack/internal/github"
	"github.com/rodrifs/jjstack/internal/jj"
)

// SubmitResult holds the outcome of a submit operation.
type SubmitResult struct {
	Stack   Stack
	Created []string // bookmark names for newly created PRs
	Updated []string // bookmark names for updated PRs
}

// Submit pushes all entries in the stack to origin and creates or updates
// their GitHub PRs. stackState is updated in place; call config.Save afterwards.
func Submit(s Stack, stackState *config.StackState, dryRun bool) (*SubmitResult, error) {
	result := &SubmitResult{Stack: s}

	for i := range s {
		entry := &s[i]

		if dryRun {
			fmt.Printf("  [dry-run] would push bookmark %q\n", entry.Bookmark)
			fmt.Printf("  [dry-run] would create/update PR: %q → %q\n", entry.Bookmark, entry.ParentBookmark)
			continue
		}

		// Push the bookmark to origin.
		if err := jj.Push(entry.Bookmark); err != nil {
			return nil, fmt.Errorf("pushing %q: %w", entry.Bookmark, err)
		}

		// Check if we already have a PR recorded in state.
		bs, hasState := stackState.Bookmarks[entry.Bookmark]

		var pr *github.PR
		if hasState && bs.PR > 0 {
			var err error
			pr, err = github.GetPR(bs.PR)
			if err != nil {
				return nil, fmt.Errorf("fetching PR #%d for %q: %w", bs.PR, entry.Bookmark, err)
			}
		} else {
			// Check if there's already an open PR for this head (e.g. from a previous run
			// that lost state, or created manually).
			var err error
			pr, err = github.FindPRForHead(entry.Bookmark)
			if err != nil {
				return nil, fmt.Errorf("searching for existing PR for %q: %w", entry.Bookmark, err)
			}
		}

		if pr == nil {
			// No existing PR — create with a placeholder body; we'll update all bodies
			// after the full stack is known (so URLs can be included).
			number, url, err := github.CreatePR(entry.Description, "", entry.ParentBookmark, entry.Bookmark)
			if err != nil {
				return nil, fmt.Errorf("creating PR for %q: %w", entry.Bookmark, err)
			}
			stackState.Bookmarks[entry.Bookmark] = config.BookmarkState{PR: number, PRURL: url}
			entry.PR = &github.PR{Number: number, URL: url, State: "OPEN", BaseRefName: entry.ParentBookmark}
			result.Created = append(result.Created, entry.Bookmark)
		} else {
			// Existing PR — update base if it drifted.
			if pr.BaseRefName != entry.ParentBookmark {
				if err := github.UpdatePRBase(pr.Number, entry.ParentBookmark); err != nil {
					return nil, fmt.Errorf("updating base of PR #%d for %q: %w", pr.Number, entry.Bookmark, err)
				}
				pr.BaseRefName = entry.ParentBookmark
				result.Updated = append(result.Updated, entry.Bookmark)
			}
			stackState.Bookmarks[entry.Bookmark] = config.BookmarkState{PR: pr.Number, PRURL: pr.URL}
			entry.PR = pr
		}
	}

	if dryRun {
		return result, nil
	}

	// Persist stack order (bottom-up) so sync and status can use it.
	order := make([]string, len(s))
	for i, e := range s {
		order[i] = e.Bookmark
	}
	stackState.Order = order

	// Second pass: update every PR body now that all URLs are known.
	// We preserve any user-written content and only manage the jjstack section.
	for i, entry := range s {
		if entry.PR == nil {
			continue
		}
		// For new PRs use the user body collected before submit (avoids a round-trip).
		// For existing PRs fetch the current body to preserve any edits made on GitHub.
		var existing string
		if entry.UserBody != "" {
			existing = entry.UserBody
		} else {
			var err error
			existing, err = github.GetPRBody(entry.PR.Number)
			if err != nil {
				return nil, fmt.Errorf("fetching body of PR #%d for %q: %w", entry.PR.Number, entry.Bookmark, err)
			}
		}
		section := buildStackSection(s, i)
		merged := mergeBody(existing, section)
		if err := github.UpdatePRBody(entry.PR.Number, merged); err != nil {
			return nil, fmt.Errorf("updating body of PR #%d for %q: %w", entry.PR.Number, entry.Bookmark, err)
		}
	}

	return result, nil
}

const stackSentinelStart = "<!-- jjstack:start -->"
const stackSentinelEnd = "<!-- jjstack:end -->"

// buildStackSection builds only the jjstack-managed block (wrapped in sentinels).
func buildStackSection(s Stack, currentIdx int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", stackSentinelStart)
	sb.WriteString("Stacked PRs\n")
	for i := len(s) - 1; i >= 0; i-- {
		e := s[i]
		url := "(pending)"
		if e.PR != nil {
			url = e.PR.URL
		}
		if i == currentIdx {
			fmt.Fprintf(&sb, "- -> %s\n", url)
		} else {
			fmt.Fprintf(&sb, "- %s\n", url)
		}
	}
	fmt.Fprintf(&sb, "%s", stackSentinelEnd)
	return sb.String()
}

// mergeBody splices the jjstack section into the existing PR body.
// If a jjstack section already exists it is replaced; otherwise the section
// is appended after the user's content.
func mergeBody(existing, section string) string {
	start := strings.Index(existing, stackSentinelStart)
	end := strings.Index(existing, stackSentinelEnd)
	if start != -1 && end != -1 {
		// Replace the existing jjstack block.
		userContent := strings.TrimRight(existing[:start], "\n")
		return userContent + "\n\n" + section
	}
	// No existing block — append after user content.
	trimmed := strings.TrimRight(existing, "\n")
	if trimmed == "" {
		return section
	}
	return trimmed + "\n\n" + section
}
