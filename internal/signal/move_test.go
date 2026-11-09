package signal

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func moveFixture(t *testing.T) (Store, Result, string, []byte) {
	t.Helper()
	s := fixture(t)
	for _, name := range []string{"second", "third", "personal"} {
		if err := project.Write(s.GridHome, project.Record{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.Add(Input{Source: "typed", Title: "Move me", Body: "status: new\nUntrusted body.  \n"})
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.path(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, r, path, b
}

func TestMoveConcurrent(t *testing.T) {
	s, r, source, _ := moveFixture(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, to := range []string{"second", "third"} {
		wg.Go(func() { <-start; _, err := s.Move(r.ID, "example", to); results <- err })
	}
	close(start)
	wg.Wait()
	close(results)
	won, lost := 0, 0
	for err := range results {
		if err == nil {
			won++
		} else if err.Error() == "no such note" {
			lost++
		} else {
			t.Fatal(err)
		}
	}
	if won != 1 || lost != 1 {
		t.Fatalf("wins=%d missing=%d", won, lost)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source: %v", err)
	}
	copies, err := filepath.Glob(filepath.Join(s.GridHome, "projects", "*", "notes", r.Note))
	if err != nil || len(copies) != 1 {
		t.Fatalf("copies=%d err=%v", len(copies), err)
	}
}

func TestMoveWaitsForAddLock(t *testing.T) {
	s, r, source, before := moveFixture(t)
	unlock, err := localdata.Lock(s.GridHome, ".signals.lock")
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			unlock()
		}
	}()
	done := make(chan error, 1)
	started := make(chan struct{})
	go func() { close(started); _, err := s.Move(r.ID, "example", "second"); done <- err }()
	<-started
	select {
	case err := <-done:
		t.Fatalf("move bypassed signals lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("note changed while lock held")
	}
	unlock()
	released = true
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("move did not release lock")
	}
}

func TestMoveInvalidIDBeforeFileOperations(t *testing.T) {
	for _, id := range []string{"../x", "a/b", `a\b`, "", "\x00"} {
		t.Run(id, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "absent")
			s := Store{GridHome: home}
			_, err := s.Move(id, "example", "second")
			var bad *BadInputError
			if !errors.As(err, &bad) || bad.Message != "invalid signal id" {
				t.Fatalf("error %v", err)
			}
			if _, err := os.Stat(home); !os.IsNotExist(err) {
				t.Fatalf("invalid id performed file operations: %v", err)
			}
		})
	}
}

func TestMovePersonalAccessAndGuest(t *testing.T) {
	for _, kind := range []string{"guest", "personal-source", "personal-target"} {
		t.Run(kind, func(t *testing.T) {
			s, r, source, before := moveFixture(t)
			from, to := "example", "personal"
			if kind == "guest" {
				s.Resolver.Caller = where.Caller{Role: "guest"}
			} else {
				s.Resolver.PersonalAllowed = func(where.Caller) bool { return false }
			}
			if kind == "personal-source" {
				from, to = "personal", "example"
			}
			_, err := s.Move(r.ID, from, to)
			var refused *RefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("error %v", err)
			}
			want := "no such project"
			if kind == "guest" {
				want = "guests cannot act on signals"
			}
			if err.Error() != want {
				t.Fatalf("error %v", err)
			}
			after, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refusal changed note")
			}
		})
	}
}

func TestMoveRenameFailureRestoresOriginal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows chmod does not prevent directory writes")
	}
	s, r, source, before := moveFixture(t)
	dir := filepath.Join(s.GridHome, "projects", "second", "notes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	probe := filepath.Join(dir, "probe")
	if err := os.WriteFile(probe, nil, 0o600); err == nil {
		_ = os.Remove(probe)
		t.Skip("caller bypasses directory write permissions")
	}
	_, err := s.Move(r.ID, "example", "second")
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("rename error: %v", err)
	}
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rename failure did not restore original")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("target changed: %v", err)
	}
	entries, err = os.ReadDir(filepath.Dir(source))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files remain: %v", err)
	}
}

func TestMoveSymlinksFailClosed(t *testing.T) {
	for _, kind := range []string{"source-note", "target-note", "target-folder"} {
		t.Run(kind, func(t *testing.T) {
			s, r, source, before := moveFixture(t)
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, before, 0o600); err != nil {
				t.Fatal(err)
			}
			targetDir := filepath.Join(s.GridHome, "projects", "second", "notes")
			if err := os.MkdirAll(targetDir, 0o755); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(targetDir, r.Note)
			if kind == "source-note" {
				link = source
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "target-folder" {
				link = targetDir
				if err := os.Remove(targetDir); err != nil {
					t.Fatal(err)
				}
				outside = filepath.Dir(outside)
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Skip(err)
			}
			if _, err := s.Move(r.ID, "example", "second"); !errors.Is(err, localdata.ErrUnsafePath) {
				t.Fatalf("error %v", err)
			}
			after, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("unsafe move changed source")
			}
		})
	}
}

func TestMovePreservesExtendedFrontMatter(t *testing.T) {
	s, r, source, before := moveFixture(t)
	before = bytes.Replace(before, []byte("status: new"), []byte("status: 'new' # retain comment\nextra: keep me"), 1)
	if err := os.WriteFile(source, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(source, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.Move(r.ID, "example", "second")
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(s.GridHome, "projects", "second", "notes", r.Note))
	if err != nil {
		t.Fatal(err)
	}
	want := bytes.Replace(before, []byte("'new'"), []byte("'placed'"), 1)
	if !bytes.Equal(want, after) || got.Became != "placed" {
		t.Fatalf("content changed beyond status: %s", after)
	}
	st, err := os.Stat(filepath.Join(s.GridHome, "projects", "second", "notes", r.Note))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", st.Mode().Perm())
	}
}

func TestMoveStatusSpellings(t *testing.T) {
	for _, tc := range []struct{ before, after string }{
		{`"new"`, `"placed"`},
		{`"n\x65w"`, `"placed"`},
		{"!!str new", "!!str placed"},
		{"&state new", "&state placed"},
		{"|-\n  new", "|-\n  placed"},
		{">-\n  new", ">-\n  placed"},
	} {
		t.Run(tc.before, func(t *testing.T) {
			s, r, path, before := moveFixture(t)
			before = bytes.Replace(before, []byte("status: new"), []byte("status: "+tc.before), 1)
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Move(r.ID, "example", "second"); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(filepath.Join(s.GridHome, "projects", "second", "notes", r.Note))
			if err != nil {
				t.Fatal(err)
			}
			expected := bytes.Replace(before, []byte("status: "+tc.before), []byte("status: "+tc.after), 1)
			if !bytes.Equal(expected, after) {
				t.Fatalf("status spelling changed other bytes: %s", after)
			}
		})
	}
}
