//go:build linux || darwin

package env

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/ports"
)

func TestDevServerLifecycle(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "server")
	build := exec.Command("go", "build", "-o", binary, "./testdata/server")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v %s", err, out)
	}
	for _, mode := range []string{"healthy", "tree", "term", "stubborn", "unhealthy", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			dir := t.TempDir()
			m := Manager{Ports: ports.Store{GridHome: home}, TaskDir: filepath.Join(home, "tasks", "T-1"), Dir: dir, Grace: 100 * time.Millisecond}
			// Avoid assuming the standard dev range is unused on the test machine.
			port := freeBlock(t)
			c := Config{Command: "exec '" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "' " + mode, HealthURL: "http://127.0.0.1:{port}/health", Timeout: "3s", PortRange: []int{port, port + 9}}
			if mode == "unhealthy" || mode == "redirect" {
				c.Timeout = "300ms"
			}
			t.Cleanup(func() {
				if err := m.Down("J-1"); err != nil {
					t.Error(err)
				}
			})
			started := time.Now()
			r, err := m.Up(context.Background(), "J-1", c)
			if mode == "unhealthy" || mode == "redirect" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected health timeout: %v", err)
				}
				assertEmpty(t, m.Ports)
				data, readErr := os.ReadFile(filepath.Join(dir, "server.pid"))
				if readErr != nil {
					t.Fatal(readErr)
				}
				pid, _ := strconv.Atoi(string(data))
				if ports.Alive(pid, true) {
					t.Fatalf("timeout left process group %d", pid)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if r.PID <= 0 || r.PIDStart == "" || !r.Group || !ports.Alive(r.PID, true) {
				t.Fatalf("server not recorded: %+v", r)
			}
			if mode == "tree" {
				data, err := os.ReadFile(filepath.Join(dir, "server.pid"))
				if err != nil {
					t.Fatal(err)
				}
				child, _ := strconv.Atoi(string(data))
				if child == r.PID {
					t.Fatal("test did not start a descendant")
				}
			}
			// A second up returns the same healthy server, and release cannot orphan it.
			again, err := m.Up(context.Background(), "J-1", c)
			if err != nil || again != r {
				t.Fatalf("repeat up: %+v %v", again, err)
			}
			if err := m.Ports.Release(m.TaskDir, "J-1"); err == nil {
				t.Fatal("released live server")
			}
			if err := m.Down("J-1"); err != nil {
				t.Fatal(err)
			}
			if ports.Alive(r.PID, true) {
				t.Fatalf("down left process group %d", r.PID)
			}
			assertEmpty(t, m.Ports)
			signals, err := os.ReadFile(filepath.Join(dir, "signals.log"))
			if err != nil {
				t.Fatal(err)
			}
			want := "interrupt\n"
			if mode == "term" || mode == "stubborn" {
				want += "terminated\n"
			}
			if string(signals) != want {
				t.Fatalf("signal order %q, want %q", signals, want)
			}
			if mode == "stubborn" && time.Since(started) < 2*m.Grace {
				t.Fatal("did not observe grace periods")
			}
		})
	}
}

// freeBlock returns the start of ten ports with no listener, since a machine
// can have other servers in the ephemeral range.
func freeBlock(t *testing.T) int {
	t.Helper()
	for i := 0; i < 100; i++ {
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		start := min(ln.Addr().(*net.TCPAddr).Port, 65526)
		ln.Close()
		if (ports.Range{Start: start, End: start + ports.BlockSize - 1}).Available() {
			return start
		}
	}
	t.Fatal("no free ten-port block")
	return 0
}
func assertEmpty(t *testing.T, s ports.Store) {
	t.Helper()
	records, err := s.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("reservations remain: %+v %v", records, err)
	}
}

