package brain

import (
	"reflect"
	"strings"
	"testing"
)

// pbSpVault is a vault with knowledge and history that all say "zebra": a home
// note, a project note, and, under projects/demo/tasks/T-1/, task.md, outcome.md,
// a Job node, a result post, a question post and a checkpoint post. The files of
// history sort before the note on disk, and the project level is read first, so
// an ordering that follows the walk puts history first.
func pbSpVault(t *testing.T) Store {
	t.Helper()
	s := pbVrVault(t)
	pbVrWrite(t, s.BrainDir, "wiki/Stripes.md", "Zebra.\n")                                      // home note, score 5
	pbVrWrite(t, s.BrainDir, "projects/demo/wiki/Herd.md", "Zebra herd.\n")                      // project note
	pbVrWrite(t, s.BrainDir, "projects/demo/tasks/T-1/task.md", "Zebra herd stripes.\n")         // task
	pbVrWrite(t, s.BrainDir, "projects/demo/tasks/T-1/outcome.md", "Zebra.\n")                   // task, score 5
	pbVrWrite(t, s.BrainDir, "projects/demo/tasks/T-1/jobs/J-5.md", "Zebra.\n")                  // job, score 5
	pbVrWrite(t, s.BrainDir, "projects/demo/tasks/T-1/board/000042-J-5-result.md", "Zebra.\n")   // post, score 5
	pbVrWrite(t, s.BrainDir, "projects/demo/tasks/T-1/board/000043-J-5-question.md", "Zebra.\n") // post, score 5
	pbVrWrite(t, s.BrainDir, "projects/demo/tasks/T-1/board/000044-J-5-checkpoint.md", "Zebra.\n")
	pbVrWrite(t, s.BrainDir, "projects/demo/tasks/T-1/board/000045-J-5-heartbeat.md", "Zebra.\n")
	return s
}

func pbSpIDs(hits []Hit) []string {
	ids := []string{}
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	return ids
}

// V15: knowledge ranks above history for an equal score, and above a higher one.
func TestPbSpKnowledgeRanksAboveHistoryForEqualScore(t *testing.T) {
	s := pbSpVault(t)
	hits, err := s.Search("zebra", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	score := map[string]int{}
	for _, h := range hits {
		score[h.ID] = h.Score
	}
	// The fixtures are built so the home note and the result post tie.
	note, post := "brain:home/wiki/Stripes", "brain:project/tasks/T-1/board/000042-J-5-result"
	if score[note] == 0 || score[note] != score[post] {
		t.Fatalf("fixture: note and post must tie, got %v", score)
	}
	pos := map[string]int{}
	for i, h := range hits {
		pos[h.ID] = i
	}
	for _, history := range []string{post, "brain:project/tasks/T-1/task", "brain:project/tasks/T-1/jobs/J-5"} {
		if pos[note] > pos[history] {
			t.Errorf("history %s ranks above the note: %v", history, pbSpIDs(hits))
		}
	}
	// Notes come first as a class: both notes before any history hit.
	for i, h := range hits[:2] {
		if h.Kind != "" {
			t.Errorf("hit %d is %q, want a note: %v", i, h.Kind, pbSpIDs(hits))
		}
	}
	for _, h := range hits[2:] {
		if h.Kind == "" {
			t.Errorf("a note ranks after history: %v", pbSpIDs(hits))
		}
	}
	// A history hit that scores far higher still ranks after every note, and
	// within history the higher score comes first.
	hits, err = s.Search("zebra herd stripes", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 4 || hits[0].Kind != "" || hits[1].Kind != "" || hits[2].ID != "brain:project/tasks/T-1/task" {
		t.Fatalf("order = %v", pbSpIDs(hits))
	}
	if hits[2].Score <= hits[0].Score {
		t.Fatalf("fixture: the task (%d) must outscore the notes (%d)", hits[2].Score, hits[0].Score)
	}
}

// V15: --kind works. One search per kind value and one without.
func TestPbSpKindFilter(t *testing.T) {
	s := pbSpVault(t)
	want := map[string][]string{
		"": {"brain:home/wiki/Stripes", "brain:project/wiki/Herd", "brain:project/tasks/T-1/task", "brain:project/tasks/T-1/jobs/J-5",
			"brain:project/tasks/T-1/board/000042-J-5-result", "brain:project/tasks/T-1/board/000043-J-5-question", "brain:project/tasks/T-1/outcome"},
		KindNote:     {"brain:home/wiki/Stripes", "brain:project/wiki/Herd"},
		KindTask:     {"brain:project/tasks/T-1/task", "brain:project/tasks/T-1/outcome"},
		KindJob:      {"brain:project/tasks/T-1/jobs/J-5"},
		KindPost:     {"brain:project/tasks/T-1/board/000042-J-5-result", "brain:project/tasks/T-1/board/000043-J-5-question"},
		KindProposal: {},
	}
	for kind, ids := range want {
		hits, err := s.SearchKind("zebra", "", kind, 100)
		if err != nil {
			t.Fatalf("kind %q: %v", kind, err)
		}
		got := pbSpIDs(hits)
		if !pbSpSameSet(got, ids) {
			t.Errorf("kind %q = %v, want %v", kind, got, ids)
		}
		if kind != "" && kind != KindNote {
			for _, h := range hits {
				if h.Kind != kind {
					t.Errorf("kind %q hit %s has kind %q", kind, h.ID, h.Kind)
				}
			}
		}
	}
	// Checkpoints and heartbeats are in no search.
	for _, kind := range append([]string{""}, Kinds...) {
		hits, _ := s.SearchKind("zebra", "", kind, 100)
		for _, h := range hits {
			if strings.Contains(h.ID, "checkpoint") || strings.Contains(h.ID, "heartbeat") {
				t.Errorf("kind %q returned an unindexed post %s", kind, h.ID)
			}
		}
	}
	// Search is the search with no kind.
	a, _ := s.Search("zebra", "", 100)
	b, _ := s.SearchKind("zebra", "", "", 100)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Search differs from SearchKind with no kind")
	}
	if _, err := s.SearchKind("zebra", "", "board", 5); err == nil || !strings.Contains(err.Error(), "kind must be") {
		t.Errorf("an unknown kind must be named in an error, got %v", err)
	}
}

func pbSpSameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
	}
	for _, n := range m {
		if n != 0 {
			return false
		}
	}
	return true
}

// V15: a proposal is searched only with --kind proposal.
func TestPbSpProposalKind(t *testing.T) {
	s := testStore(t)
	n := sample()
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	hits, err := s.SearchKind("checks", "", KindProposal, 5)
	if err != nil || len(hits) != 1 || hits[0].Kind != KindProposal || hits[0].ID != "brain:project/checks" {
		t.Fatalf("proposal search = %+v, %v", hits, err)
	}
	for _, kind := range []string{"", KindNote} {
		if hits, err = s.SearchKind("checks", "", kind, 5); err != nil || len(hits) != 0 {
			t.Fatalf("kind %q found a proposal: %+v, %v", kind, hits, err)
		}
	}
	if _, err := s.Accept("project", "checks"); err != nil {
		t.Fatal(err)
	}
	if hits, err = s.SearchKind("checks", "", KindProposal, 5); err != nil || len(hits) != 0 {
		t.Fatalf("an accepted note is still a proposal: %+v, %v", hits, err)
	}
	if hits, err = s.SearchKind("checks", "", KindNote, 5); err != nil || len(hits) != 1 {
		t.Fatalf("accepted note: %+v, %v", hits, err)
	}
}
