package brain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// pbVrWrite puts a file below dir, making its folders.
func pbVrWrite(t *testing.T, dir, rel, data string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pbVrVault builds a vault with the six fixtures of the lane: F1 no front matter,
// F2 tags, aliases and cssclasses, F3 nested folders, F4 spaces and Unicode in
// names (the file name is stored decomposed, as macOS does), F5 [[x|y]] and
// [[x#heading]] links, F6 an .obsidian folder.
func pbVrVault(t *testing.T) Store {
	t.Helper()
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	pbVrWrite(t, vault, "Plain.md", "Just a paragraph about plain things.\n")                                                                                                                                                                                                                                                           // F1
	pbVrWrite(t, vault, "Ideas/Tagged.md", "---\ntags: [alpha, \"#beta\"]\naliases: [Tg, Tagged note]\ncssclasses: [wide]\n---\n# Tagged\nBody with #inline and #2024 and `#code`.\n")                                                                                                                                                  // F2
	pbVrWrite(t, vault, "Ideas/Deep/Deeper/Nested.md", "Nested note.\n")                                                                                                                                                                                                                                                                // F3
	pbVrWrite(t, vault, "Ideas/Café plan.md", "# Plan\n\nPlan the **café** with [[Plain]].\n")                                                                                                                                                                                                                                         // F4
	pbVrWrite(t, vault, "Links.md", "Intro.\n\n| a | b |\n|---|---|\n| [[Plain\\|the plain note]] | x |\n\n[[Tagged#Heading]] ![[Nested]] [md](Ideas/Caf%C3%A9%20plan.md) [[#same]] [[Ghost]] `[[InCode]]`\n\n```\n[[InFence]]\n```\n\n## Links\n- [[Plain]] - explains: why this one matters\n- [[Nested]] - bogus: not a relation\n") // F5
	pbVrWrite(t, vault, ".obsidian/app.json", "{}")                                                                                                                                                                                                                                                                                     // F6
	pbVrWrite(t, vault, ".obsidian/Skipped.md", "no\n")
	pbVrWrite(t, vault, ".trash/Old.md", "no\n")
	pbVrWrite(t, vault, "Proposals/p.md", "no\n")
	pbVrWrite(t, vault, "Ideas/image.png", "png")
	pbVrWrite(t, vault, "projects/demo/Proj.md", "Project note.\n")
	pbVrWrite(t, vault, "projects/other/Other.md", "Other project.\n")
	return Store{GridHome: filepath.Join(dir, "grid"), BrainDir: vault, Project: "demo", Author: "Sam"}
}

func pbVrRows(t *testing.T, s Store) map[string]ListRow {
	t.Helper()
	l, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]ListRow{}
	for _, r := range l.Notes {
		out[r.ID] = r
	}
	return out
}

func TestPbVrFixturesList(t *testing.T) {
	s := pbVrVault(t)
	rows := pbVrRows(t, s)
	var ids []string
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	want := []string{
		"brain:home/Ideas/Deep/Deeper/Nested", "brain:home/Ideas/Tagged", "brain:home/Ideas/Café plan",
		"brain:home/Links", "brain:home/Plain", "brain:project/Proj",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("listed ids\n got %q\nwant %q", ids, want)
	}
	// F1: no front matter still lists, title from the file name, description from the text.
	p := rows["brain:home/Plain"]
	if p.Title != "Plain" || p.Description != "Just a paragraph about plain things." || p.Summary != p.Description || p.Topic != "-" || p.Kind != "note" || p.Origin != "person" {
		t.Fatalf("F1 row: %+v", p)
	}
	// F2: tags (front, # stripped, inline, no digits-only, none from code), aliases kept.
	tg := rows["brain:home/Ideas/Tagged"]
	if !reflect.DeepEqual(tg.Tags, []string{"alpha", "beta", "inline"}) || !reflect.DeepEqual(tg.Aliases, []string{"Tg", "Tagged note"}) || tg.Topic != "Ideas" {
		t.Fatalf("F2 row: %+v", tg)
	}
	// F3: any depth, the topic is the first folder.
	if n := rows["brain:home/Ideas/Deep/Deeper/Nested"]; n.Path != "Ideas/Deep/Deeper/Nested.md" || n.Topic != "Ideas" {
		t.Fatalf("F3 row: %+v", n)
	}
	// F4: the id is NFC, the path stays as on disk.
	c := rows["brain:home/Ideas/Café plan"]
	if c.Path != "Ideas/Café plan.md" || c.Title != "Café plan" || c.Description != "Plan the café with Plain." {
		t.Fatalf("F4 row: %+v", c)
	}
	// F6: nothing from .obsidian, .trash, Proposals, another project, or non-.md files.
	for id := range rows {
		if strings.Contains(id, "obsidian") || strings.Contains(id, "Old") || strings.Contains(id, "Other") || strings.Contains(id, "p") && id == "brain:home/Proposals/p" {
			t.Fatalf("skipped file listed: %s", id)
		}
	}
	if pr := rows["brain:project/Proj"]; pr.Path != "projects/demo/Proj.md" || pr.Level != "project" || pr.Project != "demo" {
		t.Fatalf("project row: %+v", pr)
	}
}

