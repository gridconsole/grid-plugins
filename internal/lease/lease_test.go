package lease

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/testenv"
)

func TestLifecycle(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := Store{ProjectDir: t.TempDir(), Now: func() time.Time { return now }}
	status, err := s.Status()
	if err != nil || status.Held {
		t.Fatalf("initial status: %+v, %v", status, err)
	}
	if _, err := os.Stat(filepath.Join(s.ProjectDir, "lease")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("status wrote files: %v", err)
	}
	first, err := s.Take("session-a")
	if err != nil || first.Fence != 1 || first.Until.Sub(now) != Duration {
		t.Fatalf("take: %+v, %v", first, err)
	}
	for _, holder := range []string{"session-a", "session-b"} {
		if _, err := s.Take(holder); !errors.Is(err, ErrHeld) {
			t.Fatalf("take held: %v", err)
		}
	}
	if _, err := s.Renew("session-b", first.Fence); !errors.Is(err, ErrNotHolder) {
		t.Fatalf("wrong holder: %v", err)
	}
	now = now.Add(14 * time.Minute)
	renewed, err := s.Renew("session-a", first.Fence)
	if err != nil || renewed.Until.Sub(now) != Duration || renewed.Since != first.Since {
		t.Fatalf("renew: %+v, %v", renewed, err)
	}
	now = renewed.Until
	if _, err := s.Renew("session-a", first.Fence); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired renew: %v", err)
	}
	if _, err := s.Release("session-a", first.Fence); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired release: %v", err)
	}
	status, err = s.Status()
	if err != nil || status.Held {
		t.Fatalf("expired status: %+v, %v", status, err)
	}
	// Reusing the same holder must still fence out the old invocation.
	second, err := s.Take("session-a")
	if err != nil || second.Fence != 2 {
		t.Fatalf("steal: %+v, %v", second, err)
	}
	if _, err := s.Release("session-a", first.Fence); !errors.Is(err, ErrNotHolder) {
		t.Fatalf("old release: %v", err)
	}
	if _, err := s.Renew("session-a", first.Fence); !errors.Is(err, ErrNotHolder) {
		t.Fatalf("old renew: %v", err)
	}
	if _, err := s.Release("session-a", second.Fence); err != nil {
		t.Fatal(err)
	}
	third, err := s.Take("session-b")
	if err != nil || third.Fence != 3 {
		t.Fatalf("take after release: %+v, %v", third, err)
	}
}

func TestCorruptionAndSymlinks(t *testing.T) {
	for _, name := range []string{"merge.json", "seq"} {
		t.Run(name, func(t *testing.T) {
			s := Store{ProjectDir: t.TempDir()}
			if _, err := s.Take("session-a"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(s.ProjectDir, "lease", name), []byte("broken"), 0o600); err != nil {
				t.Fatal(err)
			}
			s.Now = func() time.Time { return time.Now().Add(time.Hour) }
			if _, err := s.Take("session-b"); err == nil {
				t.Fatal("corrupt state granted a lease")
			}
		})
	}
	dir := t.TempDir()
	s := Store{ProjectDir: dir}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "lease")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Take("session-a"); err == nil {
		t.Fatal("followed lease directory symlink")
	}
}

// rewrite writes the new record before it truncates, so a kill in between
// leaves the new record followed by the tail of the longer record it replaced.
// tear runs change, then puts that old tail back after the new record.
func tear(t *testing.T, path string, change func()) {
	t.Helper()
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	change()
	next, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) >= len(old) {
		t.Fatalf("the new record must be shorter to leave a tail: %d >= %d", len(next), len(old))
	}
	if err := os.WriteFile(path, append(append([]byte{}, next...), old[len(next):]...), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readOnlyRecord(t *testing.T, path string) Record {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec Record
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("file holds more than one record: %q: %v", b, err)
	}
	return rec
}

