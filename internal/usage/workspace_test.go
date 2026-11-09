package usage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func TestWorkspaceBudgetPreservesBytes(t *testing.T) {
	for _, tc := range []struct{ name, original, set, clear string }{
		{"comments", "---\n# keep this comment\nname: 'My workspace' # keep\ntokens_per_task: 100 # old\nother: [a, b]\n---\n\nBody\n tokens_per_task: 777\n", "---\n# keep this comment\nname: 'My workspace' # keep\ntokens_per_task: 200\nother: [a, b]\n---\n\nBody\n tokens_per_task: 777\n", "---\n# keep this comment\nname: 'My workspace' # keep\nother: [a, b]\n---\n\nBody\n tokens_per_task: 777\n"},
		{"absent-key", "---\nname: 'workspace'\n# last comment\n---\nBody", "---\nname: 'workspace'\n# last comment\ntokens_per_task: 200\n---\nBody", "---\nname: 'workspace'\n# last comment\n---\nBody"},
		{"no-front-matter", "# Workspace\nBody\n", "---\ntokens_per_task: 200\n---\n# Workspace\nBody\n", "---\n---\n# Workspace\nBody\n"},
		{"missing-file", "", "---\ntokens_per_task: 200\n---\n", "---\n---\n"},
		{"empty-file", "", "---\ntokens_per_task: 200\n---\n", "---\n---\n"},
		{"empty-header", "---\n---\nBody", "---\ntokens_per_task: 200\n---\nBody", "---\n---\nBody"},
		{"blank-header", "---\n\n---\nBody", "---\n\ntokens_per_task: 200\n---\nBody", "---\n\n---\nBody"},
		{"crlf", "---\r\nname: workspace\r\ntokens_per_task: 100\r\n# keep\r\n---\r\nBody\r\n", "---\r\nname: workspace\r\ntokens_per_task: 200\r\n# keep\r\n---\r\nBody\r\n", "---\r\nname: workspace\r\n# keep\r\n---\r\nBody\r\n"},
		{"crlf-add", "---\r\nname: workspace\r\n---\r\nBody", "---\r\nname: workspace\r\ntokens_per_task: 200\r\n---\r\nBody", "---\r\nname: workspace\r\n---\r\nBody"},
		{"duplicates", "---\ntokens_per_task: 1\n# middle\ntokens_per_task: 2\n---", "---\ntokens_per_task: 200\n# middle\n---", "---\n# middle\n---"},
		{"unicode", "---\nname: café\ntokens_per_task: '100'\n---\n雪", "---\nname: café\ntokens_per_task: 200\n---\n雪", "---\nname: café\n---\n雪"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Store{GridHome: t.TempDir()}
			path := filepath.Join(s.GridHome, "workspace.md")
			if tc.name != "missing-file" {
				if err := os.WriteFile(path, []byte(tc.original), 0600); err != nil {
					t.Fatal(err)
				}
			}
			n := uint64(200)
			for _, step := range []struct {
				value *uint64
				want  string
			}{{&n, tc.set}, {nil, tc.clear}} {
				if err := s.SetWorkspaceTokens(step.value); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil || string(data) != step.want {
					t.Fatalf("workspace bytes: got %q, want %q (%v)", data, step.want, err)
				}
				got := s.WorkspaceTokens()
				if step.value == nil && got != nil || step.value != nil && (got == nil || *got != *step.value) {
					t.Fatalf("stored token value does not match mutation")
				}
			}
			if tc.name != "missing-file" {
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("workspace permissions changed: %v", err)
				}
			}
		})
	}
}

func TestWorkspaceBudgetNoopClear(t *testing.T) {
	for _, contents := range []string{"missing", "", "plain body", "---\nname: workspace\n---\nBody", "---\n---\n"} {
		t.Run(contents, func(t *testing.T) {
			s := Store{GridHome: t.TempDir()}
			path := filepath.Join(s.GridHome, "workspace.md")
			var before os.FileInfo
			if contents != "missing" {
				if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				stamp := time.Unix(1000000000, 0)
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				var err error
				before, err = os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := s.SetWorkspaceTokens(nil); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(path)
			if contents == "missing" {
				if !os.IsNotExist(err) {
					t.Fatal("clear created a workspace file")
				}
			} else {
				if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("clear wrote an unchanged file")
				}
				data, err := os.ReadFile(path)
				if err != nil || string(data) != contents {
					t.Fatal("clear changed bytes")
				}
			}
		})
	}
}

