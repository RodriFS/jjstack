package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Styles
var (
	bold      = color.New(color.Bold)
	dim       = color.New(color.Faint)
	green     = color.New(color.FgGreen)
	yellow    = color.New(color.FgYellow)
	red       = color.New(color.FgRed)
	cyan      = color.New(color.FgCyan)
	boldCyan  = color.New(color.FgCyan, color.Bold)
	boldGreen = color.New(color.FgGreen, color.Bold)
)

// Header prints a bold section header with a unicode underline.
func Header(title string) {
	bold.Println(title)
	dim.Println(strings.Repeat("─", len(title)+2))
}

// PRList prints the stacked PR list with the current entry highlighted.
func PRList(entries []PREntry) {
	fmt.Println()
	bold.Println("Stacked PRs")
	dim.Println(strings.Repeat("─", 50))
	for _, e := range entries {
		if e.Current {
			boldCyan.Printf("  →  ")
			fmt.Printf("#%-4d  %s\n", e.Number, e.URL)
		} else {
			dim.Printf("     ")
			fmt.Printf("#%-4d  %s\n", e.Number, e.URL)
		}
	}
}

// PREntry is a single row in the PR list.
type PREntry struct {
	Number  int
	URL     string
	Current bool
}

// StatusTable prints the status table with colored state indicators.
func StatusTable(rows []StatusRow) {
	fmt.Println()
	bold.Printf("  %-22s  %-10s  %-10s  %-6s  %s\n", "BOOKMARK", "LOCAL", "REMOTE", "PR", "STATE")
	dim.Println("  " + strings.Repeat("─", 66))
	for _, r := range rows {
		stateStr := colorState(r.State)
		notes := ""
		if r.Notes != "" {
			notes = "  " + dim.Sprint(r.Notes)
		}
		fmt.Printf("  %-22s  %-10s  %-10s  %-6s  %s%s\n",
			r.Bookmark, r.Local[:min(8, len(r.Local))], remoteDisplay(r.Remote), prDisplay(r.PRNum), stateStr, notes)
	}
	fmt.Println()
}

// StatusRow is one bookmark row in the status table.
type StatusRow struct {
	Bookmark string
	Local    string
	Remote   string
	PRNum    int
	State    string
	Notes    string
}

// DryRunSubmit prints the dry-run output for submit.
func DryRunSubmit(logOutput string, actions []SubmitAction) {
	fmt.Println()
	Header("Current stack")
	fmt.Print(logOutput)
	fmt.Println()
	Header("Would submit")
	for _, a := range actions {
		action := dim.Sprint("create PR")
		if a.PRNum > 0 {
			action = dim.Sprintf("update PR #%d", a.PRNum)
		}
		fmt.Printf("  %s  %s  %s  %s\n",
			cyan.Sprintf("%-20s", a.Bookmark),
			dim.Sprint("→"),
			cyan.Sprintf("%-20s", a.Parent),
			action,
		)
	}
	fmt.Println()
}

// SubmitAction is one planned action in a submit dry-run.
type SubmitAction struct {
	Bookmark string
	Parent   string
	PRNum    int
}

// DryRunSync prints the dry-run output for sync.
func DryRunSync(logOutput string, actions []SyncAction) {
	fmt.Println()
	Header("Current stack")
	fmt.Print(logOutput)
	fmt.Println()
	if len(actions) == 0 {
		dim.Println("  Nothing to sync — no merged PRs found.")
		fmt.Println()
		return
	}
	Header("Would sync")
	for _, a := range actions {
		if a.RebasedFrom == "" {
			fmt.Printf("  %s  %s  stack complete\n",
				yellow.Sprintf("%-20s", a.MergedBookmark),
				dim.Sprint("merged →"),
			)
		} else {
			fmt.Printf("  %s  %s  rebase %s onto %s\n",
				yellow.Sprintf("%-20s", a.MergedBookmark),
				dim.Sprint("merged →"),
				cyan.Sprint(a.RebasedFrom),
				boldGreen.Sprint(a.RebasedOnto),
			)
		}
	}
	fmt.Println()
}

// SyncAction is one planned action in a sync dry-run.
type SyncAction struct {
	MergedBookmark string
	RebasedFrom    string
	RebasedOnto    string
}

// SyncResult prints the result of a completed sync.
func SyncResult(logOutput string, actions []SyncAction) {
	fmt.Println()
	Header("Sync complete")
	for _, a := range actions {
		if a.RebasedFrom == "" {
			fmt.Printf("  %s  %s merged → stack complete\n",
				green.Sprint("✓"),
				yellow.Sprint(a.MergedBookmark),
			)
		} else {
			fmt.Printf("  %s  %s merged → rebased %s onto %s\n",
				green.Sprint("✓"),
				yellow.Sprint(a.MergedBookmark),
				cyan.Sprint(a.RebasedFrom),
				boldGreen.Sprint(a.RebasedOnto),
			)
		}
	}
	if logOutput != "" {
		fmt.Println()
		Header("Updated stack")
		fmt.Print(logOutput)
	}
	fmt.Println()
}

func colorState(state string) string {
	switch state {
	case "OPEN":
		return green.Sprint("● OPEN")
	case "MERGED":
		return dim.Sprint("✓ MERGED")
	case "CLOSED":
		return red.Sprint("✗ CLOSED")
	case "":
		return dim.Sprint("─")
	default:
		return state
	}
}

func remoteDisplay(remote string) string {
	if remote == "(not pushed)" || remote == "(unknown)" {
		return dim.Sprint("not pushed")
	}
	if len(remote) > 8 {
		return remote[:8]
	}
	return remote
}

func prDisplay(n int) string {
	if n == 0 {
		return dim.Sprint("─")
	}
	return fmt.Sprintf("#%d", n)
}

