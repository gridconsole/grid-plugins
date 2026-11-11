//go:build !linux && !darwin && !windows

package eval

import "os/exec"

// Remaining platforms retain CommandContext cancellation of the direct process.
func configureProcess(cmd *exec.Cmd) {}
func cleanupProcess(cmd *exec.Cmd)   {}
