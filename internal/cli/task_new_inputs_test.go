package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTaskNewInputsCopiesFiles(t *testing.T) {
	h := newHarness(t)
	png := []byte{0x89, 'P', 'N', 'G', 0, 7, 0xff}
	h.write(filepath.Join(h.tmp, "drop", "a.png"), string(png))
	h.write(filepath.Join(h.tmp, "drop", "notes.txt"), "notes\n")
	code := h.run(h.home, "task", "new", "Look at these", "--json",
		"--input", filepath.Join(h.tmp, "drop", "a.png"), "--input", filepath.Join(h.tmp, "drop", "notes.txt"))
	if code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, h.errb.String())
	}
	var made struct{ Path string }
	if err := json.Unmarshal(h.out.Bytes(), &made); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{"a.png": png, "notes.txt": []byte("notes\n")} {
		got, err := os.ReadFile(filepath.Join(made.Path, "inputs", name))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("inputs/%s = %q, %v", name, got, err)
		}
	}
}

func TestTaskNewInputsRelativePathUsesCwd(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.home, "rel.txt"), "relative")
	if code := h.run(h.home, "task", "new", "Relative", "--input", "rel.txt"); code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, h.errb.String())
	}
	matches, _ := filepath.Glob(filepath.Join(h.gridHome, "personal", "tasks", "T-01-*", "inputs", "rel.txt"))
	if len(matches) != 1 {
		t.Errorf("rel.txt not copied: %v", matches)
	}
}

func TestTaskNewInputsMissingOrFolderWritesNothing(t *testing.T) {
	h := newHarness(t)
	folder := filepath.Join(h.tmp, "adir")
	h.mkdir(folder)
	for _, bad := range []string{filepath.Join(h.tmp, "nope.png"), folder} {
		before := taskTestSnapshot(t, h.gridHome)
		code := h.run(h.home, "task", "new", "Bad", "--input", bad)
		if code == 0 {
			t.Errorf("%s: exit 0, want non-zero", bad)
		}
		if !bytes.Contains(h.errb.Bytes(), []byte(bad)) {
			t.Errorf("%s: stderr %q does not name the path", bad, h.errb.String())
		}
		after := taskTestSnapshot(t, h.gridHome)
		if len(before) != len(after) {
			t.Errorf("%s: Grid home changed: %v -> %v", bad, before, after)
		}
	}
}

func TestTaskNewInputsSameNameNumbered(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.tmp, "x", "a.txt"), "one")
	h.write(filepath.Join(h.tmp, "y", "a.txt"), "two")
	if code := h.run(h.home, "task", "new", "Twins",
		"--input", filepath.Join(h.tmp, "x", "a.txt"), "--input", filepath.Join(h.tmp, "y", "a.txt")); code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, h.errb.String())
	}
	for name, want := range map[string]string{"a.txt": "one", "a-2.txt": "two"} {
		matches, _ := filepath.Glob(filepath.Join(h.gridHome, "personal", "tasks", "T-01-*", "inputs", name))
		if len(matches) != 1 {
			t.Fatalf("inputs/%s missing", name)
		}
		if got, _ := os.ReadFile(matches[0]); string(got) != want {
			t.Errorf("inputs/%s = %q, want %q", name, got, want)
		}
	}
}

func TestTaskNewWithoutInputsMakesNoInputsFolder(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "task", "new", "Plain"); code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, h.errb.String())
	}
	matches, _ := filepath.Glob(filepath.Join(h.gridHome, "personal", "tasks", "T-01-*", "inputs"))
	if len(matches) != 0 {
		t.Errorf("inputs folder made without --input: %v", matches)
	}
}
