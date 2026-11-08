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
			ln, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := ln.Addr().(*net.TCPAddr).Port
			ln.Close()
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
			if r.PID <= 0 || !r.Group || !ports.Alive(r.PID, true) {
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
