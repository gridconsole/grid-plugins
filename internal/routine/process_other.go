//go:build !linux && !darwin

package routine

import "os/exec"

func configureProcess(*exec.Cmd) {}
func cleanupProcess(*exec.Cmd)   {}
