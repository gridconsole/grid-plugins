package hook

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// pbAdDirect is a brain folder with one note whose name has a space, no task
// folder and no history-log setting: only the Grid config names the brain.
type pbAdDirect struct {
	vault, note string
	o           Options
	store       brain.Store
}

const pbAdNoteRel = "Ideas/My plan.md"

func pbAdDirectSetup(t *testing.T) *pbAdDirect {
	t.Helper()
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, "home")
	grid := filepath.Join(root, "grid")
	vault := filepath.Join(root, "vault")
	note := filepath.Join(vault, filepath.FromSlash(pbAdNoteRel))
	for _, d := range []string{grid, filepath.Dir(note)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(note, []byte("# Plan\n\nfirst version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"GRID_HOME": grid, "XDG_STATE_HOME": filepath.Join(root, "state")}
	getenv := func(k string) string { return env[k] }
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: grid, Brain: vault, Member: "Sam"}}}
	if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, getenv, home), cfg); err != nil {
		t.Fatal(err)
	}
	d := &pbAdDirect{vault: vault, note: note}
	d.o = Options{Resolver: where.Resolver{GridHome: grid, Home: home, Machine: filepath.Join(root, "machine")}, Cwd: root, Getenv: getenv}
	d.store = brain.Store{GridHome: grid}.WithConfig(home, getenv)
	if !d.store.Vault() || d.store.MachineDir == "" {
		t.Fatalf("the fixture is not a vault: %+v", d.store)
	}
	// Grid has seen the first version: it is a known note.
	if _, err := d.store.Adopt(); err != nil {
		t.Fatal(err)
	}
	return d
}

// run drives one tool event through the real hook entry point.
func (d *pbAdDirect) run(t *testing.T, event, tool string) (out, log string, code int) {
	t.Helper()
	b, err := json.Marshal(hookio.Input{ToolName: tool, Cwd: d.o.Cwd, SessionID: "sess-1",
		ToolInput: map[string]any{"file_path": d.note, "old_string": "a", "new_string": "b"}})
	if err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	o := d.o
	o.Event, o.Stdin, o.Stdout, o.Stderr = event, bytes.NewReader(b), &so, &se
	code = Run(o)
	return so.String(), se.String(), code
}

func (d *pbAdDirect) records(t *testing.T) []brain.ChangeRecord {
	t.Helper()
	var recs []brain.ChangeRecord
	root := filepath.Join(d.vault, ".grid", "changes")
	_ = filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(p, ".change") {
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatal(rerr)
		}
		var r brain.ChangeRecord
		if yerr := yaml.Load(raw, &r); yerr != nil {
			t.Fatal(yerr)
		}
		recs = append(recs, r)
		return nil
	})
	return recs
}

func (d *pbAdDirect) find(t *testing.T, status, kind, data string) *brain.ChangeRecord {
	t.Helper()
	for _, r := range d.records(t) {
		if r.Status == status && r.OriginKind == kind && r.Hash == brain.NoteRev([]byte(data)) && r.ID == "Ideas/My plan" {
			return &r
		}
	}
	return nil
}

func (d *pbAdDirect) versionStored(t *testing.T, data string) bool {
	t.Helper()
	p := filepath.Join(d.vault, ".grid", "versions", "home", "Ideas", "My plan", brain.NoteRev([]byte(data))[:16]+".version")
	got, err := os.ReadFile(p)
	return err == nil && string(got) == data
}

func pbAdNoDecision(t *testing.T, what, out string, code int) {
	t.Helper()
	if code != 0 || out != "" {
		t.Fatalf("%s: exit %d, printed %q; want exit 0 and no permission decision", what, code, out)
	}
}

// V19, 4.3: the person's unseen save is versioned before the agent's tool writes,
// the agent's bytes stand and are recorded after, and check finds nothing.
func TestPbAdDirectEditAdoptsBeforeAndRecordsAfter(t *testing.T) {
	d := pbAdDirectSetup(t)
	person := "# Plan\n\nfirst version\n\nthe person added this in Obsidian\n"
	agent := person + "and the agent added this\n"
	if err := os.WriteFile(d.note, []byte(person), 0o644); err != nil {
		t.Fatal(err)
	}
	out, log, code := d.run(t, "pretooluse", "Edit")
	pbAdNoDecision(t, "PreToolUse", out, code)
	if !d.versionStored(t, person) {
		t.Fatalf("the person's bytes are not under .grid/versions/; log %q", log)
	}
	if r := d.find(t, "edited", "person", person); r == nil {
		t.Fatalf("no edited record with origin person: %+v", d.records(t))
	}
	// The agent's tool writes its bytes.
	if err := os.WriteFile(d.note, []byte(agent), 0o644); err != nil {
		t.Fatal(err)
	}
	out, log, code = d.run(t, "posttooluse", "Edit")
	pbAdNoDecision(t, "PostToolUse", out, code)
	if r := d.find(t, "edited", "agent", agent); r == nil || r.OriginID != "sess-1" {
		t.Fatalf("no agent change record for the agent's bytes: %+v; log %q", d.records(t), log)
	}
	res, err := d.store.Check()
	if err != nil || len(res.Findings) != 0 {
		t.Fatalf("grid brain check: %+v, %v", res, err)
	}
	if got, _ := os.ReadFile(d.note); string(got) != agent {
		t.Fatalf("the note is %q, want the agent's bytes", got)
	}
	if _, err := os.Stat(filepath.Join(d.vault, "Proposals")); err == nil {
		t.Fatal("a Proposals folder was written")
	}
	if !d.versionStored(t, person) {
		t.Fatal("the person's version is gone after the agent's write")
	}
}

