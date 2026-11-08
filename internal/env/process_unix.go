//go:build linux || darwin

package env

import (
	"errors"
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
func stop(pid int, grace time.Duration) error {
	if pid <= 0 {
		return errors.New("invalid dev server process group")
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL} {
		err := syscall.Kill(-pid, sig)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return err
		}
		until := time.Now().Add(grace)
		for time.Now().Before(until) {
			if !ports.Alive(pid, true) {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	// SIGKILL has been delivered to the whole group. Orphaned zombies can remain
	// visible to kill(0) until their parent reaps them, but cannot keep serving.
	return nil
}
