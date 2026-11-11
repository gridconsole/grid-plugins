//go:build windows

package session

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// ProcessStart reads the process creation time in UTC at second precision, the
// precision the Unix side reads from ps.
func ProcessStart(pid int) (time.Time, error) {
	if pid < 1 {
		return time.Time{}, fmt.Errorf("pid must be positive")
	}
	t, err := processid.CreationTime(pid)
	if err != nil {
		return time.Time{}, err
	}
	return t.Truncate(time.Second), nil
}

// probeHost is the set of OS reads ProbeLocal makes, so tests can stand in for
// the OS with no real process.
type probeHost struct {
	creation func(pid int) (time.Time, error)
}

var localHost = probeHost{creation: processid.CreationTime}

// ProbeLocal proves absence or compares process start identity on this host.
// Callers must first establish that the holder names this machine. A process
// that cannot be opened, and a token this host cannot read, stay Unknown; a pid
// alone never proves life.
func ProbeLocal(live Live) ProcessState { return localHost.probe(live) }

func (h probeHost) probe(live Live) ProcessState {
	if live.PID < 1 || live.PIDStart == "" {
		return Unknown
	}
	// A proc: token is a Linux identity: this host cannot read it back, and
	// neither the pid being gone nor present says anything about it.
	if strings.HasPrefix(live.PIDStart, "proc:") {
		return Unknown
	}
	got, err := h.creation(live.PID)
	if errors.Is(err, processid.ErrGone) {
		return Dead
	}
	if err != nil {
		return Unknown
	}
	want, ok := tokenTime(live.PIDStart)
	if !ok {
		return Unknown
	}
	if got.UTC().Truncate(time.Second).Equal(want) {
		return Alive
	}
	return Dead
}

// tokenTime reads a recorded start as an RFC3339 time or as the UTC lstart
// line processid.Start writes on Windows, at second precision.
func tokenTime(token string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, token); err == nil {
		return t.UTC().Truncate(time.Second), true
	}
	if t, err := processid.ParseLstart(token, time.UTC); err == nil {
		return t, true
	}
	return time.Time{}, false
}
