package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Open opens the user's preferred editor with the given header as context
// (shown as comment lines). Returns the trimmed content with comment lines
// stripped. Returns an empty string if the user saves nothing.
func Open(header string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	f, err := os.CreateTemp("", "jjstack-*.md")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(f.Name())

	// Structure mirrors a git commit message: first line = title, blank line,
	// then body. Cursor lands at the top.
	fmt.Fprintf(f, "Title\n")
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "Description\n")
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "# ---------------------------------------------------------------\n")
	for _, line := range strings.Split(header, "\n") {
		fmt.Fprintf(f, "# %s\n", line)
	}
	fmt.Fprintf(f, "# ---------------------------------------------------------------\n")
	fmt.Fprintf(f, "# First line: PR title.\n")
	fmt.Fprintf(f, "# After the blank line: PR description.\n")
	fmt.Fprintf(f, "# Lines starting with '#' are ignored. Save and close to continue.\n")
	f.Close()

	cmd := exec.Command(editor, f.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor: %w", err)
	}

	content, err := os.ReadFile(f.Name())
	if err != nil {
		return "", err
	}

	var lines []string
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}
