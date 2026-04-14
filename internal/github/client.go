package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PR represents a GitHub pull request.
type PR struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	State       string `json:"state"`  // "OPEN", "MERGED", "CLOSED"
	BaseRefName string `json:"baseRefName"`
	Title       string `json:"title"`
}

// run executes a gh subcommand and returns stdout.
func run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

// RepoName returns the "owner/repo" string for the current repo.
func RepoName() (string, error) {
	out, err := run("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CreatePR creates a new pull request and returns its number and URL.
// gh pr create prints the PR URL to stdout on success.
func CreatePR(title, body, base, head string) (int, string, error) {
	out, err := run("pr", "create",
		"--title", title,
		"--body", body,
		"--base", base,
		"--head", head,
	)
	if err != nil {
		return 0, "", err
	}
	url := strings.TrimSpace(out)
	// URL format: https://github.com/owner/repo/pull/123
	// Parse number from the last path segment.
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return 0, url, fmt.Errorf("unexpected pr create output: %q", out)
	}
	number, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0, url, fmt.Errorf("parsing PR number from URL %q: %w", url, err)
	}
	return number, url, nil
}

// GetPR fetches a PR by number.
func GetPR(number int) (*PR, error) {
	out, err := run("pr", "view", fmt.Sprintf("%d", number),
		"--json", "number,url,state,baseRefName,title",
	)
	if err != nil {
		return nil, err
	}
	var pr PR
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return nil, fmt.Errorf("parsing pr view output: %w", err)
	}
	return &pr, nil
}

// FindPRForHead returns the open PR for a given head branch, or nil if none.
func FindPRForHead(head string) (*PR, error) {
	out, err := run("pr", "list",
		"--head", head,
		"--state", "open",
		"--json", "number,url,state,baseRefName,title",
	)
	if err != nil {
		return nil, err
	}
	var prs []PR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("parsing pr list output: %w", err)
	}
	if len(prs) == 0 {
		return nil, nil
	}
	return &prs[0], nil
}

// UpdatePRBase changes the base branch of an existing PR.
func UpdatePRBase(number int, newBase string) error {
	_, err := run("pr", "edit", fmt.Sprintf("%d", number), "--base", newBase)
	return err
}

// GetPRBody returns the current body of a PR.
func GetPRBody(number int) (string, error) {
	out, err := run("pr", "view", fmt.Sprintf("%d", number), "--json", "body", "--jq", ".body")
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

// UpdatePRBody updates the body/description of an existing PR.
func UpdatePRBody(number int, body string) error {
	_, err := run("pr", "edit", fmt.Sprintf("%d", number), "--body", body)
	return err
}

// DeleteBranch deletes a remote branch via the GitHub API.
func DeleteBranch(branch string) error {
	repo, err := RepoName()
	if err != nil {
		return err
	}
	_, err = run("api", "--method", "DELETE",
		fmt.Sprintf("repos/%s/git/refs/heads/%s", repo, branch),
	)
	return err
}
