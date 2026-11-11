//go:build linux

package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/processid"
)

// flakePSNoPS puts a ps that exits 1 at once as the only command on PATH, so a
// test fails if the code under test runs ps by name.
func flakePSNoPS(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// ProcessStart and ProbeLocal answer for a live pid with no ps they can run.
func TestFlakePSLinuxRunsNoPS(t *testing.T) {
	flakePSNoPS(t)
	self := os.Getpid()
	start, err := ProcessStart(self)
	if err != nil {
		t.Fatalf("ProcessStart needed ps: %v", err)
	}
	if got := ProbeLocal(Live{PID: self, PIDStart: contract.FormatTime(start)}); got != Alive {
		t.Errorf("RFC3339 token: got %v, want Alive", got)
	}
	raw := start.UTC().Format(processid.LstartLayout)
	if got := ProbeLocal(Live{PID: self, PIDStart: raw}); got != Alive {
		t.Errorf("raw UTC lstart token %q: got %v, want Alive", raw, got)
	}
}

// A different start is still proven Dead with no ps to ask.
func TestFlakePSLinuxOldTokenIsDeadWithoutPS(t *testing.T) {
	flakePSNoPS(t)
	if got := ProbeLocal(Live{PID: os.Getpid(), PIDStart: "2000-01-01T00:00:00.000Z"}); got != Dead {
		t.Fatalf("got %v, want Dead", got)
	}
}

// lstartLocal on Linux is not processid.PSLstart, which runs /bin/ps by its
// absolute path: PATH cannot stand in for it, so the function itself is compared.
func TestFlakePSLinuxLocalIsNotPSLstart(t *testing.T) {
	got := reflect.ValueOf(localHost.lstartLocal).Pointer()
	if got == reflect.ValueOf(processid.PSLstart).Pointer() {
		t.Fatal("lstartLocal is processid.PSLstart, which forks /bin/ps")
	}
}

// flakePSRunPS runs /bin/ps with no deadline for one pid; env is its whole environment.
func flakePSRunPS(t *testing.T, pid int, env ...string) string {
	t.Helper()
	if _, err := os.Stat("/bin/ps"); err != nil {
		t.Skipf("no /bin/ps to compare with: %v", err)
	}
	cmd := exec.Command("/bin/ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	cmd.Env = env
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("ps -p %d: %v", pid, err)
	}
	return strings.TrimSpace(string(b))
}

// ps is what stored tokens were written with: the start read from /proc must equal it.
func TestFlakePSLinuxMatchesPS(t *testing.T) {
	for _, pid := range []int{os.Getpid(), 1} {
		utcText := flakePSRunPS(t, pid, "TZ=UTC", "LC_ALL=C")
		want, err := processid.ParseLstart(utcText, time.UTC)
		if err != nil {
			t.Fatalf("pid %d: parse ps text %q: %v", pid, utcText, err)
		}
		got, err := ProcessStart(pid)
		if err != nil {
			t.Fatalf("pid %d: ProcessStart: %v", pid, err)
		}
		if !got.Equal(want) {
			t.Errorf("pid %d: ProcessStart %v, ps %v", pid, got, want)
		}
		if text, err := localHost.lstartUTC(pid); err != nil || text != utcText {
			t.Errorf("pid %d: lstartUTC %q (%v), ps %q", pid, text, err, utcText)
		}
		// PSLstart runs ps with LC_ALL=C and no TZ, the zone of /etc/localtime.
		localText := flakePSRunPS(t, pid, "LC_ALL=C")
		// probePS trims what lstartLocal returns, as PSLstart's line ends in a newline.
		if text, err := localHost.lstartLocal(pid); err != nil || strings.TrimSpace(text) != localText {
			t.Errorf("pid %d: lstartLocal %q (%v), ps %q", pid, text, err, localText)
		}
	}
}
