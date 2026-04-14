package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rodrifs/jjstack/internal/config"
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
	root.AddCommand(submitCmd(), statusCmd(), syncCmd())
	return root
}

// ---------------------------------------------------------------------------
// submit
// ---------------------------------------------------------------------------

func submitCmd() *cobra.Command {
	var dryRun bool
	var base string

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
			if base == "" {
				base = state.BaseBranch
			}

			if dryRun {
				return runSubmitDryRun(target, base, state)
			}

			s, err := stack.Detect(target, base)
			if err != nil {
				return err
			}

			result, err := stack.Submit(s, state, false)
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
	cmd.Flags().StringVar(&base, "base", "", "Base branch (default: value from state, or 'main')")
	return cmd
}

func runSubmitDryRun(target, base string, state *config.State) error {
	logOut, err := jj.LogRaw(fmt.Sprintf("%s::%s", base, target))
	if err != nil {
		return err
	}

	s, err := stack.Detect(target, base)
	if err != nil {
		return err
	}

	actions := make([]ui.SubmitAction, len(s))
	for i, e := range s {
		prNum := 0
		if bs, ok := state.Bookmarks[e.Bookmark]; ok {
			prNum = bs.PR
		}
		actions[i] = ui.SubmitAction{
			Bookmark: e.Bookmark,
			Parent:   e.ParentBookmark,
			PRNum:    prNum,
		}
	}

	ui.DryRunSubmit(logOut, actions)
	return nil
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

func statusCmd() *cobra.Command {
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
			if len(state.Bookmarks) == 0 {
				fmt.Println("No stacked PRs tracked. Run 'jjstack submit <bookmark>' to get started.")
				return nil
			}

			// Use StackOrder for deterministic top-down display; fall back to map keys.
			names := state.StackOrder
			if len(names) == 0 {
				names = bookmarkNames(state)
			}
			// Reverse bottom-up order to top-down for display.
			for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
				names[i], names[j] = names[j], names[i]
			}

			infos, err := jj.ListBookmarks(names)
			if err != nil {
				return err
			}

			var rows []ui.StatusRow
			for _, info := range infos {
				bs := state.Bookmarks[info.Name]
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
		},
	}
	return cmd
}

// ---------------------------------------------------------------------------
// sync
// ---------------------------------------------------------------------------

func syncCmd() *cobra.Command {
	var dryRun bool
	var base string
	var target string

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
			if base == "" {
				base = state.BaseBranch
			}
			if len(state.Bookmarks) == 0 {
				fmt.Println("No stacked PRs tracked.")
				return nil
			}

			if target == "" {
				order := state.StackOrder
				if len(order) == 0 {
					fmt.Println("No stack order in state. Run 'jjstack submit' first.")
					return nil
				}
				target = order[len(order)-1]
			}

			s, err := stack.Detect(target, base)
			if err != nil {
				return err
			}

			if dryRun {
				beforeLog, _ := jj.LogRaw(fmt.Sprintf("%s::%s", base, target))
				result, err := stack.Sync(s, state, true)
				if err != nil {
					return err
				}
				uiActions := toUISyncActions(result.Actions)
				ui.DryRunSync(beforeLog, uiActions)
				return nil
			}

			result, err := stack.Sync(s, state, false)
			if err != nil {
				return err
			}

			if len(result.Actions) == 0 {
				fmt.Println("Nothing to sync — no merged PRs found.")
				return nil
			}

			if err := config.Save(state); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}

			afterLog, _ := jj.LogRaw(fmt.Sprintf("%s::%s", base, target))
			ui.SyncResult(afterLog, toUISyncActions(result.Actions))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without modifying anything")
	cmd.Flags().StringVar(&base, "base", "", "Base branch (default: value from state, or 'main')")
	cmd.Flags().StringVar(&target, "target", "", "Top bookmark of the stack (default: last in state)")
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

func bookmarkNames(state *config.State) []string {
	names := make([]string, 0, len(state.Bookmarks))
	for name := range state.Bookmarks {
		names = append(names, name)
	}
	return names
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