func TestWorkspaceBudgetConcurrent(t *testing.T) {
	s := Store{GridHome: t.TempDir()}
	path := filepath.Join(s.GridHome, "workspace.md")
	before := "---\n# keep\nname: workspace\n"
	after := "---\nBody\n"
	if err := os.WriteFile(path, []byte(before+after), 0600); err != nil {
		t.Fatal(err)
	}
	start, done := make(chan struct{}), make(chan error, 20)
	for i := uint64(1); i <= 20; i++ {
		go func() { <-start; done <- s.SetWorkspaceTokens(&i) }()
	}
	close(start)
	for range 20 {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	value := s.WorkspaceTokens()
	if value == nil || *value < 1 || *value > 20 {
		t.Fatal("concurrent writers left an invalid token value")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(data), "\n")
	var rest strings.Builder
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "tokens_per_task:") {
			count++
		} else {
			rest.WriteString(line)
		}
	}
	if count != 1 || rest.String() != before+after {
		t.Fatalf("concurrent writers changed other bytes or duplicated the key: %q", data)
	}
}

func TestWorkspaceBudgetWaitsForLock(t *testing.T) {
	s := Store{GridHome: t.TempDir()}
	root, err := os.OpenRoot(s.GridHome)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	lock, err := workspaceLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	defer fslock.Unlock(lock)
	original := []byte("---\nname: original\ntokens_per_task: 1\n---\nBody\n")
	if err := atomicfile.WriteRoot(root, "workspace.md", original, 0600); err != nil {
		t.Fatal(err)
	}
	done, started := make(chan error, 1), make(chan struct{})
	go func() { close(started); n := uint64(200); done <- s.SetWorkspaceTokens(&n) }()
	<-started
	select {
	case err := <-done:
		t.Fatalf("workspace writer bypassed held lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	// A cooperating writer changes an unrelated key while holding the lock.
	// The waiting token writer must read this revision after acquiring the lock.
	updated := bytes.ReplaceAll(original, []byte("name: original"), []byte("name: updated"))
	if err := atomicfile.WriteRoot(root, "workspace.md", updated, 0600); err != nil {
		t.Fatal(err)
	}
	fslock.Unlock(lock)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("workspace writer did not finish after unlocking")
	}
	data, err := root.ReadFile("workspace.md")
	want := bytes.ReplaceAll(updated, []byte("tokens_per_task: 1\n"), []byte("tokens_per_task: 200\n"))
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("writer lost a concurrent update: %q (%v)", data, err)
	}
}

func TestWorkspaceBudgetUnsafeLock(t *testing.T) {
	for _, kind := range []string{"linked", "directory"} {
		t.Run(kind, func(t *testing.T) {
			s := Store{GridHome: t.TempDir()}
			path := filepath.Join(s.GridHome, workspaceLockName)
			if kind == "linked" {
				if err := os.WriteFile(filepath.Join(s.GridHome, "other"), []byte("unchanged"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("other", path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			n := uint64(100)
			if err := s.SetWorkspaceTokens(&n); err == nil {
				t.Fatal("unsafe lock accepted")
			}
			if _, err := os.Lstat(filepath.Join(s.GridHome, "workspace.md")); !os.IsNotExist(err) {
				t.Fatal("unsafe lock permitted a write")
			}
		})
	}
}

func TestWorkspaceBudgetSizeBoundary(t *testing.T) {
	s := Store{GridHome: t.TempDir()}
	path := filepath.Join(s.GridHome, "workspace.md")
	data := "---\ntokens_per_task: 1\n---\n"
	data += strings.Repeat("x", maxWorkspaceBytes-len(data))
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if n := s.WorkspaceTokens(); n == nil || *n != 1 {
		t.Fatal("size boundary must be readable")
	}
	n := uint64(100)
	if err := s.SetWorkspaceTokens(&n); err == nil {
		t.Fatal("writer produced an unreadably large file")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != data {
		t.Fatal("size refusal changed file")
	}
}

func TestWorkspaceBudgetInputError(t *testing.T) {
	zero, positive := uint64(0), uint64(100)
	for _, tc := range []struct {
		name    string
		value   *uint64
		input   bool
		message string
	}{
		{"zero", &zero, true, "workspace token budget must be at least 1"},
		{"set-missing-home", &positive, false, "cannot find the Grid home; set GRID_HOME"},
		{"clear-missing-home", nil, false, "cannot find the Grid home; set GRID_HOME"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (Store{}).SetWorkspaceTokens(tc.value)
			if errors.Is(err, ErrWorkspaceTokensInput) != tc.input {
				t.Errorf("input classification = %v, want %v", err, tc.input)
			}
			if err == nil || err.Error() != tc.message {
				t.Fatalf("error = %v, want %q", err, tc.message)
			}
			if tc.input && errors.Unwrap(err) != ErrWorkspaceTokensInput {
				t.Fatal("input error does not wrap the sentinel")
			}
		})
	}
}
