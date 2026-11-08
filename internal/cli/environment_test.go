//go:build linux || darwin

package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/ports"
)

func environmentFixture(t *testing.T) (*harness, string, string) {
	t.Helper()
	h := newHarness(t)
	dir := filepath.Join(h.root, "checkout")
	h.mkdir(dir)
	task := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-1-dev")
	h.write(filepath.Join(task, "task.md"), "---\nid: T-01\ntitle: Dev server\nproject: example\nstate: running\ncreated: 2026-11-08T10:00:00Z\n---\n")
	h.write(filepath.Join(task, "intent.md"), "Check the development server.\n")
	h.write(filepath.Join(h.gridHome, "projects", "example", "project.md"), fmt.Sprintf("---\nid: example\nfolders:\n  - machine: %s\n    path: %q\nenvironments:\n  port_range: [44000, 44199]\n---\n", h.env.Machine, dir))
	return h, task, dir
}
func TestPortsCommands(t *testing.T) {
	h, task, _ := environmentFixture(t)
	if code := h.run(task, "ports", "claim", "J-1", "--json"); code != 0 {
		t.Fatalf("claim %d: %s", code, h.errb)
	}
	var r ports.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Start != 44000 || r.End != 44009 || r.Job != "J-1" || r.PID <= 0 {
		t.Fatalf("claim: %+v", r)
	}
	if code := h.run(h.home, "ports", "list", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	var listed []ports.Record
	if err := json.Unmarshal(h.out.Bytes(), &listed); err != nil || len(listed) != 1 || listed[0] != r {
		t.Fatalf("list: %s %v", h.out, err)
	}
	if code := h.run(task, "ports", "release", "J-1"); code != 0 {
		t.Fatal(h.errb)
	}
	if code := h.run(h.home, "ports", "list", "--json"); code != 0 || h.out.String() != "[]\n" {
		t.Fatalf("empty list: %s %s", h.out, h.errb)
	}
	if code := h.run(h.home, "ports", "gc"); code != 0 {
		t.Fatal(h.errb)
	}
	// Outside a task, the saved Job identifies it; duplicates must be refused.
	h.mkdir(filepath.Join(task, "jobs", "J-2"))
	if code := h.run(h.home, "ports", "claim", "J-2"); code != 0 {
		t.Fatal(h.errb)
	}
	other := filepath.Join(filepath.Dir(task), "T-2-dev")
	h.write(filepath.Join(other, "task.md"), "---\nid: T-02\ntitle: Other\nproject: example\nstate: running\ncreated: 2026-11-08T10:00:00Z\n---\n")
	h.write(filepath.Join(other, "intent.md"), "Other task.\n")
	h.mkdir(filepath.Join(other, "jobs", "J-2"))
	if code := h.run(h.home, "ports", "claim", "J-2"); code != 1 || !strings.Contains(h.errb.String(), "multiple tasks") {
		t.Fatalf("ambiguous: %d %s", code, h.errb)
	}
}
func TestEnvironmentCLI(t *testing.T) {
	h, task, dir := environmentFixture(t)
	binary := filepath.Join(h.root, "server")
	build := exec.Command("go", "build", "-o", binary, "../env/testdata/server")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("helper build: %v %s", err, out)
	}
	// The environment probes the whole block, so one free port is insufficient
	// when other package tests or local servers occupy adjacent ports.
	port := 0
	for i := 0; i < 100; i++ {
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		candidate := min(ln.Addr().(*net.TCPAddr).Port, 65526)
		ln.Close()
		if (ports.Range{Start: candidate, End: candidate + 9}).Available() {
			port = candidate
			break
		}
	}
	if port == 0 {
		t.Fatal("no free ten-port block")
	}
	command := "exec '" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "'"
	h.write(filepath.Join(h.gridHome, "projects", "example", "project.md"), fmt.Sprintf("---\nid: example\nfolders:\n  - machine: %s\n    path: %q\nenvironments:\n  command: %q\n  health_url: http://127.0.0.1:{port}/health\n  timeout: 3s\n  port_range: [%d, %d]\n---\n", h.env.Machine, dir, command, port, port+9))
	t.Cleanup(func() {
		if code := h.run(task, "env", "down", "J-1"); code != 0 {
			t.Error(h.errb)
		}
	})
	if code := h.run(task, "env", "up", "J-1"); code != 0 {
		t.Fatalf("up %d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), fmt.Sprintf("%d-%d", port, port+9)) {
		t.Fatal(h.out)
	}
	if code := h.run(task, "env", "down", "J-1"); code != 0 {
		t.Fatal(h.errb)
	}
	if code := h.run(task, "ports", "list", "--json"); code != 0 || h.out.String() != "[]\n" {
		t.Fatalf("ports remain: %s %s", h.out, h.errb)
	}
}
func TestEnvironmentUsage(t *testing.T) {
	h, task, _ := environmentFixture(t)
	for _, args := range [][]string{{"ports", "unknown"}, {"ports", "claim"}, {"ports", "list", "extra"}, {"ports", "gc", "--json"}, {"env", "unknown"}, {"env", "up"}, {"env", "down", "J-1", "--json"}} {
		if code := h.run(task, args...); code != 2 {
			t.Fatalf("%v: code %d, %s", args, code, h.errb)
		}
	}
	for _, args := range [][]string{{"ports", "--help"}, {"ports", "claim", "--help"}, {"env", "--help"}, {"env", "down", "--help"}} {
		if code := h.run(task, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, h.errb)
		}
	}
}
