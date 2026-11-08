package golden

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalization(t *testing.T) {
	n := newNormalizer("/case", map[string]string{"/case/demo": "$GRID_HOME", "/case/old-plugin": "$OLD_PLUGIN", "/case/recorder": "$RECORDER"})
	input := "/case/demo/a /case/user /case/old-plugin /case/recorder 2025-01-02T03:04 2025-01-02T03:04:05.123Z 42:1735787045000.25:1735787045001\" 01ARZ3NDEKTSV4RRFFQ69G5FAV 01ARZ3NDEKTSV4RRFFQ69G5FAW\r\n"
	want := "$GRID_HOME/a $GRID_HOME/../user $OLD_PLUGIN $RECORDER <TIME> <TIME> 42:<TIME>:<TIME>\" <ID-1> <ID-2>\r\n"
	if got := n.text(input); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := n.text("01ARZ3NDEKTSV4RRFFQ69G5FAV"); got != "<ID-1>" {
		t.Fatal(got)
	}
	unicodeInput := "é2025-01-02T03:04 ٢٠٢٥-٠١-٠٢T٠٣:٠٤ é01ARZ3NDEKTSV4RRFFQ69G5FAV :١٧٣٥٧٨٧٠٤٥٠٠٠\u00a0"
	unicodeWant := "é2025-01-02T03:04 <TIME> é01ARZ3NDEKTSV4RRFFQ69G5FAV :<TIME>\u00a0"
	if got := n.text(unicodeInput); got != unicodeWant {
		t.Fatalf("Unicode normalization: got %q want %q", got, unicodeWant)
	}
	untouched := `{"b":2, "a":"literal"}`
	if got := n.text(untouched); got != untouched {
		t.Fatal(got)
	}
}

func TestUnifiedDiff(t *testing.T) {
	tests := []struct{ name, old, now, want string }{
		{"new", "", "hello\n", "--- a\n+++ b\n@@ -0,0 +1 @@\n+hello\n"},
		{"delete", "hello\n", "", "--- a\n+++ b\n@@ -1 +0,0 @@\n-hello\n"},
		{"no newline", "old", "new", "--- a\n+++ b\n@@ -1 +1 @@\n-old\n\\ No newline at end of file\n+new\n\\ No newline at end of file\n"},
		{"crlf", "a\r\nb\r\n", "a\r\nc\r\n", "--- a\n+++ b\n@@ -1,2 +1,2 @@\n a\r\n-b\r\n+c\r\n"},
		{"same", "a\n", "a\n", ""},
		{"repeated", "a\nb\na\nb\n", "a\nb\nx\na\nb\n", "--- a\n+++ b\n@@ -1,4 +1,5 @@\n a\n b\n+x\n a\n b\n"},
		{"separate hunks", "0\n1\n2\n3\n4\n5\n6\n7\n8\n9\n", "x\n1\n2\n3\n4\n5\n6\n7\n8\ny\n", "--- a\n+++ b\n@@ -1,4 +1,4 @@\n-0\n+x\n 1\n 2\n 3\n@@ -7,4 +7,4 @@\n 6\n 7\n 8\n-9\n+y\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := unifiedDiff(tc.old, tc.now, "a", "b"); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestSnapshotAndModes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "data")
	if err := os.WriteFile(path, []byte("2025-01-02T03:04Z"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := takeSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("2025-01-02T03:05Z"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := takeSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	got := folderDiff(before, after, newNormalizer(root, nil))
	if got != "diff --grid a/data b/data\nOnly normalized values changed.\n" {
		t.Fatal(got)
	}
	// Mode-only and type changes are observable even with identical content.
	before = snapshot{"data": entry{"file", "same", 0o644}}
	after = snapshot{"data": entry{"link", "same", 0o777}}
	got = folderDiff(before, after, newNormalizer(root, nil))
	if got != "diff --grid a/data b/data\nmode/type file 0644 -> link 0777\n" {
		t.Fatal(got)
	}
	got = folderDiff(nil, after, newNormalizer(root, nil))
	if !strings.Contains(got, "new link mode 0777\n--- /dev/null\n") {
		t.Fatal(got)
	}
	got = folderDiff(before, nil, newNormalizer(root, nil))
	if !strings.Contains(got, "deleted file mode 0644\n--- a/data\n+++ /dev/null\n") {
		t.Fatal(got)
	}
}

func TestFixturePathContainment(t *testing.T) {
	box, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := safePath(box, filepath.Join(box, "missing", "ok")); err != nil {
		t.Fatal(err)
	}
	if _, err := safePath(box, filepath.Join(box, "..", "escape")); err == nil {
		t.Fatal("allowed escape")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(box, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := safePath(box, filepath.Join(box, "link", "escape")); err == nil {
		t.Fatal("allowed symlink escape")
	}
}
