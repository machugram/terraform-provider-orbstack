package orb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNotFound is returned when orbctl cannot find a machine.
var ErrNotFound = errors.New("machine not found")

// Client runs orbctl.
type Client struct {
	Path string
}

// Run executes orbctl with args and returns stdout and stderr.
func (c *Client) Run(ctx context.Context, args ...string) (string, string, error) {
	path := c.Path
	if path == "" {
		path = "orbctl"
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// IsNotFound reports whether orbctl output says the machine does not exist.
func IsNotFound(stdout, stderr string) bool {
	msg := strings.ToLower(stdout + stderr)
	return strings.Contains(msg, "machine not found")
}

func commandErr(err error, stdout, stderr string) error {
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = strings.TrimSpace(stdout)
	}
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, detail)
}
