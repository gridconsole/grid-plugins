//go:build !linux && !darwin

package eval

import "os/exec"

// Other platforms retain CommandContext cancellation of the direct process.
func configureProcess(cmd *exec.Cmd) {}
func cleanupProcess(cmd *exec.Cmd)   {}
