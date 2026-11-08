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

func TestWaitingScopesAndMalformed(t *testing.T) {
	s := Store{GridHome: t.TempDir(), Project: "example"}
	for _, prefix := range []string{"", filepath.Join("projects", "example")} {
		dir := filepath.Join(s.GridHome, prefix, "brain-proposals")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("---\nstatus: proposed\ntopic: testing\n---\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.Waiting("")
	if err != nil || len(rows) != 2 || rows[0].Path >= rows[1].Path {
		t.Fatal(rows, err)
	}
	path := filepath.Join(s.GridHome, "brain-proposals", "note.md")
	if err = os.WriteFile(path, []byte("---\nstatus: [\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Waiting(""); err == nil || !strings.Contains(err.Error(), "proposal note.md") {
		t.Fatal(err)
	}
}
