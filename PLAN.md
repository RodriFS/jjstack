# jjstack (jst) — Project Plan

## Overview

`jst` is a CLI tool that manages stacked GitHub PRs on top of [Jujutsu (jj)](https://docs.jj-vcs.dev/latest/) repositories.

Users create a chain of commits, each with a jj bookmark (equivalent to a git branch), and `jst` turns them into stacked PRs — each PR targeting the previous bookmark as its base.

```
jj log
@  tnosltrr  profile-edit  Add profile editing
○  muozxulo  profile       Add user profile page
○  ytpxqlll  auth          Add user authentication
◆  orzzzyxs  main          more work 1 (#10)
~

jjstack submit profile-edit
# Creates: auth→main, profile→auth, profile-edit→profile

Stacked PRs
- -> https://github.com/org/repo/pull/3  (profile-edit → profile)
-    https://github.com/org/repo/pull/2  (profile → auth)
-    https://github.com/org/repo/pull/1  (auth → main)
```

---

## Design Decisions

- **jj integration**: exec subprocess with custom log templates using `--no-graph -r <revset>`. Revset `base::target ~ base & bookmarks()` finds exactly the stack entries without manual ancestor traversal.
- **Stack detection**: filter commits with local bookmarks between base and target, reverse for bottom-up order.
- **GitHub integration**: `gh` CLI — handles auth, SSO, enterprise. No token management needed.
- **Squash merge support**: detect merges by GitHub PR state (`MERGED`), not commit ancestry — so squash merges work identically to regular merges.
- **State**: `.jjstack/state.json` at repo root, gitignored. Maps bookmark names → PR numbers/URLs.
- **Rebase after merge**: `jj rebase -b <next> -d <base>` — the `-b` flag carries descendants automatically; jj bookmarks auto-follow rewrites.

---

## Directory Structure

```
jjstack/
├── cmd/jst/main.go               # cobra CLI: submit, status, sync
├── internal/
│   ├── jj/
│   │   ├── client.go             # exec wrapper for jj
│   │   ├── log.go                # jj log with template, parse into LogEntry
│   │   └── bookmark.go           # push, fetch, rebase, bookmark info
│   ├── github/
│   │   └── client.go             # gh pr create/view/list/edit wrappers
│   ├── stack/
│   │   ├── stack.go              # Entry and Stack types
│   │   ├── detect.go             # build Stack from jj log + revsets
│   │   ├── submit.go             # create/update PRs for each entry
│   │   └── sync.go               # detect merged PRs, rebase, clean up
│   └── config/
│       └── config.go             # read/write .jjstack/state.json
├── go.mod
└── PLAN.md
```

---

## Commands

### `jjstack submit <bookmark> [--dry-run] [--base <branch>]`

Creates or updates PRs for the full stack from `<bookmark>` down to `<base>` (default: `main`).

- Detects stack using revset `base::bookmark ~ base & bookmarks()`
- Pushes each bookmark to origin (`jj git push --bookmark <name> --allow-new`)
- Creates PRs bottom-up (so each base PR exists before the one above it)
- If a PR already exists for a bookmark, updates its base if it drifted
- Saves PR numbers/URLs to `.jjstack/state.json`
- `--dry-run`: shows current jj log and what would be submitted, no side effects

### `jjstack status`

Shows local vs remote commit state and GitHub PR state for all tracked bookmarks.

```
BOOKMARK      LOCAL       REMOTE          PR      STATE    NOTES
auth          abc123      abc123          #1      MERGED   merged, run 'jst sync'
profile       def456      (not pushed)    #2      OPEN     not pushed (run jst submit)
profile-edit  ghi789      ghi789          #3      OPEN
```

### `jjstack sync [--dry-run] [--target <bookmark>]`

After PRs are merged into the base branch (regular merge or squash merge):

1. Fetches latest remote state (`jj git fetch`)
2. Checks GitHub PR state for each tracked PR (MERGED beats commit ancestry — handles squash)
3. For each merged PR (bottom-up): rebases the next bookmark onto the new base
4. Force-pushes rebased bookmarks, updates their PR bases on GitHub
5. Removes merged bookmarks from state
6. `--dry-run`: shows before state and planned actions, no side effects

---

## State File (`.jjstack/state.json`)

```json
{
  "base_branch": "main",
  "bookmarks": {
    "auth":         { "pr": 1, "pr_url": "https://github.com/org/repo/pull/1" },
    "profile":      { "pr": 2, "pr_url": "https://github.com/org/repo/pull/2" },
    "profile-edit": { "pr": 3, "pr_url": "https://github.com/org/repo/pull/3" }
  }
}
```

Local only — the `.jjstack/` directory contains a `*` gitignore so it is never committed.

---

## Milestones

- [x] **Phase 1 — Project scaffold**
  - [x] `go mod init github.com/rodrifs/jjstack`
  - [x] Add cobra, set up `cmd/jst/main.go` with root command
  - [x] Wire `submit`, `status`, `sync` subcommands

- [x] **Phase 2 — jj integration**
  - [x] `internal/jj/client.go`: generic exec runner with stderr capture
  - [x] `internal/jj/log.go`: run `jj log` with template, parse into `[]LogEntry`
  - [x] `internal/jj/bookmark.go`: push, fetch, rebase, compare local vs remote

- [x] **Phase 3 — Config & state**
  - [x] `internal/config/config.go`: read/write `.jjstack/state.json`
  - [x] Auto-detect repo root via `jj workspace root`

- [x] **Phase 4 — GitHub integration**
  - [x] `internal/github/client.go`: `gh pr create/view/list/edit` wrappers
  - [x] Auto-detect repo via `gh repo view --json nameWithOwner`

- [x] **Phase 5 — `jst submit`**
  - [x] Push all bookmarks in the stack
  - [x] Create or update PRs with correct bases
  - [x] Save state, print PR list

- [x] **Phase 6 — `jst status`**
  - [x] Compare local vs remote commit IDs
  - [x] Query PR states from GitHub
  - [x] Print status table

- [x] **Phase 7 — `jst sync`**
  - [x] Detect merged PRs via GitHub state (handles squash merges)
  - [x] Rebase subsequent stack entries with `jj rebase -b`
  - [x] Update PR bases, clean up state

- [x] **Phase 8 — `--dry-run`**
  - [x] `submit --dry-run`: show current log + planned actions
  - [x] `sync --dry-run`: show before log + planned rebase actions

- [x] **Phase 9 — Testing**
  - [x] Test with https://github.com/RodriFS/jjstacktest
  - [x] Verify submit creates correct stacked PRs
  - [x] Verify status output is accurate
  - [x] Verify sync works after squash merge
  - [x] Verify sync works after regular merge

- [x] **Phase 10 — Polish**
  - [x] Ordered status output (bookmarks in stack order, not map order)
  - [x] Stack order preserved in state file
  - [x] Better error messages for common failures (not in jj repo, gh not authenticated)
