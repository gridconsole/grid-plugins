package brain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// queryNote builds a valid note whose fields share no words unless a test puts
// them there, so a score can only come from the field the test chose.
func queryNote(name, topic, level, desc, summary, body string) Note {
	n := sample()
	n.Name, n.Topic, n.Level = name, topic, level
	n.Description, n.Summary, n.Body = desc, summary, body
	n.Links = []Link{}
	return n
}

func wordSet(ws ...string) map[string]bool {
	out := map[string]bool{}
	for _, w := range ws {
		out[w] = true
	}
	return out
}

func TestShowResolvesProjectFirstAndExactLevel(t *testing.T) {
	s := testStore(t)
	accepted(t, s, queryNote("deploy", "decisions", "home", "Home deploy note.", "Home copy.", "Home body.\n"))
	accepted(t, s, queryNote("deploy", "decisions", "project", "Project deploy note.", "Project copy.", "Project body.\n"))
	accepted(t, s, queryNote("homeonly", "decisions", "home", "Only at home.", "Home only.", "Body.\n"))

	for _, tc := range []struct{ id, level, id2 string }{
		{"deploy", "project", "brain:project/deploy"},
		{"brain:home/deploy", "home", "brain:home/deploy"},
		{"brain:project/deploy", "project", "brain:project/deploy"},
		{"homeonly", "home", "brain:home/homeonly"},
	} {
		e, err := s.Show(tc.id)
		if err != nil || e.Note.Level != tc.level || e.ID != tc.id2 {
			t.Fatalf("Show(%q) = %+v, %v; want level %s id %s", tc.id, e, err, tc.level, tc.id2)
		}
	}
	if e, _ := s.Show("deploy"); e.Note.Body != "Project body.\n" {
		t.Fatalf("Show returns the note body: %q", e.Note.Body)
	}

	for _, tc := range []struct{ id, want string }{
		{"brain:team/deploy", "id must be brain:<home|project>/<name>"},
		{"brain:deploy", "id must be brain:<home|project>/<name>"},
		{"brain:home", "id must be brain:<home|project>/<name>"},
		{"../x", "note id must be a plain name"},
		{"brain:home/../x", "note id must be a plain name"},
		{"", "note id must be a plain name"},
		{"missing", "not found"},
		{"brain:home/missing", "not found"},
		// An explicit level is exact: no fallback to the other level.
		{"brain:project/homeonly", "not found"},
	} {
		if _, err := s.Show(tc.id); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Show(%q) error = %v; want %q", tc.id, err, tc.want)
		}
	}
	if _, err := s.Show("missing"); err == nil || !strings.Contains(err.Error(), `accepted note "missing" not found`) {
		t.Fatalf("not found wording: %v", err)
	}
}