func TestPbVrLinks(t *testing.T) {
	s := pbVrVault(t)
	r := pbVrRows(t, s)["brain:home/Links"]
	got := map[string]Edge{}
	for _, e := range r.Edges {
		got[e.To+"|"+e.Rel+"|"+e.Anchor] = e
	}
	for _, key := range []string{
		"brain:home/Plain|related|",
		"brain:home/Ideas/Tagged|related|Heading", // F5 [[x#heading]]
		"brain:home/Ideas/Deep/Deeper/Nested|related|",
		"brain:home/Ideas/Café plan|related|", // markdown link with %20 and %C3%A9
		"Ghost|related|",
		"brain:home/Plain|explains|", // the ## Links line
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing edge %q in %+v", key, r.Edges)
		}
	}
	if e := got["brain:home/Plain|explains|"]; e.Why != "why this one matters" || !e.Resolved {
		t.Errorf("links line edge: %+v", e)
	}
	if e := got["Ghost|related|"]; e.Resolved {
		t.Errorf("Ghost must not resolve: %+v", e)
	}
	for _, e := range r.Edges {
		if strings.Contains(e.To, "InCode") || strings.Contains(e.To, "InFence") || e.To == "" || strings.Contains(e.To, "same") {
			t.Errorf("link from code or empty target: %+v", e)
		}
		if e.Rel == "bogus" {
			t.Errorf("unknown relation kept: %+v", e)
		}
	}
	if !reflect.DeepEqual(r.Concepts, []string{"Ghost"}) {
		t.Errorf("concepts: %q", r.Concepts)
	}
	// The same note appears twice as related and as explains; exact duplicates only collapse.
	count := 0
	for _, e := range r.Edges {
		if e.To == "brain:home/Plain" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("Plain edges: %d, want 2 (related, explains)", count)
	}
}

func TestPbVrOldFrontMatterLinks(t *testing.T) {
	s := pbVrVault(t)
	pbVrWrite(t, s.BrainDir, "Old.md", "---\nlinks:\n  - to: brain:home/Plain#top\n    rel: depends-on\n    why: it needs the plain note\nrelated: \"[[Ideas/Tagged]]\"\n---\nBody.\n")
	r := pbVrRows(t, s)["brain:home/Old"]
	want := []Edge{
		{To: "brain:home/Plain", Rel: "depends-on", Why: "it needs the plain note", Anchor: "top", Resolved: true},
		{To: "brain:home/Ideas/Tagged", Rel: "related", Resolved: true},
	}
	if !reflect.DeepEqual(r.Edges, want) {
		t.Fatalf("edges\n got %+v\nwant %+v", r.Edges, want)
	}
}

