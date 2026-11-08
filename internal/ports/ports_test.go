//go:build linux || darwin

package ports

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTwentyConcurrentClaims(t *testing.T) {
	s := Store{GridHome: t.TempDir()}
	task := filepath.Join(s.GridHome, "projects", "example", "tasks", "T-1")
	var wg sync.WaitGroup
	results := make(chan Record, 20)
	errs := make(chan error, 20)
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := s.Claim(task, fmt.Sprintf("J-%d", i), os.Getpid(), Range{})
			if err != nil {
				errs <- err
			} else {
				results <- r
			}
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	used := map[int]bool{}
	for r := range results {
		for p := r.Start; p <= r.End; p++ {
			if used[p] {
				t.Fatalf("shared port %d", p)
			}
			used[p] = true
		}
	}
	if len(used) != 200 {
		t.Fatalf("got %d distinct ports", len(used))
	}
	records, err := s.List()
	if err != nil || len(records) != 20 {
		t.Fatalf("list: %d %v", len(records), err)
	}
}

func TestGCDeadProcess(t *testing.T) {
	s := Store{GridHome: t.TempDir()}
	task := filepath.Join(s.GridHome, "tasks", "T-1")
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if _, err := s.Claim(task, "J-1", cmd.Process.Pid, Range{}); err != nil {
		t.Fatal(err)
	}
	if removed, err := s.GC(); err != nil || len(removed) != 0 {
		t.Fatalf("live gc: %v %v", removed, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if removed, err := s.GC(); err != nil || len(removed) != 1 {
		t.Fatalf("dead gc: %v %v", removed, err)
	}
	if records, err := s.List(); err != nil || len(records) != 0 {
		t.Fatalf("list: %v %v", records, err)
	}
}

func TestRangeReleaseAndSameJob(t *testing.T) {
	s := Store{GridHome: t.TempDir()}
	task := filepath.Join(s.GridHome, "tasks", "T-1")
	r, err := s.Claim(task, "J-1", os.Getpid(), Range{42001, 42015})
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Claim(task, "J-1", os.Getpid(), Range{42001, 42015})
	if err != nil || r != again {
		t.Fatalf("repeat: %v %v", again, err)
	}
	if _, err := s.Claim(task, "J-2", os.Getpid(), Range{42001, 42015}); err == nil {
		t.Fatal("allocated incomplete block")
	}
	if err := s.Release(task, "J-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Release(task, "J-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(task, "J-2", os.Getpid(), Range{42001, 42015}); err != nil {
		t.Fatal(err)
	}
	// The same Job id in another task must not alias this allocation.
	if _, err := s.Claim(filepath.Join(s.GridHome, "tasks", "T-2"), "J-2", os.Getpid(), Range{42001, 42015}); err == nil {
		t.Fatal("aliased tasks")
	}
}

func TestGCExpiredLeaseAndListener(t *testing.T) {
	s := Store{GridHome: t.TempDir(), Now: func() time.Time { return time.Date(2026, 11, 8, 11, 0, 0, 0, time.UTC) }}
	task := filepath.Join(s.GridHome, "tasks", "T-1")
	holder := "worker@01K9J000000000000000000002"
	write := func(path string, v any) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(v)
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	leasePath := filepath.Join(task, "lease", "J-1.json")
	write(leasePath, map[string]any{"lease": "J-1", "holder": holder, "fence": 2, "gen": 0, "attempt": 1, "since": "2026-11-08T10:00:00.000Z", "pid": nil, "pid_start": nil})
	livePath := filepath.Join(task, ".live", holder+".2.json")
	write(livePath, map[string]any{"lease": "J-1", "holder": holder, "fence": 2, "attempt": 1, "pid": os.Getpid(), "pid_start": "2026-11-08T10:00:00.000Z", "beat_at": "2026-11-08T10:00:00.000Z", "output_at": "2026-11-08T10:00:00.000Z", "until": "2026-11-08T10:10:00.000Z"})
	before, _ := os.ReadFile(leasePath)
	beatBefore, _ := os.ReadFile(livePath)
	// Let the OS choose a free port and place it in this block.
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	start := port
	if start > 65526 {
		start = 65526
	}
	_, err = s.Claim(task, "J-1", os.Getpid(), Range{start, start + 9})
	if err != nil {
		t.Fatal(err)
	}
	if removed, err := s.GC(); err != nil || len(removed) != 0 {
		t.Fatalf("listener gc: %v %v", removed, err)
	}
	ln.Close()
	if removed, err := s.GC(); err != nil || len(removed) != 1 {
		t.Fatalf("expired gc: %v %v", removed, err)
	}
	after, _ := os.ReadFile(leasePath)
	beatAfter, _ := os.ReadFile(livePath)
	if string(before) != string(after) || string(beatBefore) != string(beatAfter) {
		t.Fatal("gc wrote lease or heartbeat")
	}
}

func TestRefuseSymlinksAndBadPaths(t *testing.T) {
	s := Store{GridHome: t.TempDir()}
	if err := os.Symlink(t.TempDir(), filepath.Join(s.GridHome, "ports")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(); err == nil {
		t.Fatal("followed ports symlink")
	}
	s.GridHome = t.TempDir()
	if _, err := s.Claim(filepath.Dir(s.GridHome), "J-1", os.Getpid(), Range{}); err == nil {
		t.Fatal("accepted outside task")
	}
	if _, err := s.Claim(filepath.Join(s.GridHome, "task"), "../J-1", os.Getpid(), Range{}); err == nil {
		t.Fatal("accepted traversal")
	}
}

func TestGCIPv6Listener(t *testing.T) {
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 is unavailable")
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if !listening(Range{port, port}) {
		t.Fatal("did not detect IPv6-only listener")
	}
}

func TestClaimInSeparateProcesses(t *testing.T) {
	if home := os.Getenv("GRID_PORT_CLAIM_TEST_HOME"); home != "" {
		s := Store{GridHome: home}
		_, err := s.Claim(filepath.Join(home, "tasks", "T-1"), os.Getenv("GRID_PORT_CLAIM_TEST_JOB"), os.Getppid(), Range{})
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	home := t.TempDir()
	const count = 20
	cmds := make([]*exec.Cmd, count)
	for i := range cmds {
		cmd := exec.Command(os.Args[0], "-test.run=^TestClaimInSeparateProcesses$")
		cmd.Env = append(os.Environ(), "GRID_PORT_CLAIM_TEST_HOME="+home, fmt.Sprintf("GRID_PORT_CLAIM_TEST_JOB=J-%d", i+1))
		cmds[i] = cmd
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	records, err := (Store{GridHome: home}).List()
	if err != nil {
		t.Fatal(err)
	}
	used := map[int]bool{}
	for _, r := range records {
		for port := r.Start; port <= r.End; port++ {
			if used[port] {
				t.Fatalf("shared port %d", port)
			}
			used[port] = true
		}
	}
	if len(records) != count || len(used) != count*BlockSize {
		t.Fatalf("got %d blocks and %d ports", len(records), len(used))
	}
}
