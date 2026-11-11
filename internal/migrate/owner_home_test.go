package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// ownerHome is a home shaped like the owner's: two projects, one task folder with
// no task.md, notes/, knowledge/ with a brain index, and feed files.
const ownerHome = "testdata/owner-home"

const skippedTask = "projects/dw6/tasks/T-346-sms-unsent-report"

// copyOwnerHome copies the owner-shaped home, empty folders too, to a new place.
func copyOwnerHome(t *testing.T, to string) {
	t.Helper()
	err := filepath.WalkDir(ownerHome, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(ownerHome, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0700)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		put(t, filepath.Join(to, rel), string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// writeMarks is view plus the modification time of every file, so a file written
// again with the same bytes still shows.
func writeMarks(t *testing.T, root string) map[string]string {
	t.Helper()
	out := view(t, root)
	s, err := snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	for rel, e := range s.m {
		if e.Mode.IsRegular() {
			out[rel] += fmt.Sprintf(" mtime=%d", e.ModTime)
		}
	}
	return out
}

// diffViews names what differs between two views, for a failure message.
func diffViews(a, b map[string]string) string {
	var out []string
	for k, v := range a {
		if w, ok := b[k]; !ok {
			out = append(out, "gone: "+k)
		} else if v != w {
			out = append(out, "changed: "+k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			out = append(out, "extra: "+k)
		}
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}

// oldTasks reads the task.md files of a home as the old tool left them: folder, as
// projects/<project>/tasks/<folder>, to the id in the file.
func oldTasks(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for rel, e := range view(t, root) {
		parts := strings.Split(rel, "/")
		if len(parts) == 5 && parts[0] == "projects" && parts[2] == "tasks" && parts[4] == "task.md" && strings.HasPrefix(e, "file") {
			r, err := parseRecord([]byte(read(t, root, rel)))
			if err != nil {
				t.Fatal(err)
			}
			out[path.Dir(rel)] = r.value("id")
		}
	}
	return out
}

func ownerPlan(t *testing.T, from, home string) *Plan {
	t.Helper()
	t.Setenv("CLAUDE_PLUGIN_DATA", "")
	return scan(t, from, home)
}

// A task folder with no task.md is listed as skipped, and does not stop apply.
func TestTaskFolderWithoutRecordIsSkipped(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	from, home := filepath.Join(root, "owner"), filepath.Join(root, ".grid")
	copyOwnerHome(t, from)
	p := ownerPlan(t, from, home)
	if len(p.Unmapped) != 0 || len(p.Conflicts) != 0 {
		t.Fatalf("blocked: unmapped %q conflicts %q", p.Unmapped, p.Conflicts)
	}
	if len(p.Skipped) != 1 || p.Skipped[0] != skippedTask {
		t.Fatalf("skipped %q", p.Skipped)
	}
	var out bytes.Buffer
	if err := p.WriteText(&out); err != nil {
		t.Fatal(err)
	}
	var skips []string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "skip: ") {
			skips = append(skips, line)
		}
	}
	want := "skip: " + skippedTask + ": no task.md; kept in place, copied untouched"
	if len(skips) != 1 || skips[0] != want {
		t.Fatalf("skip lines %q, want %q\n%s", skips, want, out.String())
	}
	if !strings.Contains(out.String(), "0 conflicts, 0 unmapped") {
		t.Fatal(out.String())
	}
	if _, err := p.Apply(); err != nil {
		t.Fatalf("apply after a skip: %v", err)
	}
}

// Only a task folder is skipped. A file where a task folder should be, and a
// task.md that cannot be read, still block apply.
func TestOtherUnmappedKindsStillBlock(t *testing.T) {
	for name, build := range map[string]func(root string){
		"stray file under tasks": func(root string) {
			put(t, filepath.Join(root, "projects/dw6/tasks/README.md"), "not a task\n")
		},
		"task.md with no frontmatter": func(root string) {
			put(t, filepath.Join(root, "projects/dw6/tasks/T-345-sms-sent/task.md"), "not frontmatter")
		},
		"project without project.md": func(root string) {
			if err := os.Remove(filepath.Join(root, "projects/dw6/project.md")); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := gridhome.Real(t.TempDir())
			from, home := filepath.Join(root, "owner"), filepath.Join(root, ".grid")
			copyOwnerHome(t, from)
			build(from)
			p := ownerPlan(t, from, home)
			if len(p.Unmapped) != 1 {
				t.Fatalf("unmapped %q", p.Unmapped)
			}
			if _, err := p.Apply(); err == nil || !strings.Contains(err.Error(), "migration blocked") {
				t.Fatalf("apply: %v", err)
			}
		})
	}
}

// The owner's home, copied, migrates: every task has a ULID id, every old id is in
// the id map, the folder with no task.md is copied untouched, and no file under the
// source is written.
func TestOwnerHomeMigratesOnACopy(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	from, home := filepath.Join(root, "owner"), filepath.Join(root, ".grid")
	copyOwnerHome(t, from)
	before := writeMarks(t, from)
	tasks := oldTasks(t, from)
	if len(tasks) != 2 {
		t.Fatalf("fixture has %d tasks with a task.md: %v", len(tasks), tasks)
	}

	p := ownerPlan(t, from, home)
	res, err := p.Apply()
	if err != nil || !res.Changed {
		t.Fatalf("apply: %+v %v", res, err)
	}

	if after := writeMarks(t, from); !sameView(before, after) {
		t.Fatalf("the source was written:\n%s", diffViews(before, after))
	}

	// migration.log carries the id map.
	var man manifest
	if err := json.Unmarshal([]byte(read(t, home, "migration.log")), &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Skipped) != 1 || man.Skipped[0] != skippedTask {
		t.Fatalf("migration.log skipped %q", man.Skipped)
	}
	mapped := map[string]IDMapping{}
	for _, m := range man.IDs {
		mapped[m.Kind+" "+m.Path] = m
	}
	for folder, old := range tasks {
		r, err := parseRecord([]byte(read(t, home, path.Join(folder, "task.md"))))
		if err != nil {
			t.Fatal(err)
		}
		id := r.value("id")
		if !ulidRE.MatchString(id) {
			t.Errorf("%s: id %q is not a ULID", folder, id)
		}
		if r.value("display") != old {
			t.Errorf("%s: display %q, want the old id %q", folder, r.value("display"), old)
		}
		m, ok := mapped["task "+folder]
		if !ok || m.Old != old || m.New != id {
			t.Errorf("%s: id map has %+v, want %s -> %s", folder, m, old, id)
		}
	}
	for _, project := range []string{"dw6", "grid"} {
		m, ok := mapped["project projects/"+project]
		if !ok || m.Old != project || !ulidRE.MatchString(m.New) {
			t.Errorf("project %s: id map has %+v", project, m)
		}
	}
	if len(man.IDs) != 4 {
		t.Errorf("id map has %d entries, want 4: %+v", len(man.IDs), man.IDs)
	}

	// The folder with no task.md, and the rest of the home, arrived as it was.
	absent(t, filepath.Join(home, skippedTask, "task.md"))
	for rel, e := range view(t, from) {
		if rel == "." || !strings.HasPrefix(e, "file") || strings.HasSuffix(rel, "task.md") || strings.HasSuffix(rel, "project.md") {
			continue
		}
		if got := read(t, home, rel); got != read(t, from, rel) {
			t.Errorf("%s changed in the copy", rel)
		}
	}
}

// Migrating the owner's home where it stands, then rolling back, gives the old tree
// again, byte for byte: every path, mode and checksum.
func TestOwnerHomeRollbackRestoresTheTree(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, ".grid")
	copyOwnerHome(t, home)
	before := view(t, home)

	res, err := ownerPlan(t, home, home).Apply()
	if err != nil || !res.Changed {
		t.Fatalf("apply: %+v %v", res, err)
	}
	if sameView(before, view(t, home)) {
		t.Fatal("apply left the tree as it was")
	}
	if _, err := Rollback(home); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if after := view(t, home); !sameView(before, after) {
		t.Fatalf("rollback did not restore the tree:\n%s", diffViews(before, after))
	}
}

// The same on a copy kept apart from the home: the source is never written, and a
// rollback leaves it as it was.
func TestOwnerHomeRollbackFromACopyLeavesTheSource(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	from, home := filepath.Join(root, "owner"), filepath.Join(root, ".grid")
	copyOwnerHome(t, from)
	before := writeMarks(t, from)
	if res, err := ownerPlan(t, from, home).Apply(); err != nil || !res.Changed {
		t.Fatalf("apply: %+v %v", res, err)
	}
	if _, err := Rollback(home); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if after := writeMarks(t, from); !sameView(before, after) {
		t.Fatalf("the source changed:\n%s", diffViews(before, after))
	}
	if _, err := os.Stat(filepath.Join(home, "VERSION")); !os.IsNotExist(err) {
		t.Fatalf("rollback left a migrated home (%v)", err)
	}
}

// A second --apply finds the home current: nothing is written, no backup is made.
func TestOwnerHomeApplyTwiceIsANoOp(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, ".grid")
	copyOwnerHome(t, home)

	first := ownerPlan(t, home, home)
	if res, err := first.Apply(); err != nil || !res.Changed {
		t.Fatalf("first apply: %+v %v", res, err)
	}
	homeAfter := writeMarks(t, home)
	siblingsAfter := siblings(t, home)
	backups := writeMarks(t, home+"-backup")

	second := ownerPlan(t, home, home)
	if !second.Current {
		t.Fatal("the migrated home is not current")
	}
	res, err := second.Apply()
	if err != nil || res.Changed || res.Backup != "" {
		t.Fatalf("second apply: %+v %v", res, err)
	}
	// The plan that made the first run, applied again, is also a no-op.
	if res, err = first.Apply(); err != nil || res.Changed {
		t.Fatalf("first plan applied again: %+v %v", res, err)
	}
	if now := writeMarks(t, home); !sameView(homeAfter, now) {
		t.Fatalf("the second apply wrote the home:\n%s", diffViews(homeAfter, now))
	}
	if now := writeMarks(t, home+"-backup"); !sameView(backups, now) {
		t.Fatalf("the second apply wrote a backup:\n%s", diffViews(backups, now))
	}
	if now := siblings(t, home); strings.Join(now, ",") != strings.Join(siblingsAfter, ",") {
		t.Fatalf("siblings changed: %v -> %v", siblingsAfter, now)
	}
}
