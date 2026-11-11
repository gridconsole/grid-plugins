package intent

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// snapshot maps every path below root to its bytes, or to "<dir>" for a folder.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	got := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			got[rel] = "<dir>"
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func wantErrText(t *testing.T, err error, text string) {
	t.Helper()
	if err == nil || err.Error() != text {
		t.Fatalf("error: got %v, want %q", err, text)
	}
}

func TestDirectoryRefusals(t *testing.T) {
	t.Run("empty task dir", func(t *testing.T) {
		_, err := Store{TaskDir: ""}.List(false)
		wantErrText(t, err, "task directory is required")
	})

	t.Run("missing task dir", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		s := Store{TaskDir: missing}
		_, err := s.Enqueue("stop", "task", nil, "member", "")
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Enqueue: got %v, want os.ErrNotExist", err)
		}
		if _, err := os.Lstat(missing); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Enqueue created the task folder: %v", err)
		}
		rows, err := s.List(false)
		if err != nil || rows == nil || len(rows) != 0 {
			t.Fatalf("List of a missing task: got %v, %v; want an empty list and nil", rows, err)
		}
	})

	t.Run("task dir is a regular file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "task")
		const body = "not a folder\n"
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Store{TaskDir: file}.Enqueue("stop", "task", nil, "member", "")
		if err == nil {
			t.Fatal("Enqueue into a regular file succeeded")
		}
		b, rerr := os.ReadFile(file)
		if rerr != nil || string(b) != body {
			t.Fatalf("file changed: %q, %v", b, rerr)
		}
	})
}

func TestListRefusesBrokenFiles(t *testing.T) {
	t.Run("record whose id is another ULID", func(t *testing.T) {
		s := testStore(t)
		r := enqueue(t, s, "stop", nil)
		dir := filepath.Join(s.TaskDir, "intents")
		b, err := os.ReadFile(filepath.Join(dir, r.ID+".json"))
		if err != nil {
			t.Fatal(err)
		}
		other, err := newID(s.now())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, other+".json"), b, 0o600); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, s.TaskDir)
		_, err = s.List(false)
		wantErrText(t, err, "invalid intent record")
		if after := snapshot(t, s.TaskDir); !reflect.DeepEqual(before, after) {
			t.Fatalf("List changed files: before %v, after %v", before, after)
		}
	})

	t.Run("release marker is a directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("opening a directory for reading fails with a different error on Windows")
		}
		s := testStore(t)
		r := enqueue(t, s, "stop", nil)
		if _, err := s.Claim(r.ID, "worker"); err != nil {
			t.Fatal(err)
		}
		release := filepath.Join(s.TaskDir, "intents", r.ID+".release.json")
		if err := os.Mkdir(release, 0o700); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, s.TaskDir)
		_, err := s.List(false)
		wantErrText(t, err, "intent file must be regular")
		if after := snapshot(t, s.TaskDir); !reflect.DeepEqual(before, after) {
			t.Fatalf("List changed files: before %v, after %v", before, after)
		}
	})

	t.Run("done file with a second JSON value", func(t *testing.T) {
		s := testStore(t)
		r := enqueue(t, s, "stop", nil)
		c, err := s.Claim(r.ID, "worker")
		if err != nil {
			t.Fatal(err)
		}
		done := filepath.Join(s.TaskDir, "intents", r.ID+".done.json")
		if err := os.WriteFile(done, []byte(`{"token":"`+c.Token+`"}{}`), 0o600); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, s.TaskDir)
		_, err = s.List(false)
		wantErrText(t, err, "unexpected data after intent JSON")
		if after := snapshot(t, s.TaskDir); !reflect.DeepEqual(before, after) {
			t.Fatalf("List changed files: before %v, after %v", before, after)
		}
	})
}

func TestEnqueueKeyOnlyForSend(t *testing.T) {
	// "pause" is one of the verbs TestVerbs accepts.
	s := testStore(t)
	_, err := s.Enqueue("pause", "task", nil, "member", "k")
	wantErrText(t, err, "idempotency keys apply only to send")
	if _, err := os.Stat(filepath.Join(s.TaskDir, "intents")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused request wrote files: %v", err)
	}
}

func TestDoneRefusals(t *testing.T) {
	s := testStore(t)
	token, err := newID(s.now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done("x", token, ""); !errors.Is(err, ErrClaim) {
		t.Fatalf("Done with a bad id: got %v, want ErrClaim", err)
	}
	r := enqueue(t, s, "stop", nil)
	if _, err := s.Done(r.ID, "x", ""); !errors.Is(err, ErrClaim) {
		t.Fatalf("Done with a bad token: got %v, want ErrClaim", err)
	}

	// intents/ exists now; a fresh ULID has no record.
	fresh, err := newID(s.now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done(fresh, token, ""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Done without a record: got %v, want os.ErrNotExist", err)
	}

	c, err := s.Claim(r.ID, "worker")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Done(r.ID, c.Token, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done(r.ID, c.Token, "boom"); !errors.Is(err, ErrDone) {
		t.Fatalf("Done again with another failure: got %v, want ErrDone", err)
	}
	again, err := s.Done(r.ID, c.Token, "")
	if err != nil || again.Token != first.Token || again.Error != first.Error || !again.Created.Equal(first.Created) {
		t.Fatalf("Done again with the same text: got %+v, %v; want %+v", again, err, first)
	}
}
