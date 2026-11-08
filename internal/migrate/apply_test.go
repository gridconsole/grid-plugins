package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestApplyTwice(t *testing.T) {
	for _, layout := range []string{"separate", "in-place", "single-home", "nested-source"} {
		t.Run(layout, func(t *testing.T) {
			from, home := fixture(t)
			switch layout {
			case "in-place":
				home = from
			case "single-home":
				from = filepath.Join(from, "alpha")
			case "nested-source":
				if err := os.RemoveAll(filepath.Join(from, "beta")); err != nil {
					t.Fatal(err)
				}
				home = from
				from = filepath.Join(from, "alpha")
			}
			before := view(t, from)
			p := scan(t, from, home)
			res, err := p.Apply()
			if err != nil {
				t.Fatal(err)
			}
			if !res.Changed || res.Backup == "" {
				t.Fatal("missing apply result")
			}
			if !sameView(before, view(t, filepath.Join(res.Backup, "source"))) {
				t.Fatal("backup differs from original")
			}
			got := view(t, home)
			if read(t, home, "VERSION") != "1\n" {
				t.Fatal("missing format marker")
			}
			machine := gridhome.Machine(home)
			if !ulidRE.MatchString(machine) {
				t.Fatalf("missing machine id: %q", machine)
			}
			for _, a := range p.Actions {
				if filepath.Base(a.From) == "history.md" || strings.HasSuffix(a.From, ".jsonl") || strings.HasSuffix(a.From, ".usage") || strings.HasSuffix(a.From, ".recalled") {
					if before[a.From] != got[a.To] {
						t.Errorf("changed archived content: %s", a.From)
					}
				}
				if a.Kind != "identify" {
					continue
				}
				if strings.HasSuffix(a.To, "/project.md") {
					rec, err := project.Parse(filepath.Base(filepath.Dir(a.To)), read(t, home, a.To))
					if err != nil {
						t.Fatalf("%s: %v", a.To, err)
					}
					if !ulidRE.MatchString(rec.ID) || len(rec.FoldersOn(machine)) != 1 {
						t.Errorf("%s: id %q, folders on this machine %v", a.To, rec.ID, rec.FoldersOn(machine))
					}
					continue
				}
				r, err := parseRecord([]byte(read(t, home, a.To)))
				if err != nil {
					t.Fatal(err)
				}
				if !ulidRE.MatchString(r.value("id")) || !taskRE.MatchString(r.value("display")) {
					t.Errorf("%s: id %q, display %q", a.To, r.value("id"), r.value("display"))
				}
			}
			task := filepath.Join(home, "projects/widget/tasks/T-07-first")
			if read(t, home, "sessions/session-a") != task+"\n" {
				t.Fatal("pointer not remapped")
			}
			var session map[string]any
			if err := json.Unmarshal([]byte(read(t, home, "projects/widget/tasks/T-07-first/sessions/session-a.json")), &session); err != nil {
				t.Fatal(err)
			}
			if session["cwd"] != filepath.Join(home, "projects/widget/worktrees/T-07/worker-a") {
				t.Fatal("session cwd not remapped")
			}
			var log manifest
			if err := json.Unmarshal([]byte(read(t, home, "migration.log")), &log); err != nil {
				t.Fatal(err)
			}
			if log.Backup != res.Backup || len(log.SourceFiles) != len(before) {
				t.Fatal("incomplete migration log")
			}
			backupAll := view(t, res.Backup)
			again, err := scan(t, from, home).Apply()
			if err != nil || again.Changed || again.Backup != "" {
				t.Fatalf("second apply: %+v %v", again, err)
			}
			// Applying the same plan object again is also safe.
			if again, err = p.Apply(); err != nil || again.Changed {
				t.Fatalf("repeat plan apply: %+v %v", again, err)
			}
			if !sameView(got, view(t, home)) {
				t.Fatal("second apply changed the home")
			}
			if !sameView(backupAll, view(t, res.Backup)) {
				t.Fatal("second apply changed the backup")
			}
			entries, err := os.ReadDir(home + "-backup")
			if err != nil || len(entries) != 1 {
				t.Fatal("second apply created another backup")
			}
			if layout == "separate" || layout == "single-home" {
				if !sameView(before, view(t, from)) {
					t.Fatal("external source changed")
				}
			}
			if layout == "in-place" {
				absent(t, filepath.Join(home, "alpha"))
			}
			for _, name := range []string{".migrate-lock", ".migrate-journal"} {
				absent(t, home+name)
			}
			for _, s := range siblings(t, home) {
				if strings.Contains(s, ".migrate-") {
					t.Errorf("left behind: %s", s)
				}
			}
		})
	}
}

func TestApplyRefusalsLeaveDataUntouched(t *testing.T) {
	for _, kind := range []string{"unmapped", "conflict", "stale-source", "stale-home", "backup-file", "backup-symlink", "locked"} {
		t.Run(kind, func(t *testing.T) {
			from, home := fixture(t)
			put(t, filepath.Join(home, "machine"), "existing-machine\n")
			if kind == "unmapped" {
				put(t, filepath.Join(from, "unknown.bin"), "unknown")
			}
			if kind == "conflict" {
				put(t, filepath.Join(home, "sessions/session-a"), "other")
			}
			p := scan(t, from, home)
			switch kind {
			case "stale-source":
				put(t, filepath.Join(from, "changed.txt"), "change")
			case "stale-home":
				put(t, filepath.Join(home, "changed.txt"), "change")
			case "backup-file":
				put(t, home+"-backup", "blocked")
			case "backup-symlink":
				if err := os.Symlink(t.TempDir(), home+"-backup"); err != nil {
					t.Skip(err)
				}
			case "locked":
				// A lock held by a running process, here this one.
				if _, err := acquireLock(lockPath(home)); err != nil {
					t.Fatal(err)
				}
			}
			beforeSrc, beforeHome := view(t, from), view(t, home)
			if _, err := p.Apply(); err == nil {
				t.Fatal("apply should refuse")
			}
			if !sameView(beforeSrc, view(t, from)) || !sameView(beforeHome, view(t, home)) {
				t.Fatal("refusal changed data")
			}
			absent(t, filepath.Join(home, "VERSION"))
			absent(t, journalPath(home))
		})
	}
}

