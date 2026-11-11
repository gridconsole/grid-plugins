package feed

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// identityPrepareErr fails unless err is non-nil and carries exactly want.
func identityPrepareErr(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want %q", want)
	}
	if err.Error() != want {
		t.Fatalf("error %q, want %q", err.Error(), want)
	}
}

func TestIdentityPrepareMachineIDNotULID(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "machine")
	if err := os.WriteFile(path, []byte("not-an-id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := MachineID(home)
	identityPrepareErr(t, err, "machine must contain a ULID")
	if id != "" {
		t.Fatalf("id %q returned with the error", id)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "not-an-id\n" {
		t.Fatalf("machine file changed to %q", got)
	}
}

func TestIdentityPrepareEnsureTaskIDDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("opening a directory fails on windows before the regular-file check")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "task.md"), 0o700); err != nil {
		t.Fatal(err)
	}
	id, err := EnsureTaskID(dir)
	identityPrepareErr(t, err, "task.md must be a regular file")
	if id != "" {
		t.Fatalf("id %q returned with the error", id)
	}
}

func TestIdentityPrepareWithTaskID(t *testing.T) {
	a, b := testTask, testMachine
	held := []byte("---\nulid: " + a + "\ntitle: x\n---\nbody\n")
	for _, tc := range []struct {
		name string
		data []byte
		id   string
		want string // exact error text; empty means success
		out  []byte // exact bytes on success
	}{
		{"id not a ULID", held, "abc", "task id must be a ULID", nil},
		{"conflicting id", held, b, "conflicting stable task ids", nil},
		{"same id returns the data", held, a, "", held},
		{"frontmatter never closed", []byte("---\ntitle: x\n"), a, "task.md frontmatter is not closed", nil},
		{"nonsense ulid", []byte("---\nulid: nonsense\n---\nbody\n"), a, "invalid task ulid", nil},
		{"CRLF without ulid", []byte("---\r\ntitle: x\r\n---\r\nbody\r\n"), a, "",
			[]byte("---\r\ntitle: x\r\nulid: " + a + "\r\n---\r\nbody\r\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := append([]byte(nil), tc.data...)
			got, err := WithTaskID(in, tc.id)
			if tc.want != "" {
				identityPrepareErr(t, err, tc.want)
				if got != nil {
					t.Fatalf("data %q returned with the error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.out) {
				t.Fatalf("got %q, want %q", got, tc.out)
			}
		})
	}
}

func TestIdentityPreparePrepare(t *testing.T) {
	t.Run("task id not a ULID", func(t *testing.T) {
		s := Store{TaskDir: t.TempDir(), TaskID: "x"}
		_, err := s.SaveState()
		identityPrepareErr(t, err, "task id must be a ULID")
		if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
			t.Fatalf("feed folder after the refusal: %v", err)
		}
	})
	t.Run("task dir is a file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "task")
		if err := os.WriteFile(file, []byte("not a folder"), 0o600); err != nil {
			t.Fatal(err)
		}
		s := Store{TaskDir: file, TaskID: testTask}
		_, err := s.SaveState()
		identityPrepareErr(t, err, "task is not a folder")
		if _, err := os.Stat(s.Dir()); err == nil {
			t.Fatal("feed folder under a regular file")
		}
		info, err := os.Stat(file)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("task file changed: %v %v", info, err)
		}
	})
}

func TestIdentityPrepareUnreadableBody(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	in := Input{Type: "knowledge", Body: map[string]any{"title": "Example", "text": strings.Repeat("x", MaxLine)}}
	first := mustPost(t, s, in, w)
	x := first.ID
	path := filepath.Join(s.Dir(), writerFile(w.Worker, w.Fence))
	lines, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lines), `"ref":"bodies/`+x+`.json"`) {
		t.Fatalf("record %s was not written with a body reference: %.200s", x, lines)
	}
	in.ID = x
	// Contrast: with the body file intact the retry returns the first record.
	retry, err := s.Post(in, w)
	if err != nil {
		t.Fatal(err)
	}
	if retry.ID != first.ID || retry.Seq != first.Seq {
		t.Fatalf("retry %s seq %d, want %s seq %d", retry.ID, retry.Seq, first.ID, first.Seq)
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), "bodies", x+".json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = s.Post(in, w)
	identityPrepareErr(t, err, "body of "+x+" is unreadable")
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, lines) {
		t.Fatalf("writer file changed: %d bytes before, %d after", len(lines), len(after))
	}
}
