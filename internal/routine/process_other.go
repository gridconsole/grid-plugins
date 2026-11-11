//go:build !linux && !darwin && !windows

package routine

import "os/exec"

// Remaining platforms retain CommandContext cancellation of the direct process.
func configureProcess(*exec.Cmd) {}
func cleanupProcess(*exec.Cmd)   {}
