package job_test

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// saved is inputs.json as a reader sees it: decoded generically, so a field the
// writer does not emit shows as absent and the tests compile on any base.
type saved struct {
	raw     map[string]any
	sources map[string]map[string]any
}

func decodeInputs(t *testing.T, in job.Inputs) saved {
	t.Helper()
	b, e := in.JSON()
	if e != nil {
		t.Fatal(e)
	}
	s := saved{sources: map[string]map[string]any{}}
	if e := json.Unmarshal(b, &s.raw); e != nil {
		t.Fatal(e)
	}
	list, _ := s.raw["sources"].([]any)
	for _, x := range list {
		m := x.(map[string]any)
		s.sources[m["pointer"].(string)] = m
	}
	return s
}

// shown is the saved "shown" of a source, or "" when the file has none.
func (s saved) shown(t *testing.T, pointer string) string {
	t.Helper()
	m, ok := s.sources[pointer]
	if !ok {
		t.Fatalf("%s is not in inputs.json", pointer)
	}
	v, _ := m["shown"].(string)
	return v
}

func (s saved) textRunes() int {
	n := 0
	for _, m := range s.sources {
		n += utf8.RuneCountInString(m["text"].(string))
	}
	return n
}

const (
	bigRead  = "repo:docs/big.md"
	bigLine  = "BIG-BODY-LINE the file's own text, named once in read and never pasted.\n"
	deployMD = "repo:docs/deploy.md"
)

// bigBody is a long file: the heading is its summary, the rest is body text.
func bigBody() string { return "# Big file\n\n" + strings.Repeat(bigLine, 200) }

// Row 1: a Job with an intent file, a brain link and a read file. inputs.json must
// say the read file was not pasted; on main it has no field that says so.
func TestInputsSaysTheReadFileWasNotPasted(t *testing.T) {
	j, f, h := sample(t)
	writeRepoFile(t, h.Folder, "docs/big.md", bigBody())
	j.Read = []string{bigRead}
	got := render(t, j, f)
	in := decodeInputs(t, got.Inputs)
	if g := in.shown(t, bigRead); g != "summary" {
		t.Fatalf("the read file %s has shown %q in inputs.json, want \"summary\" (its brief line shows only the summary)", bigRead, g)
	}
	if strings.Contains(got.Brief, "BIG-BODY-LINE") {
		t.Fatal("the read file's text was pasted into the brief")
	}
}

// Row 2: each use of a source gets its mode, with the real shapes the resolver
// gives: a plan item, a brain note, a note the budget cut, a repo target, a read
// file, a personal note and the previous acceptance.
func TestInputsShownModePerUse(t *testing.T) {
	j, f, h := sample(t)
	writeRepoFile(t, h.Folder, "docs/deploy.md", "# How deploys are tagged\n\nDEPLOY-BODY\n")
	writeRepoFile(t, h.Folder, "docs/big.md", bigBody())
	writeRepoFile(t, h.Folder, "docs/nosummary.md", "\n")
	h.Note("home", "my-habits", "decisions", "HOME-SUMMARY-MARKER", "HOME-BODY-MARKER\n")
	writeRepoFile(t, h.Task, "jobs/J-0/brief.md", "# Job\n\n## Acceptance\n\n- A1: Earlier row. (traces: item)\n")
	prev := "J-0"
	j.Continues = &prev
	j.Links = append(j.Links, job.Link{To: deployMD, Rel: "uses", Why: "Tag names"})
	j.Read = []string{bigRead, "brain:home/my-habits"}
	got := render(t, j, f)
	in := decodeInputs(t, got.Inputs)
	for p, want := range map[string]string{
		"intent.md#request":           "text",
		"plan.md#step-1":              "text",
		"plan.md#verify":              "text",
		"brain:project/export-format": "excerpt",
		deployMD:                      "summary",
		bigRead:                       "summary",
		"brain:home/my-habits":        "pointer",
		"job:J-0#acceptance":          "text",
	} {
		if g := in.shown(t, p); g != want {
			t.Errorf("%s: shown %q, want %q", p, g, want)
		}
	}
	// A read file with no summary is named by its path alone.
	j.Read = []string{"repo:docs/nosummary.md"}
	in = decodeInputs(t, render(t, j, f).Inputs)
	if g := in.shown(t, "repo:docs/nosummary.md"); g != "pointer" {
		t.Errorf("a read with no summary: shown %q, want \"pointer\"", g)
	}
}

