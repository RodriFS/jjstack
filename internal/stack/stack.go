package stack

import "github.com/rodrifs/jjstack/internal/github"

// Entry represents one commit in a stack, tied to a bookmark.
type Entry struct {
	Bookmark       string
	ChangeID       string
	CommitID       string
	Description    string
	ParentBookmark string // bookmark below this one, or base branch for the bottom entry
	PR             *github.PR
	UserTitle      string // user-written PR title, set before Submit is called (overrides Description)
	UserBody       string // user-written PR body, set before Submit is called
}

// Stack is an ordered slice of entries, bottom-up (index 0 = closest to base).
type Stack []Entry
