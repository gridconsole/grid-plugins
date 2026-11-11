//go:build windows

package feed

import (
	"os"
	"path/filepath"
	"testing"
)

func winLaneStore(t *testing.T) Store {
	t.Helper()
	s := testStore(t)
	task := "---\nid: " + testTask + "\n---\n"
	if err := os.WriteFile(filepath.Join(s.TaskDir, "task.md"), []byte(task), 0o600); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWinLanePostAndRead(t *testing.T) {
	s := winLaneStore(t)
	w := testWriter()
	first := mustPost(t, s, message("first"), w)
	if first.Seq != 1 {
		t.Fatalf("first seq: %d", first.Seq)
	}
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 1 || read.Records[0].ID != first.ID || read.Records[0].Seq != 1 {
		t.Fatalf("read after first post: %+v", read.Records)
	}
	second := mustPost(t, s, message("second"), w)
	if second.Seq != 2 {
		t.Fatalf("second seq: %d", second.Seq)
	}
	read, err = s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 2 {
		t.Fatalf("read after second post: %d records", len(read.Records))
	}
}

func TestWinLaneCheckFilesystemLocal(t *testing.T) {
	if err := CheckFilesystem(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestWinLaneSyncFolderIsNoop(t *testing.T) {
	d, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := syncFile(d); err != nil {
		t.Fatalf("syncFile on a folder: %v", err)
	}
}

func TestWinLaneSyncFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "written"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := syncFile(f); err != nil {
		t.Fatalf("syncFile on a written file: %v", err)
	}
}
