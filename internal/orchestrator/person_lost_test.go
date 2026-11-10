package orchestrator

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func personJobBody(t *testing.T, id string) map[string]any {
	body := jobBody(t, id)
	body["role"] = "Person"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	return body
}

func messages(t *testing.T, f *fixture) []string {
	t.Helper()
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range r.Records {
		if text, _ := e.Body["text"].(string); e.Type == "post" && text != "" {
			out = append(out, text)
		}
	}
	return out
}

// Row 1: a task that has had nothing posted has no Feed folder; a person's first
// Job post makes it, with mode 0700, and writes the Job.
func TestPersonFirstJobOfFreshTask(t *testing.T) {
	store := feed.Store{TaskDir: t.TempDir(), TaskID: taskID}
	if _, err := os.Stat(store.Dir()); !os.IsNotExist(err) {
		t.Fatalf("the Feed folder exists before the first post: %v", err)
	}
	rec, err := PostPerson(store, personWriterOf("member"), feed.Input{Type: "job", Body: personJobBody(t, "J-1"), Job: "J-1"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Lease != nil || rec.Fence != 0 {
		t.Fatalf("want an unleased record, got lease=%v fence=%d", rec.Lease, rec.Fence)
	}
	info, err := os.Stat(store.Dir())
	if err != nil || !info.IsDir() {
		t.Fatalf("the Feed folder was not made: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		t.Fatalf("the Feed folder is %o, want 700", info.Mode().Perm())
	}
	st, err := store.State()
	if err != nil || st.Jobs["J-1"] == nil {
		t.Fatalf("the Job is not in the Feed: %v", err)
	}
}

// exclusive makes the Feed folder but never the task folder: a path that is not
// a task stays as it was.
func TestExclusiveDoesNotMakeTheTaskFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no-such-task")
	s := Store{Feed: feed.Store{TaskDir: dir, TaskID: taskID}}
	if err := s.exclusive(func() error { return nil }); err == nil {
		t.Fatal("exclusive ran without a task folder")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("exclusive made the task folder: %v", err)
	}
}

// Row 2: two people answer one person Job before a tick. The tick keeps the
// first, and writes one message that names the second and says it was not used.
func TestSecondPersonAnswerIsNamedNotUsed(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	sam := feed.Writer{Author: feed.Author{ID: "sam", Role: "owner"}, Worker: "sam@" + machine}
	if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-1", "Call it Lark.", f.now); err != nil {
		t.Fatal(err)
	}
	// Records are stamped to the second; the second answer must come after it.
	time.Sleep(1100 * time.Millisecond)
	late, err := AnswerPerson(f.s.Feed, sam, "J-1", "Call it Wren.", f.now)
	if err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	if got := jobStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s", got)
	}
	var taken, unused []string
	for _, m := range messages(t, f) {
		switch {
		case strings.Contains(m, "dana answered J-1 at ") && strings.Contains(m, "Call it Lark."):
			taken = append(taken, m)
		case strings.Contains(m, "was not used"):
			unused = append(unused, m)
		}
	}
	if len(taken) != 1 {
		t.Fatalf("want the first answer taken once, got %q", taken)
	}
	if len(unused) != 1 {
		t.Fatalf("want one message for the second answer, got %q", unused)
	}
	for _, want := range []string{"sam", "J-1", late.ID, "Call it Wren.", "dana answered first"} {
		if !strings.Contains(unused[0], want) {
			t.Fatalf("the message lacks %q: %s", want, unused[0])
		}
	}
	if strings.Contains(unused[0], "Call it Lark.") {
		t.Fatalf("the message names the answer that was used: %s", unused[0])
	}
	// Later passes add nothing: the Job is closed.
	f.tick(t)
	n := 0
	for _, m := range messages(t, f) {
		if strings.Contains(m, "was not used") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("a later pass wrote %d messages about the second answer, want 1", n)
	}
}

// One answer is used and nothing is said about another.
func TestSingleAnswerWritesNoNotice(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	if _, err := AnswerPerson(f.s.Feed, personWriterOf("member"), "J-1", "Call it Lark.", f.now); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	for _, m := range messages(t, f) {
		if strings.Contains(m, "was not used") {
			t.Fatalf("a lone answer left a notice: %s", m)
		}
	}
}
