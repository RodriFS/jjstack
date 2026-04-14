package stack

import (
	"fmt"

	"github.com/rodrifs/jjstack/internal/config"
	"github.com/rodrifs/jjstack/internal/github"
	"github.com/rodrifs/jjstack/internal/jj"
)

// SyncAction describes a single rebase/cleanup action during sync.
type SyncAction struct {
	MergedBookmark string // the bookmark whose PR was merged
	RebasedFrom    string // bookmark that was rebased
	RebasedOnto    string // what it was rebased onto
}

// SyncResult holds the outcome of a sync operation.
type SyncResult struct {
	Actions []SyncAction
}

// Sync detects merged PRs in the stack (bottom-up) and rebases subsequent
// entries onto the new base. Handles both regular and squash merges by checking
// GitHub PR state rather than commit ancestry.
func Sync(s Stack, state *config.State, dryRun bool) (*SyncResult, error) {
	result := &SyncResult{}

	// Fetch latest remote state so jj knows where origin/* bookmarks are.
	if !dryRun {
		if err := jj.Fetch(); err != nil {
			return nil, fmt.Errorf("fetching from origin: %w", err)
		}
	}

	// Walk the stack bottom-up, looking for merged PRs.
	// When we find one, rebase everything above it onto the current base.
	currentBase := state.BaseBranch
	toRemove := []string{}

	for i, entry := range s {
		bs, ok := state.Bookmarks[entry.Bookmark]
		if !ok || bs.PR == 0 {
			// No PR recorded — skip.
			currentBase = entry.Bookmark
			continue
		}

		pr, err := github.GetPR(bs.PR)
		if err != nil {
			return nil, fmt.Errorf("fetching PR #%d for %q: %w", bs.PR, entry.Bookmark, err)
		}

		if pr.State != "MERGED" {
			// Not merged yet — nothing to do for this entry or anything above it.
			currentBase = entry.Bookmark
			continue
		}

		// This PR was merged (squash or regular). Rebase the next entry onto currentBase.
		// jj rebase -b <next> -d <currentBase> also carries all descendants with it.
		if i+1 < len(s) {
			nextBookmark := s[i+1].Bookmark
			action := SyncAction{
				MergedBookmark: entry.Bookmark,
				RebasedFrom:    nextBookmark,
				RebasedOnto:    currentBase,
			}

			if !dryRun {
				if err := jj.Rebase(nextBookmark, currentBase); err != nil {
					return nil, fmt.Errorf("rebasing %q onto %q: %w", nextBookmark, currentBase, err)
				}
				// Force-push the rebased bookmark (and update subsequent ones too).
				if err := jj.ForcePush(nextBookmark); err != nil {
					return nil, fmt.Errorf("force-pushing %q after rebase: %w", nextBookmark, err)
				}
				// Update the next PR's base to currentBase.
				nextBS := state.Bookmarks[nextBookmark]
				if nextBS.PR > 0 {
					if err := github.UpdatePRBase(nextBS.PR, currentBase); err != nil {
						return nil, fmt.Errorf("updating PR #%d base to %q: %w", nextBS.PR, currentBase, err)
					}
				}
			}

			result.Actions = append(result.Actions, action)
		}

		// Mark the merged bookmark for removal from state.
		toRemove = append(toRemove, entry.Bookmark)
		// currentBase stays as the same base (the merged entry's parent) — the next
		// entry is now directly on top of currentBase after the rebase.
	}

	if !dryRun {
		removing := make(map[string]bool, len(toRemove))
		for _, name := range toRemove {
			delete(state.Bookmarks, name)
			removing[name] = true
		}
		// Prune removed bookmarks from StackOrder too.
		filtered := state.StackOrder[:0]
		for _, name := range state.StackOrder {
			if !removing[name] {
				filtered = append(filtered, name)
			}
		}
		state.StackOrder = filtered
	}

	return result, nil
}
