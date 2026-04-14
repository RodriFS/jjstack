package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rodrifs/jjstack/internal/config"
	"github.com/rodrifs/jjstack/internal/editor"
	gh "github.com/rodrifs/jjstack/internal/github"
	"github.com/rodrifs/jjstack/internal/jj"
	"github.com/rodrifs/jjstack/internal/stack"
	"github.com/rodrifs/jjstack/internal/ui"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// checkDeps verifies that jj is on PATH and we're inside a jj repository.
func checkDeps() error {
	if _, err := jj.Run("root"); err != nil {
		if strings.Contains(err.Error(), "There is no jj repo") ||
			strings.Contains(err.Error(), "jj root") {
			return fmt.Errorf("not inside a jj repository (no .jj/ found in parent directories)")
		}
		return fmt.Errorf("jj not found on PATH — install it from https://github.com/jj-vcs/jj")
	}
	return nil
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "jjstack",
		Short: "jjstack — manage stacked PRs with Jujutsu",
		Long: `jjstack manages stacked GitHub PRs on top of Jujutsu (jj) repositories.

Create a chain of bookmarked commits, then use jjstack to turn them into
stacked PRs, track their status, and clean up after merging.`,
	}
	root.AddCommand(submitCmd(), statusCmd(), syncCmd(), importCmd(), untrackCmd())
	return root
}

// ---------------------------------------------------------------------------
// submit
// ---------------------------------------------------------------------------

