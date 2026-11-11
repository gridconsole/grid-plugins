package project

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadAndWriteWhenProjectsIsAFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows answers such a path as not-exist.")
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "projects"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if recs, err := Load(home); err == nil {
		t.Fatalf("Load gave %v and no error", recs)
	}
	if err := Write(home, Record{Name: "a"}); err == nil {
		t.Fatal("Write gave no error")
	}
}

func TestLoadSkipsAStrayFile(t *testing.T) {
	home := t.TempDir()
	if err := Write(home, Record{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "projects", "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	recs, err := Load(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Name != "a" {
		t.Fatalf("Load = %+v, want exactly [a]", recs)
	}
}

func TestLoadRefusesAProjectMdDirectory(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "projects", "b", "project.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if recs, err := Load(home); err == nil {
		t.Fatalf("Load gave %v and no error", recs)
	}
}

func TestFormatOfAnEmptyRecord(t *testing.T) {
	b, err := Format(Record{})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "---\n---\n" {
		t.Fatalf("Format(Record{}) = %q, want %q", b, "---\n---\n")
	}
}
