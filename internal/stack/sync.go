package stack

import (
	"fmt"

	"github.com/rodrifs/jjstack/internal/config"
	"github.com/rodrifs/jjstack/internal/github"
	"github.com/rodrifs/jjstack/internal/jj"
	"github.com/rodrifs/jjstack/internal/ui"
)

// SyncAction describes a single rebase/cleanup action during sync.
type SyncAction struct {
	MergedBookmark string // the bookmark whose PR was merged
	RebasedFrom    string // bookmark that was rebased; empty if it was the top of the stack
	RebasedOnto    string // what it was rebased onto
}

// SyncResult holds the outcome of a sync operation.
type SyncResult struct {
	Actions []SyncAction
}

// Sync detects merged PRs in the stack (bottom-up) and rebases subsequent
// entries onto the new base. Handles both regular and squash merges by checking
// GitHub PR state rather than commit ancestry.
// stackState is updated in place; call config.Save afterwards.
func Sync(s Stack, stackState *config.StackState, dryRun bool) (*SyncResult, error) {
	result := &SyncResult{}

	// Fetch latest remote state so jj knows where origin/* bookmarks are.
	if !dryRun {
		sp := ui.NewSpinner("Fetching from origin")
		sp.Start()
		fetchErr := jj.Fetch()
		sp.Stop()
		if fetchErr != nil {
			return nil, fmt.Errorf("fetching from origin: %w", fetchErr)
		}
	}

	// Walk the stack bottom-up, looking for merged PRs.
	// When we find one, rebase everything above it onto the current base.
	currentBase := stackState.Base
	toRemove := []string{}

	for i, entry := range s {
		bs, ok := stackState.Bookmarks[entry.Bookmark]
		if !ok || bs.PR == 0 {
			// No PR recorded — skip.
			currentBase = entry.Bookmark
			continue
		}

		sp := ui.NewSpinner(fmt.Sprintf("Checking PR #%d", bs.PR))
		sp.Start()
		pr, err := github.GetPR(bs.PR)
		sp.Stop()
		if err != nil {
			return nil, fmt.Errorf("fetching PR #%d for %q: %w", bs.PR, entry.Bookmark, err)
		}

		if pr.State != "MERGED" {
			// Not merged yet — nothing to do for this entry or anything above it.
			currentBase = entry.Bookmark
			continue
		}

		// This PR was merged. If there's a next entry, rebase it onto currentBase.
		if i+1 < len(s) {
			nextBookmark := s[i+1].Bookmark
			if !dryRun {
				sp := ui.NewSpinner(fmt.Sprintf("Rebasing %s onto %s", nextBookmark, currentBase))
				sp.Start()
				rebaseErr := jj.Rebase(nextBookmark, currentBase)
				sp.Stop()
				if rebaseErr != nil {
					return nil, fmt.Errorf("rebasing %q onto %q: %w", nextBookmark, currentBase, rebaseErr)
				}
				sp = ui.NewSpinner(fmt.Sprintf("Pushing %s", nextBookmark))
				sp.Start()
				pushErr := jj.ForcePush(nextBookmark)
				sp.Stop()
				if pushErr != nil {
					return nil, fmt.Errorf("force-pushing %q after rebase: %w", nextBookmark, pushErr)
				}
				nextBS := stackState.Bookmarks[nextBookmark]
				if nextBS.PR > 0 {
					sp = ui.NewSpinner(fmt.Sprintf("Updating base of PR #%d", nextBS.PR))
					sp.Start()
					updateErr := github.UpdatePRBase(nextBS.PR, currentBase)
					sp.Stop()
					if updateErr != nil {
						return nil, fmt.Errorf("updating PR #%d base to %q: %w", nextBS.PR, currentBase, updateErr)
					}
				}
			}
			result.Actions = append(result.Actions, SyncAction{
				MergedBookmark: entry.Bookmark,
				RebasedFrom:    nextBookmark,
				RebasedOnto:    currentBase,
			})
		} else {
			// Top of stack was merged — record it so the caller can report it.
			result.Actions = append(result.Actions, SyncAction{
				MergedBookmark: entry.Bookmark,
			})
		}

		// Mark the merged bookmark for removal from state.
		toRemove = append(toRemove, entry.Bookmark)
		// currentBase stays the same — the next entry is now directly on top of it.
	}

	if !dryRun {
		removing := make(map[string]bool, len(toRemove))
		for _, name := range toRemove {
			delete(stackState.Bookmarks, name)
			removing[name] = true
		}
		// Prune removed bookmarks from Order too.
		filtered := stackState.Order[:0]
		for _, name := range stackState.Order {
			if !removing[name] {
				filtered = append(filtered, name)
			}
		}
		stackState.Order = filtered
	}

	return result, nil
}
