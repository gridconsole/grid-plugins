package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"go.yaml.in/yaml/v4"
)

// The reader takes a value from its key's own line, and a "- item" list under an
// empty one; an indented continuation line is invisible to it. The dump joins
// that line into the value, so a write could change a key the edit never set.
var hiddenValues = []struct{ name, key, header string }{
	{"step continuation", "step", "size: normal\n" + inlineList + "step: plan\n  build\n"},
	{"step on the next line", "step", "size: normal\n" + inlineList + "step:\n  build\n"},
	{"size on the next line", "size", "size:\n  large\nworkflow: normal\nstep: plan\n"},
	{"status continuation", "status", "size: normal\nworkflow: normal\nstatus: waiting\n  running\n"},
	{"unlisted key continuation", "next", "size: normal\nworkflow: normal\nnext: Review it\n  twice\n"},
}

// headerTask writes a task.md whose header is exactly header between the fixed keys.
func headerTask(t *testing.T, task Task, header, state string) string {
	t.Helper()
	path := filepath.Join(task.Path, "task.md")
	write(t, path, fmt.Sprintf("---\nid: %s\ntitle: %s\nproject: %s\n%sstate: %s\ncreated: %s\n---\n", task.ID, task.Title, task.Project, header, state, task.Created))
	return path
}

func readTask(t *testing.T, path string) (string, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	history, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "history.md"))
	return string(data), string(history)
}

// refusedKey asserts a write was refused for key and left both files as they were.
func refusedKey(t *testing.T, err error, key, path, text, history string) {
	t.Helper()
	if err == nil {
		t.Fatalf("the write was not refused; task.md:\n%s", taskText(t, path))
	}
	for _, want := range []string{fmt.Sprintf("key %q would read differently", key), "on the key's own line", "or remove the key, then retry", "nothing was written"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err=%q lacks %q", err, want)
		}
	}
	if gotText, gotHistory := readTask(t, path); gotText != text || gotHistory != history {
		t.Fatalf("a refused write changed the task:\n%s", gotText)
	}
}

func taskText(t *testing.T, path string) string {
	t.Helper()
	text, _ := readTask(t, path)
	return text
}

func TestChangeRefusesAKeyThatWouldReadDifferently(t *testing.T) {
	for _, c := range hiddenValues {
		for _, action := range []string{"close", "status"} {
			t.Run(c.name+"/"+action, func(t *testing.T) {
				s, task := writeFixture(t)
				path := headerTask(t, task, c.header, "waiting")
				text, history := readTask(t, path)
				before := taskfile.ParseFrontmatter(text).Data[c.key]
				_, err := s.Change(WriteOptions{ID: task.ID, Action: action, State: "running"})
				refusedKey(t, err, c.key, path, text, history)
				if got := taskfile.ParseFrontmatter(taskText(t, path)).Data[c.key]; fmt.Sprint(got) != fmt.Sprint(before) {
					t.Fatalf("the reader's %s went from %#v to %#v", c.key, before, got)
				}
			})
		}
	}
}

// Deliver sets step itself, so the key it must still refuse is one it does not set.
func TestDeliverRefusesAKeyThatWouldReadDifferently(t *testing.T) {
	s, task := deliverFixture(t, "")
	path := filepath.Join(task.Path, "task.md")
	text, _ := readTask(t, path)
	write(t, path, strings.Replace(text, "size: auto\n", "size:\n  large\n", 1))
	text, history := readTask(t, path)
	_, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"})
	refusedKey(t, err, "size", path, text, history)
}

func TestRewriteTaskLetsTheEditSetAKeyThatReadsDifferently(t *testing.T) {
	text := "---\nid: T-01\nworkflow: normal\nstep: plan\n  build\nstate: queued\n---\n"
	out, err := rewriteTask([]byte(text), func(mapping *yaml.Node) error {
		setScalar(mapping, "step", "ship", false)
		return nil
	}, nil)
	if err != nil || !strings.Contains(string(out), "step: ship\n") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

// Ordinary one-line headers, as every writer meets them, are not refused.
const ordinaryHeader = "size: normal\n# kept\n" + inlineList + "step: plan\nnext: 'Review it'\nextra: [one, two]\n"

// keepsHeader asserts the keys the writer did not set read as they did.
func keepsHeader(t *testing.T, before, after string, set ...string) {
	t.Helper()
	b, a := taskfile.ParseFrontmatter(before), taskfile.ParseFrontmatter(after)
	skip := map[string]bool{}
	for _, key := range set {
		skip[key] = true
	}
	for key, want := range b.Data {
		if !skip[key] && fmt.Sprint(a.Data[key]) != fmt.Sprint(want) {
			t.Errorf("%s read %#v, now %#v", key, want, a.Data[key])
		}
	}
}

func TestChangeStillWritesAnOrdinaryTask(t *testing.T) {
	s, task := writeFixture(t)
	path := headerTask(t, task, ordinaryHeader, "waiting")
	before := taskText(t, path)
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "status", State: "running"}); err != nil {
		t.Fatal(err)
	}
	after := taskText(t, path)
	if !strings.Contains(after, "state: running\n") || !strings.Contains(after, "# kept\n") {
		t.Fatalf("task.md:\n%s", after)
	}
	keepsHeader(t, before, after, "state")
}

func TestDeliverStillWritesAnOrdinaryTask(t *testing.T) {
	s, task := deliverFixture(t, "")
	path := filepath.Join(task.Path, "task.md")
	text, _ := readTask(t, path)
	write(t, path, strings.NewReplacer("workflow: ''\n", "", "size: auto\n", "").Replace(strings.Replace(text, "state: running\n", ordinaryHeader+"state: running\n", 1)))
	before := taskText(t, path)
	if _, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	after := taskText(t, path)
	if !strings.Contains(after, "step: ship\n") {
		t.Fatalf("task.md:\n%s", after)
	}
	keepsHeader(t, before, after, "state", "step", "gate", "next", "outcome", "updated")
}

func TestRenameStillWritesAnOrdinaryTask(t *testing.T) {
	s, task := writeFixture(t)
	path := headerTask(t, task, ordinaryHeader, "waiting")
	before := taskText(t, path)
	if _, err := s.Rename(RenameOptions{WriteOptions: WriteOptions{ID: task.ID}, Title: "Changed"}); err != nil {
		t.Fatal(err)
	}
	after := taskText(t, path)
	if !strings.Contains(after, "title: \"Changed\"\n") {
		t.Fatalf("task.md:\n%s", after)
	}
	keepsHeader(t, before, after, "title", "updated")
}

func TestApproveStillWritesAnOrdinaryTask(t *testing.T) {
	s, task := writeFixture(t)
	path := headerTask(t, task, ordinaryHeader, "waiting")
	before := taskText(t, path)
	event, err := s.Approve(WriteOptions{ID: task.ID}, setPlan(t, task, planV1))
	if err != nil {
		t.Fatal(err)
	}
	if after := taskText(t, path); after != before {
		t.Fatalf("approve changed task.md:\n%s", after)
	}
	// The result is posted after Approve; a task with no Feed identity gets its ulid
	// line when its Job is posted, and nothing else in the header changes.
	postApprovalResult(t, task.Path, event)
	id, _, err := FeedIdentity(task.Path)
	if err != nil {
		t.Fatal(err)
	}
	if after := strings.Replace(taskText(t, path), "ulid: "+id+"\n", "", 1); after != before {
		t.Fatalf("posting the result changed task.md beyond the ulid line:\n%s", after)
	}
	if !Approved(task.Path) {
		t.Fatal("not approved")
	}
}
