package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// noTempRecords fails when the artifacts folder still holds a .record-* file,
// and returns the folder.
func noTempRecords(t *testing.T, task string) string {
	t.Helper()
	dir := filepath.Join(task, "artifacts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".record-") {
			t.Errorf("temp record left behind: %s", e.Name())
		}
	}
	return dir
}

func requireRegular(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is not a regular file: %v", path, info.Mode())
	}
}

func TestPublishReplacesImportedRecord(t *testing.T) {
	task := t.TempDir()
	src := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Store{TaskPath: task}
	r, err := s.Import(src, "Report")
	if err != nil {
		t.Fatal(err)
	}
	if r.Published {
		t.Fatal("a new import must not be published")
	}
	pub, err := s.Publish(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Published {
		t.Fatal("Publish returned an unpublished record")
	}
	again, err := s.Publish(r.ID)
	if err != nil || !again.Published {
		t.Fatalf("second Publish = %+v, %v", again, err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != r.ID || !list[0].Published || list[0].Kind != "file" {
		t.Fatalf("List = %+v", list)
	}
	dir := noTempRecords(t, task)
	requireRegular(t, filepath.Join(dir, r.ID+".json"))
}

func TestPublishReplacesLinkRecord(t *testing.T) {
	task := t.TempDir()
	s := Store{TaskPath: task}
	r, err := s.Link("https://example.com/doc", "Doc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(r.ID); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != r.ID || list[0].Kind != "link" || !list[0].Published {
		t.Fatalf("List = %+v", list)
	}
	dir := noTempRecords(t, task)
	requireRegular(t, filepath.Join(dir, r.ID+".json"))
}