// An adoption that fails (an unreadable .grid/) is logged and nothing more: the
// hook exits 0 and prints no decision on both events.
func TestPbAdDirectEditAdoptionFailureIsLoggedOnly(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a folder permission does not stop this process here")
	}
	d := pbAdDirectSetup(t)
	if err := os.WriteFile(d.note, []byte("# Plan\n\nchanged in Obsidian\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	grid := filepath.Join(d.vault, ".grid")
	if err := os.Chmod(grid, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(grid, 0o755) })
	out, log, code := d.run(t, "pretooluse", "Edit")
	pbAdNoDecision(t, "PreToolUse with an unreadable .grid/", out, code)
	if !strings.Contains(log, "brain adopt") {
		t.Fatalf("the failed adoption is not in the log: %q", log)
	}
	out, log, code = d.run(t, "posttooluse", "Edit")
	pbAdNoDecision(t, "PostToolUse with an unreadable .grid/", out, code)
	if !strings.Contains(log, "brain record") {
		t.Fatalf("the failed record is not in the log: %q", log)
	}
}

// pbAdLockHolder is the helper process of the lock test: it takes the local brain
// lock, says so on stdout and keeps it until its stdin closes.
func TestPbAdLockHolderHelper(t *testing.T) {
	grid := os.Getenv("PBAD_HOLD_GRID")
	if grid == "" {
		t.Skip("helper process of TestPbAdDirectEditBusyLockSkips")
	}
	s := brain.Store{GridHome: grid, BrainDir: os.Getenv("PBAD_HOLD_BRAIN"), MachineDir: os.Getenv("PBAD_HOLD_MACHINE")}
	unlock, err := s.LockLocal(false)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	os.Stdout.WriteString("locked\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n') // until the parent closes stdin
}

// With another process holding the local lock, the pre-tool hook does not wait:
// it exits 0 at once, prints no decision and logs that it skipped. No timing is
// asserted; the skip line with the lock still held shows the hook returned.
func TestPbAdDirectEditBusyLockSkips(t *testing.T) {
	d := pbAdDirectSetup(t)
	person := "# Plan\n\nchanged in Obsidian while the lock is held\n"
	if err := os.WriteFile(d.note, []byte(person), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPbAdLockHolderHelper$")
	cmd.Env = append(os.Environ(), "PBAD_HOLD_GRID="+d.o.Resolver.GridHome, "PBAD_HOLD_BRAIN="+d.store.BrainDir, "PBAD_HOLD_MACHINE="+d.store.MachineDir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			stdin.Close()
			_ = cmd.Wait()
		}
	}
	t.Cleanup(release)
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		t.Fatalf("the helper did not take the lock: %q %v", line, err)
	}
	out, log, code := d.run(t, "pretooluse", "Edit")
	pbAdNoDecision(t, "PreToolUse with the lock held", out, code)
	if !strings.Contains(log, "brain adopt skipped") {
		t.Fatalf("the skip is not in the log: %q", log)
	}
	if d.versionStored(t, person) {
		t.Fatal("the person's bytes were adopted while another process held the lock")
	}
	out, log, code = d.run(t, "posttooluse", "Edit")
	pbAdNoDecision(t, "PostToolUse with the lock held", out, code)
	if !strings.Contains(log, "brain record skipped") {
		t.Fatalf("the skipped record is not in the log: %q", log)
	}
	// Once the other process lets go, the same hook adopts.
	release()
	out, _, code = d.run(t, "pretooluse", "Edit")
	pbAdNoDecision(t, "PreToolUse after the lock is free", out, code)
	if !d.versionStored(t, person) {
		t.Fatal("the person's bytes were not adopted once the lock was free")
	}
}
