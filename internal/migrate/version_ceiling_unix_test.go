//go:build unix

package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

const pipeText = ": socket, pipe or device file cannot be copied; remove it or move it away"

func mkfifo(t *testing.T, name string) {
	t.Helper()
	if err := syscall.Mkfifo(name, 0600); err != nil {
		t.Skip(err)
	}
}

// pipeLines picks the Unmapped lines about special files.
func pipeLines(p *Plan) []string {
	var out []string
	for _, u := range p.Unmapped {
		if strings.Contains(u, pipeText) {
			out = append(out, u)
		}
	}
	return out
}

func TestNamedPipeInTheSourceBlocksThePlan(t *testing.T) {
	from, home := fixture(t)
	mkfifo(t, filepath.Join(from, "alpha", "projects", "widget", "events.pipe"))
	p := scan(t, from, home)
	want := "alpha/projects/widget/events.pipe" + pipeText
	if len(p.Unmapped) != 1 || p.Unmapped[0] != want {
		t.Fatalf("unmapped %q, want exactly %q", p.Unmapped, want)
	}
	if len(p.Conflicts) != 0 {
		t.Fatalf("conflicts %q", p.Conflicts)
	}
	var b strings.Builder
	if err := p.WriteText(&b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "unmapped: "+want+"\n") || !strings.Contains(b.String(), "0 conflicts, 1 unmapped\n") {
		t.Fatalf("the plan text does not show the blocker:\n%s", b.String())
	}
	if _, err := p.Apply(); err == nil || !strings.HasPrefix(err.Error(), "migration blocked: resolve 0 conflicts and 1 unmapped") {
		t.Fatalf("Apply error = %v, want the blocked plan refusal", err)
	}
	absent(t, home)
}

func TestNamedPipeInTheDestinationBlocksThePlan(t *testing.T) {
	from, home := fixture(t)
	mkfifo(t, filepath.Join(mustDir(t, home), "stray.pipe"))
	p := scan(t, from, home)
	want := "stray.pipe" + pipeText
	if len(p.Unmapped) != 1 || p.Unmapped[0] != want {
		t.Fatalf("unmapped %q, want exactly %q", p.Unmapped, want)
	}
	if _, err := p.Apply(); err == nil || !strings.HasPrefix(err.Error(), "migration blocked") {
		t.Fatalf("Apply error = %v, want the blocked plan refusal", err)
	}
}

func TestNamedPipeIsListedOncePerPath(t *testing.T) {
	// Migrating a home in place makes the source and the destination one folder,
	// so the pipe is in both snapshots under one path.
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, "grid")
	put(t, filepath.Join(home, "projects", "p", "project.md"), "---\nfolder: "+filepath.Join(root, "work")+"\n---\n")
	mkfifo(t, filepath.Join(home, "projects", "p", "events.pipe"))
	mkfifo(t, filepath.Join(home, "other.pipe"))
	p := scan(t, home, home)
	want := []string{"other.pipe" + pipeText, "projects/p/events.pipe" + pipeText}
	got := pipeLines(p)
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("pipe lines %q, want %q", got, want)
	}
	if len(p.Unmapped) != len(want) {
		t.Fatalf("unmapped %q", p.Unmapped)
	}
}

// mustDir makes the folder, and returns it.
func mustDir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
