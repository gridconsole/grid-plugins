// Package jobtest builds a small Grid home for the Job tests: a project with a
// task, one accepted project note and a project folder on this computer.
package jobtest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const (
	Machine = "m1"
	Project = "demo"
)

// Home is the layout one test works in. Folder is the project's folder on Machine.
type Home struct {
	GridHome string
	Task     string
	Folder   string
	Store    brain.Store
}

// New copies testdata/job/task into a fresh Grid home and accepts the project note
// brain:project/export-format. root is the repository's testdata folder.
func New(t *testing.T, testdata string) Home {
	t.Helper()
	gh := t.TempDir()
	h := Home{GridHome: gh, Task: filepath.Join(gh, "projects", Project, "tasks", "T-01"), Folder: filepath.Join(gh, "work")}
	if err := os.MkdirAll(h.Folder, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(testdata, "job", "task")
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(src, path)
		if e != nil {
			return e
		}
		target := filepath.Join(h.Task, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata := "---\nid: T-01\ntitle: Export documentation\nproject: demo\nstate: waiting\ncreated: 2026-01-01T00:00:00Z\nowner: member\n---\n# Export documentation\n"
	if err := os.WriteFile(filepath.Join(h.Task, "task.md"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	record := "---\nfolders:\n  - {machine: " + Machine + ", path: " + h.Folder + "}\n---\n"
	if err := os.WriteFile(filepath.Join(gh, "projects", Project, "project.md"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	h.Store = brain.Store{GridHome: gh, Project: Project, Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }}
	h.Note("project", "export-format", "conventions", "The export column order and compatibility rule.",
		"Keep the id column before the title column.\nChanging the column order requires a recorded decision.\n", contract.PersonTeam)
	return h
}

// Note proposes and accepts one note at level "project" or "home".
func (h Home) Note(level, name, topic, summary, body string, tier contract.TrustTier) brain.Entry {
	n := brain.Note{Name: name, Topic: topic, Type: "convention", Description: summary, Summary: summary, Source: "fixture", Level: level, Body: body}
	if _, err := h.Store.Propose(n, brain.Origin{Kind: "session", ID: "fixture", Tier: tier}); err != nil {
		panic(err)
	}
	e, err := h.Store.Accept(level, name)
	if err != nil {
		panic(err)
	}
	return e
}

// Approve records a real receipt for the current fixture plan, including reapproval.
func (h Home) Approve(t *testing.T) {
	t.Helper()
	text, err := (plan.Store{TaskDir: h.Task}).Read()
	if err != nil {
		t.Fatal(err)
	}
	current, err := task.Read(h.Task)
	if err != nil {
		t.Fatal(err)
	}
	s := task.Store{Resolver: where.Resolver{GridHome: h.GridHome}}
	if _, err := s.Approve(task.WriteOptions{Cwd: h.Task, ID: current.ID}, plan.Hash(text)); err != nil {
		t.Fatal(err)
	}
}
