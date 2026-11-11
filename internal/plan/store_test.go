package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func testStore(t *testing.T) Store {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fslock is not implemented on Windows")
	}
	return Store{TaskDir: t.TempDir()}
}

func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWithHash(t *testing.T) {
	s := testStore(t)
	if err := s.WithHash(func(string) error { t.Fatal("called without a plan"); return nil }); !errors.Is(err, ErrNoPlan) {
		t.Fatalf("missing plan: %v", err)
	}
	written, err := s.Write(fixture(t, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := ""
	if err := s.WithHash(func(hash string) error { got = hash; return nil }); err != nil || got != written.Hash {
		t.Fatalf("%q, want %q: %v", got, written.Hash, err)
	}
	// The hash follows plan.md itself, however it was edited.
	put(t, filepath.Join(s.TaskDir, "plan.md"), "# Plan, rev 1 \u00b7 000000\n\nedited\n")
	if err := s.WithHash(func(hash string) error { got = hash; return nil }); err != nil || got == written.Hash {
		t.Fatalf("%q after an edit: %v", got, err)
	}
}

func TestWriteRevisionAndArchive(t *testing.T) {
	s := testStore(t)
	base := fixture(t, "plan.md")
	first, err := s.Write(base)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.Hash != Hash(base) {
		t.Fatalf("%+v", first)
	}
	previous, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Write(strings.ReplaceAll(base, "[ ]", "[x]"))
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != 2 || second.Hash != first.Hash {
		t.Fatalf("%+v", second)
	}
	archive := filepath.Join(s.TaskDir, "plan-revisions", "1.md")
	if got := get(t, archive); got != previous {
		t.Fatal("archive did not preserve exact bytes")
	}
	current, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if rev, err := Revision(current); err != nil || rev != 2 || Hash(current) != second.Hash {
		t.Fatalf("%s: %v", current, err)
	}
	// An archive left by an interrupted write is safe to reuse if identical.
	put(t, filepath.Join(s.TaskDir, "plan-revisions", "2.md"), current)
	if _, err := s.Write(base); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := get(t, archive); got != previous {
		t.Fatal("later write changed old archive")
	}
	entries, err := os.ReadDir(s.TaskDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".plan-") {
			t.Errorf("temporary file left: %s", entry.Name())
		}
	}
}

func TestWriteIdenticalIsNoOp(t *testing.T) {
	s := testStore(t)
	base := fixture(t, "plan.md")
	first, err := s.Write(base)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Write(base)
	if err != nil || again != first {
		t.Fatalf("%+v %v", again, err)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "plan-revisions", "1.md")); !os.IsNotExist(err) {
		t.Fatalf("archive written for identical plan: %v", err)
	}
}

func TestRecallGate(t *testing.T) {
	for _, text := range []string{
		"Used: brain:project/tag-matching", "Recall: nothing found", "- **Used:** brain:project/tag-matching", "**Recall:** nothing found", "  + __Used__: conventions/tags", "used: conventions/tags",
		"```text\nExample\n```\nRecall: nothing found",
	} {
		if !HasRecall(text) {
			t.Errorf("refused %q", text)
		}
	}
	for _, text := range []string{
		"", "Used:", "Used: **", "Recall:", "Recall: maybe later", "Recall: nothing found yet", "This mentions Used: somewhere", "> Used: quoted",
		"```markdown\nUsed: example\n```", "~~~\nRecall: nothing found\n~~~", "````\n```\nUsed: example\n````", "<!--\nUsed: hidden\n-->", "Used: <!-- hidden -->", "    Used: code", "\tRecall: nothing found",
	} {
		if HasRecall(text) {
			t.Errorf("accepted %q", text)
		}
	}
}

func TestWriteRefusalsLeavePlanUntouched(t *testing.T) {
	for name, input := range map[string]string{
		"missing recall": "# Plan\n- [ ] 1 One",
		"empty citation": "# Plan\nUsed: \n- [ ] 1 One",
		"example only":   "# Plan\n```\nRecall: nothing found\n```\n- [ ] 1 One",
		"duplicate item": "# Plan\nUsed: conventions/tags\n- [ ] 1 One\n- [ ] 1 Two",
		"bad field":      "# Plan\nRecall: nothing found\n- [ ] 1 One\n  write: [broken",
	} {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			original := fixture(t, "plan.md")
			put(t, filepath.Join(s.TaskDir, "plan.md"), original)
			if _, err := s.Write(input); err == nil {
				t.Fatal("write accepted")
			}
			if got := get(t, filepath.Join(s.TaskDir, "plan.md")); got != original {
				t.Fatal("plan changed on refusal")
			}
			entries, err := os.ReadDir(s.TaskDir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("unexpected side effects: %v %v", entries, err)
			}
		})
	}
}

