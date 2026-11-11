//go:build windows

package session

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/processid"
)

// absentPID is far above any pid Windows hands out, so OpenProcess answers
// ERROR_INVALID_PARAMETER for it.
const absentPID = 0x7FFFFFF0

// TestHelperHoldWindows is the child the tests below start: it holds until its
// stdin closes. Run on its own it does nothing.
func TestHelperHoldWindows(t *testing.T) {
	if os.Getenv("GRID_SESSION_HELPER") != "1" {
		return
	}
	io.Copy(io.Discard, os.Stdin)
}

// holdChild starts a child that lives until the returned release is called.
func holdChild(t *testing.T) (*exec.Cmd, func()) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldWindows$")
	cmd.Env = append(os.Environ(), "GRID_SESSION_HELPER=1")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			in.Close()
			cmd.Wait()
		}
	}
	t.Cleanup(release)
	return cmd, release
}

func TestProbeLocalWindows(t *testing.T) {
	if ProbeLocal(Live{}) != Unknown {
		t.Fatal("missing identity is not unknown")
	}
	self := os.Getpid()
	start, err := ProcessStart(self)
	if err != nil {
		t.Fatal(err)
	}
	if got := ProbeLocal(Live{PID: self, PIDStart: contract.FormatTime(start)}); got != Alive {
		t.Fatalf("current process with its own start: %v, want Alive", got)
	}
	// The start the hook side records on Windows: the lstart line in UTC.
	if got := ProbeLocal(Live{PID: self, PIDStart: processid.Start(self)}); got != Alive {
		t.Fatalf("current process with processid.Start: %v, want Alive", got)
	}
	// A sub-second difference is not a different start.
	if got := ProbeLocal(Live{PID: self, PIDStart: start.Add(900 * time.Millisecond).Format(time.RFC3339Nano)}); got != Alive {
		t.Fatalf("same second: %v, want Alive", got)
	}
	// The pid is live but its start differs: the pid was reused.
	for _, other := range []string{
		"2000-01-01T00:00:00.000Z",
		start.Add(-time.Hour).Format(time.RFC3339Nano),
		start.Add(time.Second).Format(time.RFC3339Nano),
		start.Add(-time.Hour).Format(processid.LstartLayout),
	} {
		if got := ProbeLocal(Live{PID: self, PIDStart: other}); got != Dead {
			t.Errorf("start %q on the current pid: %v, want Dead", other, got)
		}
	}
	// A token this host cannot read says nothing about the process.
	for _, token := range []string{"proc:boot-a:222", "not a time", "12345"} {
		if got := ProbeLocal(Live{PID: self, PIDStart: token}); got != Unknown {
			t.Errorf("token %q: %v, want Unknown", token, got)
		}
	}
}

func TestProbeLocalGonePIDWindows(t *testing.T) {
	const token = "2026-11-08T10:00:00.000Z"
	if got := ProbeLocal(Live{PID: absentPID, PIDStart: token}); got != Dead {
		t.Fatalf("absent pid: %v, want Dead", got)
	}
	cmd, release := holdChild(t)
	pid := cmd.Process.Pid
	start, err := ProcessStart(pid)
	if err != nil {
		t.Fatal(err)
	}
	live := Live{PID: pid, PIDStart: contract.FormatTime(start)}
	if got := ProbeLocal(live); got != Alive {
		t.Fatalf("running child: %v, want Alive", got)
	}
	// Keep a handle open so the pid is not reused once the child exits.
	h, err := syscall.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)
	release()
	if got := ProbeLocal(live); got != Dead {
		t.Fatalf("exited child: %v, want Dead", got)
	}
}

func TestProcessStartWindows(t *testing.T) {
	start, err := ProcessStart(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if start.Location() != time.UTC || !start.Equal(start.Truncate(time.Second)) {
		t.Fatalf("start %v is not UTC at second precision", start)
	}
	if start.After(time.Now()) {
		t.Fatalf("start %v is in the future", start)
	}
	// It is the time processid.Start writes.
	if got := processid.Start(os.Getpid()); got != start.Format(processid.LstartLayout) {
		t.Fatalf("processid.Start %q, ProcessStart %v", got, start)
	}
	for _, pid := range []int{0, -1, absentPID} {
		if _, err := ProcessStart(pid); err == nil {
			t.Errorf("accepted pid %d", pid)
		}
	}
}

func TestProbeLocalFakeHostWindows(t *testing.T) {
	at := time.Date(2026, 11, 3, 9, 15, 30, 123456700, time.UTC)
	denied := fmt.Errorf("open process 42: %w", syscall.Errno(5))
	for _, tc := range []struct {
		name  string
		start string
		got   time.Time
		err   error
		want  ProcessState
	}{
		{"same start", "2026-11-03T09:15:30.000Z", at, nil, Alive},
		{"same start with sub-second", "2026-11-03T09:15:30.9Z", at, nil, Alive},
		{"same start as lstart", "Tue Nov  3 09:15:30 2026", at, nil, Alive},
		{"same start in another zone offset", "2026-11-03T11:15:30+02:00", at, nil, Alive},
		{"later start", "2026-11-03T09:15:31.000Z", at, nil, Dead},
		{"earlier start as lstart", "Tue Nov  3 09:15:29 2026", at, nil, Dead},
		{"process gone", "2026-11-03T09:15:30.000Z", time.Time{}, processid.ErrGone, Dead},
		{"process gone, token unreadable", "garbage", time.Time{}, processid.ErrGone, Dead},
		{"open denied", "2026-11-03T09:15:30.000Z", time.Time{}, denied, Unknown},
		{"times unreadable", "2026-11-03T09:15:30.000Z", time.Time{}, errors.New("process 42 times: failed"), Unknown},
		{"token unreadable", "garbage", at, nil, Unknown},
		{"linux token", "proc:boot-a:222", at, nil, Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := probeHost{creation: func(pid int) (time.Time, error) {
				if pid != 42 {
					t.Errorf("read pid %d", pid)
				}
				return tc.got, tc.err
			}}
			if got := h.probe(Live{PID: 42, PIDStart: tc.start}); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	// No pid or no start: the host is not read at all.
	h := probeHost{creation: func(int) (time.Time, error) {
		t.Error("host read with no identity")
		return time.Time{}, processid.ErrGone
	}}
	for _, live := range []Live{{}, {PID: 42}, {PIDStart: "2026-11-03T09:15:30.000Z"}, {PID: -1, PIDStart: "x"}} {
		if got := h.probe(live); got != Unknown {
			t.Errorf("%+v: %v, want Unknown", live, got)
		}
	}
}
