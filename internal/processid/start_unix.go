//go:build unix

// Package processid reads local process identities to detect PID reuse.
package processid

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// host is the set of OS reads Start makes, so tests can stand in for the OS.
type host struct {
	goos     string
	readFile func(path string) ([]byte, error)
	lstart   func(pid int) (string, error)
}

var system = host{goos: runtime.GOOS, readFile: os.ReadFile, lstart: PSLstart}

// PSLstart runs `ps -o lstart= -p <pid>` under LC_ALL=C in the system zone and
// returns its output. Start records this text on every Unix but Linux.
func PSLstart(pid int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	cmd.Env = []string{"LC_ALL=C"}
	out, err := cmd.Output()
	return string(out), err
}

// Start is an opaque start token, or empty when identity cannot be established.
// Linux includes the boot ID because process ticks alone repeat after reboot.
func Start(pid int) string { return system.start(pid) }

func (h host) start(pid int) string {
	if pid <= 0 {
		return ""
	}
	if h.goos == "linux" {
		b, err := h.readFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			return ""
		}
		boot, err := h.readFile(BootIDPath)
		if err != nil {
			return ""
		}
		ticks, ok := ProcStartTicks(string(b))
		if !ok {
			return ""
		}
		return ProcToken(strings.TrimSpace(string(boot)), ticks)
	}
	out, err := h.lstart(pid)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
