package stack

import (
	"fmt"
	"strings"

	"github.com/rodrifs/jjstack/internal/config"
	"github.com/rodrifs/jjstack/internal/jj"
)

// Resolve returns the StackState to operate on.
//
//   - If stackFlag is non-empty: look up by ID prefix; error if not found.
//   - If only one stack is tracked: return it.
//   - If multiple stacks: try to infer from the current jj revision (@).
//   - If inference fails: return an error listing all stacks with --stack options.
func Resolve(state *config.State, stackFlag string) (*config.StackState, error) {
	if stackFlag != "" {
		st := state.FindStackByID(stackFlag)
		if st == nil {
			return nil, fmt.Errorf("no stack with id %q\n%s", stackFlag, listStacks(state))
		}
		return st, nil
	}

	switch len(state.Stacks) {
	case 0:
		return nil, fmt.Errorf("no stacks tracked — run 'jjstack submit <bookmark>' first")
	case 1:
		return state.Stacks[0], nil
	}

	// Multiple stacks: try to infer from the current working copy revision.
	st, err := inferFromCurrentRevision(state)
	if err != nil {
		// Inference query failed; not fatal — fall through to the ambiguous error.
		_ = err
	}
	if st != nil {
		return st, nil
	}

	return nil, fmt.Errorf(
		"multiple stacks tracked — cannot infer which one from the current revision\n"+
			"pass --stack <id> to select one:\n%s",
		listStacks(state),
	)
}

// inferFromCurrentRevision walks ancestors of @ (up to 64 commits) and returns
// the single stack that contains one of those bookmarks. Returns nil if zero or
// more than one stacks match (ambiguous).
func inferFromCurrentRevision(state *config.State) (*config.StackState, error) {
	names, err := jj.AncestorBookmarks(64)
	if err != nil {
		return nil, err
	}
	inAncestry := make(map[string]bool, len(names))
	for _, n := range names {
		inAncestry[n] = true
	}

	var matches []*config.StackState
	for _, st := range state.Stacks {
		for _, b := range st.Order {
			if inAncestry[b] {
				matches = append(matches, st)
				break
			}
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	return nil, nil
}

// listStacks formats a human-readable bullet list of all tracked stacks.
func listStacks(state *config.State) string {
	var sb strings.Builder
	for _, st := range state.Stacks {
		chain := st.Base + " ← " + strings.Join(st.Order, " → ")
		fmt.Fprintf(&sb, "  %s  %s\n", st.ID, chain)
	}
	return sb.String()
}