// Row 2: a brain note the budget cut is listed in not_rendered and has no mode.
func TestInputsShownIsAbsentForACutNote(t *testing.T) {
	j, f, _ := sample(t)
	got, e := job.Render(j, job.RenderOptions{Resolver: f, BrainTokens: 32})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Inputs.NotRendered) != 1 || got.Inputs.NotRendered[0] != "brain:project/export-format" {
		t.Fatalf("the Job no longer cuts the note: %v", got.Inputs.NotRendered)
	}
	in := decodeInputs(t, got.Inputs)
	if g := in.shown(t, "brain:project/export-format"); g != "" {
		t.Fatalf("a cut note has shown %q", g)
	}
	if _, has := in.sources["brain:project/export-format"]["shown"]; has {
		t.Fatal("a cut note carries a shown key")
	}
}

// Row 3: the strongest use wins, so an excerpt that is also read stays an excerpt.
func TestInputsShownStrongestUseWins(t *testing.T) {
	j, f, _ := sample(t)
	j.Read = []string{"brain:project/export-format", "plan.md#step-1"}
	got := render(t, j, f)
	in := decodeInputs(t, got.Inputs)
	if g := in.shown(t, "brain:project/export-format"); g != "excerpt" {
		t.Errorf("a note used as excerpt and read: shown %q, want \"excerpt\"", g)
	}
	if g := in.shown(t, "plan.md#step-1"); g != "text" {
		t.Errorf("a plan item used as intent and read: shown %q, want \"text\"", g)
	}
	// A cut note that is also read is a pointer, not nothing.
	cut, e := job.Render(j, job.RenderOptions{Resolver: f, BrainTokens: 32})
	if e != nil {
		t.Fatal(e)
	}
	if g := decodeInputs(t, cut.Inputs).shown(t, "brain:project/export-format"); g != "summary" {
		t.Errorf("a cut note that is read: shown %q, want \"summary\"", g)
	}
}

// Row 4: brief_bytes is the brief's length and pasted_chars the text written into it.
func TestInputsSizes(t *testing.T) {
	j, f, h := sample(t)
	writeRepoFile(t, h.Folder, "docs/big.md", bigBody())
	j.Read = []string{bigRead}
	got := render(t, j, f)
	in := decodeInputs(t, got.Inputs)
	if g, _ := in.raw["brief_bytes"].(float64); int(g) != len(got.Brief) || len(got.Brief) == 0 {
		t.Fatalf("brief_bytes is %v, want %d", in.raw["brief_bytes"], len(got.Brief))
	}
	// Pasted: three intent sources and the one brain excerpt, each by its rune count.
	src := got.Inputs.Resolver()
	want := 0
	for _, p := range []string{"intent.md#request", "plan.md#step-1", "plan.md#verify", "brain:project/export-format"} {
		want += utf8.RuneCountInString(src[p].Text)
	}
	if g, _ := in.raw["pasted_chars"].(float64); int(g) != want || want == 0 {
		t.Fatalf("pasted_chars is %v, want %d", in.raw["pasted_chars"], want)
	}
	if total := in.textRunes(); want >= total {
		t.Fatalf("pasted_chars %d is not below the summed sources[].text %d", want, total)
	}
	t.Logf("brief_bytes %d, pasted_chars %d, summed sources[].text %d", len(got.Brief), want, in.textRunes())
}

