package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/proof"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"go.yaml.in/yaml/v4"
)

// A writer rewrites only the lines of the keys it sets. Rename writes a title
// with its literal characters; a whole-header dump spells them differently
// (a non-BMP character as \UXXXXXXXX, which the plugin's reader does not decode),
// so the next write found the title "changed" and refused to deliver or close.
var keptTitles = []struct{ name, title string }{
	{"non-BMP emoji", "emoji 😀 x"},
	{"BMP non-ASCII", "café ✓ x"},
	{"quoted value", `say "hi" and 'bye' x`},
}

// setByWriters are the header keys Deliver sets or adds; every other line is its own.
var setByWriters = map[string]bool{"state": true, "step": true, "gate": true, "next": true, "outcome": true, "updated": true}

// untouchedLines is the header's lines, less those of the given keys.
func untouchedLines(t *testing.T, text string, set map[string]bool) []string {
	t.Helper()
	fm := strings.SplitN(strings.TrimPrefix(text, "---\n"), "\n---\n", 2)[0]
	var out []string
	for _, line := range strings.Split(fm, "\n") {
		if key, _, ok := strings.Cut(line, ":"); ok && set[key] {
			continue
		}
		out = append(out, line)
	}
	return out
}

func sameLines(t *testing.T, what string, before, after []string) {
	t.Helper()
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("%s: the lines the writer did not set changed\nbefore:\n%s\nafter:\n%s", what, strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

// renamedTask is `grid task new` with the title, then `grid task rename`, as a
// person would, with a plan the person approved and the proof Deliver needs.
func renamedTask(t *testing.T, title string) (Store, Task, string) {
	t.Helper()
	s, home := storeFor(t)
	task, err := s.New(NewOptions{Cwd: home, Words: title, Project: "personal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rename(RenameOptions{WriteOptions: WriteOptions{ID: task.ID}, Title: title + " renamed"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(task.Path, "task.md")
	data, _ := os.ReadFile(path)
	write(t, path, strings.Replace(string(data), "state: queued", "state: running", 1))
	write(t, filepath.Join(task.Path, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  jobs: []\n")
	approveFixturePlan(t, s, task)
	if _, err := (proof.Store{TaskDir: task.Path}).Add(proof.Input{Item: "1", Text: "ok", Kind: "check", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	return s, task, path
}

func TestDeliverAndCloseKeepARenamedTitle(t *testing.T) {
	for _, c := range keptTitles {
		t.Run(c.name, func(t *testing.T) {
			s, task, path := renamedTask(t, c.title)
			renamed := taskText(t, path)
			if got := taskfile.ParseFrontmatter(renamed).Data["title"]; got != c.title+" renamed" {
				t.Fatalf("title reads %#v after the rename", got)
			}
			if _, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"}); err != nil {
				t.Fatalf("deliver: %v", err)
			}
			delivered := taskText(t, path)
			sameLines(t, "deliver", untouchedLines(t, renamed, setByWriters), untouchedLines(t, delivered, setByWriters))
			if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
				t.Fatalf("close: %v", err)
			}
			closed := taskText(t, path)
			sameLines(t, "close", untouchedLines(t, delivered, map[string]bool{"state": true}), untouchedLines(t, closed, map[string]bool{"state": true}))
			if got := taskfile.ParseFrontmatter(closed).Data; got["state"] != "closed" || got["title"] != c.title+" renamed" {
				t.Fatalf("state %#v, title %#v", got["state"], got["title"])
			}
		})
	}
}

// The same sequence with close alone, so a task that is not delivered is not bricked either.
func TestCloseKeepsARenamedTitle(t *testing.T) {
	for _, c := range keptTitles {
		t.Run(c.name, func(t *testing.T) {
			s, task, path := renamedTask(t, c.title)
			renamed := taskText(t, path)
			if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
				t.Fatalf("close: %v", err)
			}
			sameLines(t, "close", untouchedLines(t, renamed, map[string]bool{"state": true}), untouchedLines(t, taskText(t, path), map[string]bool{"state": true}))
		})
	}
}

// A line the edit did not set keeps its spelling, not only its meaning.
func TestRewriteTaskKeepsUntouchedLinesByteForByte(t *testing.T) {
	header := "id: T-01\n" +
		"title: \"emoji 😀 x\"\n" +
		"# a comment between keys\n" +
		"alias:   'it''s'   # kept as written\n" +
		"extra:   [one,   two]\n" +
		"next: \"café \\\"quoted\\\"\"\n" +
		"folded: >\n  one\n  two\n" +
		inlineList +
		"step: plan\n"
	for name, newline := range map[string]string{"LF": "\n", "CRLF": "\r\n"} {
		t.Run(name, func(t *testing.T) {
			data := strings.ReplaceAll("---\n"+header+"state: queued # note\n---\nBody stays.\n", "\n", newline)
			out, err := rewriteTask([]byte(data), func(mapping *yaml.Node) error {
				setScalar(mapping, "state", "running", false)
				return nil
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.ReplaceAll("---\n"+header+"state: running # note\n---\nBody stays.\n", "\n", newline)
			if string(out) != want {
				t.Fatalf("got:\n%q\nwant:\n%q", out, want)
			}
		})
	}
}

// A key the edit adds is written after the others, before the closing line.
func TestRewriteTaskAddsAKeyAfterTheHeader(t *testing.T) {
	out, err := rewriteTask([]byte("---\nid: T-01\nstate: queued\n# last words\n---\nBody.\n"), func(mapping *yaml.Node) error {
		setScalar(mapping, "step", "ship", true)
		return nil
	}, nil)
	if want := "---\nid: T-01\nstate: queued\n# last words\nstep: ship\n---\nBody.\n"; err != nil || string(out) != want {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

// An edit that changes a value node in place would be lost with its lines kept;
// it is refused rather than dropped.
func TestRewriteTaskRefusesAnEditMadeInPlace(t *testing.T) {
	data := []byte("---\nid: T-01\nstate: queued\n---\n")
	out, err := rewriteTask(data, func(mapping *yaml.Node) error {
		mapping.Content[3].Value = "closed"
		return nil
	}, nil)
	if err == nil || out != nil || !strings.Contains(err.Error(), `key "state"`) {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

// The reader's view of every key the edit did not set is the same afterwards, in
// a header of every spelling the writers meet.
func TestRewriteTaskLeavesTheReadersViewAlone(t *testing.T) {
	for _, c := range keptTitles {
		t.Run(c.name, func(t *testing.T) {
			data := fmt.Sprintf("---\nid: T-01\ntitle: %q\n%sstate: queued\n---\n", c.title+" renamed", inlineList)
			out, err := rewriteTask([]byte(data), func(mapping *yaml.Node) error {
				setScalar(mapping, "state", "closed", false)
				return nil
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			keepsHeader(t, data, string(out), "state")
		})
	}
}