func TestExistingMachineAndDestinationAreBackedUp(t *testing.T) {
	from, home := fixture(t)
	put(t, filepath.Join(home, "machine"), "existing-machine\n")
	put(t, filepath.Join(home, "settings.json"), "{}\n")
	before := view(t, home)
	res, err := scan(t, from, home).Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !sameView(before, view(t, filepath.Join(res.Backup, "home"))) {
		t.Fatal("existing home backup differs")
	}
	after := view(t, home)
	for _, f := range []string{"machine", "settings.json"} {
		if after[f] != before[f] {
			t.Fatalf("existing home data lost: %s", f)
		}
	}
	// The records carry the machine that was already there.
	rec, err := project.Parse("widget", read(t, home, "projects/widget/project.md"))
	if err != nil || len(rec.FoldersOn("existing-machine")) != 1 {
		t.Fatalf("record %+v: %v", rec, err)
	}
}

func TestCurrentVersionDoesNotInspectOtherFiles(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, ".grid")
	put(t, filepath.Join(home, "VERSION"), "1\n")
	if err := os.Symlink("missing", filepath.Join(home, "unrelated-link")); err != nil {
		t.Skip(err)
	}
	p := scan(t, filepath.Join(root, "missing-source"), home)
	if !p.Current {
		t.Fatal("not current")
	}
	if _, err := p.Apply(); err != nil {
		t.Fatal(err)
	}
}

func TestFlatInPlaceKeepsPointersAndMachine(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	put(t, filepath.Join(root, "projects/example/project.md"), "---\nfolder: /example/work/project\n---\n")
	task := filepath.Join(root, "projects/example/tasks/T-01-example")
	put(t, filepath.Join(task, "task.md"), "---\nid: T-01\n---\n")
	put(t, filepath.Join(root, "sessions/session-a"), task+"\n")
	put(t, filepath.Join(root, "machine"), "existing-machine\n")
	if _, err := scan(t, root, root).Apply(); err != nil {
		t.Fatal(err)
	}
	if read(t, root, "sessions/session-a") != task+"\n" || read(t, root, "machine") != "existing-machine\n" {
		t.Fatal("in-place data changed")
	}
}

// A new machine id is made once, at apply, and every record carries it.
func TestNewMachineIDIsInEveryRecord(t *testing.T) {
	from, home := fixture(t)
	p := scan(t, from, home)
	if !strings.Contains(string(p.result["projects/widget/project.md"].Data), pendingMachine) {
		t.Fatal("the plan should show a placeholder for the machine id")
	}
	if _, err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	machine := gridhome.Machine(home)
	if !ulidRE.MatchString(machine) {
		t.Fatalf("machine id %q", machine)
	}
	recs, err := project.Load(home)
	if err != nil || len(recs) != 3 {
		t.Fatalf("project.Load: %d records, %v", len(recs), err)
	}
	for _, r := range recs {
		if len(r.FoldersOn(machine)) != 1 {
			t.Errorf("%s has no folder on this machine: %+v", r.Name, r.Folders)
		}
	}
	if strings.Contains(read(t, home, "projects/widget/project.md"), pendingMachine) {
		t.Fatal("placeholder was written")
	}
}

// Files are streamed, never held: a large file costs buffers, not its size.
func TestLargeFilesAreStreamed(t *testing.T) {
	from, home := fixture(t)
	const size = 96 << 20
	big := filepath.Join(from, "alpha/projects/widget/knowledge/big.bin")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	f.Close()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	res, err := scan(t, from, home).Apply()
	if err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&m1)
	if used := m1.TotalAlloc - m0.TotalAlloc; used > 24<<20 {
		t.Fatalf("migrating a %d MiB file allocated %d MiB", size>>20, used>>20)
	}
	for _, p := range []string{filepath.Join(home, "projects/widget/knowledge/big.bin"), filepath.Join(res.Backup, "source/alpha/projects/widget/knowledge/big.bin")} {
		if fi, err := os.Stat(p); err != nil || fi.Size() != size {
			t.Fatalf("%s: %v", p, err)
		}
	}
}

// A source that changes while it is copied is caught, and the home stays as it was.
func TestChangedDuringCopyIsRefused(t *testing.T) {
	from, home := fixture(t)
	p := scan(t, from, home)
	afterStep = func(step string) {
		if step == "journal-preparing" {
			put(t, filepath.Join(from, "alpha/projects/widget/knowledge/research/sample.md"), "changed after the plan\n")
		}
	}
	t.Cleanup(func() { afterStep = nil })
	res, err := p.Apply()
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("apply: %+v %v", res, err)
	}
	absent(t, home)
	absent(t, journalPath(home))
	absent(t, lockPath(home))
	if siblingsHave(t, home, ".migrate-stage-") {
		t.Fatal("staged copy left behind")
	}
}

func siblingsHave(t *testing.T, home, part string) bool {
	t.Helper()
	for _, s := range siblings(t, home) {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}
