package brain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// openScope opens the Grid home root of a level for the unexported readers.
func openScope(t *testing.T, s Store, level string) *os.Root {
	t.Helper()
	root, err := s.scope(level, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func rowByTopic(t *testing.T, rs []TopicRow, topic string) TopicRow {
	t.Helper()
	for _, r := range rs {
		if r.Topic == topic {
			return r
		}
	}
	t.Fatalf("no row for topic %s in %+v", topic, rs)
	return TopicRow{}
}

// putNote writes an accepted note file straight into a level's tree.
func putNote(t *testing.T, s Store, path string, n Note) {
	t.Helper()
	n.Status = "accepted"
	b, err := Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, s, path, string(b))
}

func topicDoc(name, level string) string {
	return "---\nname: " + name + "\nkind: knowledge\nlevel: " + level + "\ndescription: About " + name + "\n---\nBody.\n"
}

func TestRowsCountsEntriesInvalidAndLastChange(t *testing.T) {
	s := testStore(t)
	const kn = "projects/example/knowledge/"
	// The newer note sorts first by name, so taking the last one seen would be wrong.
	n := sample()
	n.Topic, n.Name, n.Updated = "decisions", "alpha", "2026-03-02T08:30:00Z"
	putNote(t, s, kn+"decisions/alpha.md", n)
	n.Name, n.Updated = "beta", "2026-01-05T10:00:00Z"
	putNote(t, s, kn+"decisions/beta.md", n)
	putFile(t, s, kn+"decisions/broken.md", "---\nname: broken\n---\nInvalid\n")
	// A TOPIC.md inside a built-in topic is never read, so it adds nothing.
	putFile(t, s, kn+"decisions/TOPIC.md", "not a definition\n")
	// A custom topic whose TOPIC.md fails to validate is one invalid row entry.
	putFile(t, s, kn+"testing/TOPIC.md", "---\nname: other\nkind: knowledge\nlevel: project\n---\nBad.\n")
	// A config topic counts its decodable files as entries and the rest as invalid.
	putFile(t, s, kn+"environments/dev.md", "---\nname: dev\n---\nNotes\n")
	putFile(t, s, kn+"environments/bad.md", "no front matter\n")
	putFile(t, s, kn+"environments/TOPIC.md", "ignored\n")

	rs, _, err := rows(openScope(t, s, "project"), "project")
	if err != nil {
		t.Fatal(err)
	}
	d := rowByTopic(t, rs, "decisions")
	if d.Entries != 2 || d.Invalid != 1 || d.LastChange != "2026-03-02T08:30:00Z" {
		t.Fatalf("decisions row: %+v", d)
	}
	bad := rowByTopic(t, rs, "testing")
	if bad.Entries != 0 || bad.Invalid != 1 || bad.Description != "Invalid topic definition" {
		t.Fatalf("invalid TOPIC.md row: %+v", bad)
	}
	if d.Invalid+bad.Invalid != 2 {
		t.Fatalf("invalid across note file and TOPIC.md: %d", d.Invalid+bad.Invalid)
	}
	env := rowByTopic(t, rs, "environments")
	if env.Entries != 1 || env.Invalid != 1 || env.LastChange != "" {
		t.Fatalf("config row: %+v", env)
	}
}

func TestRowsShowsWrongLevelFileAsExtraRow(t *testing.T) {
	for _, tc := range []struct{ level, path, topic string }{
		{"project", "projects/example/knowledge/machines/laptop.md", "machines"},
		{"home", "knowledge/conventions/style.md", "conventions"},
	} {
		t.Run(tc.level, func(t *testing.T) {
			s := testStore(t)
			n := sample()
			n.Topic, n.Name, n.Level = tc.topic, strings.TrimSuffix(filepath.Base(tc.path), ".md"), tc.level
			putNote(t, s, tc.path, n)
			rs, _, err := rows(openScope(t, s, tc.level), tc.level)
			if err != nil {
				t.Fatal(err)
			}
			last := rs[len(rs)-1]
			if last.Topic != tc.topic || last.Description != "Invalid topic level" || last.Invalid != 1 || last.Entries != 0 {
				t.Fatalf("extra row: %+v", last)
			}
			for _, r := range rs[:len(rs)-1] {
				if r.Topic == tc.topic {
					t.Fatalf("topic listed twice: %+v", rs)
				}
			}
		})
	}
}

func TestReadBacklinksMissingAndMalformed(t *testing.T) {
	s := testStore(t)
	root := openScope(t, s, "project")
	got, err := readBacklinks(root)
	if err != nil || got.Generation != 0 || got.Links == nil || len(got.Links) != 0 {
		t.Fatalf("missing backlinks.json: %+v %v", got, err)
	}
	putFile(t, s, "projects/example/knowledge/backlinks.json", "{not json")
	if _, err := readBacklinks(root); err == nil || !strings.Contains(err.Error(), "invalid backlinks index") {
		t.Fatalf("malformed backlinks.json: %v", err)
	}
	// index.go:142-146 returns the error from Index; it is not swallowed.
	if _, err := s.Index(); err == nil || !strings.Contains(err.Error(), "invalid backlinks index") {
		t.Fatalf("Index on malformed backlinks.json: %v", err)
	}
	// index.go:219-222 makes RebuildIndex stop on it as well, leaving the file alone.
	if _, err := s.RebuildIndex(); err == nil {
		t.Fatal("RebuildIndex repaired a malformed backlinks.json")
	}
	b, _ := os.ReadFile(filepath.Join(s.GridHome, "projects/example/knowledge/backlinks.json"))
	if string(b) != "{not json" {
		t.Fatalf("backlinks.json rewritten: %s", b)
	}
}

func TestRebuildIndexWritesBacklinkPerLinkAndBumpsGeneration(t *testing.T) {
	s := testStore(t)
	b := sample()
	b.Name, b.Topic, b.Summary = "bee", "decisions", "Bee is the target note."
	b.Links = nil
	accepted(t, s, b)
	a := sample()
	a.Name, a.Topic, a.Summary = "ay", "decisions", "Ay links to bee."
	a.Links = []Link{
		{To: "brain:project/bee", Rel: "explains", Summary: "The note ay explains.", Why: "Gives the background for bee."},
		{To: "job:T-09/J-09", Rel: "decided-by", Summary: "The job behind ay.", Why: "Shows who decided this."},
	}
	accepted(t, s, a)

	idx, err := s.RebuildIndex()
	if err != nil || idx.Generations["project"] != 1 || idx.Generations["home"] != 1 {
		t.Fatalf("first rebuild: %+v %v", idx.Generations, err)
	}
	read := func() Backlinks {
		raw, err := os.ReadFile(filepath.Join(s.GridHome, "projects/example/knowledge/backlinks.json"))
		if err != nil {
			t.Fatal(err)
		}
		var links Backlinks
		if err := json.Unmarshal(raw, &links); err != nil {
			t.Fatal(err)
		}
		return links
	}
	links := read()
	want := Backlink{From: "brain:project/ay", Rel: "explains", Summary: "Ay links to bee.", Why: "Gives the background for bee."}
	if got := links.Links["brain:project/bee"]; !reflect.DeepEqual(got, []Backlink{want}) {
		t.Fatalf("backlink to bee: %+v", got)
	}
	if got := links.Links["job:T-09/J-09"]; len(got) != 1 || got[0].Rel != "decided-by" || got[0].From != "brain:project/ay" {
		t.Fatalf("backlink to job: %+v", got)
	}
	if len(links.Links) != 2 {
		t.Fatalf("one entry per link target: %+v", links.Links)
	}
	// bee links to nothing, so it appears only as a target, never as a source.
	for _, list := range links.Links {
		for _, l := range list {
			if l.From == "brain:project/bee" {
				t.Fatalf("bee has no links: %+v", links.Links)
			}
		}
	}

	idx, err = s.RebuildIndex()
	if err != nil || idx.Generations["project"] != 2 || idx.Generations["home"] != 2 {
		t.Fatalf("second rebuild: %+v %v", idx.Generations, err)
	}
	if got := read(); got.Generation != 2 || !reflect.DeepEqual(got.Links["brain:project/bee"], []Backlink{want}) {
		t.Fatalf("second backlinks: %+v", got)
	}
	md, err := os.ReadFile(filepath.Join(s.GridHome, "projects/example/knowledge/index.md"))
	if err != nil || !strings.HasPrefix(string(md), "generation: 2\nlevel: project\n") {
		t.Fatalf("index.md header: %q %v", md, err)
	}
}

func TestSessionStartTextCutsLongDescription(t *testing.T) {
	line := func(desc string) string {
		text := SessionStartText([]TopicRow{{Topic: "x", Description: desc}})
		lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		return lines[len(lines)-1]
	}
	if got, want := line(strings.Repeat("a", 200)), "x | "+strings.Repeat("a", 77)+"... | 0 | - | 0"; got != want {
		t.Fatalf("200 bytes: %q", got)
	}
	// 80 bytes is the largest description kept whole.
	if got, want := line(strings.Repeat("a", 80)), "x | "+strings.Repeat("a", 80)+" | 0 | - | 0"; got != want {
		t.Fatalf("80 bytes: %q", got)
	}
	if got, want := line(strings.Repeat("a", 81)), "x | "+strings.Repeat("a", 77)+"... | 0 | - | 0"; got != want {
		t.Fatalf("81 bytes: %q", got)
	}
	// Pipes cannot break the table, white space collapses, a date keeps 10 bytes.
	text := SessionStartText([]TopicRow{{Topic: "x", Description: "a|b  \n c", LastChange: "2026-03-02T08:30:00Z", Entries: 3, Invalid: 1}})
	if !strings.HasSuffix(text, "x | a/b c | 3 | 2026-03-02 | 1\n") {
		t.Fatalf("row format: %q", text)
	}
}

func TestSessionStartTextElidesOverflowWithinBudget(t *testing.T) {
	var rs []TopicRow
	for i := range 40 {
		rs = append(rs, TopicRow{Topic: fmt.Sprintf("topic-%02d", i), Description: strings.Repeat("d", 80), Entries: i})
	}
	text := SessionStartText(rs)
	if len(text) > 1200 {
		t.Fatalf("%d bytes over budget", len(text))
	}
	shown := strings.Count(text, "\ntopic-")
	if shown == 0 || shown >= 40 {
		t.Fatalf("%d rows shown", shown)
	}
	banner := fmt.Sprintf("%d topics cut; grid brain topics lists them.\n", 40-shown)
	if !strings.HasSuffix(text, banner) {
		t.Fatalf("banner %q missing: %s", banner, text)
	}
	// A short table needs no banner.
	if short := SessionStartText(rs[:3]); strings.Contains(short, "cut") {
		t.Fatalf("banner on a short table: %s", short)
	}
}

func TestSessionStartTextCutsMultibyteOnRuneBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, unit string
		keep       int // whole runes that fit in 77 bytes
	}{{"two-byte", "é", 38}, {"three-byte", "界", 25}, {"four-byte", "😀", 19}} {
		t.Run(tc.name, func(t *testing.T) {
			text := SessionStartText([]TopicRow{{Topic: "x", Description: strings.Repeat(tc.unit, 100)}})
			if !utf8.ValidString(text) {
				t.Fatalf("invalid UTF-8: %q", text)
			}
			want := "x | " + strings.Repeat(tc.unit, tc.keep) + "... | 0 | - | 0\n"
			if !strings.HasSuffix(text, want) {
				t.Fatalf("cut: %q", text)
			}
		})
	}
}

