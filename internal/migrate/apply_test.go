package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
				home = from
				from = filepath.Join(from, "alpha")
			}
			before, err := snapshot(from)
			if err != nil {
				t.Fatal(err)
			}
			p := scan(t, from, home)
			res, err := p.Apply()
			if err != nil {
				t.Fatal(err)
			}
			if !res.Changed || res.Backup == "" {
				t.Fatal("missing apply result")
			}
			backup, err := snapshot(filepath.Join(res.Backup, "source"))
			if err != nil {
				t.Fatal(err)
			}
			if !equal(before, backup) {
				t.Fatal("backup differs from original")
			}
			got, err := snapshot(home)
			if err != nil {
				t.Fatal(err)
			}
			if string(got["VERSION"].Data) != "1\n" {
				t.Fatal("missing format marker")
			}
			if !ulidRE.MatchString(strings.TrimSpace(string(got["machine"].Data))) {
				t.Fatal("missing machine id")
			}
			for _, a := range p.Actions {
				if filepath.Base(a.From) == "history.md" || strings.HasSuffix(a.From, ".jsonl") || strings.HasSuffix(a.From, ".usage") || strings.HasSuffix(a.From, ".recalled") {
					if !same(before[a.From], got[a.To]) {
						t.Errorf("changed archived content: %s", a.From)
					}
				}
				if a.Kind == "identify" {
					r, err := parseRecord(got[a.To].Data)
					if err != nil {
						t.Fatal(err)
					}
					if !ulidRE.MatchString(r.value("id")) {
						t.Errorf("invalid id: %s", a.To)
					}
					if strings.HasSuffix(a.To, "/task.md") && !taskRE.MatchString(r.value("display")) {
						t.Errorf("lost display number: %s", a.To)
					}
				}
			}
			task := filepath.Join(home, "projects/widget/tasks/T-07-first")
			if string(got["sessions/session-a"].Data) != task+"\n" {
				t.Fatal("pointer not remapped")
			}
			var session map[string]any
			if err := json.Unmarshal(got["projects/widget/tasks/T-07-first/sessions/session-a.json"].Data, &session); err != nil {
				t.Fatal(err)
			}
			if session["cwd"] != filepath.Join(home, "projects/widget/worktrees/T-07/worker-a") {
				t.Fatal("session cwd not remapped")
			}
			var log journal
			if err := json.Unmarshal(got["migration.log"].Data, &log); err != nil {
				t.Fatal(err)
			}
			if log.Backup != res.Backup || len(log.SourceFiles) != len(before) {
				t.Fatal("incomplete migration log")
			}
			backupAll := gotTree(t, res.Backup)
			again, err := scan(t, from, home).Apply()
			if err != nil || again.Changed || again.Backup != "" {
				t.Fatalf("second apply: %+v %v", again, err)
			}
			// Applying the same plan object again is also safe.
			if again, err = p.Apply(); err != nil || again.Changed {
				t.Fatalf("repeat plan apply: %+v %v", again, err)
			}
			if !equal(got, gotTree(t, home)) {
				t.Fatal("second apply changed the home")
			}
			if !equal(backupAll, gotTree(t, res.Backup)) {
				t.Fatal("second apply changed the backup")
			}
			entries, err := os.ReadDir(home + "-backup")
			if err != nil || len(entries) != 1 {
				t.Fatal("second apply created another backup")
			}
			if layout == "separate" || layout == "single-home" {
				if !equal(before, gotTree(t, from)) {
					t.Fatal("external source changed")
				}
			}
			if layout == "in-place" {
				if _, err := os.Stat(filepath.Join(home, "alpha")); !os.IsNotExist(err) {
					t.Fatal("old named home remains")
				}
			}
		})
	}
}
func gotTree(t *testing.T, root string) tree {
	t.Helper()
	v, err := snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	return v
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
				if err := os.Mkdir(home+".migrate-lock", 0700); err != nil {
					t.Fatal(err)
				}
			}
			beforeSrc, beforeHome := gotTree(t, from), gotTree(t, home)
			if _, err := p.Apply(); err == nil {
				t.Fatal("apply should refuse")
			}
			if !equal(beforeSrc, gotTree(t, from)) || !equal(beforeHome, gotTree(t, home)) {
				t.Fatal("refusal changed data")
			}
			if _, err := os.Stat(filepath.Join(home, "VERSION")); !os.IsNotExist(err) {
				t.Fatal("failed apply published VERSION")
			}
		})
	}
}
func TestExistingMachineAndDestinationAreBackedUp(t *testing.T) {
	from, home := fixture(t)
	put(t, filepath.Join(home, "machine"), "existing-machine\n")
	put(t, filepath.Join(home, "settings.json"), "{}\n")
	before := gotTree(t, home)
	res, err := scan(t, from, home).Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !equal(before, gotTree(t, filepath.Join(res.Backup, "home"))) {
		t.Fatal("existing home backup differs")
	}
	after := gotTree(t, home)
	if !same(after["machine"], before["machine"]) || !same(after["settings.json"], before["settings.json"]) {
		t.Fatal("existing home data lost")
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
	root := t.TempDir()
	put(t, filepath.Join(root, "projects/example/project.md"), "---\nfolder: /example/work/project\n---\n")
	task := filepath.Join(root, "projects/example/tasks/T-01-example")
	put(t, filepath.Join(task, "task.md"), "---\nid: T-01\n---\n")
	put(t, filepath.Join(root, "sessions/session-a"), task+"\n")
	put(t, filepath.Join(root, "machine"), "existing-machine\n")
	if _, err := scan(t, root, root).Apply(); err != nil {
		t.Fatal(err)
	}
	got := gotTree(t, root)
	if string(got["sessions/session-a"].Data) != task+"\n" || string(got["machine"].Data) != "existing-machine\n" {
		t.Fatal("in-place data changed")
	}
}
