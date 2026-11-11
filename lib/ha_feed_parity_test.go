package lib

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// haFdEnviron is the one environment every call in this file takes, lib and binary alike.
func haFdEnviron(t *testing.T, h home, extra ...string) []string {
	t.Helper()
	claude, codex := filepath.Join(h.root, "claude-config"), filepath.Join(h.root, "codex-home")
	for _, d := range []string{claude, codex} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := h.environ("GRID_HOME="+h.grid, "CLAUDE_CONFIG_DIR="+claude, "CODEX_HOME="+codex, "PATH="+os.Getenv("PATH"))
	return append(env, extra...)
}

// haFdResult is what one call answered.
type haFdResult struct {
	code        int
	out, errOut string
}

// haFdLib runs the call through lib.Run.
func haFdLib(h home, dir string, environ []string, args ...string) haFdResult {
	code, out, errs := h.run(dir, environ, args...)
	return haFdResult{code, out, errs}
}

// haFdBin runs the call through the built binary.
func haFdBin(bin string, dir string, environ []string, args ...string) haFdResult {
	code, out, errs := binary(bin, dir, environ, args...)
	return haFdResult{code, out, errs}
}

// haFdSame fails unless both answers are byte-equal; it returns the lib answer.
func haFdSame(t *testing.T, name string, lib, bin haFdResult) haFdResult {
	t.Helper()
	if lib.code != bin.code {
		t.Errorf("%s: exit lib %d, binary %d\nlib:    %q %q\nbinary: %q %q", name, lib.code, bin.code, lib.out, lib.errOut, bin.out, bin.errOut)
	}
	if lib.out != bin.out {
		t.Errorf("%s: stdout differs\nlib:    %q\nbinary: %q", name, lib.out, bin.out)
	}
	if lib.errOut != bin.errOut {
		t.Errorf("%s: stderr differs\nlib:    %q\nbinary: %q", name, lib.errOut, bin.errOut)
	}
	return lib
}

var (
	haFdULID = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)
	haFdTime = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
)

// haFdNormalise replaces what differs between two fresh homes with fixed tokens.
func haFdNormalise(s string, h home) string {
	s = strings.ReplaceAll(s, h.root, "<ROOT>")
	s = haFdTime.ReplaceAllString(s, "<TIME>")
	return haFdULID.ReplaceAllString(s, "<ULID>")
}

// haFdTaskID is the id in the fixture's task.md: the app's worker names the task by it,
// and it selects the task from any folder of the home.
func haFdTaskID(t *testing.T, h home) string {
	t.Helper()
	id, err := feed.TaskID(h.task)
	if err != nil || id == "" {
		t.Fatalf("task id of %s: %q %v", h.task, id, err)
	}
	return id
}

func TestHaFdReadsWithFlags(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Read with flags")
	if code, _, errs := h.post("hello from the app", h.environ()); code != 0 {
		t.Fatalf("post: %d %s", code, errs)
	}
	id := haFdTaskID(t, h)
	env := haFdEnviron(t, h)
	argvs := [][]string{
		{"feed", "state", "--json", "--project", "example", "--task", id}, // app args.go:12
		{"feed", "read", "--json", "--project", "example", "--task", id},  // app args.go:16
		{"feed", "state", "--json", "--task", id},                         // app judge.go:285
		{"feed", "read", "--json", "--task", id},                          // app judge.go:294
	}
	for _, dir := range []string{h.grid, h.task} {
		for _, args := range argvs {
			name := strings.Join(args, " ") + " in " + filepath.Base(dir)
			l := haFdSame(t, name, haFdLib(h, dir, env, args...), haFdBin(bin, dir, env, args...))
			if l.code != 0 {
				t.Errorf("%s: exit %d, want 0: %s", name, l.code, l.errOut)
			}
			if args[1] == "read" && !strings.Contains(l.out, "hello from the app") {
				t.Errorf("%s: the post is not in the answer: %s", name, l.out)
			}
		}
	}
}

func TestHaFdPostWrite(t *testing.T) {
	bin := buildBinary(t)
	hl := newHome(t, "Write through lib")
	hb := newHome(t, "Write through binary")
	body := `{"ref":"task","kind":"message","text":"x"}`
	lid, bid := haFdTaskID(t, hl), haFdTaskID(t, hb)
	lenv, benv := haFdEnviron(t, hl), haFdEnviron(t, hb)
	lib := haFdLib(hl, hl.grid, lenv, "feed", "post", lid, "--type", "post", "--body", body, "--json")
	bres := haFdBin(bin, hb.grid, benv, "feed", "post", bid, "--type", "post", "--body", body, "--json")
	if lib.code != 0 || bres.code != 0 {
		t.Fatalf("post: lib %d %q, binary %d %q", lib.code, lib.errOut, bres.code, bres.errOut)
	}
	if lib.code != bres.code {
		t.Errorf("exit lib %d, binary %d", lib.code, bres.code)
	}
	if l, b := haFdNormalise(lib.out, hl), haFdNormalise(bres.out, hb); l != b {
		t.Errorf("normalised stdout differs\nlib:    %q\nbinary: %q", l, b)
	}
	if l, b := haFdNormalise(lib.errOut, hl), haFdNormalise(bres.errOut, hb); l != b {
		t.Errorf("normalised stderr differs\nlib:    %q\nbinary: %q", l, b)
	}
	// Each record is read back through both paths on its own home.
	for _, c := range []struct {
		name string
		h    home
		env  []string
		id   string
	}{{"written by lib", hl, lenv, lid}, {"written by binary", hb, benv, bid}} {
		read := []string{"feed", "read", "--json", "--task", c.id}
		l := haFdLib(c.h, c.h.grid, c.env, read...)
		b := haFdBin(bin, c.h.grid, c.env, read...)
		haFdSame(t, c.name+": read back", l, b)
		if l.code != 0 || !strings.Contains(l.out, `"text":"x"`) {
			t.Errorf("%s: read back exit %d, the post is missing: %s %s", c.name, l.code, l.out, l.errOut)
		}
	}
}

func TestHaFdRefusals(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Refusals")
	env := haFdEnviron(t, h)
	missing := []string{"feed", "state", "--json", "--task", "T-99"}
	l := haFdSame(t, "missing task", haFdLib(h, h.task, env, missing...), haFdBin(bin, h.task, env, missing...))
	if l.code != 1 || !strings.Contains(l.errOut, "not found") {
		t.Errorf("missing task: exit %d, want 1 not found: %q", l.code, l.errOut)
	}

	stale := haFdEnviron(t, h, "GRID_LEASE=task", "GRID_LEASE_EPOCH=1.1")
	post := []string{"feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"late"}`, "--json"}
	l = haFdSame(t, "stale lease", haFdLib(h, h.task, stale, post...), haFdBin(bin, h.task, stale, post...))
	if l.code != 3 || !strings.Contains(l.errOut, "stale lease") {
		t.Errorf("stale lease: exit %d, want 3 stale lease: %q", l.code, l.errOut)
	}
}

func TestHaFdTypes(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Types")
	env := haFdEnviron(t, h)
	for _, args := range [][]string{{"feed", "types"}, {"feed", "types", "--json"}} {
		name := strings.Join(args, " ")
		l := haFdSame(t, name, haFdLib(h, h.task, env, args...), haFdBin(bin, h.task, env, args...))
		if l.code != 0 || !strings.Contains(l.out, "post") {
			t.Errorf("%s: exit %d, want 0 with the types: %q %q", name, l.code, l.out, l.errOut)
		}
	}
}
