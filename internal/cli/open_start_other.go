//go:build !unix && !windows

package cli

import "os/exec"

// detach does nothing on a platform with no detach call; the child is still started
// without a terminal and may end with grid open's session.
func detach(*exec.Cmd) {}
