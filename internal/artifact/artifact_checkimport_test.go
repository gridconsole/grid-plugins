package artifact

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// treeOf lists every path below root, relative to it, in walk order.
func treeOf(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// sourceFile writes size bytes to a new file called name in a fresh folder that
// is not the store, and returns its path.
func sourceFile(t *testing.T, name string, size int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckImportAcceptsFileAndCreatesNothing(t *testing.T) {
	source := sourceFile(t, "report.txt", 5)

	// No artifacts folder yet: the task folder must still be empty afterwards.
	task := t.TempDir()
	before := treeOf(t, task)
	if err := (Store{TaskPath: task, MaxBytes: 10}).CheckImport(source); err != nil {
		t.Fatalf("CheckImport of a file under the limit: %v", err)
	}
	if after := treeOf(t, task); !reflect.DeepEqual(before, after) {
		t.Fatalf("CheckImport changed the store folder: before %v, after %v", before, after)
	}

	// An artifacts folder with a record: nothing is added beside it either.
	if _, err := (Store{TaskPath: task}).Import(sourceFile(t, "first.txt", 3), ""); err != nil {
		t.Fatal(err)
	}
	before = treeOf(t, task)
	if len(before) < 3 {
		t.Fatalf("fixture store is empty: %v", before)
	}
	if err := (Store{TaskPath: task, MaxBytes: 10}).CheckImport(source); err != nil {
		t.Fatalf("CheckImport with an artifacts folder: %v", err)
	}
	if after := treeOf(t, task); !reflect.DeepEqual(before, after) {
		t.Fatalf("CheckImport changed the artifacts folder: before %v, after %v", before, after)
	}
}

func TestCheckImportRefusesFolder(t *testing.T) {
	err := (Store{TaskPath: t.TempDir()}).CheckImport(t.TempDir())
	if err == nil || err.Error() != "source must be a regular file" {
		t.Fatalf("CheckImport of a folder: %v", err)
	}
}

func TestCheckImportSizeLimit(t *testing.T) {
	s := Store{TaskPath: t.TempDir(), MaxBytes: 10}
	if err := s.CheckImport(sourceFile(t, "exact.txt", 10)); err != nil {
		t.Fatalf("CheckImport of a file at the limit: %v", err)
	}
	err := s.CheckImport(sourceFile(t, "over.txt", 11))
	if err == nil || err.Error() != errTooLarge(10).Error() {
		t.Fatalf("CheckImport of a file one byte over: %v", err)
	}
}

// A MaxBytes of zero or below falls back to DefaultMaxImportBytes
// (artifact.go:114-117). If it were taken as a limit of zero, a one-byte file
// would be refused as too large.
func TestCheckImportZeroMaxBytesUsesDefault(t *testing.T) {
	source := sourceFile(t, "small.txt", 1)
	for _, max := range []int64{0, -1} {
		if err := (Store{TaskPath: t.TempDir(), MaxBytes: max}).CheckImport(source); err != nil {
			t.Fatalf("CheckImport with MaxBytes %d: %v", max, err)
		}
	}
}

func TestCheckImportMissingSource(t *testing.T) {
	err := (Store{TaskPath: t.TempDir()}).CheckImport(filepath.Join(t.TempDir(), "absent.txt"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("CheckImport of a missing path: %v", err)
	}
}