func submitCmd() *cobra.Command {
	var dryRun bool
	var base string
	var stackID string
	var noEdit bool

	cmd := &cobra.Command{
		Use:   "submit <bookmark>",
		Short: "Create or update stacked PRs for a bookmark and everything below it",
		Args:  cobra.ExactArgs(1),
		Example: `  jjstack submit profile-edit          # submits auth→main, profile→auth, profile-edit→profile
  jjstack submit profile-edit --dry-run # preview without pushing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkDeps(); err != nil {
				return err
			}
			target := args[0]
			state, err := config.Load()
			if err != nil {
				return err
			}

			effectiveBase, err := resolveBase(base)
			if err != nil {
				return err
			}

			if dryRun {
				return runSubmitDryRun(target, effectiveBase, state, stackID)
			}

			s, err := stack.Detect(target, effectiveBase)
			if err != nil {
				return err
			}

			// Find the existing stack that tracks any of these bookmarks, or create one.
			var stackState *config.StackState
			if stackID != "" {
				stackState = state.FindStackByID(stackID)
				if stackState == nil {
					return fmt.Errorf("no stack with id %q", stackID)
				}
			} else {
				for _, entry := range s {
					if st := state.FindStackByBookmark(entry.Bookmark); st != nil {
						stackState = st
						break
					}
				}
				if stackState == nil {
					stackState = state.NewStack(effectiveBase)
				}
			}

			// Open an editor for each bookmark that doesn't have a PR yet,
			// so the user can write a description before the PR is created.
			if !noEdit {
				for i := range s {
					if _, exists := stackState.Bookmarks[s[i].Bookmark]; exists {
						continue // existing PR — skip
					}
					header := fmt.Sprintf("PR: %s → %s\nTitle: %s", s[i].Bookmark, s[i].ParentBookmark, s[i].Description)
					content, err := editor.Open(header)
					if err != nil {
						return fmt.Errorf("editor for %q: %w", s[i].Bookmark, err)
					}
					// First line is the title; everything after the first blank line is the body.
					parts := strings.SplitN(content, "\n", 2)
					s[i].UserTitle = strings.TrimSpace(parts[0])
					if len(parts) > 1 {
						s[i].UserBody = strings.TrimSpace(parts[1])
					}
				}
			}

			result, err := stack.Submit(s, stackState, false)
			if err != nil {
				return err
			}

			if err := config.Save(state); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}

			printStack(result.Stack, target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without pushing or creating PRs")
	cmd.Flags().StringVar(&base, "base", "", "Base branch (default: main)")
	cmd.Flags().StringVar(&stackID, "stack", "", "Stack ID to operate on")
	cmd.Flags().BoolVar(&noEdit, "no-edit", false, "Skip editor and create PRs with no description")
	return cmd
}

func runSubmitDryRun(target, flagBase string, state *config.State, stackID string) error {
	base, err := resolveBase(flagBase)
	if err != nil {
		return err
	}

	s, err := stack.Detect(target, base)
	if err != nil {
		return err
	}

	// Find existing stack for PR numbers (best-effort; no creation in dry-run).
	var stackState *config.StackState
	if stackID != "" {
		stackState = state.FindStackByID(stackID)
	} else {
		for _, entry := range s {
			if st := state.FindStackByBookmark(entry.Bookmark); st != nil {
				stackState = st
				break
			}
		}
	}

	actions := make([]ui.SubmitAction, len(s))
	for i, e := range s {
		prNum := 0
		if stackState != nil {
			if bs, ok := stackState.Bookmarks[e.Bookmark]; ok {
				prNum = bs.PR
			}
		}
		actions[i] = ui.SubmitAction{
			Bookmark: e.Bookmark,
			Parent:   e.ParentBookmark,
			PRNum:    prNum,
		}
	}

	logOut, _ := jj.LogFromBase(base, target)
	ui.DryRunSubmit(logOut, actions)
	return nil
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

func statusCmd() *cobra.Command {
	var stackID string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the status of all tracked stacked PRs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkDeps(); err != nil {
				return err
			}
			state, err := config.Load()
			if err != nil {
				return err
			}
			if len(state.Stacks) == 0 {
				fmt.Println("No stacked PRs tracked. Run 'jjstack submit <bookmark>' to get started.")
				return nil
			}

			// Determine which stacks to display.
			var stacks []*config.StackState
			if stackID != "" {
				st := state.FindStackByID(stackID)
				if st == nil {
					return fmt.Errorf("no stack with id %q", stackID)
				}
				stacks = []*config.StackState{st}
			} else {
				stacks = state.Stacks
			}

			for _, st := range stacks {
				if err := printStackStatus(st); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&stackID, "stack", "", "Stack ID to show (default: all stacks)")
	return cmd
}

func printStackStatus(st *config.StackState) error {
	// Use Order for deterministic top-down display; fall back to map keys.
	names := st.Order
	if len(names) == 0 {
		names = bookmarkNames(st)
	}
	// Reverse bottom-up order to top-down for display.
	names = reversed(names)

	infos, err := jj.ListBookmarks(names)
	if err != nil {
		return err
	}

	fmt.Println()
	ui.Header(fmt.Sprintf("Stack %s  (base: %s)", st.ID, st.Base))
	fmt.Println()

	var rows []ui.StatusRow
	for _, info := range infos {
		bs := st.Bookmarks[info.Name]
		row := ui.StatusRow{
			Bookmark: info.Name,
			Local:    info.LocalCommit,
			Remote:   info.RemoteCommit,
			PRNum:    bs.PR,
		}
		if bs.PR > 0 {
			pr, ghErr := gh.GetPR(bs.PR)
			if ghErr != nil {
				row.State = "error"
				row.Notes = ghErr.Error()
			} else {
				row.State = pr.State
				switch pr.State {
				case "MERGED":
					row.Notes = "run 'jjstack sync'"
				case "OPEN":
					if info.LocalCommit != info.RemoteCommit {
						row.Notes = "not pushed (run jjstack submit)"
					}
				case "CLOSED":
					row.Notes = "closed without merging"
				}
			}
		}
		rows = append(rows, row)
	}

	ui.StatusTable(rows)
	return nil
}

// ---------------------------------------------------------------------------
// sync
// ---------------------------------------------------------------------------

func syncCmd() *cobra.Command {
	var dryRun bool
	var base string
	var target string
	var stackID string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Rebase stack after merged PRs, cleaning up merged bookmarks",
		Long: `sync detects which PRs in the stack have been merged (including squash merges)
by checking their GitHub state, then rebases subsequent entries onto the new base.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkDeps(); err != nil {
				return err
			}
			state, err := config.Load()
			if err != nil {
				return err
			}
			if len(state.Stacks) == 0 {
				fmt.Println("No stacked PRs tracked.")
				return nil
			}

			stackState, err := stack.Resolve(state, stackID)
			if err != nil {
				return err
			}

			effectiveBase := base
			if effectiveBase == "" {
				effectiveBase = stackState.Base
			}

			effectiveTarget := target
			if effectiveTarget == "" {
				if len(stackState.Order) == 0 {
					fmt.Println("No stack order in state. Run 'jjstack submit' first.")
					return nil
				}
				effectiveTarget = stackState.Order[len(stackState.Order)-1]
			}

			s, err := stack.Detect(effectiveTarget, effectiveBase)
			if err != nil {
				return err
			}

			if dryRun {
				beforeLog, _ := jj.LogFromBase(effectiveBase, effectiveTarget)
				result, err := stack.Sync(s, stackState, true)
				if err != nil {
					return err
				}
				uiActions := toUISyncActions(result.Actions)
				ui.DryRunSync(beforeLog, uiActions)
				return nil
			}

			result, err := stack.Sync(s, stackState, false)
			if err != nil {
				return err
			}

			if len(result.Actions) == 0 {
				fmt.Println("Nothing to sync — no merged PRs found.")
				return nil
			}

			// Remove the stack from state entirely if all bookmarks were merged.
			if len(stackState.Bookmarks) == 0 {
				state.RemoveStack(stackState.ID)
			}

			if err := config.Save(state); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}

			afterLog, _ := jj.LogFromBase(effectiveBase, effectiveTarget)
			ui.SyncResult(afterLog, toUISyncActions(result.Actions))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without modifying anything")
	cmd.Flags().StringVar(&base, "base", "", "Base branch (default: value from stack state)")
	cmd.Flags().StringVar(&target, "target", "", "Top bookmark of the stack (default: last in stack order)")
	cmd.Flags().StringVar(&stackID, "stack", "", "Stack ID to sync")
	return cmd
}