// Row 4: a guest source's fenced text counts as pasted, and its summary does not.
func TestInputsPastedCountsFencedText(t *testing.T) {
	j, f, _ := sample(t)
	src := replayWith(t, j, f, contract.External, "A summary")
	s := src["plan.md#verify"]
	s.Tier = contract.External
	src[s.Pointer] = s
	got := render(t, j, src)
	in := decodeInputs(t, got.Inputs)
	want := 0
	for _, p := range []string{"intent.md#request", "plan.md#step-1", "plan.md#verify"} {
		want += utf8.RuneCountInString(src[p].Text)
	}
	want += utf8.RuneCountInString(src["brain:project/export-format"].Text)
	if g, _ := in.raw["pasted_chars"].(float64); int(g) != want {
		t.Fatalf("pasted_chars is %v, want %d", in.raw["pasted_chars"], want)
	}
	if g := in.shown(t, "plan.md#verify"); g != "text" {
		t.Fatalf("an external intent source: shown %q, want \"text\"", g)
	}
}

// Row 5: the new file replays byte for byte, and a file written by main, with no
// shown and no sizes, still replays and comes out with them.
func TestInputsReplayWithAndWithoutShown(t *testing.T) {
	j, f, h := sample(t)
	writeRepoFile(t, h.Folder, "docs/big.md", bigBody())
	j.Read = []string{bigRead}
	first := render(t, j, f)
	b, e := first.Inputs.JSON()
	if e != nil {
		t.Fatal(e)
	}
	var loaded job.Inputs
	if e := json.Unmarshal(b, &loaded); e != nil {
		t.Fatal(e)
	}
	again := render(t, j, loaded.Resolver())
	if again.Brief != first.Brief {
		t.Fatal("replay from the new inputs.json differs from the first brief")
	}
	// Strip the new keys, as main wrote the file.
	var m map[string]any
	if e := json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	delete(m, "brief_bytes")
	delete(m, "pasted_chars")
	for _, x := range m["sources"].([]any) {
		delete(x.(map[string]any), "shown")
	}
	old, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	var legacy job.Inputs
	if e := json.Unmarshal(old, &legacy); e != nil {
		t.Fatal(e)
	}
	replay := render(t, j, legacy.Resolver())
	if replay.Brief != first.Brief {
		t.Fatal("replay from an inputs.json with no shown differs")
	}
	rb, _ := replay.Inputs.JSON()
	if string(rb) != string(b) {
		t.Fatalf("replaying a legacy file does not rebuild the shown and size fields:\n%s\n---\n%s", rb, b)
	}
}

// Row 6: the new inputs.json and one written by main both pass the saved-format schema.
func TestInputsValidateAgainstTheSchema(t *testing.T) {
	j, f, h := sample(t)
	writeRepoFile(t, h.Folder, "docs/big.md", bigBody())
	j.Read = []string{bigRead}
	got := render(t, j, f)
	b, e := got.Inputs.JSON()
	if e != nil {
		t.Fatal(e)
	}
	wrap := func(inputs []byte) []byte {
		out, e := json.Marshal(struct {
			Brief  string          `json:"brief"`
			Inputs json.RawMessage `json:"inputs"`
		}{got.Brief, inputs})
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	if e := contract.Validate("fmt-job-render", wrap(b)); e != nil {
		t.Fatalf("new inputs.json: %v", e)
	}
	var m map[string]any
	if e := json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	delete(m, "brief_bytes")
	delete(m, "pasted_chars")
	for _, x := range m["sources"].([]any) {
		delete(x.(map[string]any), "shown")
	}
	old, _ := json.Marshal(m)
	if e := contract.Validate("fmt-job-render", wrap(old)); e != nil {
		t.Fatalf("inputs.json written by main: %v", e)
	}
	// A mode outside the four is refused.
	m["sources"].([]any)[0].(map[string]any)["shown"] = "everything"
	bad, _ := json.Marshal(m)
	if contract.Validate("fmt-job-render", wrap(bad)) == nil {
		t.Fatal("the schema accepts shown: everything")
	}
}