func TestBadConfigAndFailedStart(t *testing.T) {
	home := t.TempDir()
	m := Manager{Ports: ports.Store{GridHome: home}, TaskDir: filepath.Join(home, "task"), Dir: t.TempDir(), Grace: 10 * time.Millisecond}
	for _, c := range []Config{
		{}, {Command: "exit 0", HealthURL: "http://example.com"},
		{Command: "exit 0", HealthURL: "http://127.0.0.1:{port}", Timeout: "-1s"},
		{Command: "exit 0", HealthURL: "http://127.0.0.1:{port}", PortRange: []int{1, 2}},
	} {
		if _, err := m.Up(context.Background(), "J-1", c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
	c := Config{Command: "exit 7", HealthURL: "http://127.0.0.1:{port}", Timeout: "1s"}
	if _, err := m.Up(context.Background(), "J-1", c); err == nil || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("failed command: %v", err)
	}
	assertEmpty(t, m.Ports)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Up(ctx, "J-1", c); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestUpRefusesExistingListener(t *testing.T) {
	home := t.TempDir()
	m := Manager{Ports: ports.Store{GridHome: home}, TaskDir: filepath.Join(home, "tasks", "T-1"), Dir: t.TempDir()}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := min(listener.Addr().(*net.TCPAddr).Port, 65526)
	c := Config{Command: "touch started", HealthURL: "http://127.0.0.1:{port}", PortRange: []int{port, port + 9}}
	if _, err := m.Up(context.Background(), "J-1", c); err == nil || !strings.Contains(err.Error(), "listener") {
		t.Fatalf("expected port conflict: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir, "started")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("started command with occupied block")
	}
	assertEmpty(t, m.Ports)
}

// The parent `env up` is killed after the server is healthy, so nothing runs
// grid env down. gc must stop the server once the Job's lease has expired.
func TestGCStopsOrphanedServerAfterParentKilledAndLeaseExpiry(t *testing.T) {
	const holder = "worker@01K9J000000000000000000002"
	if home := os.Getenv("GRID_ENV_UP_HOME"); home != "" {
		start, _ := strconv.Atoi(os.Getenv("GRID_ENV_UP_PORT"))
		m := Manager{Ports: ports.Store{GridHome: home}, TaskDir: filepath.Join(home, "tasks", "T-1"), Dir: os.Getenv("GRID_ENV_UP_DIR")}
		c := Config{Command: os.Getenv("GRID_ENV_UP_COMMAND"), HealthURL: "http://127.0.0.1:{port}/health", Timeout: "10s", PortRange: []int{start, start + 9}}
		if _, err := m.Up(context.Background(), "J-1", c); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Minute)
		return
	}
	binary := filepath.Join(t.TempDir(), "server")
	if out, err := exec.Command("go", "build", "-o", binary, "./testdata/server").CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v %s", err, out)
	}
	home, dir := t.TempDir(), t.TempDir()
	task := filepath.Join(home, "tasks", "T-1")
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(task, "feed", "lease", "J-1.json"), `{"lease":"J-1","holder":"`+holder+`","fence":2,"gen":0,"attempt":1,"since":"2026-11-08T10:00:00.000Z","pid":null,"pid_start":null}`)
	write(filepath.Join(task, "feed", ".live", holder+".2.json"), `{"lease":"J-1","holder":"`+holder+`","fence":2,"attempt":1,"pid":1234,"pid_start":"2026-11-08T10:00:00.000Z","beat_at":"2026-11-08T10:00:00.000Z","output_at":"2026-11-08T10:00:00.000Z","until":"2026-11-08T10:10:00.000Z"}`)
	start := freeBlock(t)
	parent := exec.Command(os.Args[0], "-test.run=^TestGCStopsOrphanedServerAfterParentKilledAndLeaseExpiry$")
	parent.Env = append(os.Environ(), "GRID_ENV_UP_HOME="+home, "GRID_ENV_UP_DIR="+dir, "GRID_ENV_UP_PORT="+strconv.Itoa(start), "GRID_ENV_UP_COMMAND=exec '"+binary+"' healthy")
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	defer func() {
		if !reaped {
			_ = parent.Process.Kill()
			_ = parent.Wait()
		}
	}()
	at := func(clock string) ports.Store {
		now, _ := time.Parse(time.RFC3339, clock)
		return ports.Store{GridHome: home, Now: func() time.Time { return now }, Grace: 100 * time.Millisecond}
	}
	var rec ports.Record
	deadline := time.Now().Add(20 * time.Second)
	for rec.PID == 0 || !rec.Group {
		if time.Now().After(deadline) {
			t.Fatal("env up never recorded a server")
		}
		time.Sleep(20 * time.Millisecond)
		if records, err := at("2026-11-08T10:05:00Z").List(); err == nil && len(records) == 1 {
			rec = records[0]
		}
	}
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = parent.Wait()
	reaped = true
	t.Cleanup(func() { _ = ports.Stop(rec.PID, 100*time.Millisecond) })
	if !ports.Alive(rec.PID, true) {
		t.Fatal("server died with its parent")
	}
	if removed, err := at("2026-11-08T10:05:00Z").GC(); err != nil || len(removed) != 0 || !ports.Alive(rec.PID, true) {
		t.Fatalf("gc touched a server under a live lease: %v %v", removed, err)
	}
	if removed, err := at("2026-11-08T11:00:00Z").GC(); err != nil || len(removed) != 1 {
		t.Fatalf("gc after expiry: %v %v", removed, err)
	}
	if ports.Alive(rec.PID, true) {
		t.Fatalf("expired lease left the server group %d running", rec.PID)
	}
	assertEmpty(t, ports.Store{GridHome: home})
}