func TestPbVrLookupOrder(t *testing.T) {
	s := pbVrVault(t)
	pbVrWrite(t, s.BrainDir, "A/Same.md", "a\n")
	pbVrWrite(t, s.BrainDir, "B/Same.md", "b\n")
	pbVrWrite(t, s.BrainDir, "B/same2.md", "c\n")
	for id, want := range map[string]string{
		"brain:home/Plain":            "Plain.md",
		"brain:home/plain":            "Plain.md",            // case-insensitive
		"brain:home/Café plan":        "Ideas/Café plan.md", // bare file name, composed
		"brain:home/Café plan":       "Ideas/Café plan.md", // decomposed input
		"brain:home/Ideas/Café plan":  "Ideas/Café plan.md",
		"brain:home/Nested":           "Ideas/Deep/Deeper/Nested.md",
		"brain:home/Deeper/Nested.md": "Ideas/Deep/Deeper/Nested.md", // shortest path, with extension
		"brain:project/Proj":          "projects/demo/Proj.md",
		"A/Same":                      "A/Same.md",
	} {
		e, err := s.Show(id)
		if err != nil {
			t.Errorf("Show(%q): %v", id, err)
			continue
		}
		if e.Path != want || e.Note.Status != "accepted" {
			t.Errorf("Show(%q) path %q, want %q", id, e.Path, want)
		}
	}
	if e, err := s.Show("brain:home/Same"); err == nil {
		t.Errorf("a bare name two notes share is ambiguous, got %q", e.Path)
	}
	if e, err := s.Show("brain:home/Proj"); err == nil {
		t.Errorf("a project note is not at home: %q", e.Path)
	}
	if _, err := s.Show("brain:home/../Plain"); err == nil {
		t.Error("a .. id must fail")
	}
}

func TestPbVrVaultID(t *testing.T) {
	s := pbVrVault(t)
	cases := []struct {
		rel, level, id string
		ok             bool
	}{
		{"Ideas/Café plan.md", "home", "Ideas/Café plan", true},
		{"Plain.md", "home", "Plain", true},
		{"projects/demo/Proj.md", "project", "Proj", true},
		{"projects/other/Other.md", "", "", false},
		{".obsidian/Skipped.md", "", "", false},
		{"Proposals/p.md", "", "", false},
		{"Ideas/image.png", "", "", false},
	}
	for _, c := range cases {
		level, id, ok := s.VaultID(filepath.Join(s.BrainDir, filepath.FromSlash(c.rel)))
		if level != c.level || id != c.id || ok != c.ok {
			t.Errorf("VaultID(%q) = %q %q %v, want %q %q %v", c.rel, level, id, ok, c.level, c.id, c.ok)
		}
	}
	if _, _, ok := s.VaultID(filepath.Join(filepath.Dir(s.BrainDir), "elsewhere.md")); ok {
		t.Error("a file outside the vault has no id")
	}
}

func TestPbVrSearchAndIndexWriteNothing(t *testing.T) {
	s := pbVrVault(t)
	hits, err := s.Search("Tg", "", 5) // an alias
	if err != nil || len(hits) == 0 || hits[0].ID != "brain:home/Ideas/Tagged" {
		t.Fatalf("alias search: %+v %v", hits, err)
	}
	hits, err = s.Search("nested", "Ideas", 5)
	if err != nil || len(hits) == 0 {
		t.Fatalf("topic search: %+v %v", hits, err)
	}
	if hits[0].Path != "Ideas/Deep/Deeper/Nested.md" {
		t.Fatalf("hit path %q", hits[0].Path)
	}
	before := pbVrTree(t, s.BrainDir)
	if _, err := s.RebuildIndex(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Index(); err != nil {
		t.Fatal(err)
	}
	if after := pbVrTree(t, s.BrainDir); !reflect.DeepEqual(before, after) {
		t.Fatalf("index wrote into the vault:\n before %q\n after  %q", before, after)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "knowledge")); err == nil {
		t.Fatal("index wrote knowledge/ into the Grid home")
	}
}

func pbVrTree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func TestPbVrSymlinkRefused(t *testing.T) {
	s := pbVrVault(t)
	if err := os.Symlink(filepath.Join(s.BrainDir, "Plain.md"), filepath.Join(s.BrainDir, "Link.md")); err != nil {
		t.Skip("no symlinks here")
	}
	if _, err := s.List(); err == nil || !strings.Contains(err.Error(), "symbolic links") {
		t.Fatalf("a symbolic link must be refused, got %v", err)
	}
}

