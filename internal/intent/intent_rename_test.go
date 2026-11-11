package intent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Renew replaces the claim file through replaceJSON, which renames through
// atomicfile.Rename: the claim keeps one file and carries the longer deadline.
func TestRenameRenewReplacesClaim(t *testing.T) {
	s := testStore(t)
	r := enqueue(t, s, "start", nil)
	first, err := s.Claim(r.ID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	now := s.now().Add(ClaimTimeout / 2)
	s.Now = func() time.Time { return now }
	renewed, err := s.Renew(r.ID, first.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !renewed.Until.After(first.Until) {
		t.Fatalf("claim was not extended: %v then %v", first.Until, renewed.Until)
	}
	var onDisk Claim
	if err := readJSON(filepath.Join(s.TaskDir, "intents", r.ID+".claim"), &onDisk); err != nil {
		t.Fatal(err)
	}
	if !onDisk.Until.Equal(renewed.Until) {
		t.Fatalf("file holds %v, Renew returned %v", onDisk.Until, renewed.Until)
	}
	left, _ := filepath.Glob(filepath.Join(s.TaskDir, "intents", ".*tmp*"))
	if len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

// replaceJSON over a non-empty folder reaches the rename, which fails on every
// OS. The error surfaces, the temporary file is removed and nothing else under
// the folder changed. (Renew and Release read the target before they write it,
// so a folder there is refused earlier; the failure is shown on replaceJSON.)
func TestRenameReplaceJSONOverFolderFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.claim")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(path, "keep.txt")
	if err := os.WriteFile(inner, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceJSON(path, map[string]string{"a": "b"}); err == nil {
		t.Fatal("replacing a folder succeeded")
	}
	if got, err := os.ReadFile(inner); err != nil || string(got) != "keep\n" {
		t.Fatalf("folder content changed: %q, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "x.claim" {
		t.Fatalf("something else changed under the folder: %v, %v", entries, err)
	}
}

// replaceJSON over an existing file lands the new content.
func TestRenameReplaceJSONReplacesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.claim")
	if err := os.WriteFile(path, []byte(`{"a":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceJSON(path, map[string]string{"a": "new"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := readJSON(path, &got); err != nil || got["a"] != "new" {
		t.Fatalf("got %v, %v", got, err)
	}
}
