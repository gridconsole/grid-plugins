package lib

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jzMgRig is an old Grid folder with one project and one task, and three plugin data
// folders that each hold a session pointer naming that task: host (the folder the
// test process's own CLAUDE_PLUGIN_DATA names), call (the folder a call's Environ
// names) and other (the folder a typed --plugin-data names).
type jzMgRig struct {
	root, grid, old, host, call, other string
}

func jzMgWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func jzMgNewRig(t *testing.T) jzMgRig {
	t.Helper()
	root := realPath(t, t.TempDir())
	r := jzMgRig{
		root: root, grid: filepath.Join(root, "home", ".grid"), old: filepath.Join(root, "old"),
		host: filepath.Join(root, "host-data"), call: filepath.Join(root, "call-data"), other: filepath.Join(root, "other-data"),
	}
	if err := os.MkdirAll(filepath.Join(root, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	jzMgWrite(t, filepath.Join(r.old, "projects", "example", "project.md"), "---\nfolder: /example/work/project\n---\n")
	task := filepath.Join(r.old, "projects", "example", "tasks", "T-01-example")
	jzMgWrite(t, filepath.Join(task, "task.md"), "---\nid: T-01\nproject: example\n---\n")
	for _, dir := range []string{r.host, r.call, r.other} {
		jzMgWrite(t, filepath.Join(dir, "sessions", "s1.tasks"), task+"\n")
	}
	// The host process names its own folder; no call below may take it.
	t.Setenv("CLAUDE_PLUGIN_DATA", r.host)
	return r
}

// apply runs `grid migrate --apply` through lib.Run with the given Environ entries
// and extra arguments, and expects it to be applied.
func (r jzMgRig) apply(t *testing.T, environ []string, extra ...string) {
	t.Helper()
	var out, errb strings.Builder
	args := append([]string{"migrate", "--apply", "--from", r.old}, extra...)
	code := Run(Call{
		Args: args, GridHome: r.grid, Dir: r.root,
		Environ: append([]string{"HOME=" + filepath.Join(r.root, "home"), "GRID_MEMBER=member", "GRID_ROLE=owner"}, environ...),
		Stdout:  &out, Stderr: &errb,
	})
	if code != 0 || !strings.Contains(out.String(), "migrate: applied\n") {
		t.Fatalf("migrate %v: exit %d\nstdout: %s\nstderr: %s", args, code, out.String(), errb.String())
	}
}

// manifest is the plugin data part of the one backup the apply made.
func (r jzMgRig) manifest(t *testing.T) (pluginData string, files int) {
	t.Helper()
	entries, err := os.ReadDir(r.grid + "-backup")
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups: %v %v", entries, err)
	}
	b, err := os.ReadFile(filepath.Join(r.grid+"-backup", entries[0].Name(), "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man struct {
		PluginData  string                     `json:"plugin_data"`
		PluginFiles map[string]json.RawMessage `json:"plugin_files"`
	}
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	return man.PluginData, len(man.PluginFiles)
}

// jzMgTree lists a folder as one line per path: type, permissions and the bytes of a file.
func jzMgTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			out[rel] = fmt.Sprintf("dir %o", info.Mode().Perm())
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = fmt.Sprintf("file %o %q", info.Mode().Perm(), b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func jzMgSame(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// pointer is the text of a plugin data folder's session pointer.
func (r jzMgRig) pointer(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "sessions", "s1.tasks"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (r jzMgRig) oldPointer() string {
	return filepath.Join(r.old, "projects", "example", "tasks", "T-01-example") + "\n"
}

func (r jzMgRig) newPointer() string {
	return filepath.Join(r.grid, "projects", "example", "tasks", "T-01-example") + "\n"
}

// The test process names a folder in CLAUDE_PLUGIN_DATA; the call's Environ names
// another. The call's folder is the plugin data of the migration, and the host's
// folder is not touched.
func TestJzMgLibMigrateTakesPluginDataFromCallEnviron(t *testing.T) {
	r := jzMgNewRig(t)
	hostBefore := jzMgTree(t, r.host)
	r.apply(t, []string{"CLAUDE_PLUGIN_DATA=" + r.call})

	data, files := r.manifest(t)
	if want := filepath.Join(r.call, "sessions"); data != want || files == 0 {
		t.Fatalf("the manifest records plugin data %q with %d files, want %q", data, files, want)
	}
	if got := r.pointer(t, r.call); got != r.newPointer() {
		t.Fatalf("the call's pointer is %q, want %q", got, r.newPointer())
	}
	if !jzMgSame(hostBefore, jzMgTree(t, r.host)) {
		t.Fatal("the host's plugin data folder changed")
	}
	if got := r.pointer(t, r.host); got != r.oldPointer() {
		t.Fatalf("the host's pointer is %q, want %q", got, r.oldPointer())
	}
}

// A typed --plugin-data wins over the Environ value, and neither the Environ's folder
// nor the host's is touched.
func TestJzMgLibMigratePluginDataFlagWinsOverEnviron(t *testing.T) {
	r := jzMgNewRig(t)
	hostBefore, callBefore := jzMgTree(t, r.host), jzMgTree(t, r.call)
	r.apply(t, []string{"CLAUDE_PLUGIN_DATA=" + r.call}, "--plugin-data", r.other)

	data, files := r.manifest(t)
	if want := filepath.Join(r.other, "sessions"); data != want || files == 0 {
		t.Fatalf("the manifest records plugin data %q with %d files, want %q", data, files, want)
	}
	if got := r.pointer(t, r.other); got != r.newPointer() {
		t.Fatalf("the typed folder's pointer is %q, want %q", got, r.newPointer())
	}
	if !jzMgSame(hostBefore, jzMgTree(t, r.host)) || !jzMgSame(callBefore, jzMgTree(t, r.call)) {
		t.Fatal("a folder other than the typed one changed")
	}
}

// A call whose Environ has no CLAUDE_PLUGIN_DATA migrates no plugin data, whatever
// the host process names.
func TestJzMgLibMigrateEnvironWithoutPluginDataTakesNone(t *testing.T) {
	r := jzMgNewRig(t)
	hostBefore := jzMgTree(t, r.host)
	r.apply(t, nil)

	if data, files := r.manifest(t); data != "" || files != 0 {
		t.Fatalf("the manifest records plugin data %q with %d files, want none", data, files)
	}
	if !jzMgSame(hostBefore, jzMgTree(t, r.host)) {
		t.Fatal("the host's plugin data folder changed")
	}
}