func TestTruncateNeverSplitsARune(t *testing.T) {
	s := "aé界😀z"
	for n := 0; n <= len(s)+2; n++ {
		got := truncate(s, n)
		if !utf8.ValidString(got) || len(got) > n || !strings.HasPrefix(s, got) {
			t.Fatalf("truncate(%d) = %q", n, got)
		}
		// It keeps as much as fits: the next rune would not.
		if len(got) < len(s) {
			_, size := utf8.DecodeRuneInString(s[len(got):])
			if len(got)+size <= n {
				t.Fatalf("truncate(%d) = %q dropped a rune that fits", n, got)
			}
		}
	}
	if truncate("abc", 3) != "abc" || truncate("abc", 0) != "" {
		t.Fatal("ASCII bounds")
	}
}

func TestDeclaresLevelAndAllows(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared any
		home, pr bool
	}{
		{"home string", "home", true, false},
		{"project string", "project", false, true},
		{"pair string", "home, project", true, true},
		{"padded", "  project ", false, true},
		{"empty", "", false, false},
		{"unknown", "team", false, false},
		{"no separator", "homeproject", false, false},
		{"list", []any{"home", "project"}, true, true},
		{"list of one", []any{"home"}, true, false},
		{"list with a non-string", []any{1, "project"}, false, true},
		{"nil", nil, false, false},
		{"number", 42, false, false},
	} {
		if got := declaresLevel(tc.declared, "home"); got != tc.home {
			t.Errorf("%s: home = %v", tc.name, got)
		}
		if got := declaresLevel(tc.declared, "project"); got != tc.pr {
			t.Errorf("%s: project = %v", tc.name, got)
		}
	}
	for _, tc := range []struct {
		name     string
		levels   []string
		home, pr bool
	}{
		{"home only", []string{"home"}, true, false},
		{"project only", []string{"project"}, false, true},
		{"both", []string{"home", "project"}, true, true},
		{"none", nil, false, false},
	} {
		topic := Topic{Name: "t", Levels: tc.levels}
		if allows(topic, "home") != tc.home || allows(topic, "project") != tc.pr {
			t.Errorf("allows %s: home %v project %v", tc.name, allows(topic, "home"), allows(topic, "project"))
		}
	}
}

