package orchestrator

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// plGrPostedAt is the fixed time the agent's answer is stamped with; the grants
// below expire relative to it, never to the wall clock.
var plGrPostedAt = time.Date(2030, 1, 1, 10, 0, 0, 0, time.UTC)

const plGrLink = "grid/people/dana/grants/answers"

func plGrGrant(agent, until string) []contract.Grant {
	return []contract.Grant{{Link: plGrLink, Person: "dana", Role: "member", Agents: []string{agent}, Posts: []string{"answer"}, Until: until}}
}

// plGrAgentAnswers sets up a person Job J-1 for dana and has the worker of J-3,
// a launched agent with its Job lease, answer J-1 for dana under plGrLink.
func plGrAgentAnswers(t *testing.T, grants []contract.Grant) (*fixture, feed.Record) {
	t.Helper()
	f := personFixture(t)
	f.s.Grants = grants
	f.tick(t)
	state, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	j3, j1 := state.Jobs["J-3"], state.Jobs["J-1"]
	if j3 == nil || j3.Assign == nil || j1 == nil {
		t.Fatalf("J-3 is not assigned or J-1 is missing: %+v", state.Jobs)
	}
	w := f.worker
	w.Lease, w.Fence, w.Gen = "J-3", j3.Assign.Fence, j3.Generation
	if _, err := Write(f.s.Feed, w, feed.Input{Type: "ack", Job: "J-3", Attempt: j3.Attempt, Body: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	f.s.Feed.Now = func() time.Time { return plGrPostedAt }
	body := map[string]any{"answer": "Call it Lark.", "for": "dana", "by": w.Author.ID, "grant": "[[" + plGrLink + "]]"}
	rec := f.post(t, feed.Input{Type: "answer", Re: j1.History[0], Job: "J-3", Attempt: j3.Attempt, Body: body}, w)
	f.tick(t)
	return f, rec
}

func plGrControl(t *testing.T, f *fixture, target string) map[string]any {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range read.Records {
		if e.Record.Type == "control" && e.Record.Body["target"] == target && e.Record.Body["status"] != nil {
			return e.Record.Body
		}
	}
	return nil
}

// Row 1 (V22): an agent's answer for dana under her covering grant closes her
// person Job as dana's, and the record keeps the agent as its author.
func TestPlGrGrantedAnswerFoldsAsThePersons(t *testing.T) {
	f, rec := plGrAgentAnswers(t, plGrGrant("worker", "2030-01-02T00:00:00Z"))
	if got := jobStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s, want accepted by dana's granted answer", got)
	}
	ctl := plGrControl(t, f, "J-1")
	if ctl == nil || ctl["reason"] != "answered by dana" || ctl["answer"] != "Call it Lark." {
		t.Fatalf("control on J-1 is %v, want answered by dana", ctl)
	}
	if rec.Author.ID != "worker" || rec.Author.Role != "worker" || rec.Lease == nil {
		t.Fatalf("the answer record's author changed: %+v", rec.Author)
	}
	read, err := f.s.Feed.Read(feed.Filter{Type: "post"})
	if err != nil {
		t.Fatal(err)
	}
	named := false
	for _, e := range read.Records {
		text, _ := e.Record.Body["text"].(string)
		named = named || strings.HasPrefix(text, "dana answered J-1") && strings.Contains(text, "(posted by worker under "+plGrLink+")")
	}
	if !named {
		t.Fatal("no message says dana answered J-1, posted by the agent under the grant")
	}
}

// Row 2 (V22): expired, wrong-agent or no grant leaves the answer the agent's
// own, which does not answer the person's Job; nothing is refused.
func TestPlGrUngrantedAnswerFoldsAsTheAgents(t *testing.T) {
	for _, c := range []struct {
		name   string
		grants []contract.Grant
	}{
		{"expired", plGrGrant("worker", "2030-01-01T09:59:59Z")},
		{"expired at the post", plGrGrant("worker", "2030-01-01T10:00:00Z")},
		{"wrong agent", plGrGrant("agent-9", "2030-01-02T00:00:00Z")},
		{"no grant", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, rec := plGrAgentAnswers(t, c.grants)
			if rec.ID == "" {
				t.Fatal("the agent's answer was not kept")
			}
			if got := jobStatus(t, f, "J-1"); got != contract.Posted {
				t.Fatalf("J-1 is %s; an agent's answer without a covering grant closed dana's Job", got)
			}
			if ctl := plGrControl(t, f, "J-1"); ctl != nil {
				t.Fatalf("a control answered J-1: %v", ctl)
			}
		})
	}
}

// LoadGrants reads grid/people/<person>/grants/*.md with the person's role from
// grid/people/<person>.md, and a brain without people has no grants.
func TestPlGrLoadGrants(t *testing.T) {
	brain := t.TempDir()
	plGrWrite(t, brain, "grid/people/dana.md", "---\nrole: member\n---\nDana.\n")
	plGrWrite(t, brain, "grid/people/dana/grants/answers.md", "---\nagents: [worker, builder@01J000000000000000000000AA]\nposts: answer\nkinds: [decide]\ntasks: all\nuntil: 2030-01-02T00:00:00Z\n---\nAnswers for me.\n")
	plGrWrite(t, brain, "grid/people/dana/grants/broken.md", "no front matter\n")
	got, err := LoadGrants(brain)
	if err != nil {
		t.Fatal(err)
	}
	want := []contract.Grant{{Link: plGrLink, Person: "dana", Role: "member", Agents: []string{"worker", "builder@01J000000000000000000000AA"}, Posts: []string{"answer"}, Kinds: []string{"decide"}, Tasks: []string{"all"}, Until: "2030-01-02T00:00:00Z"}}
	if len(got) < 1 || !reflect.DeepEqual(got[:1], want) {
		t.Fatalf("LoadGrants = %+v, want %+v", got, want)
	}
	if none, err := LoadGrants(t.TempDir()); err != nil || none != nil {
		t.Fatalf("a brain without people: %v %v", none, err)
	}
}

func plGrWrite(t *testing.T, root, name, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
