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

// jwPrCrash parks J-1 the way jrParkCrash does: one Job, a tick, then three
// times ack, crash exit and tick.
func jwPrCrash(t *testing.T) *fixture {
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

// jwPrFiles lists the Q-n.md names in the task's questions folder.
func jwPrFiles(t *testing.T, f *fixture) []string {
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

// jwPrPosts returns the question posts on the Feed.
func jwPrPosts(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{Type: "question"})
	if err != nil {
		t.Fatal(err)
	}
	return read.Records
}

// jwPrDecode reads one question file the way a reader of the folder does: the
// JSON front matter between the two --- lines, with no help from the Feed.
func jwPrDecode(t *testing.T, f *fixture, name string) question.Record {
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

// jwPrCrashAfterPost leaves J-1 as a crash between the question post and the
// park leaves it: the question is on the Feed and its marker is in the folder,
// but the question file is gone. It returns the post and the failure the park
// is asked again for.
func jwPrCrashAfterPost(t *testing.T) (*fixture, feed.Entry, string) {
	t.Helper()
	f := jwPrCrash(t)
	posts := jwPrPosts(t, f)
	if len(posts) != 1 {
		t.Fatalf("%d question posts on the Feed, want 1", len(posts))
	}
	if posts[0].Wake == nil || !strings.HasSuffix(*posts[0].Wake, "/1") {
		t.Fatalf("question wake %v, want <failure>/1", posts[0].Wake)
	}
	if err := os.Remove(filepath.Join(f.s.Feed.TaskDir, "questions", "Q-1.md")); err != nil {
		t.Fatal(err)
	}
	if names := jwPrFiles(t, f); len(names) != 0 {
		t.Fatalf("question files %v after the crash, want none", names)
	}
	return f, posts[0], strings.TrimSuffix(*posts[0].Wake, "/1")
}

// jwPrPark runs the park for J-1 again, on a fresh read of the Feed.
func jwPrPark(t *testing.T, f *fixture, failure string) error {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	return f.s.park(snap, snap.State.Jobs["J-1"], &jobMemory{Failure: failure, Failures: []string{failure}})
}

// Row 1 and row 3: the park that reuses the open question writes its file back,
// with the same id, and writes one file only.
func TestJwPrReuseWritesQuestionFile(t *testing.T) {
	f, post, failure := jwPrCrashAfterPost(t)
	if err := jwPrPark(t, f, failure); err != nil {
		t.Fatal(err)
	}
	if got := f.state(t).Status; got != contract.Parked {
		t.Fatalf("J-1 status %q, want %q", got, contract.Parked)
	}
	if posts := jwPrPosts(t, f); len(posts) != 1 || posts[0].ID != post.ID {
		t.Fatalf("question posts %+v after the park, want only %s", posts, post.ID)
	}
	if names := jwPrFiles(t, f); !slices.Equal(names, []string{"Q-1.md"}) {
		t.Fatalf("question files %v after the park, want [Q-1.md]", names)
	}
	r := jwPrDecode(t, f, "Q-1.md")
	if r.ID != "Q-1" || r.Post != post.ID || r.Job != "J-1" || r.State != "open" {
		t.Fatalf("file id %q post %q job %q state %q, want Q-1 %s J-1 open", r.ID, r.Post, r.Job, r.State, post.ID)
	}
	// A second park is the same: still one file, still the one post.
	if err := jwPrPark(t, f, failure); err != nil {
		t.Fatal(err)
	}
	if names := jwPrFiles(t, f); !slices.Equal(names, []string{"Q-1.md"}) {
		t.Fatalf("question files %v after a second park, want [Q-1.md]", names)
	}
	if posts := jwPrPosts(t, f); len(posts) != 1 {
		t.Fatalf("%d question posts after a second park, want 1", len(posts))
	}
}

// Row 2: with no TaskID the reuse branch projects nothing, as on the base.
func TestJwPrReuseNoTaskIDProjectsNothing(t *testing.T) {
	f, _, failure := jwPrCrashAfterPost(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	// A Feed store with no TaskID takes the stable id from task.md on a write, so
	// the same id stands there; only the park's Store has no TaskID set.
	if err := os.WriteFile(filepath.Join(f.s.Feed.TaskDir, "task.md"), []byte("---\nid: "+taskID+"\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := recordCount(t, f)
	f.s.Feed.TaskID = ""
	err = f.s.park(snap, snap.State.Jobs["J-1"], &jobMemory{Failure: failure, Failures: []string{failure}})
	f.s.Feed.TaskID = taskID
	if err != nil {
		t.Fatal(err)
	}
	if after := recordCount(t, f); after != before+1 {
		t.Fatalf("records %d -> %d, want the park's one control post", before, after)
	}
	if names := jwPrFiles(t, f); len(names) != 0 {
		t.Fatalf("question files %v with TaskID empty, want none", names)
	}
	if posts := jwPrPosts(t, f); len(posts) != 1 {
		t.Fatalf("%d question posts, want 1", len(posts))
	}
}
