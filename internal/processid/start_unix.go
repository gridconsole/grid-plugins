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

// Start is an opaque start token, or empty when identity cannot be established.
// Linux includes the boot ID because process ticks alone repeat after reboot.
func Start(pid int) string {
	if pid <= 0 {
		return ""
	}
	if runtime.GOOS == "linux" {
		b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			return ""
		}
		boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
		if err != nil {
			return ""
		}
		s := string(b)
		if i := strings.LastIndexByte(s, ')'); i >= 0 {
			if f := strings.Fields(s[i+1:]); len(f) > 19 {
				return "proc:" + strings.TrimSpace(string(boot)) + ":" + f[19]
			}
		}
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	cmd.Env = []string{"LC_ALL=C"}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
