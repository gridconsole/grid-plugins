package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// home is a Grid home made through Run itself: one project and one task.
type home struct {
	root, grid, folder, task string
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// environ is the complete environment of the fixture's member.
func (h home) environ(extra ...string) []string {
	return append([]string{"HOME=" + filepath.Join(h.root, "home"), "GRID_MEMBER=member", "GRID_ROLE=owner"}, extra...)
}

// run makes one call in dir with the fixture's environment and returns the exit code and streams.
func (h home) run(dir string, environ []string, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = Run(Call{Args: args, GridHome: h.grid, Dir: dir, Environ: environ, Stdout: &out, Stderr: &errb})
	return code, out.String(), errb.String()
}

func newHome(t *testing.T, title string) home {
	t.Helper()
	root := realPath(t, t.TempDir())
	// The person's home is a folder of its own: a project folder under it would belong to Personal.
	h := home{root: root, grid: filepath.Join(root, "home", ".grid"), folder: filepath.Join(root, "work")}
	if err := os.MkdirAll(h.folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	testgit.Isolate(t)
	testgit.Init(t, h.folder, "git@example.com:org/app.git", true)
	if code, _, errs := h.run(root, h.environ(), "project", "add", h.folder, "--name", "example"); code != 0 {
		t.Fatalf("project add: %d %s", code, errs)
	}
	code, out, errs := h.run(h.folder, h.environ(), "task", "new", title, "--project", "example", "--json")
	if code != 0 {
		t.Fatalf("task new: %d %s", code, errs)
	}
	var made struct{ ID, Path string }
	if err := json.Unmarshal([]byte(out), &made); err != nil || made.Path == "" {
		t.Fatalf("task new output: %v %s", err, out)
	}
	h.task = made.Path
	return h
}

func (h home) post(text string, environ []string, flags ...string) (int, string, string) {
	body := fmt.Sprintf(`{"ref":"task","kind":"message","text":%q}`, text)
	args := append([]string{"feed", "post", "--type", "post", "--body", body, "--json"}, flags...)
	return h.run(h.task, environ, args...)
}

// buildBinary builds the grid binary from this module once per package run into the folder of TestMain.
func buildBinary(t *testing.T) string {
	t.Helper()
	jmGridOnce.Do(func() {
		jmGridBuilds.Add(1)
		bin := filepath.Join(jmGridDir, "grid")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/grid")
		cmd.Dir = ".."
		if out, err := cmd.CombinedOutput(); err != nil {
			jmGridErr = fmt.Errorf("%v\n%s", err, out)
			return
		}
		jmGridBinary = bin
	})
	if jmGridErr != nil {
		t.Fatalf("go build ./cmd/grid: %v", jmGridErr)
	}
	return jmGridBinary
}

// binary runs the built grid with exactly the given environment and returns what a person would see.
func binary(bin, dir string, environ []string, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, environ, &out, &errb
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return code, out.String(), errb.String()
}

func TestLibFeedPostReadState(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Read the Feed")
	if code, _, errs := h.post("hello from the lib", h.environ()); code != 0 {
		t.Fatalf("post: %d %s", code, errs)
	}
	for _, args := range [][]string{
		{"feed", "read"},
		{"feed", "read", "--json"},
		{"feed", "state", "--json"},
	} {
		code, out, errs := h.run(h.task, h.environ(), args...)
		bcode, bout, berrs := binary(bin, h.task, append(h.environ(), "GRID_HOME="+h.grid), args...)
		if code != 0 || bcode != 0 {
			t.Fatalf("%v: lib %d %s, binary %d %s", args, code, errs, bcode, berrs)
		}
		if out != bout || errs != berrs {
			t.Errorf("%v: lib and binary differ\nlib:    %q\nbinary: %q", args, out, bout)
		}
		if !strings.Contains(out, "hello from the lib") && args[1] == "read" {
			t.Errorf("%v: the post is not in the answer: %s", args, out)
		}
	}
}

// Two calls at once with different Grid homes each see only their own task, while the
// host process's environment points at a third home. Run with -race.
func TestLibParallelHomesDoNotSeeEachOther(t *testing.T) {
	decoy := newHome(t, "Decoy task")
	t.Setenv("GRID_HOME", decoy.grid)
	t.Setenv("GRID_FEED_TASK", decoy.task)
	t.Setenv("GRID_MEMBER", "intruder")
	t.Setenv("GRID_ROLE", "guest")
	homes := map[string]home{"alpha": newHome(t, "Alpha task"), "bravo": newHome(t, "Bravo task")}
	const posts = 8
	var wg sync.WaitGroup
	fail := make(chan string, 8*posts)
	for name, h := range homes {
		other := "bravo"
		if name == "bravo" {
			other = "alpha"
		}
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < posts; i++ {
				if code, _, errs := h.post(fmt.Sprintf("%s-%d", name, i), h.environ()); code != 0 {
					fail <- fmt.Sprintf("%s post %d: %d %s", name, i, code, errs)
				}
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < posts; i++ {
				code, out, errs := h.run(h.task, h.environ(), "feed", "read", "--json")
				if code != 0 {
					fail <- fmt.Sprintf("%s read: %d %s", name, code, errs)
				}
				if strings.Contains(out, other+"-") {
					fail <- fmt.Sprintf("%s saw %s's record: %s", name, other, out)
				}
				code, out, errs = h.run(h.root, h.environ(), "task", "list", "--project", "example")
				if code != 0 || strings.Contains(strings.ToLower(out), other+" task") {
					fail <- fmt.Sprintf("%s task list: %d %s %s", name, code, out, errs)
				}
			}
		}()
	}
	wg.Wait()
	close(fail)
	for msg := range fail {
		t.Error(msg)
	}
	for name, h := range homes {
		_, out, _ := h.run(h.task, h.environ(), "feed", "read", "--json")
		if n := strings.Count(out, name+"-"); n != posts {
			t.Errorf("%s has %d of its %d records: %s", name, n, posts, out)
		}
		_, list, _ := h.run(h.root, h.environ(), "task", "list", "--project", "example")
		if !strings.Contains(strings.ToLower(list), name+" task") {
			t.Errorf("%s does not list its own task: %s", name, list)
		}
	}
	if _, out, _ := decoy.run(decoy.task, decoy.environ(), "feed", "read", "--json"); strings.Contains(out, "alpha-") || strings.Contains(out, "bravo-") {
		t.Errorf("the host environment's home was written: %s", out)
	}
}

type posted struct {
	Post feed.Record `json:"post"`
}

func decodePost(t *testing.T, out string) feed.Record {
	t.Helper()
	var got posted
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("post output: %v %s", err, out)
	}
	return got.Post
}

// The host's own GRID_* values are not the call's: the member, the lease epoch and
// the job come from Environ (or the global flags) alone.
func TestLibHostEnvironmentIsNotRead(t *testing.T) {
	h := newHome(t, "Host environment")
	t.Setenv("GRID_MEMBER", "intruder")
	t.Setenv("GRID_LEASE_EPOCH", "broken")
	t.Setenv("GRID_JOB", "J-wrong")
	t.Setenv("GRID_ROLE", "guest")
	t.Setenv("GRID_HOME", filepath.Join(h.root, "elsewhere"))

	// The right values in Environ win: the writer and the job are the call's, and the
	// epoch is read as 3.4 (a host "broken" would be exit 1, "lease epoch must be gen.fence";
	// 3.4 on a lease that was never taken is exit 3, "stale lease").
	code, out, errs := h.post("with environ", h.environ("GRID_JOB=J-1"))
	if code != 0 {
		t.Fatalf("with Environ: %d %s", code, errs)
	}
	p := decodePost(t, out)
	if p.Author.ID != "member" || p.Job == nil || *p.Job != "J-1" {
		t.Fatalf("Environ values not used: author %q job %v", p.Author.ID, p.Job)
	}
	if code, _, errs = h.post("epoch", h.environ("GRID_LEASE=task", "GRID_LEASE_EPOCH=3.4")); code != 3 || !strings.Contains(errs, "stale lease") {
		t.Fatalf("Environ epoch: exit %d, want 3 stale lease: %s", code, errs)
	}

	// Nil Environ: none of the host's values reach the command. The member comes from a
	// global flag; a leaked GRID_LEASE_EPOCH=broken would fail the call with
	// "lease epoch must be gen.fence", and a leaked GRID_JOB would show on the record.
	code, out, errs = h.run(h.task, nil, "--member", "member", "--role", "owner", "feed", "post", "--type", "post",
		"--body", `{"ref":"task","kind":"message","text":"no environ"}`, "--json")
	if code != 0 {
		t.Fatalf("nil Environ: %d %s", code, errs)
	}
	p = decodePost(t, out)
	if p.Author.ID != "member" || p.Job != nil || p.Gen != 0 || p.Fence != 0 || p.Wake != nil || p.Attempt != nil {
		t.Fatalf("a host value leaked into the record: %+v", p)
	}
}

// The cli.Env a Call builds never falls back to the host process.
func TestLibEnvIsTheCallsAlone(t *testing.T) {
	t.Setenv("GRID_MEMBER", "intruder")
	t.Setenv("GRID_HOME", filepath.Join(t.TempDir(), "host-grid"))
	t.Setenv("TMPDIR", "/host-tmp")

	e := Call{}.env(nil)
	if e.Getenv == nil {
		t.Fatal("Env.Getenv is nil: cli would read the host's environment")
	}
	if got := e.Getenv("GRID_MEMBER"); got != "" {
		t.Errorf("nil Environ shows the host's GRID_MEMBER %q", got)
	}
	if e.GridHome != "" || e.Machine != "" {
		t.Errorf("nil Environ found the host's Grid home %q machine %q", e.GridHome, e.Machine)
	}
	for _, r := range e.TempRoots {
		if r == "/host-tmp" || r == os.TempDir() && r != "/tmp" {
			t.Errorf("a host temp folder is a temp root: %v", e.TempRoots)
		}
	}
	if e.OpenURL == nil {
		t.Fatal("Env.OpenURL is nil: cli would open the system browser")
	}
	if err := e.OpenURL("http://127.0.0.1:1/never"); err != nil {
		t.Errorf("the default OpenURL returned %v", err)
	}
	if e.AppStart != nil {
		t.Error("Env.AppStart is set: the lib would start the app")
	}
	if e.Executable != nil {
		t.Error("Env.Executable is set although the Call has none")
	}

	var opened []string
	e = Call{Executable: "/opt/grid", OpenURL: func(u string) error { opened = append(opened, u); return nil }}.env(nil)
	if err := e.OpenURL("http://example.test/t"); err != nil || len(opened) != 1 || opened[0] != "http://example.test/t" {
		t.Errorf("the Call's OpenURL is not the Env's: %v %v", opened, err)
	}
	if path, err := e.Executable(); err != nil || path != "/opt/grid" {
		t.Errorf("Executable: %q %v", path, err)
	}

	e = Call{Environ: []string{"GRID_HOME=/a", "TMPDIR=/t", "GRID_HOME=/b", "HOME=/h"}, GridHome: "/field"}.env(nil)
	if e.Getenv("GRID_HOME") != "/field" || e.Getenv("TMPDIR") != "/t" || e.TempRoots[0] != "/t" {
		t.Errorf("Environ and the GridHome field: %q %v", e.Getenv("GRID_HOME"), e.TempRoots)
	}
	e = Call{Environ: []string{"GRID_HOME=/a", "GRID_HOME=/b"}}.env(nil)
	if e.Getenv("GRID_HOME") != "/b" {
		t.Errorf("the last entry of a name should win: %q", e.Getenv("GRID_HOME"))
	}
}

func TestLibExitCodes(t *testing.T) {
	h := newHome(t, "Exit codes")
	cases := []struct {
		name string
		run  func() (int, string, string)
		want int
		text string
	}{
		{"done", func() (int, string, string) { return h.run(h.task, h.environ(), "feed", "state", "--json") }, 0, ""},
		{"usage", func() (int, string, string) { return h.run(h.task, h.environ(), "nonsense") }, 2, "unknown command"},
		{"missing task", func() (int, string, string) {
			return h.run(h.folder, h.environ(), "task", "show", "T-99")
		}, 1, ""},
		{"stale lease", func() (int, string, string) {
			return h.post("late", h.environ("GRID_LEASE=task", "GRID_LEASE_EPOCH=1.1"))
		}, 3, "stale lease"},
	}
	for _, c := range cases {
		code, out, errs := c.run()
		if code != c.want || !strings.Contains(errs, c.text) {
			t.Errorf("%s: exit %d, want %d; stdout %q stderr %q", c.name, code, c.want, out, errs)
		}
	}
}

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) { panic("writer failed") }

func TestLibPanicIsExitTwo(t *testing.T) {
	var errb bytes.Buffer
	code := Run(Call{Args: []string{"version"}, Stdout: panicWriter{}, Stderr: &errb})
	if code != 2 || errb.String() != "grid: internal error\n" {
		t.Fatalf("panic: exit %d stderr %q", code, errb.String())
	}
	// A Stderr that panics too does not escape the call.
	if code := Run(Call{Args: []string{"version"}, Stdout: panicWriter{}, Stderr: panicWriter{}}); code != 2 {
		t.Fatalf("panic with a panicking Stderr: exit %d", code)
	}
	// The same call without the fault is 0, so the 2 above is the writer's.
	if code := Run(Call{Args: []string{"version"}}); code != 0 {
		t.Fatalf("version: exit %d", code)
	}
}
