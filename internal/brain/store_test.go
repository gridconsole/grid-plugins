package brain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/golden"
)

func testStore(t *testing.T) Store {
	t.Helper()
	return Store{GridHome: filepath.Join(t.TempDir(), ".grid"), Project: "example", Now: func() time.Time {
		return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	}}
}

func putFile(t *testing.T, s Store, path, data string) {
	t.Helper()
	path = filepath.Join(s.GridHome, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func accepted(t *testing.T, s Store, n Note) Entry {
	t.Helper()
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	e, err := s.Accept(n.Level, n.Name)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestProposalLifecycle(t *testing.T) {
	s := testStore(t)
	n := sample()
	// Text cannot self-approve or choose its own provenance.
	n.Status, n.Origin = "accepted", Origin{Kind: "server", ID: "forged"}
	origin := Origin{Kind: "job", ID: "T-02/J-02"}
	e, err := s.Propose(n, origin)
	if err != nil {
		t.Fatal(err)
	}
	if e.Note.Status != "proposed" || e.Note.Origin != origin {
		t.Fatalf("untrusted fields retained: %+v", e.Note)
	}
	if _, err := s.Show(n.Name); err == nil {
		t.Fatal("proposal appeared as knowledge")
	}
	if _, err := s.Propose(n, origin); err == nil {
		t.Fatal("proposal overwritten")
	}
	e, err = s.Accept("project", n.Name)
	if err != nil || e.Note.Status != "accepted" {
		t.Fatalf("accept: %+v, %v", e, err)
	}
	if _, err := s.Accept("project", n.Name); err != nil {
		t.Fatalf("repeat accept: %v", err)
	}
	log, err := os.ReadFile(filepath.Join(s.GridHome, "projects/example/knowledge/log.md"))
	if err != nil || strings.Count(string(log), "| accepted | checks |") != 1 {
		t.Fatalf("accept log should appear once: %s %v", log, err)
	}
	p, err := s.Proposal("project", n.Name)
	if err != nil || p.Note.Status != "accepted" {
		t.Fatalf("proposal status: %+v, %v", p, err)
	}
	if _, err := s.Reject("project", n.Name, "A different decision applies."); err == nil {
		t.Fatal("accepted proposal rejected")
	}
	n.Name = "unneeded"
	if _, err := s.Propose(n, origin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reject("project", n.Name, "The existing decision already covers this."); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept("project", n.Name); err == nil {
		t.Fatal("rejected proposal resurrected")
	}
}

func TestAcceptanceGates(t *testing.T) {
	for _, tc := range []struct {
		name, topic, kind string
	}{
		{"config", "agents", "session"}, {"import", "decisions", "import"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			n := sample()
			n.Topic, n.Origin.Kind = tc.topic, tc.kind
			if _, err := s.Propose(n, n.Origin); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Accept(n.Level, n.Name); err == nil {
				t.Fatal("gate allowed acceptance")
			}
			p, err := s.Proposal(n.Level, n.Name)
			if err != nil || p.Note.Status != "rejected" || p.Note.Reason == "" {
				t.Fatalf("rejection not recorded: %+v %v", p, err)
			}
			if _, err := s.Show(n.Name); err == nil {
				t.Fatal("rejected content visible")
			}
		})
	}
}

func TestSecretsNeverPersist(t *testing.T) {
	s := testStore(t)
	n := sample()
	n.Body = "password: example-sensitive-value"
	e, err := s.Propose(n, n.Origin)
	if err == nil || e.Note.Status != "rejected" {
		t.Fatalf("secret proposal: %+v %v", e, err)
	}
	b, err := os.ReadFile(filepath.Join(s.GridHome, e.Path))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "example-sensitive-value") {
		t.Fatal("secret persisted")
	}
	// Recheck a manually edited proposal at acceptance too.
	n = sample()
	n.Name = "edited"
	p, err := s.Propose(n, n.Origin)
	if err != nil {
		t.Fatal(err)
	}
	p.Note.Body = "password: another-sensitive-value"
	b, _ = Encode(p.Note)
	putFile(t, s, p.Path, string(b))
	if _, err := s.Accept("project", n.Name); err == nil {
		t.Fatal("edited secret accepted")
	}
	b, _ = os.ReadFile(filepath.Join(s.GridHome, p.Path))
	if strings.Contains(string(b), "another-sensitive-value") {
		t.Fatal("secret not redacted")
	}
}

func TestHomeProjectIsolationAndSearch(t *testing.T) {
	s := testStore(t)
	n := sample()
	n.Topic = "decisions"
	project := accepted(t, s, n)
	n.Level, n.Body, n.Summary = "home", "Personal facts.\n", "Personal facts about checks."
	home := accepted(t, s, n)
	e, err := s.Show(n.Name)
	if err != nil || e.Note.Level != "project" {
		t.Fatalf("project precedence: %+v %v", e, err)
	}
	e, err = s.Show(home.ID)
	if err != nil || e.Note.Level != "home" {
		t.Fatalf("explicit home: %+v %v", e, err)
	}
	hits, err := s.Search("checks", "", 5)
	if err != nil || len(hits) != 2 || hits[0].ID != project.ID {
		t.Fatalf("search: %+v %v", hits, err)
	}
	if _, err := s.RebuildIndex(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"projects/example/knowledge/index.md", "projects/example/knowledge/backlinks.json"} {
		b, err := os.ReadFile(filepath.Join(s.GridHome, path))
		if err != nil || strings.Contains(string(b), "Personal facts") || strings.Contains(string(b), "brain:home/checks") {
			t.Fatalf("private data in %s: %s %v", path, b, err)
		}
	}
	s.Project = "other"
	if _, err := s.Show(project.ID); err == nil {
		t.Fatal("other project accessed shared note")
	}
	if _, err := s.Show(home.ID); err != nil {
		t.Fatal(err)
	}
}

func TestIndexGoldenAndGenerations(t *testing.T) {
	s := testStore(t)
	accepted(t, s, sample())
	n := sample()
	n.Name, n.Topic, n.Level = "personal", "decisions", "home"
	accepted(t, s, n)
	putFile(t, s, "projects/example/knowledge/decisions/broken.md", "---\nname: broken\n---\nInvalid\n")
	idx, err := s.RebuildIndex()
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "brain-session-index", idx.Text)
	if idx.Generations["home"] != 1 || idx.Generations["project"] != 1 {
		t.Fatalf("generation: %v", idx.Generations)
	}
	idx, err = s.RebuildIndex()
	if err != nil || idx.Generations["project"] != 2 {
		t.Fatalf("second generation: %+v %v", idx, err)
	}
	b, err := os.ReadFile(filepath.Join(s.GridHome, "projects/example/knowledge/backlinks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var links Backlinks
	if err := json.Unmarshal(b, &links); err != nil {
		t.Fatal(err)
	}
	if links.Generation != 2 || len(links.Links["job:T-01/J-01"]) != 1 {
		t.Fatalf("backlinks: %+v", links)
	}
}

func TestReadOnlyEmptyIndexAndBudget(t *testing.T) {
	s := testStore(t)
	idx, err := s.Index()
	if err != nil || len(idx.Topics) != 8 {
		t.Fatalf("empty index: %+v %v", idx, err)
	}
	if _, err := os.Stat(s.GridHome); !os.IsNotExist(err) {
		t.Fatal("read created Grid home")
	}
	var rows []TopicRow
	for range 100 {
		rows = append(rows, TopicRow{Topic: "custom", Description: strings.Repeat("界", 200)})
	}
	text := SessionStartText(rows)
	if len(text) > 1200 || !strings.Contains(text, "topics cut; grid brain topics lists them.") {
		t.Fatalf("budget: %d bytes: %s", len(text), text)
	}
}

func TestCustomTopic(t *testing.T) {
	s := testStore(t)
	putFile(t, s, "projects/example/knowledge/testing/TOPIC.md", "---\nname: testing\nkind: knowledge\nlevel: project\ndescription: Test guidance\n---\nLocal test advice.\n")
	n := sample()
	n.Topic = "testing"
	accepted(t, s, n)
	topics, err := s.Topics()
	if err != nil || len(topics) != 9 || topics[8].Name != "testing" || !topics[8].Custom {
		t.Fatalf("custom topics: %+v %v", topics, err)
	}
	n.Name, n.Topic = "missing", "undefined"
	if _, err := s.Propose(n, n.Origin); err == nil {
		t.Fatal("undefined topic accepted")
	}
	n.Topic, n.Level = "conventions", "home"
	if _, err := s.Propose(n, n.Origin); err == nil {
		t.Fatal("wrong topic level accepted")
	}
}

func TestCustomTopicLevelForms(t *testing.T) {
	for name, level := range map[string]string{"scalar": "project", "pair": "home, project", "flow": "[home, project]", "block": "\n  - home\n  - project"} {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			putFile(t, s, "projects/example/knowledge/testing/TOPIC.md", "---\nname: testing\nkind: knowledge\nlevel: "+level+"\n---\nLocal test advice.\n")
			n := sample()
			n.Topic = "testing"
			accepted(t, s, n)
		})
	}
	s := testStore(t)
	putFile(t, s, "projects/example/knowledge/testing/TOPIC.md", "---\nname: testing\nkind: knowledge\nlevel: home\n---\nLocal test advice.\n")
	n := sample()
	n.Topic = "testing"
	if _, err := s.Propose(n, n.Origin); err == nil {
		t.Fatal("topic declared for home accepted at project level")
	}
}

func TestTraversalSymlinksAndLocks(t *testing.T) {
	s := testStore(t)
	n := sample()
	s.Project = "../escape"
	if _, err := s.Propose(n, n.Origin); err == nil {
		t.Fatal("project traversal accepted")
	}
	s.Project = "example"
	accepted(t, s, n)
	if _, err := s.Show("../checks"); err == nil {
		t.Fatal("note traversal accepted")
	}
	n.Name = "lockfile"
	if err := os.Remove(filepath.Join(s.GridHome, "projects", "example", lockName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(s.GridHome, "projects", "example", lockName)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Propose(n, n.Origin); err == nil {
		t.Fatal("symbolic link lock file accepted")
	}
	if err := os.Remove(filepath.Join(s.GridHome, "projects", "example", lockName)); err != nil {
		t.Fatal(err)
	}
	// A relative symlink to a project also violates personal isolation, even
	// though its target is within the outer Grid home.
	if err := os.Symlink("projects/example/knowledge", filepath.Join(s.GridHome, "knowledge")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Index(); err == nil {
		t.Fatal("personal/project symlink accepted")
	}
}

func TestConcurrentProposalsDoNotClobber(t *testing.T) {
	s := testStore(t)
	n := sample()
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Go(func() { _, err := s.Propose(n, n.Origin); results <- err })
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successful writers, want one", successes)
	}
	if _, err := s.Accept("project", n.Name); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverAcceptanceAndRefuseOverwrite(t *testing.T) {
	s := testStore(t)
	n := sample()
	p, err := s.Propose(n, n.Origin)
	if err != nil {
		t.Fatal(err)
	}
	n = p.Note
	n.Status = "accepted"
	b, _ := Encode(n)
	putFile(t, s, "projects/example/knowledge/conventions/checks.md", string(b))
	if _, err := s.Accept("project", n.Name); err != nil {
		t.Fatalf("recover interrupted accept: %v", err)
	}
	n.Body = "A different accepted note.\n"
	b, _ = Encode(n)
	putFile(t, s, "projects/example/knowledge/conventions/checks.md", string(b))
	if _, err := s.Accept("project", n.Name); err == nil {
		t.Fatal("existing knowledge overwritten")
	}
}