func TestScopeTopicsAndTopicsMergeBothLevels(t *testing.T) {
	s := testStore(t)
	const proj = "projects/example/knowledge/"
	const home = "knowledge/"
	putFile(t, s, home+"homeonly/TOPIC.md", topicDoc("homeonly", "home"))
	putFile(t, s, home+"both/TOPIC.md", topicDoc("both", "home, project"))
	putFile(t, s, proj+"both/TOPIC.md", topicDoc("both", "home, project"))
	putFile(t, s, proj+"projonly/TOPIC.md", topicDoc("projonly", "project"))
	putFile(t, s, proj+"misplaced/TOPIC.md", topicDoc("misplaced", "home"))
	putFile(t, s, proj+"weird/TOPIC.md", topicDoc("weird", "team"))

	// Built-in topics come first at each level, only where they are allowed.
	for _, level := range []string{"project", "home"} {
		got, err := scopeTopics(openScope(t, s, level), level)
		if err != nil {
			t.Fatal(err)
		}
		var builtin []string
		for _, b := range BuiltinTopics() {
			if allows(b, level) {
				builtin = append(builtin, b.Name)
			}
		}
		var names []string
		kinds := map[string]string{}
		for _, topic := range got {
			names = append(names, topic.Name)
			kinds[topic.Name] = topic.Kind
		}
		if !reflect.DeepEqual(names[:len(builtin)], builtin) {
			t.Fatalf("%s built-ins: %v want %v", level, names, builtin)
		}
		custom := names[len(builtin):]
		var want map[string]string
		if level == "project" {
			want = map[string]string{"both": "knowledge", "misplaced": "invalid", "projonly": "knowledge", "weird": "invalid"}
		} else {
			want = map[string]string{"both": "knowledge", "homeonly": "knowledge"}
		}
		if len(custom) != len(want) {
			t.Fatalf("%s custom topics: %v", level, custom)
		}
		for _, name := range custom {
			if kinds[name] != want[name] {
				t.Fatalf("%s: %s kind %q want %q", level, name, kinds[name], want[name])
			}
		}
	}

	all, err := s.Topics()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Topic{}
	for _, topic := range all {
		if _, dup := byName[topic.Name]; dup {
			t.Fatalf("%s listed twice: %+v", topic.Name, all)
		}
		byName[topic.Name] = topic
	}
	// Every built-in is present at each level it allows, unchanged by custom folders.
	for _, b := range BuiltinTopics() {
		if got, ok := byName[b.Name]; !ok || !reflect.DeepEqual(got.Levels, b.Levels) || got.Custom {
			t.Fatalf("builtin %s: %+v", b.Name, got)
		}
	}
	for name, want := range map[string][]string{
		"homeonly": {"home"},
		"projonly": {"project"},
		"both":     {"project", "home"},
	} {
		got := byName[name]
		if got.Kind != "knowledge" || !got.Custom || !reflect.DeepEqual(got.Levels, want) {
			t.Fatalf("%s: %+v want levels %v", name, got, want)
		}
	}
	if got := byName["homeonly"]; got.Description != "About homeonly" {
		t.Fatalf("description: %+v", got)
	}
	// Unknown or wrong-level declarations stay visible, flagged invalid.
	for _, name := range []string{"weird", "misplaced"} {
		if got := byName[name]; got.Kind != "invalid" || got.Description != "Invalid topic definition" {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	if len(all) != len(BuiltinTopics())+5 {
		t.Fatalf("topic count %d: %+v", len(all), all)
	}
}
