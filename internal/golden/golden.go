// Package golden compares test output with files under testdata/golden. Tests only.
package golden

import (
	"os"
	"path/filepath"
	"testing"
)

// Assert fails the test when got differs from testdata/golden/<name>.golden.
// UPDATE_GOLDEN=1 rewrites the file instead.
func Assert(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(dir(t), name+".golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v\nrun UPDATE_GOLDEN=1 make golden to create it", err)
	}
	if string(want) != got {
		t.Errorf("%s differs from the golden file\n--- want\n%s--- got\n%s", name, want, got)
	}
}

// dir finds testdata/golden at the module root, whatever package runs the test.
func dir(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return filepath.Join(d, "testdata", "golden")
		}
		if parent := filepath.Dir(d); parent != d {
			d = parent
			continue
		}
		t.Fatal("go.mod not found above the test folder")
	}
}
