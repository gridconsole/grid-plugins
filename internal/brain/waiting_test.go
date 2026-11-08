package brain

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWaiting(t *testing.T) {
	for _, tc := range []struct {
		status string
		count  int
		fails  bool
	}{{"proposed", 1, false}, {"accepted", 0, false}, {"merged", 0, false}, {"declined", 0, false}, {"rejected", 0, false}, {"", 0, true}, {"broken", 0, true}} {
		t.Run(tc.status, func(t *testing.T) {
			home := t.TempDir()
			task := t.TempDir()
			dir := filepath.Join(task, "brain-proposals")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "fact.md")
			data := []byte("---\nstatus: " + tc.status + "\ntopic: testing\nreason: Reuse the check\n---\nA fact.\n")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			rows, err := (Store{GridHome: home}).Waiting(task)
			if (err != nil) != tc.fails {
				t.Fatal(err)
			}
			if len(rows) != tc.count {
				t.Fatal(rows)
			}
			if len(rows) > 0 && (rows[0].Topic != "testing" || rows[0].Reason != "Reuse the check") {
				t.Fatal(rows)
			}
			after, err := os.ReadFile(path)
			if err != nil || !reflect.DeepEqual(data, after) {
				t.Fatal("queue read wrote proposal", err)
			}
			entries, err := os.ReadDir(home)
			if err != nil || len(entries) != 0 {
				t.Fatal("queue read wrote home", entries, err)
			}
		})
	}
}

// proposalFile is a full note, as Propose writes it.
func proposalFile(name, level, source, originID string) string {
	return "---\nname: " + name + "\ntopic: testing\ntype: reference\ndescription: A fact\nsummary: A fact\nupdated: 2026-01-01T00:00:00Z\nsource: " + source +
		"\nsuperseded_by: \"\"\nlinks: []\nlevel: " + level + "\nstatus: proposed\nreason: \"\"\norigin:\n  kind: job\n  id: " + originID + "\n  tier: agent\n---\nA fact.\n"
}

func writeProposal(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWaitingScopesAndMalformed(t *testing.T) {
	s := Store{GridHome: t.TempDir(), Project: "example"}
	home := filepath.Join(s.GridHome, "brain-proposals")
	project := filepath.Join(s.GridHome, "projects", "example", "brain-proposals")
	writeProposal(t, home, "note", proposalFile("note", "home", "T-01", "T-01/J-01"))
	writeProposal(t, project, "note", proposalFile("note", "project", "T-01", "T-01/J-01"))
	rows, err := s.Waiting("")
	if err != nil || len(rows) != 2 || rows[0].Path >= rows[1].Path {
		t.Fatal(rows, err)
	}
	path := filepath.Join(home, "note.md")
	if err = os.WriteFile(path, []byte("---\nstatus: [\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Waiting(""); err == nil || !strings.Contains(err.Error(), "proposal note.md") {
		t.Fatal(err)
	}
}

// Home and project proposals must parse the way accept parses them: an unknown
// field, a name that differs from the file or a level that differs from the
// folder is an error, while the task folder keeps its lenient reader.
func TestWaitingStoreLevelsAreStrict(t *testing.T) {
	for name, text := range map[string]string{
		"unknown field":  strings.Replace(proposalFile("note", "home", "T-01", "T-01"), "topic: testing", "topic: testing\nsurprise: yes", 1),
		"wrong name":     proposalFile("other", "home", "T-01", "T-01"),
		"wrong level":    proposalFile("note", "project", "T-01", "T-01"),
		"alias":          strings.Replace(proposalFile("note", "home", "T-01", "T-01"), "topic: testing", "topic: &a testing", 1),
		"lenient legacy": "---\nstatus: proposed\ntopic: testing\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			s := Store{GridHome: t.TempDir()}
			writeProposal(t, filepath.Join(s.GridHome, "brain-proposals"), "note", text)
			if _, err := s.Waiting(""); err == nil || !strings.Contains(err.Error(), "proposal note.md") {
				t.Fatal(err)
			}
			legacy := t.TempDir()
			writeProposal(t, filepath.Join(legacy, "brain-proposals"), "note", text)
			rows, err := Store{GridHome: t.TempDir()}.Waiting(legacy)
			if name == "lenient legacy" && (err != nil || len(rows) != 1) {
				t.Fatal(rows, err)
			}
		})
	}
}

func TestWaitingForTaskNamesTheTaskOrProject(t *testing.T) {
	s := Store{GridHome: t.TempDir(), Project: "example"}
	home := filepath.Join(s.GridHome, "brain-proposals")
	writeProposal(t, home, "by-task", proposalFile("by-task", "home", "T-07", "T-07/J-02"))
	writeProposal(t, home, "by-folder", proposalFile("by-folder", "home", "T-07-title", "session-1"))
	writeProposal(t, home, "by-project", proposalFile("by-project", "home", "example", "session-2"))
	writeProposal(t, home, "other-task", proposalFile("other-task", "home", "T-70", "T-70/J-01"))
	writeProposal(t, home, "other-project", proposalFile("other-project", "home", "elsewhere", "session-3"))
	writeProposal(t, filepath.Join(s.GridHome, "projects", "example", "brain-proposals"), "own", proposalFile("own", "project", "T-99", "T-99"))
	task := t.TempDir()
	writeProposal(t, filepath.Join(task, "brain-proposals"), "local", "---\nstatus: proposed\ntopic: testing\n---\n")

	rows, err := s.WaitingForTask(task, "T-07")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, row := range rows {
		got = append(got, row.Path)
	}
	want := []string{"brain:home/brain-proposals/by-folder.md", "brain:home/brain-proposals/by-project.md", "brain:home/brain-proposals/by-task.md", "brain:project/brain-proposals/own.md", "task:brain-proposals/local.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	// keep still lists every waiting proposal, so the keeper sees the rest.
	if all, err := s.Waiting(task); err != nil || len(all) != 7 {
		t.Fatal(len(all), err)
	}
	// A personal task has no project name to match.
	rows, err = Store{GridHome: s.GridHome}.WaitingForTask("", "T-07")
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
}
