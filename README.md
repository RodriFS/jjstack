# jjstack

Manage stacked GitHub PRs on top of [Jujutsu (jj)](https://github.com/jj-vcs/jj).

## Dependencies

- [jj](https://github.com/jj-vcs/jj) — must be on your PATH and used as the VCS for your repo
- [gh](https://cli.github.com/) — GitHub CLI, used for all GitHub operations

### Authenticate with GitHub

```sh
gh auth login
```

## Optional: use as `jj stack`

Add the following alias to your jj config (`~/.config/jj/config.toml`):

```toml
[aliases]
stack = ["util", "exec", "--", "jjstack"]
```

Then you can run all commands as `jj stack submit`, `jj stack status`, etc.

## Installation

```sh
go install github.com/rodrifs/jjstack/cmd/jjstack@latest
```

## Usage

### 1. Create your stack

Create a chain of commits in jj, each with a bookmark:

```
@ profile-edit   Add profile editing
○ profile        Add user profile page
○ auth           Add user authentication
◆ main
```

### 2. Submit

Push all bookmarks and open stacked PRs in one command:

```sh
jjstack submit profile-edit
```

Each PR targets the bookmark below it as its base. The PR body is automatically updated with a list of all PRs in the stack.

### 3. Check status

```sh
jjstack status
```

Shows local vs remote commit state and GitHub PR state for all tracked bookmarks.

### 4. Sync after a merge

When a PR at the bottom of the stack is merged (including squash merges):

```sh
jjstack sync
```

jjstack detects the merge via GitHub PR state, rebases the remaining bookmarks onto the new base, and updates the PR bases on GitHub.

### Importing an existing stack

If you already have stacked PRs created outside of jjstack:

```sh
jjstack import profile-edit
```

This looks up open PRs by branch name and writes them to state without creating or pushing anything.