func TestInvalidCurrentAndArchive(t *testing.T) {
	for _, heading := range []string{"# Plan", "# Plan, rev 0 · abcdef", "# Plan, rev 18446744073709551615 · abcdef"} {
		s := testStore(t)
		put(t, filepath.Join(s.TaskDir, "plan.md"), heading)
		if _, err := s.Write(fixture(t, "plan.md")); err == nil {
			t.Fatalf("accepted %s", heading)
		}
		if get(t, filepath.Join(s.TaskDir, "plan.md")) != heading {
			t.Fatal("changed invalid current plan")
		}
	}
	s := testStore(t)
	base := fixture(t, "plan.md")
	if _, err := s.Write(base); err != nil {
		t.Fatal(err)
	}
	current, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(s.TaskDir, "plan-revisions"), 0o700); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(s.TaskDir, "plan-revisions", "1.md")
	put(t, archive, "different archive")
	if _, err := s.Write(strings.ReplaceAll(base, "[ ]", "[x]")); err == nil {
		t.Fatal("overwrote conflicting archive")
	}
	if get(t, archive) != "different archive" || get(t, filepath.Join(s.TaskDir, "plan.md")) != current {
		t.Fatal("conflict changed files")
	}
}

func TestStorageSymlinks(t *testing.T) {
	for _, name := range []string{"plan.md", ".plan.lock", "plan-revisions", "plan-revisions/1.md"} {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			if _, err := s.Write(fixture(t, "plan.md")); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			target := filepath.Join(outside, "untouched")
			put(t, target, "outside data")
			path := filepath.Join(s.TaskDir, filepath.FromSlash(name))
			if name == "plan-revisions" {
				target = outside
			}
			if strings.Contains(name, "/") {
				if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Write(fixture(t, "revised.md")); err == nil {
				t.Fatal("followed storage symlink")
			}
			if get(t, filepath.Join(outside, "untouched")) != "outside data" {
				t.Fatal("changed symlink target")
			}
		})
	}
	// A symlink at or above the task root is intentional and must work on macOS.
	s := testStore(t)
	alias := filepath.Join(t.TempDir(), "task-link")
	if err := os.Symlink(s.TaskDir, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{TaskDir: alias}).Write(fixture(t, "plan.md")); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentWritersAndReaders(t *testing.T) {
	s := testStore(t)
	base := fixture(t, "plan.md")
	if _, err := s.Write(base); err != nil {
		t.Fatal(err)
	}
	const writers = 12
	results := make(chan Written, writers)
	errors := make(chan error, writers+1)
	stop := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
				text, err := s.Read()
				if err != nil {
					errors <- err
					return
				}
				if _, err := Revision(text); err != nil {
					errors <- err
					return
				}
				if !strings.HasSuffix(lines(text)[0], Hash(text)) {
					errors <- fmt.Errorf("partial plan read")
					return
				}
			}
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := s.Write(base + fmt.Sprintf("\nRevision note %d\n", i))
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}(i)
	}
	wg.Wait()
	close(stop)
	<-readerDone
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	close(results)
	seen := map[uint64]bool{}
	for result := range results {
		if seen[result.Revision] {
			t.Errorf("duplicate revision %d", result.Revision)
		}
		seen[result.Revision] = true
	}
	if len(seen) != writers {
		t.Fatalf("only %d writes completed", len(seen))
	}
	for i := 1; i <= writers; i++ {
		text := get(t, filepath.Join(s.TaskDir, "plan-revisions", fmt.Sprintf("%d.md", i)))
		if n, err := Revision(text); err != nil || n != uint64(i) {
			t.Fatalf("archive %d: %v", i, err)
		}
	}
}

func TestDiffPositionAndFields(t *testing.T) {
	base := "# Plan\n- [ ] 1 First\n  jobs:\n    - write: [src/one/**]\n      after: []\n      review: L1\n      risk: []\n      verify: pass\n- [ ] 2 Second\n  jobs: []\n"
	for _, pair := range [][2]string{{"src/one", "src/two"}, {"after: []", "after: [J-3]"}, {"L1", "L2"}, {"risk: []", "risk: [security]"}, {"verify: pass", "verify: check"}} {
		d, err := Compare(base, strings.ReplaceAll(base, pair[0], pair[1]))
		if err != nil || len(d.Changed) != 1 {
			t.Fatalf("%v: %+v %v", pair, d, err)
		}
	}
	d, err := Compare("# Plan\n- [ ] 1 One\n  jobs: []\n- [ ] 2 Two\n  jobs: []", "# Plan\n- [ ] 2 Two\n  jobs: []\n- [ ] 1 One\n  jobs: []")
	if err != nil || len(d.Changed) != 2 || d.Changed[0].BeforePosition != 2 {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestReadCommandsCreateNothing(t *testing.T) {
	s := testStore(t)
	put(t, filepath.Join(s.TaskDir, "plan.md"), fixture(t, "plan.md"))
	if _, err := s.Items(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(s.TaskDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read created files: %v %v", entries, err)
	}
}
