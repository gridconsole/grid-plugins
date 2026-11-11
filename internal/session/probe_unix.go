//go:build linux || darwin

package session

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// ProcessStart reads the process start time in UTC at ps's second precision.
// Linux reads it from /proc and forks no ps; darwin asks ps.
func ProcessStart(pid int) (time.Time, error) {
	if pid < 1 {
		return time.Time{}, fmt.Errorf("pid must be positive")
	}
	return procStart(pid)
}

// probeHost is the set of OS reads ProbeLocal makes, so tests can stand in for
// the OS with no real process.
type probeHost struct {
	goos        string
	kill        func(pid int) error           // signal 0
	procStat    func(pid int) (string, error) // /proc/<pid>/stat, Linux only
	bootID      func() (string, error)        // Linux boot ID, Linux only
	lstartUTC   func(pid int) (string, error) // ps with TZ=UTC
	lstartLocal func(pid int) (string, error) // ps as processid.Start runs it
}

var localHost = probeHost{
	goos: runtime.GOOS,
	kill: func(pid int) error { return syscall.Kill(pid, 0) },
	procStat: func(pid int) (string, error) {
		b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		return string(b), err
	},
	bootID: func() (string, error) {
		b, err := os.ReadFile(processid.BootIDPath)
		return strings.TrimSpace(string(b)), err
	},
	lstartUTC:   osLstartUTC,
	lstartLocal: osLstartLocal,
}

// ProbeLocal proves absence or compares process start identity on this host.
// Callers must first establish that the holder names this machine. Permission,
// command and parse failures remain Unknown; a pid alone never proves life.
func ProbeLocal(live Live) ProcessState { return localHost.probe(live) }

func (h probeHost) probe(live Live) ProcessState {
	if live.PID < 1 || live.PIDStart == "" {
		return Unknown
	}
	// A proc: token is a Linux identity. Only Linux can read it back; elsewhere
	// neither the pid being gone nor present says anything about it.
	isProc := strings.HasPrefix(live.PIDStart, "proc:")
	if isProc && h.goos != "linux" {
		return Unknown
	}
	err := h.kill(live.PID)
	if errors.Is(err, syscall.ESRCH) {
		return Dead
	}
	if err != nil {
		return Unknown
	}
	if isProc {
		return h.probeProc(live)
	}
	return h.probePS(live)
}

// probeProc compares a Linux token, "proc:<boot id>:<ticks>" or the older
// "proc:<ticks>", with /proc/<pid>/stat.
func (h probeHost) probeProc(live Live) ProcessState {
	boot, want, ok := processid.ParseProcToken(live.PIDStart)
	if !ok {
		return Unknown
	}
	stat, err := h.procStat(live.PID)
	if err != nil {
		return Unknown
	}
	ticks, ok := processid.ProcStartTicks(stat)
	if !ok {
		return Unknown
	}
	if boot != "" {
		current, err := h.bootID()
		if err != nil || current == "" {
			return Unknown
		}
		if current != boot {
			return Dead // the host rebooted since the token was written
		}
	}
	if ticks != want {
		return Dead
	}
	return Alive
}

// probePS compares a token with ps. An RFC3339 token is compared at ps's
// second precision in UTC. A raw lstart line is what processid.Start records
// on darwin, in the zone of the process that wrote it; it matches when ps
// prints the same text in UTC or in the system zone. A raw line that matches
// neither stays Unknown because its zone cannot be known.
func (h probeHost) probePS(live Live) ProcessState {
	observed, err := h.lstartUTC(live.PID)
	if err != nil || observed == "" {
		return Unknown
	}
	if observed == live.PIDStart {
		return Alive
	}
	if want, err := time.Parse(time.RFC3339Nano, live.PIDStart); err == nil {
		got, err := processid.ParseLstart(observed, time.UTC)
		if err != nil {
			return Unknown
		}
		if got.Equal(want.UTC().Truncate(time.Second)) {
			return Alive
		}
		return Dead
	}
	if _, err := processid.ParseLstart(live.PIDStart, time.UTC); err != nil {
		return Unknown
	}
	if local, err := h.lstartLocal(live.PID); err == nil && strings.TrimSpace(local) == live.PIDStart {
		return Alive
	}
	return Unknown
}
