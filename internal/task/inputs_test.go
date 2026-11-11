package task

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inputFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInputsCopiedByteEqualUnderBaseName(t *testing.T) {
	s, home := storeFor(t)
	png := []byte{0x89, 'P', 'N', 'G', 0, 1, 2, 0xff, 0xfe}
	a := inputFile(t, home, "pics/a.png", png)
	b := inputFile(t, home, "docs/notes.txt", []byte("some notes\n"))
	got, err := s.New(NewOptions{Cwd: home, Words: "With inputs", Inputs: []string{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{"a.png": png, "notes.txt": []byte("some notes\n")} {
		data, err := os.ReadFile(filepath.Join(got.Path, InputsDir, name))
		if err != nil || !bytes.Equal(data, want) {
			t.Errorf("inputs/%s = %q, %v; want %q", name, data, err, want)
		}
	}
	entries, err := os.ReadDir(filepath.Join(got.Path, InputsDir))
	if err != nil || len(entries) != 2 {
		t.Errorf("inputs entries = %v, %v; want 2", entries, err)
	}
}

func TestInputsSameBaseNameGetsNumberedName(t *testing.T) {
	s, home := storeFor(t)
	one := inputFile(t, home, "one/a.png", []byte("first"))
	two := inputFile(t, home, "two/a.png", []byte("second"))
	three := inputFile(t, home, "three/a.png", []byte("third"))
	got, err := s.New(NewOptions{Cwd: home, Words: "Same names", Inputs: []string{one, two, three}})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"a.png": "first", "a-2.png": "second", "a-3.png": "third"} {
		data, err := os.ReadFile(filepath.Join(got.Path, InputsDir, name))
		if err != nil || string(data) != want {
			t.Errorf("inputs/%s = %q, %v; want %q", name, data, err, want)
		}
	}
}

func TestInputsBadPathFailsBeforeAnythingIsWritten(t *testing.T) {
	s, home := storeFor(t)
	good := inputFile(t, home, "good.txt", []byte("ok"))
	folder := filepath.Join(home, "somefolder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(home, "missing.png")
	for _, bad := range []string{missing, folder} {
		_, err := s.New(NewOptions{Cwd: home, Words: "Bad input", Inputs: []string{good, bad}})
		if err == nil || !strings.Contains(err.Error(), bad) {
			t.Fatalf("New with %s: err = %v, want one naming the path", bad, err)
		}
		if _, statErr := os.Lstat(s.Resolver.GridHome); statErr == nil {
			t.Errorf("Grid home exists after a refused input %s", bad)
		}
	}
}

func TestInputsCopyFailureLeavesNoHalfTask(t *testing.T) {
	s, home := storeFor(t)
	// A file that vanishes after the check, simulated by a source the copy cannot read.
	src := inputFile(t, home, "unreadable.txt", []byte("x"))
	if err := os.Chmod(src, 0); err != nil {
		t.Fatal(err)
	}
	if f, err := os.Open(src); err == nil {
		f.Close()
		t.Skip("running as a user that reads mode 0 files")
	}
	if _, err := s.New(NewOptions{Cwd: home, Words: "Half", Inputs: []string{src}}); err == nil {
		t.Fatal("New succeeded with an unreadable input")
	}
	dirs, _ := filepath.Glob(filepath.Join(s.Resolver.GridHome, "personal", "tasks", "*"))
	if len(dirs) != 0 {
		t.Errorf("task folder left behind: %v", dirs)
	}
}

func TestNoInputsMakesNoInputsFolder(t *testing.T) {
	s, home := storeFor(t)
	got, err := s.New(NewOptions{Cwd: home, Words: "Plain"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(got.Path, InputsDir)); !os.IsNotExist(err) {
		t.Errorf("inputs folder exists without --input: %v", err)
	}
	entries, _ := os.ReadDir(got.Path)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "intent.md,task.md" {
		t.Errorf("task folder = %v, want intent.md and task.md", names)
	}
}
