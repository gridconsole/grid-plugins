//go:build linux || darwin

package ports

import (
	"errors"
	"syscall"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// Alive treats permission errors as alive; only ESRCH proves death. A group
// check retains a block while shell children survive their original leader.
func Alive(pid int, group bool) bool {
	if pid <= 0 {
		return false
	}
	if group {
		pid = -pid
	}
	return syscall.Kill(pid, 0) != syscall.ESRCH
}

func networkUnsupported(err error) bool {
	// EADDRNOTAVAIL: no IPv6 loopback address, as in containers with IPv6 off.
	return errors.Is(err, syscall.EAFNOSUPPORT) || errors.Is(err, syscall.EPROTONOSUPPORT) || errors.Is(err, syscall.EADDRNOTAVAIL)
}

// Stop sends SIGINT, SIGTERM and SIGKILL to the process group, waiting grace
// after each for the group to leave. gc and env down share this ladder.
func Stop(pid int, grace time.Duration) error {
	return StopRecord(Record{PID: pid, Group: true, PIDStart: processid.Start(pid)}, grace)
}

// ProcessMatches checks the leader's start token while it is alive. Once the
// leader exits, its group cannot be reused while any member still lives.
// Legacy records without a token retain the original PID/group liveness check.
func ProcessMatches(r Record) (bool, error) {
	if !Alive(r.PID, true) {
		return false, nil
	}
	if !Alive(r.PID, false) {
		return true, nil
	}
	if r.PIDStart == "" {
		return true, nil
	}
	start := processid.Start(r.PID)
	if start == "" {
		// The leader may have exited during the identity lookup.
		if !Alive(r.PID, false) {
			return Alive(r.PID, true), nil
		}
		return false, errors.New("cannot verify dev server process start time")
	}
	return start == r.PIDStart, nil
}

// StopRecord rechecks available process identity before every signal, with
// the legacy-record and surviving-group behavior described by ProcessMatches.
func StopRecord(r Record, grace time.Duration) error {
	return stopRecord(r, grace, ProcessMatches, syscall.Kill, waitStopped)
}

// Keep the signal ladder independent of wall time so its order and grace periods
// can be checked without requiring a child signal handler to win a scheduling race.
func stopRecord(r Record, grace time.Duration, matchesProcess func(Record) (bool, error), signal func(int, syscall.Signal) error, wait func(int, time.Duration) bool) error {
	pid := r.PID
	if pid <= 0 {
		return errors.New("invalid dev server process group")
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL} {
		matches, err := matchesProcess(r)
		if err != nil {
			return err
		}
		if !matches {
			return nil
		}
		err = signal(-pid, sig)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return err
		}
		if wait(pid, grace) {
			return nil
		}
	}
	// SIGKILL has been delivered to the whole group. Orphaned zombies can remain
	// visible to kill(0) until their parent reaps them, but cannot keep serving.
	return nil
}

func waitStopped(pid int, grace time.Duration) bool {
	until := time.Now().Add(grace)
	for time.Now().Before(until) {
		if !Alive(pid, true) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
