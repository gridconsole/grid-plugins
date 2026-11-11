package orchestrator

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

// jrParkCrash parks J-1 the way TestParkGateReservesQuestionFileIDs does: one
// Job, a tick, then three times ack, crash exit and tick.
func jrParkCrash(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for range 3 {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	return f
}

// jrParkFiles lists the Q-n.md names in the task's questions folder.
func jrParkFiles(t *testing.T, f *fixture) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(f.s.Feed.TaskDir, "questions"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "Q-") && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	return names
}

// jrParkPosts returns the question posts on the Feed.
func jrParkPosts(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{Type: "question"})
	if err != nil {
		t.Fatal(err)
	}
	return read.Records
}

// jrParkDecode reads one question file the way a reader of the folder does: the
// JSON front matter between the two --- lines, with no help from the Feed.
func jrParkDecode(t *testing.T, f *fixture, name string) question.Record {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.s.Feed.TaskDir, "questions", name))
	if err != nil {
		t.Fatal(err)
	}
	body, _, ok := bytes.Cut(bytes.TrimPrefix(data, []byte("---\n")), []byte("\n---\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) || !ok {
		t.Fatalf("%s has no front matter", name)
	}
	var r question.Record
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

func TestJrParkWritesQuestionFile(t *testing.T) {
	f := jrParkCrash(t)
	if got := f.state(t).Status; got != contract.Parked {
		t.Fatalf("J-1 status %q, want %q", got, contract.Parked)
	}
	posts := jrParkPosts(t, f)
	if len(posts) != 1 {
		t.Fatalf("%d question posts on the Feed, want 1", len(posts))
	}
	if names := jrParkFiles(t, f); !slices.Equal(names, []string{"Q-1.md"}) {
		t.Fatalf("question files %v, want [Q-1.md]", names)
	}
	r := jrParkDecode(t, f, "Q-1.md")
	if r.ID != "Q-1" || r.Post != posts[0].ID {
		t.Fatalf("file id %q post %q, want Q-1 and the Feed post %q", r.ID, r.Post, posts[0].ID)
	}
	if r.Job != "J-1" || r.Who != "owner" || !r.Blocking {
		t.Fatalf("file job %q who %q blocking %v", r.Job, r.Who, r.Blocking)
	}
	if !slices.Equal(r.Options, []string{"Keep parked", "Retry"}) || r.Recommended != "Keep parked" {
		t.Fatalf("file options %v recommended %q", r.Options, r.Recommended)
	}
	if r.Gate.ID != "Q-1" || r.Gate.Kind != "person" || r.Gate.Default != "Keep parked" {
		t.Fatalf("file gate %+v", r.Gate)
	}
	if r.State != "open" {
		t.Fatalf("file state %q, want open", r.State)
	}
	if r.Question != "Three attempts made no progress. How should J-1 continue?" {
		t.Fatalf("file question %q", r.Question)
	}
	// The Store reads the same question back.
	listed, err := (question.Store{TaskDir: f.s.Feed.TaskDir, TaskID: taskID}).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != "Q-1" || listed[0].Post != posts[0].ID || listed[0].State != "open" {
		t.Fatalf("List %+v", listed)
	}
	// The id marker stays beside the file.
	if _, err := os.Stat(filepath.Join(f.s.Feed.TaskDir, "questions", "Q-1.feed")); err != nil {
		t.Fatal(err)
	}
}

func TestJrParkOneFile(t *testing.T) {
	f := jrParkCrash(t)
	f.tick(t)
	if names := jrParkFiles(t, f); !slices.Equal(names, []string{"Q-1.md"}) {
		t.Fatalf("question files %v after another tick, want [Q-1.md]", names)
	}
	if posts := jrParkPosts(t, f); len(posts) != 1 {
		t.Fatalf("%d question posts on the Feed after another tick, want 1", len(posts))
	}
}
