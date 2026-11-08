//go:build linux || darwin

package session

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ProbeLocal proves absence or compares process start identity on this host.
// Callers must first establish that the holder names this machine. Permission,
// command and parse failures remain Unknown; a pid alone never proves life.
func ProbeLocal(live Live) ProcessState {
	if live.PID < 1 || live.PIDStart == "" {
		return Unknown
	}
	err := syscall.Kill(live.PID, 0)
	if errors.Is(err, syscall.ESRCH) {
		return Dead
	}
	if err != nil {
		return Unknown
	}
	if strings.HasPrefix(live.PIDStart, "proc:") {
		b, err := os.ReadFile("/proc/" + strconv.Itoa(live.PID) + "/stat")
		if err != nil {
			return Unknown
		}
		end := strings.LastIndexByte(string(b), ')')
		if end < 0 {
			return Unknown
		}
		fields := strings.Fields(string(b[end+1:]))
		if len(fields) < 20 {
			return Unknown
		}
		if "proc:"+fields[19] != live.PIDStart {
			return Dead
		}
		return Alive
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-o", "lstart=", "-p", strconv.Itoa(live.PID))
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
	b, err := cmd.Output()
	if err != nil {
		return Unknown
	}
	observed := strings.TrimSpace(string(b))
	if observed == "" {
		return Unknown
	}
	if observed == live.PIDStart {
		return Alive
	}
	want, err := time.Parse(time.RFC3339Nano, live.PIDStart)
	if err != nil {
		return Unknown
	}
	got, err := time.Parse("Mon Jan 2 15:04:05 2006", observed)
	if err != nil {
		return Unknown
	}
	if got.Equal(want.UTC().Truncate(time.Second)) {
		return Alive
	}
	return Dead
}
