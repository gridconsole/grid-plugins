package plan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func legacyText(t *testing.T, name string) string {
	t.Helper()
	return fixture(t, "legacy/"+name+".md")
}

// snapshot lists every path under dir with its size, mode and mtime.
func snapshot(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		out = append(out, fmt.Sprintf("%s|%s|%s|%d", rel, info.Mode(), info.ModTime().Format(time.RFC3339Nano), info.Size()))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func states(items []Item) map[string]string {
	m := map[string]string{}
	for _, item := range items {
		m[item.ID] = item.State
	}
	return m
}

func TestStoreItemsCurrentPlanOnly(t *testing.T) {
	s := testStore(t)
	text := fixture(t, "plan.md")
	put(t, filepath.Join(s.TaskDir, "plan.md"), text)
	got, err := s.Items()
	if err != nil {
		t.Fatal(err)
	}
	want, err := Items(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		t.Fatal("fixture has no items")
	}
	// No items.md, no subagents folder: the legacy overlay sets only State=open.
	want = WithLegacyState(want, "", nil)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	plain, _ := Items(text)
	for i := range plain {
		if plain[i].State != "open" || got[i].State != "open" {
			t.Fatalf("item %s state %q / %q, want open", plain[i].ID, plain[i].State, got[i].State)
		}
		plain[i].State = got[i].State
	}
	if !reflect.DeepEqual(got, plain) {
		t.Fatalf("Store.Items differs from plan.Items(text) beyond State")
	}
}

func TestStoreItemsFoldsLegacyStateWithoutWriting(t *testing.T) {
	s := testStore(t)
	put(t, filepath.Join(s.TaskDir, "plan.md"), fixture(t, "plan.md"))
	put(t, filepath.Join(s.TaskDir, "items.md"), "# Items\n\n## i-1 · done · Add tag filter\n")
	subagents := filepath.Join(s.TaskDir, "subagents")
	if err := os.Mkdir(subagents, 0o700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(subagents, "two.md"), "---\nstatus: done\nitem: 2\n---\nbody\n")
	put(t, filepath.Join(subagents, "four.md"), "---\nstatus: running\nitem: 4\n---\nbody\n")
	// A .txt brief is skipped, even one that says running for item 2.
	put(t, filepath.Join(subagents, "two-running.txt"), "---\nstatus: running\nitem: 2\n---\nbody\n")

	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	for _, p := range []string{"plan.md", "items.md", "subagents", "subagents/two.md", "subagents/four.md", "subagents/two-running.txt"} {
		if err := os.Chtimes(filepath.Join(s.TaskDir, p), old, old); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshot(t, s.TaskDir)

	got, err := s.Items()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"i-1": "done", "i-2": "open", "i-4": "working"}
	if !reflect.DeepEqual(states(got), want) {
		t.Fatalf("states %v, want %v", states(got), want)
	}
	if after := snapshot(t, s.TaskDir); !reflect.DeepEqual(before, after) {
		t.Fatalf("task folder changed:\nbefore %v\nafter  %v", before, after)
	}
}

func TestStoreItemsErrors(t *testing.T) {
	t.Run("no plan", func(t *testing.T) {
		if _, err := testStore(t).Items(); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("got %v, want a not-exist error", err)
		}
	})
	t.Run("task folder missing", func(t *testing.T) {
		if _, err := (Store{}).Items(); err == nil {
			t.Fatal("empty TaskDir gave no error")
		}
	})
	t.Run("plan does not parse", func(t *testing.T) {
		s := testStore(t)
		put(t, filepath.Join(s.TaskDir, "plan.md"), "---\nfront: matter\n---\n")
		_, err := s.Items()
		want := "plan line 1: front matter is refused; use checklist items with jobs lists"
		if err == nil || err.Error() != want {
			t.Fatalf("got %v, want %q", err, want)
		}
	})
	t.Run("items.md is a directory", func(t *testing.T) {
		s := testStore(t)
		put(t, filepath.Join(s.TaskDir, "plan.md"), fixture(t, "plan.md"))
		if err := os.Mkdir(filepath.Join(s.TaskDir, "items.md"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := s.Items()
		if err == nil || err.Error() != "items.md must be a regular file" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("directory named x.md in subagents", func(t *testing.T) {
		s := testStore(t)
		put(t, filepath.Join(s.TaskDir, "plan.md"), fixture(t, "plan.md"))
		if err := os.MkdirAll(filepath.Join(s.TaskDir, "subagents", "x.md"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := s.Items()
		if err == nil || err.Error() != "x.md must be a regular file" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("subagents is a regular file", func(t *testing.T) {
		// childDirectory refuses a non-directory: an error, not read as absent.
		s := testStore(t)
		put(t, filepath.Join(s.TaskDir, "plan.md"), fixture(t, "plan.md"))
		put(t, filepath.Join(s.TaskDir, "subagents"), "not a folder")
		_, err := s.Items()
		if err == nil || err.Error() != "subagents must be a real directory" {
			t.Fatalf("got %v", err)
		}
	})
}

func TestStoreDirectoryErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	put(t, file, "x")
	if _, err := (Store{TaskDir: file}).Read(); err == nil || err.Error() != "task path is not a directory" {
		t.Fatalf("file as task folder: %v", err)
	}
	if _, err := (Store{TaskDir: filepath.Join(t.TempDir(), "missing")}).Read(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing task folder: %v", err)
	}
}

func TestAtomicWriteRefusesNonRegularTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "plan.md")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(target, []byte("x")); err == nil || err.Error() != "plan.md must be a regular file" {
		t.Fatalf("got %v", err)
	}
	if err := atomicWrite(filepath.Join(dir, "missing", "plan.md"), []byte("x")); err == nil {
		t.Fatal("write into a missing folder gave no error")
	}
	ok := filepath.Join(dir, "ok.md")
	if err := atomicWrite(ok, []byte("one")); err != nil || get(t, ok) != "one" {
		t.Fatalf("new file: %v", err)
	}
	if err := atomicWrite(ok, []byte("two")); err != nil || get(t, ok) != "two" {
		t.Fatalf("replace: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".plan-") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestMigrateCurrentPlanIsNoOp(t *testing.T) {
	s := testStore(t)
	path := filepath.Join(s.TaskDir, "plan.md")
	text := fixture(t, "plan.md")
	put(t, path, text)
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, s.TaskDir)
	written, form, err := s.Migrate(false)
	if err != nil || form != "current" || written != (Written{}) {
		t.Fatalf("got %+v %q %v", written, form, err)
	}
	if get(t, path) != text {
		t.Fatal("plan.md bytes changed")
	}
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("mtime changed: %v %v", info.ModTime(), err)
	}
	if after := snapshot(t, s.TaskDir); !reflect.DeepEqual(before, after) {
		t.Fatalf("task folder changed:\n%v\n%v", before, after)
	}
}

func TestMigrateLegacyPlan(t *testing.T) {
	t.Run("form B converts", func(t *testing.T) {
		s := testStore(t)
		path := filepath.Join(s.TaskDir, "plan.md")
		put(t, path, legacyText(t, "form-b"))
		written, form, err := s.Migrate(false)
		if err != nil || form != "B" {
			t.Fatalf("got %+v %q %v", written, form, err)
		}
		now := get(t, path)
		if _, err := Items(now); err != nil {
			t.Fatalf("migrated plan does not parse: %v", err)
		}
		if written.Hash == "" || written.Hash != Hash(now) {
			t.Fatalf("Written.Hash %q, want %q", written.Hash, Hash(now))
		}
		// A second run finds a current plan and does nothing.
		again, form, err := s.Migrate(false)
		if err != nil || form != "current" || again != (Written{}) {
			t.Fatalf("second run: %+v %q %v", again, form, err)
		}
	})
	t.Run("form A has no Used or Recall line and is refused", func(t *testing.T) {
		s := testStore(t)
		path := filepath.Join(s.TaskDir, "plan.md")
		text := legacyText(t, "form-a")
		put(t, path, text)
		written, form, err := s.Migrate(false)
		want := "plan write requires Used: <note> or Recall: nothing found"
		if err == nil || err.Error() != want || form != "A" || written != (Written{}) {
			t.Fatalf("got %+v %q %v", written, form, err)
		}
		if get(t, path) != text {
			t.Fatal("plan.md changed")
		}
	})
	t.Run("neither current nor legacy", func(t *testing.T) {
		s := testStore(t)
		path := filepath.Join(s.TaskDir, "plan.md")
		text := "---\nnot: a plan\n"
		put(t, path, text)
		written, _, err := s.Migrate(true)
		if err == nil || written != (Written{}) {
			t.Fatalf("got %+v %v, want an Upgrade error", written, err)
		}
		if _, _, upErr := Upgrade(text); upErr == nil || upErr.Error() != err.Error() {
			t.Fatalf("Migrate error %v is not the Upgrade error %v", err, upErr)
		}
		if get(t, path) != text {
			t.Fatal("plan.md changed")
		}
	})
	t.Run("no plan", func(t *testing.T) {
		s := testStore(t)
		_, form, err := s.Migrate(false)
		if !errors.Is(err, os.ErrNotExist) || form != "" {
			t.Fatalf("got %q %v", form, err)
		}
	})
}

func TestDigestCoversLines(t *testing.T) {
	plan := fixture(t, "plan.md")
	// Line numbers in testdata/plan.md: 1 title, 3 "## Where it stands", 5-6 its prose,
	// 19 a ticked step, 31 and 32 plain step lines.
	for _, tc := range []struct {
		name        string
		text        string
		first, last int
		want        bool
	}{
		{"first below one", plan, 0, 1, false},
		{"last before first", plan, 3, 2, false},
		{"negative", plan, -2, 5, false},
		{"plain step lines", plan, 31, 32, true},
		{"title line", plan, 1, 1, true},
		{"ticked step line", plan, 19, 19, true},
		{"ticked step with its prose", plan, 19, 23, true},
		{"status prose", plan, 5, 5, false},
		{"range that includes status prose", plan, 2, 8, false},
		{"stands heading", plan, 3, 3, false},
		{"past the end", plan, 100, 101, false},
		{"trimmed leading blank line", "\n# Plan\n\n- [ ] 1 x\n", 1, 1, false},
		{"leading blank line within a range", "\n# Plan\n\n- [ ] 1 x\n", 1, 2, false},
		{"line after the trimmed blank", "\n# Plan\n\n- [ ] 1 x\n", 2, 2, true},
		{"trimmed trailing blank line", "# Plan\n\n- [ ] 1 x\n\n\n", 4, 4, false},
		{"internal blank line", "# Plan\n\n- [ ] 1 x\n", 1, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := DigestCoversLines(tc.text, tc.first, tc.last); got != tc.want {
				t.Fatalf("DigestCoversLines(%d, %d) = %v, want %v", tc.first, tc.last, got, tc.want)
			}
		})
	}
}
