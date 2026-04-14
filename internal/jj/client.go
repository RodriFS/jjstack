package jj

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes a jj subcommand and returns stdout.
// stderr is captured and included in the error if the command fails.
func Run(args ...string) (string, error) {
	cmd := exec.Command("jj", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("jj %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}