func TestShowIgnoresProposals(t *testing.T) {
	s := testStore(t)
	if _, err := s.Propose(queryNote("pending", "decisions", "project", "Waiting.", "Waiting.", "Body.\n"), Origin{Kind: "job", ID: "T-01/J-01"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Show("pending"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("proposal shown: %v", err)
	}
}

func TestSplitID(t *testing.T) {
	for _, tc := range []struct {
		id, level, name string
		ok              bool
	}{
		{"deploy", "", "deploy", true},
		{"brain:home/deploy", "home", "deploy", true},
		{"brain:project/deploy", "project", "deploy", true},
		{"a.b-c_d", "", "a.b-c_d", true},
		{"brain:team/deploy", "", "", false},
		{"brain:Home/deploy", "", "", false},
		{"brain:deploy", "", "", false},
		{"brain:", "", "", false},
		{"brain:/deploy", "", "", false},
		{"brain:home/", "", "", false},
		{"brain:home/a/b", "", "", false},
		{"a/b", "", "", false},
		{"../x", "", "", false},
		{"a..b", "", "", false},
		{".hidden", "", "", false},
		{"", "", "", false},
		{strings.Repeat("a", 101), "", "", false},
	} {
		level, name, err := splitID(tc.id)
		if (err == nil) != tc.ok || level != tc.level || name != tc.name {
			t.Errorf("splitID(%q) = %q, %q, %v; want %q, %q, ok=%v", tc.id, level, name, err, tc.level, tc.name, tc.ok)
		}
	}
	if _, _, err := splitID("brain:team/deploy"); err == nil || err.Error() != "id must be brain:<home|project>/<name>" {
		t.Errorf("bad level error: %v", err)
	}
	if _, _, err := splitID("../x"); err == nil || err.Error() != "note id must be a plain name or brain:<home|project>/<name>" {
		t.Errorf("bad name error: %v", err)
	}
}

func TestWords(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want map[string]bool
	}{
		{"The Libraries are running", wordSet("library", "running")},
		{"a I x go", wordSet("go")},
		{"uses", wordSet("use")},
		{"bus", wordSet("bus")},
		{"cats", wordSet("cat")},
		{"boxes", wordSet("boxe")},
		{"Deploy, DEPLOY; deploy!", wordSet("deploy")},
		{"the of and", wordSet()},
		{"", wordSet()},
		{"v2 api-keys", wordSet("v2", "api", "key")},
	} {
		if got := words(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("words(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

func TestSearchRanking(t *testing.T) {
	s := testStore(t)
	accepted(t, s, queryNote("bravo", "decisions", "project", "Plain thing.", "Plain.", "A zebra appears.\n"))
	accepted(t, s, queryNote("alpha", "decisions", "project", "Zebra stripes.", "Plain.", "Nothing.\n"))
	accepted(t, s, queryNote("charlie", "decisions", "project", "Plain.", "Plain.", "Nothing.\n"))

	hits, err := s.Search("zebra", "", 10)
	if err != nil || len(hits) != 2 {
		t.Fatalf("hits: %+v %v", hits, err)
	}
	if hits[0].ID != "brain:project/alpha" || hits[0].Score != 4 || hits[1].ID != "brain:project/bravo" || hits[1].Score != 1 {
		t.Fatalf("description 4 ahead of body 1: %+v", hits)
	}
	// Query words fold the same way as note words.
	if hits, err = s.Search("Zebras", "", 10); err != nil || len(hits) != 2 || hits[0].ID != "brain:project/alpha" {
		t.Fatalf("folded query: %+v %v", hits, err)
	}
	if hits, err = s.Search("the of a", "", 10); err != nil || len(hits) != 0 {
		t.Fatalf("stop words only: %+v %v", hits, err)
	}
	if hits, err = s.Search("absent", "", 10); err != nil || hits == nil || len(hits) != 0 {
		t.Fatalf("no match should be an empty list, got %#v %v", hits, err)
	}
}

func TestSearchScoresNameAndSummaryTwo(t *testing.T) {
	s := testStore(t)
	accepted(t, s, queryNote("zebra", "decisions", "project", "Plain.", "Plain.", "Nothing.\n"))
	accepted(t, s, queryNote("other", "decisions", "project", "Plain.", "About the zebra.", "Nothing.\n"))
	// Name and summary form one field: a word in both still scores 2.
	accepted(t, s, queryNote("zebra-both", "decisions", "project", "Plain.", "Zebra again.", "Nothing.\n"))
	// Each field adds its own points: 4 + 2 + 1.
	accepted(t, s, queryNote("everywhere", "decisions", "project", "Zebra.", "Zebra.", "Zebra.\n"))

	hits, err := s.Search("zebra", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, h := range hits {
		got[h.ID] = h.Score
	}
	want := map[string]int{
		"brain:project/zebra":      2,
		"brain:project/other":      2,
		"brain:project/zebra-both": 2,
		"brain:project/everywhere": 7,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scores = %v; want %v", got, want)
	}
	if hits[0].ID != "brain:project/everywhere" {
		t.Fatalf("highest score first: %+v", hits)
	}

	// Several query words add up per word.
	accepted(t, s, queryNote("pair", "decisions", "project", "Giraffe.", "Plain.", "Zebra.\n"))
	hits, err = s.Search("giraffe zebra", "", 10)
	if err != nil || hits[0].ID != "brain:project/everywhere" || hits[0].Score != 7 {
		t.Fatalf("multi-word: %+v %v", hits, err)
	}
	for _, h := range hits {
		if h.ID == "brain:project/pair" && h.Score != 5 {
			t.Fatalf("pair score %d, want 4+1", h.Score)
		}
	}
}

func TestSearchTiesKeepProjectBeforeHome(t *testing.T) {
	s := testStore(t)
	// Home is written first so the order is the level order, not the write order.
	accepted(t, s, queryNote("tie", "decisions", "home", "Zebra.", "Plain.", "Nothing.\n"))
	accepted(t, s, queryNote("tie", "decisions", "project", "Zebra.", "Plain.", "Nothing.\n"))

	hits, err := s.Search("zebra", "", 10)
	if err != nil || len(hits) != 2 {
		t.Fatalf("hits: %+v %v", hits, err)
	}
	if hits[0].Score != 4 || hits[1].Score != 4 {
		t.Fatalf("scores: %+v", hits)
	}
	if hits[0].Level != "project" || hits[1].Level != "home" {
		t.Fatalf("tie order: %+v", hits)
	}
	// A higher score still beats the project level.
	accepted(t, s, queryNote("better", "decisions", "home", "Zebra.", "Zebra.", "Nothing.\n"))
	hits, err = s.Search("zebra", "", 10)
	if err != nil || len(hits) != 3 || hits[0].ID != "brain:home/better" || hits[1].Level != "project" {
		t.Fatalf("score before level: %+v %v", hits, err)
	}
}

func TestSearchRefusesBadLimitAndTopic(t *testing.T) {
	s := testStore(t)
	accepted(t, s, queryNote("alpha", "decisions", "project", "Zebra.", "Plain.", "Nothing.\n"))

	for _, limit := range []int{0, -1, 101} {
		if _, err := s.Search("zebra", "", limit); err == nil || !strings.Contains(err.Error(), "limit must be between 1 and 100") {
			t.Errorf("limit %d: %v", limit, err)
		}
	}
	for _, limit := range []int{1, 100} {
		if hits, err := s.Search("zebra", "", limit); err != nil || len(hits) != 1 {
			t.Errorf("limit %d: %+v %v", limit, hits, err)
		}
	}
	for _, topic := range []string{"a/b", "../x", ".hidden"} {
		if _, err := s.Search("zebra", topic, 5); err == nil || !strings.Contains(err.Error(), "topic must be a plain name") {
			t.Errorf("topic %q: %v", topic, err)
		}
	}
}

func TestSearchTopicSupersededLimitAndNoBody(t *testing.T) {
	s := testStore(t)
	accepted(t, s, queryNote("rule", "decisions", "project", "Zebra rule.", "Plain.", "Xylophone secret text.\n"))
	accepted(t, s, queryNote("habit", "conventions", "project", "Plain.", "Plain.", "A zebra habit and a xylophone.\n"))
	accepted(t, s, queryNote("homerule", "decisions", "home", "Zebra home rule.", "Plain.", "Nothing.\n"))

	old := queryNote("oldrule", "decisions", "project", "Zebra old rule.", "Zebra.", "Zebra.\n")
	old.Status, old.Updated, old.SupersededBy = "accepted", "2026-01-02T03:04:05Z", "brain:project/rule"
	old.Origin = Origin{Kind: "job", ID: "T-01/J-01"}
	b, err := Encode(old)
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, s, "projects/example/knowledge/decisions/oldrule.md", string(b))
	// The file is a valid accepted note that Show can read, so only the
	// superseded_by field can be what hides it from Search.
	if e, err := s.Show("oldrule"); err != nil || e.Note.SupersededBy == "" {
		t.Fatalf("superseded note unreadable: %+v %v", e, err)
	}

	ids := func(hits []Hit) []string {
		out := []string{}
		for _, h := range hits {
			out = append(out, h.ID)
		}
		return out
	}

	all, err := s.Search("zebra", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"brain:project/rule", "brain:home/homerule", "brain:project/habit"}; !reflect.DeepEqual(ids(all), want) {
		t.Fatalf("all: %v (scores %+v); want %v", ids(all), all, want)
	}
	for _, h := range all {
		if h.ID == "brain:project/oldrule" {
			t.Fatalf("superseded note returned: %+v", h)
		}
	}

	decisions, err := s.Search("zebra", "decisions", 100)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"brain:project/rule", "brain:home/homerule"}; !reflect.DeepEqual(ids(decisions), want) {
		t.Fatalf("topic decisions: %v; want %v", ids(decisions), want)
	}
	for _, h := range decisions {
		if h.Topic != "decisions" {
			t.Fatalf("other topic: %+v", h)
		}
	}
	if hits, err := s.Search("zebra", "conventions", 100); err != nil || !reflect.DeepEqual(ids(hits), []string{"brain:project/habit"}) {
		t.Fatalf("topic conventions: %+v %v", hits, err)
	}
	if hits, err := s.Search("zebra", "agents", 100); err != nil || len(hits) != 0 {
		t.Fatalf("topic without notes: %+v %v", hits, err)
	}

	top, err := s.Search("zebra", "", 1)
	if err != nil || !reflect.DeepEqual(ids(top), []string{"brain:project/rule"}) || top[0].Score != 4 {
		t.Fatalf("limit 1: %+v %v", top, err)
	}
	two, err := s.Search("zebra", "", 2)
	if err != nil || !reflect.DeepEqual(ids(two), []string{"brain:project/rule", "brain:home/homerule"}) {
		t.Fatalf("limit 2: %+v %v", two, err)
	}

	// Hits carry no body: the body words match, but only the score shows it.
	hits, err := s.Search("xylophone", "", 10)
	if err != nil || len(hits) != 2 {
		t.Fatalf("body search: %+v %v", hits, err)
	}
	raw, err := json.Marshal(hits)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(raw)), "xylophone") || strings.Contains(string(raw), `"body"`) {
		t.Fatalf("hit carries body text: %s", raw)
	}
	var fields []map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, f := range fields {
		keys := []string{}
		for k := range f {
			keys = append(keys, k)
		}
		if len(keys) != 7 {
			t.Fatalf("hit fields = %v; want id path topic level description summary score", keys)
		}
	}
}
