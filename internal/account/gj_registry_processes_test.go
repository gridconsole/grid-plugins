package account

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// These tests run Status in separate processes: the test binary starts itself as
// children (TestGjRegistryChild), the way internal/cli/delivery_test.go does for
// the task writer. The tests in account_test.go call Status one at a time in one
// process; only separate processes show that registry.lock holds between them.

const gjRegPrefix = "GJREG-RESULT "

// gjRegWorld is one temporary machine: HOME, the Grid home inside it, and the bin
// folder that holds the fake provider. Nothing here is a real home.
type gjRegWorld struct {
	root, home, gridHome, bin string
}

// gjRegSpec is one child: register is the `account status` (true) or the
// `account list` (false) call; claudeDir is the child's CLAUDE_CONFIG_DIR.
type gjRegSpec struct {
	register  bool
	claudeDir string
}

// gjRegResult is what one child answered: the line it printed, and how it exited.
type gjRegResult struct {
	IDs    []string `json:"ids"`
	Error  string   `json:"error"`
	code   int
	parsed bool
	out    string
}

func gjRegUnder(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// gjRegNewWorld makes the temporary machine and points this process's own
// HOME, CLAUDE_CONFIG_DIR, CODEX_HOME and GRID_HOME into it before anything runs.
func gjRegNewWorld(t *testing.T) gjRegWorld {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	root := gridhome.Real(t.TempDir())
	w := gjRegWorld{root: root, home: filepath.Join(root, "home"), bin: filepath.Join(root, "bin")}
	w.gridHome = filepath.Join(w.home, ".grid")
	for _, d := range []string{w.home, w.gridHome, w.bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", w.home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("GRID_HOME", w.gridHome)
	t.Setenv("FAKE_HOME", w.home)
	if !gjRegUnder(root, os.Getenv("HOME")) || !gjRegUnder(root, os.Getenv("GRID_HOME")) {
		t.Fatal("HOME or GRID_HOME is not under the temporary root; the test touches no real home")
	}
	installFakeClaude(t, w.bin)
	return w
}

// env is the whole environment of a child: the same keys for every child, and
// no value taken from the parent's own.
func (w gjRegWorld) env(s gjRegSpec) []string {
	register := "0"
	if s.register {
		register = "1"
	}
	return []string{
		"HOME=" + w.home,
		"CLAUDE_CONFIG_DIR=" + s.claudeDir,
		"CODEX_HOME=",
		"GRID_HOME=" + w.gridHome,
		"FAKE_HOME=" + w.home,
		"PATH=" + w.bin,
		"GJ_REGISTRY_CHILD=1",
		"GJ_REGISTRY_REGISTER=" + register,
		"GJ_REGISTRY_ROOT=" + w.root,
	}
}

func gjRegSpecs(n int, register bool) []gjRegSpec {
	out := make([]gjRegSpec, n)
	for i := range out {
		out[i].register = register
	}
	return out
}

func gjRegCodexLogin(t *testing.T, dir, email string) {
	t.Helper()
	write(t, filepath.Join(dir, "auth.json"), `{"tokens":{"id_token":"`+idToken(email)+`"}}`)
}

// TestGjRegistryChild is the helper the tests below start as a process. It does
// nothing unless GJ_REGISTRY_CHILD is set. It waits on stdin, so the parent can
// release every child together, then calls Status once and prints the ids.
func TestGjRegistryChild(t *testing.T) {
	if os.Getenv("GJ_REGISTRY_CHILD") != "1" {
		t.Skip("helper process of the TestGjRegistry tests")
	}
	root, home, gridHome := os.Getenv("GJ_REGISTRY_ROOT"), os.Getenv("HOME"), os.Getenv("GRID_HOME")
	if !gjRegUnder(root, home) || !gjRegUnder(root, gridHome) {
		t.Fatal("the child's HOME or GRID_HOME is not under the temporary root")
	}
	_, _ = io.Copy(io.Discard, os.Stdin) // start barrier: returns when the parent closes the pipe
	env := Env{Home: home, GridHome: gridHome, Now: func() time.Time { return fixedNow }}
	accounts, err := Status(env, os.Getenv("GJ_REGISTRY_REGISTER") == "1")
	res := gjRegResult{IDs: []string{}}
	for _, a := range accounts {
		res.IDs = append(res.IDs, a.ID)
	}
	if err != nil {
		res.Error = err.Error()
	}
	line, mErr := json.Marshal(res)
	if mErr != nil {
		t.Fatal(mErr)
	}
	fmt.Printf("%s%s\n", gjRegPrefix, line)
	if err != nil {
		t.Fatal(err)
	}
}

// gjRegRun starts every child before it waits on any, releases them together,
// and answers one result per spec in spec order. A child that was started never
// outlives the test.
func gjRegRun(t *testing.T, w gjRegWorld, specs []gjRegSpec) []gjRegResult {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	n := len(specs)
	cmds := make([]*exec.Cmd, n)
	pipes := make([]io.WriteCloser, n)
	outs := make([]*bytes.Buffer, n)
	started := 0
	t.Cleanup(func() {
		for i := 0; i < started; i++ {
			_ = pipes[i].Close()
			_ = cmds[i].Process.Kill()
		}
	})
	for i, s := range specs {
		outs[i] = &bytes.Buffer{}
		cmds[i] = exec.Command(exe, "-test.run=^TestGjRegistryChild$", "-test.count=1")
		cmds[i].Env = w.env(s)
		cmds[i].Stdout, cmds[i].Stderr = outs[i], outs[i]
		if pipes[i], err = cmds[i].StdinPipe(); err != nil {
			t.Fatal(err)
		}
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
		started++
	}
	for _, p := range pipes { // every Start returned: release them all
		_ = p.Close()
	}
	results := make([]gjRegResult, n)
	for i, cmd := range cmds {
		r := &results[i]
		if err := cmd.Wait(); err != nil {
			var ee *exec.ExitError
			if !errors.As(err, &ee) {
				t.Fatal(err)
			}
			r.code = ee.ExitCode()
		}
		r.out = outs[i].String()
		for _, line := range strings.Split(r.out, "\n") {
			if rest, ok := strings.CutPrefix(line, gjRegPrefix); ok {
				r.parsed = json.Unmarshal([]byte(rest), r) == nil
			}
		}
	}
	return results
}

// gjRegCheck holds on every child's exit code, on the line it printed and on the
// error it carried. It prints the child's own output, which names paths and ids.
func gjRegCheck(t *testing.T, label string, rs []gjRegResult) {
	t.Helper()
	for i, r := range rs {
		switch {
		case r.code != 0:
			t.Errorf("%s %d exited %d:\n%s", label, i, r.code, r.out)
		case !r.parsed:
			t.Errorf("%s %d printed no result line:\n%s", label, i, r.out)
		case r.Error != "":
			t.Errorf("%s %d answered an error: %s", label, i, r.Error)
		}
	}
}

func gjRegRecords(t *testing.T, w gjRegWorld) []Record {
	t.Helper()
	recs, err := loadRecords(Env{GridHome: w.gridHome})
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func gjRegSorted(ids []string) []string {
	out := append([]string{}, ids...)
	sort.Strings(out)
	return out
}

func gjRegRecordIDs(recs []Record) []string {
	ids := make([]string, 0, len(recs))
	for _, r := range recs {
		ids = append(ids, r.ID)
	}
	return gjRegSorted(ids)
}

func gjRegSame(a, b []string) bool { return strings.Join(a, "\x00") == strings.Join(b, "\x00") }

// gjRegNoDouble fails when a folder of the form ~/.<kind>-<name>-<n> exists: the
// second folder a doubled registration would have made.
func gjRegNoDouble(t *testing.T, w gjRegWorld) {
	t.Helper()
	entries, err := os.ReadDir(w.home)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, "-2") && (strings.HasPrefix(n, ".claude-") || strings.HasPrefix(n, ".codex-")) {
			t.Errorf("a doubled folder exists: %s", n)
		}
	}
}

func gjRegExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestGjRegistryTwoProcessesOneEmail(t *testing.T) {
	w := gjRegNewWorld(t)
	login(t, filepath.Join(w.home, ".claude"), "ann@example.com")

	rs := gjRegRun(t, w, gjRegSpecs(2, true))
	gjRegCheck(t, "child", rs)

	recs := gjRegRecords(t, w)
	if len(recs) != 1 || recs[0].Email != "ann@example.com" || recs[0].ID != "claude-ann" || recs[0].Dir != "~/.claude-ann" {
		t.Fatalf("want one record claude-ann, got ids %v", gjRegRecordIDs(recs))
	}
	if !isDir(filepath.Join(w.home, ".claude-ann")) {
		t.Error("~/.claude-ann does not exist")
	}
	if gjRegExists(filepath.Join(w.home, ".claude-ann-2")) {
		t.Error("~/.claude-ann-2 exists: the email was registered twice")
	}
	for i, r := range rs {
		if !gjRegSame(r.IDs, []string{"claude-ann"}) {
			t.Errorf("child %d printed ids %v, want [claude-ann]", i, r.IDs)
		}
	}
}

func TestGjRegistryManyProcessesManyEmails(t *testing.T) {
	w := gjRegNewWorld(t)
	login(t, filepath.Join(w.home, ".claude"), "ann@example.com")
	gjRegCodexLogin(t, filepath.Join(w.home, ".codex-one"), "bob@example.com")
	gjRegCodexLogin(t, filepath.Join(w.home, ".codex-two"), "cat@example.com")
	login(t, filepath.Join(w.home, ".claude-dan"), "dan@example.com")

	rs := gjRegRun(t, w, gjRegSpecs(6, true))
	gjRegCheck(t, "child", rs)

	recs := gjRegRecords(t, w)
	emails, ids, dirs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, r := range recs {
		emails[strings.ToLower(r.Email)] = true
		ids[r.ID] = true
		dirs[r.Dir] = true
	}
	if len(recs) != 4 || len(emails) != 4 || len(ids) != 4 || len(dirs) != 4 {
		t.Fatalf("want 4 records with distinct emails, ids and dirs; got %d records, ids %v, %d emails, %d dirs", len(recs), gjRegRecordIDs(recs), len(emails), len(dirs))
	}
	want := []string{"claude-ann", "claude-dan", "codex-bob", "codex-cat"}
	if got := gjRegRecordIDs(recs); !gjRegSame(got, want) {
		t.Errorf("ids %v, want %v", got, want)
	}
	for i, e := range []string{"ann@example.com", "bob@example.com", "cat@example.com", "dan@example.com"} {
		if !emails[e] {
			t.Errorf("email number %d of the fixture has no record", i)
		}
	}
	for i, r := range rs {
		if got := gjRegSorted(r.IDs); !gjRegSame(got, want) {
			t.Errorf("child %d printed ids %v, want %v", i, got, want)
		}
	}
	gjRegNoDouble(t, w)

	// The rule of TestStatusRegistersAnEmailOnceWithItsOwnFolder: a later run
	// finds every email known and writes nothing.
	path := filepath.Join(w.gridHome, "accounts", "registry.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gjRegCheck(t, "second child", gjRegRun(t, w, gjRegSpecs(1, true)))
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("a later run changed registry.json:\n%s\n%s", before, after)
	}
}

func TestGjRegistryReaderWhileWriting(t *testing.T) {
	for round := 1; round <= 5; round++ {
		t.Run(fmt.Sprintf("round%d", round), func(t *testing.T) {
			w := gjRegNewWorld(t)
			login(t, filepath.Join(w.home, ".claude"), "ann@example.com")
			gjRegCodexLogin(t, filepath.Join(w.home, ".codex-one"), "bob@example.com")
			login(t, filepath.Join(w.home, ".claude-dan"), "dan@example.com")

			// Four `account status` and four `account list` calls, interleaved.
			var specs []gjRegSpec
			for i := 0; i < 4; i++ {
				specs = append(specs, gjRegSpec{register: true}, gjRegSpec{register: false})
			}
			rs := gjRegRun(t, w, specs)
			gjRegCheck(t, "child", rs)

			want := []string{"claude-ann", "claude-dan", "codex-bob"}
			if got := gjRegRecordIDs(gjRegRecords(t, w)); !gjRegSame(got, want) {
				t.Fatalf("registry ids %v, want %v", got, want)
			}
			for i, r := range rs {
				got := gjRegSorted(r.IDs)
				switch {
				case strings.Contains(r.out, "is not readable JSON"):
					t.Errorf("child %d read a torn registry.json", i)
				case specs[i].register && !gjRegSame(got, want):
					t.Errorf("writer %d printed ids %v, want %v", i, got, want)
				case !specs[i].register && len(got) != 0 && !gjRegSame(got, want):
					t.Errorf("reader %d saw a part of the list: %v", i, got) // the file is replaced whole
				}
			}
			left, err := filepath.Glob(filepath.Join(w.gridHome, "accounts", ".registry.json.*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(left) != 0 {
				t.Errorf("temp files left in accounts: %d", len(left))
			}
		})
	}
}

// Each child sees its own extra folder (its CLAUDE_CONFIG_DIR) beside the shared
// ones, so each has an email only it can register. A process that wrote its own
// list over another's would lose one of them.
func TestGjRegistryEachProcessBringsItsOwnEmail(t *testing.T) {
	const n = 6
	w := gjRegNewWorld(t)
	login(t, filepath.Join(w.home, ".claude"), "ann@example.com")
	specs := gjRegSpecs(n, true)
	want := []string{"claude-ann"}
	for i := range specs {
		dir := filepath.Join(w.home, fmt.Sprintf("extra%d", i))
		login(t, dir, fmt.Sprintf("p%d@example.com", i))
		specs[i].claudeDir = dir
		want = append(want, fmt.Sprintf("claude-p%d", i))
	}
	want = gjRegSorted(want)

	rs := gjRegRun(t, w, specs)
	gjRegCheck(t, "child", rs)

	if got := gjRegRecordIDs(gjRegRecords(t, w)); !gjRegSame(got, want) {
		t.Fatalf("an account was lost or doubled: registry ids %v, want %v", got, want)
	}
	for i, r := range rs {
		own, has := fmt.Sprintf("claude-p%d", i), false
		for _, id := range r.IDs {
			has = has || id == own
		}
		if !has {
			t.Errorf("child %d did not answer its own account %s: %v", i, own, r.IDs)
		}
	}
	gjRegNoDouble(t, w)
}
