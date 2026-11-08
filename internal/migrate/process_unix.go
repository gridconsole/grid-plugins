//go:build unix

package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// pidStart names when the process started, so a reused pid is told from the original.
// It is "" when the system cannot say.
func pidStart(pid int) string {
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		s := string(b)
		if i := strings.LastIndexByte(s, ')'); i >= 0 {
			if f := strings.Fields(s[i+1:]); len(f) > 19 {
				return "proc:" + f[19] // field 22, in clock ticks since boot
			}
		}
	}
	cmd := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
