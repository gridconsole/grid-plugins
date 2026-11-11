package golden

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGdSplitLinesUnicodeBreaks(t *testing.T) {
	got := splitLines("a\u0085b c d\r\ne\rf")
	want := []string{"a\u0085", "b ", "c ", "d\r\n", "e\r", "f"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitLines = %q, want %q", got, want)
	}
}

func TestGdMatchingBlocksPopularLinePruned(t *testing.T) {
	a := []string{"x\n", "u\n", "x\n"}
	b := make([]string, 250)
	for i := range b {
		b[i] = "x\n"
	}
	b[100] = "u\n"
	got := matchingBlocks(a, b)
	want := []block{{0, 99, 3}, {3, 250, 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matchingBlocks = %v, want %v", got, want)
	}
}

func TestGdTakeSnapshotMissingRoot(t *testing.T) {
	_, err := takeSnapshot(filepath.Join(t.TempDir(), "gdmissing"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("takeSnapshot error = %v, want one satisfying os.ErrNotExist", err)
	}
}

func TestGdSafePathDanglingLink(t *testing.T) {
	box := t.TempDir()
	if err := os.Symlink(filepath.Join(box, "gdnone"), filepath.Join(box, "gdlink")); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	_, err := safePath(box, filepath.Join(box, "gdlink"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("safePath error = %v, want one satisfying os.ErrNotExist", err)
	}
}
