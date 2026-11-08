//go:build !linux && !darwin && !windows

package land

import "os/exec"

// Other platforms retain CommandContext cancellation of the direct process.
func configureProcess(cmd *exec.Cmd) {}
func cleanupProcess(cmd *exec.Cmd)   {}
