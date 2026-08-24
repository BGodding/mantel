// Package feh provides small helpers shared by callers that shell out to the
// feh image viewer, so error handling isn't duplicated at each call site.
package feh

import (
	"fmt"
	"os/exec"
)

// RunError describes a failed feh invocation for path, including feh's exit
// code when available, while preserving the original error (via %w) for
// callers that unwrap with errors.Is/errors.As.
func RunError(path string, err error) error {
	if exitError, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("feh failed for %s (exit code %d): %w", path, exitError.ExitCode(), err)
	}
	return fmt.Errorf("feh failed for %s: %w", path, err)
}