// An old knowledge/ brain, with the key absent, still lists, with the old ids.
func TestPbVrLegacyBrainStillLists(t *testing.T) {
	s := testStore(t)
	if s.Vault() {
		t.Fatal("a store with no BrainDir is not a vault")
	}
	accepted(t, s, queryNote("deploy", "decisions", "home", "How we deploy.", "Deploy steps.", "Body.\n"))
	l, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Notes) != 1 {
		t.Fatalf("legacy list: %+v", l)
	}
	r := l.Notes[0]
	if r.ID != "brain:home/deploy" || r.Path != "knowledge/decisions/deploy.md" || r.Topic != "decisions" || r.Title != "deploy" ||
		r.Description != "How we deploy." || r.Summary != "Deploy steps." || r.Kind != "note" || r.Origin != "person" {
		t.Fatalf("legacy row: %+v", r)
	}
	e, err := s.Show("brain:home/deploy")
	if err != nil || e.Path != "knowledge/decisions/deploy.md" {
		t.Fatalf("legacy show: %+v %v", e, err)
	}
	if s.Root() != s.GridHome {
		t.Fatalf("legacy root %q, want the Grid home", s.Root())
	}
}

func TestPbVrListJSONShape(t *testing.T) {
	s := pbVrVault(t)
	l, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	var generic struct {
		Root  string           `json:"root"`
		Notes []map[string]any `json:"notes"`
	}
	if err := json.Unmarshal(b, &generic); err != nil {
		t.Fatal(err)
	}
	if generic.Root != s.BrainDir || len(generic.Notes) == 0 {
		t.Fatalf("envelope: %s", b)
	}
	for _, key := range strings.Fields("id path kind origin author level project task title summary description topic tags aliases concepts edges rev updated conflict") {
		for _, n := range generic.Notes {
			v, ok := n[key]
			if !ok || v == nil {
				t.Fatalf("note %v lacks %s (or it is null)", n["id"], key)
			}
		}
	}
	if generic.Notes[0]["author"] != "Sam" || generic.Notes[0]["rev"] == "" {
		t.Fatalf("author/rev: %v", generic.Notes[0])
	}
}

func TestPbVrIDsAndSplit(t *testing.T) {
	for s, want := range map[string]bool{
		"Ideas/Café plan": true, "a b": true, "日本語/ノート": true, "": false, "a//b": false, "../x": false, "a/./b": false,
		"/a": false, "a/": false, `a\b`: false, "a\nb": false, "a\x00b": false, "\xff": false, strings.Repeat("a", 256): false,
	} {
		if got := ValidID(s); got != want {
			t.Errorf("ValidID(%q) = %v, want %v", s, got, want)
		}
	}
	for s, want := range map[string]bool{"Café plan": true, "a#b": false, "a|b": false, "a^b": false, "a[b": false, "a]b": false} {
		if got := LinkableID(s); got != want {
			t.Errorf("LinkableID(%q) = %v, want %v", s, got, want)
		}
	}
	if NFC("Café ü ñ") != "Café ü ñ" {
		t.Error("NFC does not compose the Latin marks")
	}
}

func TestPbVrParseVaultLenient(t *testing.T) {
	n, err := ParseVault([]byte("---\ntags: a, b c\nunknown: [1, 2]\nweird: {x: 1}\n---\nBody #t1.\n"))
	if err != nil || n.BadFront {
		t.Fatalf("%v %+v", err, n)
	}
	if !reflect.DeepEqual(n.Tags, []string{"a", "b", "c", "t1"}) || n.Front["unknown"] == nil {
		t.Fatalf("%+v", n)
	}
	n, err = ParseVault([]byte("---\ntags: [unclosed\nb: }\n---\nStill read.\n"))
	if err != nil || !n.BadFront || n.Body != "Still read.\n" {
		t.Fatalf("bad front: %v %+v", err, n)
	}
	n, err = ParseVault([]byte("---\nno closing line\n"))
	if err != nil || n.Body != "---\nno closing line\n" {
		t.Fatalf("unclosed: %v %+v", err, n)
	}
	long := strings.Repeat("word ", 80)
	d := firstParagraph("# Title\n\n" + long + "\n\nSecond.")
	if len([]rune(d)) > 200 || !strings.HasSuffix(d, "word") {
		t.Fatalf("description cut: %d %q", len([]rune(d)), d)
	}
}
