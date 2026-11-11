package question

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// kuMkNames lists the files in a task's questions folder, without the lock file.
func kuMkNames(t *testing.T, s Store) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(s.TaskDir, "questions"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.Name() != ".lock" {
			names = append(names, e.Name())
		}
	}
	return names
}

// kuMkWorker is the kb3 shape: a worker-run agent's ask. postsJob says yes for the
// role, and the Feed then refuses the question post (a worker may not post a
// person gate).
func kuMkWorker(s Store) Store {
	s.Writer.Author.Role = "worker"
	return s
}

// kuMkOwner is the same task asked by its owner, who may post a person gate.
func kuMkOwner(s Store, calls *jobCalls) Store {
	s.Jobs = calls.port("J-9", nil)
	return s
}

const kuMkRefusal = "may not post person gates"

// kuMkPorts hands out J-1, J-2, ... under a mutex, as the Job store does.
type kuMkPorts struct {
	mu    sync.Mutex
	posts int
}

func (p *kuMkPorts) port() JobPort {
	return JobPort{Post: func(_ feed.Store, _, _ feed.Writer, _ map[string]any) (string, error) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.posts++
		return fmt.Sprintf("J-%d", p.posts), nil
	}}
}

func TestKuMkFinishedAskLeavesNoMarker(t *testing.T) {
	s, _ := setup(t)
	calls := &jobCalls{}
	s.Jobs = calls.port("J-7", nil)
	rec, err := s.Ask(input())
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != "Q-1" {
		t.Fatalf("first ask is %s", rec.ID)
	}
	if got := kuMkNames(t, s); !slices.Equal(got, []string{"Q-1.md"}) {
		t.Fatalf("questions/ holds %v, want only Q-1.md", got)
	}
	rec, err = s.Ask(input())
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != "Q-2" {
		t.Fatalf("second ask is %s, want Q-2", rec.ID)
	}
	if got := kuMkNames(t, s); !slices.Equal(got, []string{"Q-1.md", "Q-2.md"}) {
		t.Fatalf("questions/ holds %v, want Q-1.md and Q-2.md", got)
	}
}

func TestKuMkRefusedAskLeavesNoMarker(t *testing.T) {
	s, _ := setup(t)
	calls := &jobCalls{}
	w := kuMkWorker(s)
	w.Jobs = calls.port("", nil)
	if _, err := w.Ask(input()); err == nil || !strings.Contains(err.Error(), kuMkRefusal) {
		t.Fatalf("err %v, want a refusal containing %q", err, kuMkRefusal)
	}
	if len(calls.posts) != 1 {
		t.Fatalf("Post called %d times, want 1 (postsJob said yes for a worker)", len(calls.posts))
	}
	if got := kuMkNames(t, s); len(got) != 0 {
		t.Fatalf("a refused ask left %v in questions/", got)
	}
	rec, err := kuMkOwner(s, calls).Ask(input())
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != "Q-1" {
		t.Fatalf("next ask is %s, want Q-1 (the refused ask took no id)", rec.ID)
	}
}

// A caller's own --job names another Job, not questions/Q-n.md, so a refused ask
// with no Job posted leaves no marker even then.
func TestKuMkCallerJobIsNotThePostedJob(t *testing.T) {
	s, _ := setup(t)
	calls := &jobCalls{}
	w := kuMkWorker(s)
	w.Jobs = calls.port("", nil)
	in := input()
	in.Job = "J-3"
	if _, err := w.Ask(in); err == nil || !strings.Contains(err.Error(), kuMkRefusal) {
		t.Fatalf("err %v, want a refusal containing %q", err, kuMkRefusal)
	}
	if got := kuMkNames(t, s); len(got) != 0 {
		t.Fatalf("a refused ask left %v in questions/", got)
	}
}

func TestKuMkPostedJobKeepsTheId(t *testing.T) {
	s, _ := setup(t)
	calls := &jobCalls{}
	w := kuMkWorker(s)
	w.Jobs = calls.port("J-7", nil)
	if _, err := w.Ask(input()); err == nil || !strings.Contains(err.Error(), kuMkRefusal) {
		t.Fatalf("err %v, want a refusal containing %q", err, kuMkRefusal)
	}
	if got := kuMkNames(t, s); !slices.Equal(got, []string{"Q-1.feed"}) {
		t.Fatalf("questions/ holds %v, want only the Q-1.feed marker (J-7 names Q-1.md)", got)
	}
	if verify, _ := calls.posts[0]["verify"].(map[string]any); verify["path"] != "questions/Q-1.md" {
		t.Fatalf("Job J-7 verify %v", verify)
	}
	rec, err := kuMkOwner(s, calls).Ask(input())
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != "Q-2" {
		t.Fatalf("next ask is %s, want Q-2, never the id J-7 names", rec.ID)
	}
	if got := kuMkNames(t, s); !slices.Equal(got, []string{"Q-1.feed", "Q-2.md"}) {
		t.Fatalf("questions/ holds %v, want Q-1.feed and Q-2.md", got)
	}
}

func TestKuMkConcurrentAsks(t *testing.T) {
	s, _ := setup(t)
	ports := &kuMkPorts{}
	s.Jobs = ports.port()
	const n = 8
	ids := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			rec, err := s.Ask(input())
			ids <- rec.ID
			errs <- err
		})
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var got, want []string
	for id := range ids {
		got = append(got, id)
	}
	slices.Sort(got)
	for i := 1; i <= n; i++ {
		want = append(want, fmt.Sprintf("Q-%d", i))
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ids %v, want each of %v once", got, want)
	}
	var mds, feeds int
	for _, name := range kuMkNames(t, s) {
		switch {
		case strings.HasSuffix(name, ".md"):
			mds++
		case strings.HasSuffix(name, ".feed"):
			feeds++
		}
	}
	if mds != n || feeds != 0 {
		t.Fatalf("questions/ holds %d .md and %d .feed files, want %d and 0: %v", mds, feeds, n, kuMkNames(t, s))
	}
	if ports.posts != n {
		t.Fatalf("Post called %d times, want %d", ports.posts, n)
	}
}