func TestTornRecordRecovers(t *testing.T) {
	const long = "member+tag@01J00000000000000000000000-long-holder"
	// Each case leaves a torn file whose first record is held or free.
	cases := []struct {
		name      string
		change    func(s Store, first Record) error
		held      bool
		wantFence uint64
	}{
		{"release", func(s Store, first Record) error { _, err := s.Release(long, first.Fence); return err }, false, 1},
		{"takeover by a shorter holder", func(s Store, first Record) error { _, err := s.Take("b"); return err }, true, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			s := Store{ProjectDir: t.TempDir(), Now: func() time.Time { return now }}
			path := filepath.Join(s.ProjectDir, "lease", "merge.json")
			first, err := s.Take(long)
			if err != nil {
				t.Fatal(err)
			}
			if c.held {
				now = first.Until // the first grant has expired, so b takes it over
			}
			tear(t, path, func() {
				if err := c.change(s, first); err != nil {
					t.Fatal(err)
				}
			})
			torn, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// A reader ignores the tail and never writes.
			status, err := s.Status()
			if err != nil || status.Held != c.held || status.Fence != c.wantFence {
				t.Fatalf("status of a torn record: %+v, %v", status, err)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(after, torn) {
				t.Fatal("status repaired the file under a shared lock")
			}
			// The next writer cuts the tail off and grants a new fence.
			now = now.Add(Duration)
			rec, err := s.Take("c")
			if err != nil || rec.Fence != c.wantFence+1 {
				t.Fatalf("take after a torn record: %+v, %v", rec, err)
			}
			if got := readOnlyRecord(t, path); got.Fence != rec.Fence {
				t.Fatalf("file holds %+v, want fence %d", got, rec.Fence)
			}
		})
	}
	t.Run("a refused take still repairs", func(t *testing.T) {
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		s := Store{ProjectDir: t.TempDir(), Now: func() time.Time { return now }}
		path := filepath.Join(s.ProjectDir, "lease", "merge.json")
		first, err := s.Take(long)
		if err != nil {
			t.Fatal(err)
		}
		now = first.Until
		tear(t, path, func() {
			if _, err := s.Take("b"); err != nil {
				t.Fatal(err)
			}
		})
		if _, err := s.Take("c"); !errors.Is(err, ErrHeld) {
			t.Fatalf("take while held: %v", err)
		}
		if got := readOnlyRecord(t, path); got.Holder == nil || *got.Holder != "b" {
			t.Fatalf("refused take left the tail: %+v", got)
		}
	})
}

func TestFenceSurvivesAMissingCounter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := Store{ProjectDir: t.TempDir(), Now: func() time.Time { return now }}
	for want := uint64(1); want <= 2; want++ {
		if rec, err := s.Take("a"); err != nil || rec.Fence != want {
			t.Fatalf("take: %+v, %v", rec, err)
		}
		now = now.Add(Duration)
	}
	if err := os.Remove(filepath.Join(s.ProjectDir, "lease", "seq")); err != nil {
		t.Fatal(err)
	}
	if rec, err := s.Take("b"); err != nil || rec.Fence != 3 {
		t.Fatalf("take after the counter was lost: %+v, %v", rec, err)
	}
}

func TestHolderRefusesControlCharacters(t *testing.T) {
	s := Store{ProjectDir: t.TempDir()}
	for _, holder := range []string{"", " a", "a\x1b[31m", "a\u0085b", "a\x7f", "a\tb"} {
		if _, err := s.Take(holder); err == nil {
			t.Fatalf("accepted %q", holder)
		}
	}
	if _, err := s.Take("member+tag@01J00000000000000000000000"); err != nil {
		t.Fatal(err)
	}
}

// Child processes wait on stdin so the parent can release all contenders only
// after they have started. They share real flock locks, not Go mutexes.
func TestLeaseProcessHelper(t *testing.T) {
	if os.Getenv("GRID_LEASE_TEST_HELPER") != "1" {
		return
	}
	var ready [1]byte
	if _, err := os.Stdin.Read(ready[:]); err != nil {
		os.Exit(3)
	}
	seconds, err := strconv.ParseInt(os.Getenv("GRID_LEASE_TEST_NOW"), 10, 64)
	if err != nil {
		os.Exit(3)
	}
	s := Store{ProjectDir: os.Getenv("GRID_LEASE_TEST_DIR"), Now: func() time.Time { return time.Unix(seconds, 0) }}
	rec, err := s.Take("contender-" + os.Getenv("GRID_LEASE_TEST_ID"))
	if errors.Is(err, ErrHeld) {
		os.Exit(10)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	fmt.Fprintln(os.Stdout, rec.Fence)
	os.Exit(0)
}

func TestMultiProcessRace(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for round := 0; round < 2; round++ {
		// The second race steals exactly at expiry of the first winner.
		at := now.Add(time.Duration(round) * Duration)
		const count = 12
		cmds := make([]*exec.Cmd, count)
		outputs := make([]*strings.Builder, count)
		starts := make([]func(), count)
		for i := range cmds {
			cmd := exec.Command(os.Args[0], "-test.run=^TestLeaseProcessHelper$")
			cmd.Env = append(testenv.Temp(t, os.Environ()), "GRID_LEASE_TEST_HELPER=1", "GRID_LEASE_TEST_DIR="+dir,
				"GRID_LEASE_TEST_NOW="+strconv.FormatInt(at.Unix(), 10), "GRID_LEASE_TEST_ID="+strconv.Itoa(i))
			pipe, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			outputs[i] = &strings.Builder{}
			cmd.Stdout, cmd.Stderr = outputs[i], outputs[i]
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = pipe.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })
			starts[i] = func() { _, _ = pipe.Write([]byte{1}); _ = pipe.Close() }
			cmds[i] = cmd
		}
		for _, start := range starts {
			start()
		}
		winners := 0
		for i, cmd := range cmds {
			err := cmd.Wait()
			if err == nil {
				winners++
				if strings.TrimSpace(outputs[i].String()) != strconv.Itoa(round+1) {
					t.Errorf("wrong fence: %s", outputs[i])
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 10 {
					t.Errorf("contender: %v: %s", err, outputs[i])
				}
			}
		}
		if winners != 1 {
			t.Fatalf("round %d: %d winners", round, winners)
		}
	}
}
