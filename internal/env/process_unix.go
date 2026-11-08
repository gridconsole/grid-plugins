//go:build linux || darwin

package env

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/gridconsole/grid-plugins/internal/ports"
)

func supported() error { return nil }
func configure(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}
func stop(pid int, grace time.Duration) error { return ports.Stop(pid, grace) }