// ---------------------------------------------------------------------------
// import
// ---------------------------------------------------------------------------

func importCmd() *cobra.Command {
	var base string

	cmd := &cobra.Command{
		Use:   "import <bookmark>",
		Short: "Import a pre-existing stack of PRs into jjstack state",
		Long: `import detects the stack from <bookmark> down to base and looks up open GitHub
PRs for each bookmark. No pushing or PR creation is done — only state is written.

Useful when a stack was created manually or with another tool.`,
		Args:    cobra.ExactArgs(1),
		Example: `  jjstack import profile-edit           # import existing PRs for the stack
  jjstack import profile-edit --base dev # use a non-default base branch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkDeps(); err != nil {
				return err
			}
			target := args[0]
			state, err := config.Load()
			if err != nil {
				return err
			}

			effectiveBase, err := resolveBase(base)
			if err != nil {
				return err
			}

			s, err := stack.DetectForImport(target, effectiveBase)
			if err != nil {
				return err
			}

			// Check if any bookmark is already tracked to avoid duplicates.
			for _, entry := range s {
				if st := state.FindStackByBookmark(entry.Bookmark); st != nil {
					return fmt.Errorf("bookmark %q is already tracked in stack %s — use 'jjstack submit' to update it", entry.Bookmark, st.ID)
				}
			}

			stackState := state.NewStack(effectiveBase)

			fmt.Println()
			ui.Header(fmt.Sprintf("Importing stack (id: %s)", stackState.ID))
			fmt.Println()

			anyFound := false
			for _, entry := range s {
				pr, err := gh.FindPRForHead(entry.Bookmark)
				if err != nil {
					return fmt.Errorf("looking up PR for %q: %w", entry.Bookmark, err)
				}
				stackState.Order = append(stackState.Order, entry.Bookmark)
				if pr != nil {
					stackState.Bookmarks[entry.Bookmark] = config.BookmarkState{PR: pr.Number, PRURL: pr.URL}
					fmt.Printf("  %-22s  #%-6d  %s\n", entry.Bookmark, pr.Number, pr.URL)
					anyFound = true
				} else {
					fmt.Printf("  %-22s  (no open PR found — run 'jjstack submit %s' to create one)\n", entry.Bookmark, strings.SplitN(target, "@", 2)[0])
				}
			}
			fmt.Println()

			if !anyFound {
				// Don't save a useless empty stack.
				state.RemoveStack(stackState.ID)
				fmt.Println("No open PRs found for any bookmark in the stack. Nothing imported.")
				return nil
			}

			if err := config.Save(state); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "Base branch (default: main)")
	return cmd
}

// ---------------------------------------------------------------------------
// untrack
// ---------------------------------------------------------------------------

func untrackCmd() *cobra.Command {
	var stackID string

	cmd := &cobra.Command{
		Use:   "untrack",
		Short: "Stop tracking a stack without closing its PRs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkDeps(); err != nil {
				return err
			}
			state, err := config.Load()
			if err != nil {
				return err
			}

			stackState, err := stack.Resolve(state, stackID)
			if err != nil {
				return err
			}

			state.RemoveStack(stackState.ID)

			if err := config.Save(state); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}

			fmt.Printf("Stack %s untracked.\n", stackState.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&stackID, "stack", "", "Stack ID to untrack")
	return cmd
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func printStack(s stack.Stack, current string) {
	entries := make([]ui.PREntry, 0, len(s))
	for i := len(s) - 1; i >= 0; i-- {
		e := s[i]
		num := 0
		url := "(no PR)"
		if e.PR != nil {
			num = e.PR.Number
			url = e.PR.URL
		}
		entries = append(entries, ui.PREntry{
			Number:  num,
			URL:     url,
			Current: e.Bookmark == current,
		})
	}
	ui.PRList(entries)
}

func bookmarkNames(st *config.StackState) []string {
	names := make([]string, 0, len(st.Bookmarks))
	for name := range st.Bookmarks {
		names = append(names, name)
	}
	return names
}

// resolveBase returns the effective base branch: the flag value if set,
// otherwise the GitHub repo's default branch.
func resolveBase(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	base, err := gh.DefaultBranch()
	if err != nil {
		return "", fmt.Errorf("detecting default branch: %w", err)
	}
	return base, nil
}

func reversed(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func toUISyncActions(actions []stack.SyncAction) []ui.SyncAction {
	out := make([]ui.SyncAction, len(actions))
	for i, a := range actions {
		out[i] = ui.SyncAction{
			MergedBookmark: a.MergedBookmark,
			RebasedFrom:    a.RebasedFrom,
			RebasedOnto:    a.RebasedOnto,
		}
	}
	return out
}
