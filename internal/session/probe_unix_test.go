//go:build linux || darwin

package session

import (
	"context"
	"errors"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProbeLocal(t *testing.T) {
	if ProbeLocal(Live{}) != Unknown {
		t.Fatal("missing identity is not unknown")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-o", "lstart=", "-p", strconv.Itoa(os.Getpid()))
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	start := strings.TrimSpace(string(b))
	if ProbeLocal(Live{PID: os.Getpid(), PIDStart: start}) != Alive {
		t.Fatal("current process not alive")
	}
	if ProbeLocal(Live{PID: os.Getpid(), PIDStart: "2000-01-01T00:00:00.000Z"}) != Dead {
		t.Fatal("different start not detected")
	}
}

func TestProcessStartRoundTrip(t *testing.T) {
	start, err := ProcessStart(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if got := ProbeLocal(Live{PID: os.Getpid(), PIDStart: contract.FormatTime(start)}); got != Alive {
		t.Fatalf("got %v, want Alive", got)
	}
	for _, pid := range []int{0, -1} {
		if _, err := ProcessStart(pid); err == nil {
			t.Fatalf("accepted pid %d", pid)
		}
	}
}

// statLine builds a /proc/<pid>/stat line whose 22nd field is ticks.
func statLine(ticks string) string {
	return "42 (grid x) S 1 42 42 0 -1 4194304 100 0 0 0 1 1 0 0 20 0 1 0 " + ticks + " 1000 100 18446744073709551615"
}

// fakeProbeHost stands in for the OS: the pid is alive, /proc holds ticks, the
// boot is boot-a, and ps prints utc in UTC and local in the system zone.
// reads counts every /proc read and every ps run, so a test can show a path was not taken.
func fakeProbeHost(goos, ticks, utc, local string) (probeHost, *int) {
	reads := new(int)
	return probeHost{
		goos: goos,
		kill: func(int) error { return nil },
		procStat: func(int) (string, error) {
			*reads++
			return statLine(ticks), nil
		},
		bootID: func() (string, error) {
			*reads++
			return "boot-a", nil
		},
		lstartUTC: func(int) (string, error) {
			*reads++
			return utc, nil
		},
		lstartLocal: func(int) (string, error) {
			*reads++
			return local, nil
		},
	}, reads
}

// The same pid with a different start time is a reused pid: Dead on the /proc path and on the ps path.
func TestProbeLocalPidReuseIsDeadOnBothPaths(t *testing.T) {
	for _, tc := range []struct {
		name, goos, start string
		want              ProcessState
	}{
		{"proc ticks match", "linux", "proc:222", Alive},
		{"proc ticks differ", "linux", "proc:111", Dead},
		{"proc boot and ticks match", "linux", "proc:boot-a:222", Alive},
		{"proc boot matches, ticks differ", "linux", "proc:boot-a:111", Dead},
		{"proc boot differs after reboot", "linux", "proc:boot-b:222", Dead},
		{"ps start matches", "darwin", "2026-11-03T09:15:30.000Z", Alive},
		{"ps start matches within the second", "darwin", "2026-11-03T09:15:30.900Z", Alive},
		{"ps start differs", "darwin", "2026-11-03T09:15:31.000Z", Dead},
		{"ps start differs by a day", "darwin", "2026-11-04T09:15:30.000Z", Dead},
		{"ps start matches on linux too", "linux", "2026-11-03T09:15:30.000Z", Alive},
		{"ps start differs on linux too", "linux", "2026-11-03T09:15:31.000Z", Dead},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := fakeProbeHost(tc.goos, "222", "Tue Nov  3 09:15:30 2026", "Tue Nov  3 04:15:30 2026")
			if got := h.probe(Live{PID: 42, PIDStart: tc.start}); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// A proc: token cannot be read on darwin, where no /proc exists: Unknown, never Dead or Alive.
func TestProbeLocalProcTokenOnDarwinIsUnknown(t *testing.T) {
	for _, kill := range []error{nil, syscall.ESRCH, syscall.EPERM} {
		for _, start := range []string{"proc:222", "proc:boot-a:222", "proc:111", "proc:boot-b:222"} {
			h, reads := fakeProbeHost("darwin", "222", "Tue Nov  3 09:15:30 2026", "Tue Nov  3 04:15:30 2026")
			h.kill = func(int) error { return kill }
			if got := h.probe(Live{PID: 42, PIDStart: start}); got != Unknown {
				t.Errorf("kill %v, token %q: got %v, want Unknown", kill, start, got)
			}
			if *reads != 0 {
				t.Errorf("kill %v, token %q: darwin read /proc or ran ps %d times", kill, start, *reads)
			}
		}
	}
}

// A token written by a darwin process is judged through ps alone, never /proc.
func TestProbeLocalDarwinTokensUsePSOnly(t *testing.T) {
	for _, tc := range []struct {
		name, start string
		want        ProcessState
	}{
		{"raw lstart in the writer's zone", "Tue Nov  3 04:15:30 2026", Alive},
		{"raw lstart in UTC", "Tue Nov  3 09:15:30 2026", Alive},
		// The zone of a raw line is unknown, so a mismatch is not proof of reuse.
		{"raw lstart that matches no zone", "Tue Nov  3 04:15:31 2026", Unknown},
		{"garbage", "start-1", Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := fakeProbeHost("darwin", "222", "Tue Nov  3 09:15:30 2026", "Tue Nov  3 04:15:30 2026")
			h.procStat = func(int) (string, error) { t.Fatal("darwin read /proc"); return "", nil }
			h.bootID = func() (string, error) { t.Fatal("darwin read the boot id"); return "", nil }
			if got := h.probe(Live{PID: 42, PIDStart: tc.start}); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProbeLocalGoneAndUnreadable(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		start := "2026-11-03T09:15:30.000Z"
		h, _ := fakeProbeHost(goos, "222", "Tue Nov  3 09:15:30 2026", "")
		h.kill = func(int) error { return syscall.ESRCH }
		if got := h.probe(Live{PID: 42, PIDStart: start}); got != Dead {
			t.Errorf("%s: kill ESRCH: got %v, want Dead", goos, got)
		}
		// kill says the pid exists, ps no longer prints it: unknown, not dead.
		h, _ = fakeProbeHost(goos, "222", "", "")
		if got := h.probe(Live{PID: 42, PIDStart: start}); got != Unknown {
			t.Errorf("%s: ps printed nothing: got %v, want Unknown", goos, got)
		}
		h.lstartUTC = func(int) (string, error) { return "", errors.New("exit status 1") }
		if got := h.probe(Live{PID: 42, PIDStart: start}); got != Unknown {
			t.Errorf("%s: ps failed: got %v, want Unknown", goos, got)
		}
		h, _ = fakeProbeHost(goos, "222", "mar. nov. 3 09:15:30 2026", "")
		if got := h.probe(Live{PID: 42, PIDStart: start}); got != Unknown {
			t.Errorf("%s: non C locale ps line: got %v, want Unknown", goos, got)
		}
		h, _ = fakeProbeHost(goos, "222", "Tue Nov  3 09:15:30 2026", "")
		h.kill = func(int) error { return syscall.EPERM }
		if got := h.probe(Live{PID: 42, PIDStart: start}); got != Unknown {
			t.Errorf("%s: kill EPERM: got %v, want Unknown", goos, got)
		}
	}
	h, _ := fakeProbeHost("linux", "222", "", "")
	h.procStat = func(int) (string, error) { return "", os.ErrNotExist }
	if got := h.probe(Live{PID: 42, PIDStart: "proc:222"}); got != Unknown {
		t.Errorf("unreadable /proc: got %v, want Unknown", got)
	}
	h.procStat = func(int) (string, error) { return "42 (grid) S 1", nil }
	if got := h.probe(Live{PID: 42, PIDStart: "proc:222"}); got != Unknown {
		t.Errorf("short /proc line: got %v, want Unknown", got)
	}
}
