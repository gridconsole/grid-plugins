package cli

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/task"
)

// ijTitleNew runs `grid task new --json -- <words>` as the app's New task route
// does and returns the exit code and the made folder from the JSON path.
func ijTitleNew(t *testing.T, h *harness, words string) (int, string) {
	t.Helper()
	code := h.run(h.home, "task", "new", "--json", "--", words)
	if code != 0 {
		return code, ""
	}
	var made struct{ ID, Path string }
	if err := json.Unmarshal(h.out.Bytes(), &made); err != nil {
		t.Fatalf("task new --json printed %q: %v", h.out.String(), err)
	}
	return code, made.Path
}

// ijTitleMade makes one task from words and returns its task.md title and its
// folder path.
func ijTitleMade(t *testing.T, h *harness, words string) (string, string) {
	t.Helper()
	code, path := ijTitleNew(t, h, words)
	if code != 0 {
		t.Fatalf("task new %q: exit %d, stderr %q", words, code, h.errb.String())
	}
	got, err := task.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	return got.Title, path
}

// ijTitleFolders lists every folder under the Grid home whose name starts with prefix.
func ijTitleFolders(t *testing.T, root, prefix string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), prefix) {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestIjTitleFromFrontMatter(t *testing.T) {
	for _, tc := range []struct {
		name, words, title, folder string
	}{
		{"row1-front-matter-title",
			"---\ntitle: \"Fix: the login page\"\npriority: normal\nrequested_by: me\n---\n\nUsers cannot sign in\n",
			"Fix: the login page", "T-01-fix-the-login-page"},
		{"row1-no-title-uses-body",
			"---\npriority: normal\nrequested_by: me\n---\n\nUsers cannot sign in\n",
			"Users cannot sign in", "T-01-users-cannot-sign-in"},
		{"row2-blank-lines-before-block",
			"\n   \n---\npriority: normal\n---\n\n\nReal title\nmore\n",
			"Real title", "T-01-real-title"},
		{"row3-title-and-blank-body",
			"---\ntitle: Fix login\n---\n   \n\t\n",
			"Fix login", "T-01-fix-login"},
		{"row4-long-body-line",
			"---\npriority: normal\n---\n" + strings.Repeat("a", 300),
			strings.Repeat("a", 120), ""},
		{"row4-long-title",
			"---\ntitle: " + strings.Repeat("b", 300) + "\n---\nbody\n",
			strings.Repeat("b", 120), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			title, path := ijTitleMade(t, h, tc.words)
			if title != tc.title {
				t.Errorf("%s: title = %q, want %q", tc.name, title, tc.title)
			}
			if tc.folder != "" && filepath.Base(path) != tc.folder {
				t.Errorf("%s: folder = %q, want %q", tc.name, filepath.Base(path), tc.folder)
			}
		})
	}
}

func TestIjTitleOnlyFrontMatterIsRefused(t *testing.T) {
	h := newHarness(t)
	code, _ := ijTitleNew(t, h, "---\npriority: normal\n---\n  \n")
	if code != 2 {
		t.Errorf("row3-only-front-matter: exit = %d, want 2; stdout %q", code, h.out.String())
	}
	if want := "task words hold only front matter and no title"; !strings.Contains(h.errb.String(), want) {
		t.Errorf("row3-only-front-matter: stderr = %q, want it to hold %q", h.errb.String(), want)
	}
	if made := ijTitleFolders(t, h.gridHome, "T-01"); len(made) != 0 {
		t.Errorf("row3-only-front-matter: task folders written: %v", made)
	}
}

func TestIjTitleExplicitKeepsWordsAndOldTasks(t *testing.T) {
	h := newHarness(t)
	words := "---\ntitle: \"  Fix    login  \"\n---\nFirst line of the body\n"
	title, first := ijTitleMade(t, h, words)
	if title != "Fix login" {
		t.Errorf("row5-explicit-title: title = %q, want %q", title, "Fix login")
	}
	if !strings.HasSuffix(filepath.Base(first), "-fix-login") {
		t.Errorf("row5-explicit-title: folder = %q, want it to end -fix-login", filepath.Base(first))
	}
	intent, err := os.ReadFile(filepath.Join(first, "intent.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := task.IntentHead + words; string(intent) != want {
		t.Errorf("row5-intent: intent.md = %q, want %q", intent, want)
	}
	record, err := os.ReadFile(filepath.Join(first, "task.md"))
	if err != nil {
		t.Fatal(err)
	}

	if title, _ := ijTitleMade(t, h, "Plain words\nsecond line"); title != "Plain words" {
		t.Errorf("row5-no-block: title = %q, want %q", title, "Plain words")
	}
	ijTitleMade(t, h, "---\ntitle: Another one\n---\nbody\n")

	again, err := os.ReadFile(filepath.Join(first, "task.md"))
	if err != nil {
		t.Fatalf("row5-old-task: %v", err)
	}
	if !bytes.Equal(again, record) {
		t.Errorf("row5-old-task: task.md changed:\n%s\nwant\n%s", again, record)
	}
	if folders := ijTitleFolders(t, h.gridHome, "T-01"); len(folders) != 1 || folders[0] != first {
		t.Errorf("row5-old-task: T-01 folders = %v, want [%s]", folders, first)
	}
}
