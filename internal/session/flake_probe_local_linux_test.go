//go:build linux

package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// flakePLSlowPSDelay is how long the fake ps waits: longer than the 2 s deadline
// TestProbeLocal once gave its own ps, so a deadline on that call shows up.
const flakePLSlowPSDelay = 3 * time.Second

// flakePLSlowPS puts a ps first on PATH that sleeps flakePLSlowPSDelay and then
// runs the real ps with the same arguments. The rest of PATH stays, so the
// script finds sleep.
func flakePLSlowPS(t *testing.T) {
	t.Helper()
	real, err := exec.LookPath("ps")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep " + strconv.Itoa(int(flakePLSlowPSDelay/time.Second)) + "\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// The oracle text of a slow ps, taken with no deadline, is a token ProbeLocal
// proves Alive: the ps is slow, the answer is not wrong.
func TestFlakePLSlowPSOracleIsAlive(t *testing.T) {
	flakePLSlowPS(t)
	cmd := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(os.Getpid()))
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
	began := time.Now()
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took < flakePLSlowPSDelay {
		t.Fatalf("ps took %v, so the slow ps was not the one run", took)
	}
	start := strings.TrimSpace(string(b))
	if got := ProbeLocal(Live{PID: os.Getpid(), PIDStart: start}); got != Alive {
		t.Fatalf("oracle token %q: got %v, want Alive", start, got)
	}
}

// TestProbeLocal itself passes with a ps that takes longer than 2 s. On a
// TestProbeLocal that runs its ps under a 2 s deadline this fails with
// "signal: killed" at the ps call.
func TestFlakePLProbeLocalPassesWithSlowPS(t *testing.T) {
	flakePLSlowPS(t)
	TestProbeLocal(t)
}
