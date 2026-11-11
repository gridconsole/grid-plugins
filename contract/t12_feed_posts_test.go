package contractdata_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// t12PostsSection is one "## <type>" section of the page: its heading and the
// lines under it, up to the next heading outside a code fence.
type t12PostsSection struct {
	name  string
	lines []string
}

// t12PostsSplit cuts the page into sections. Headings inside a code fence do
// not count. Lines before the first heading are returned as the preface.
func t12PostsSplit(text string) (preface []string, sections []t12PostsSection) {
	fenced := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(line, "## ") {
			sections = append(sections, t12PostsSection{name: strings.TrimSpace(strings.TrimPrefix(line, "## "))})
			continue
		}
		if len(sections) == 0 {
			preface = append(preface, line)
			continue
		}
		last := &sections[len(sections)-1]
		last.lines = append(last.lines, line)
	}
	return preface, sections
}

// t12PostsLine is the registration line a section must carry right under its heading.
func t12PostsLine(t contract.PostType) string {
	return fmt.Sprintf("Version %d. Class %s. Written by %s.", t.Version, t.Class, strings.Join(t.Roles, ", "))
}

// t12PostsStructure reports, sorted and without repeats, every type name whose
// section is missing, repeated or has a wrong registration line, and every
// heading that is not a post type.
func t12PostsStructure(text string, types []contract.PostType) []string {
	_, sections := t12PostsSplit(text)
	count := map[string]int{}
	byName := map[string]t12PostsSection{}
	for _, s := range sections {
		count[s.name]++
		byName[s.name] = s
	}
	bad := map[string]bool{}
	known := map[string]bool{}
	for _, t := range types {
		known[t.Name] = true
		if count[t.Name] != 1 {
			bad[t.Name] = true
			continue
		}
		lines := byName[t.Name].lines
		if len(lines) == 0 || lines[0] != t12PostsLine(t) {
			bad[t.Name] = true
		}
	}
	for name := range count {
		if !known[name] {
			bad[name] = true
		}
	}
	return t12PostsKeys(bad)
}

// t12PostsExamples reports every type whose first json block does not validate
// as that type or whose type field differs from the heading.
func t12PostsExamples(text string, types []contract.PostType) []string {
	_, sections := t12PostsSplit(text)
	byName := map[string]t12PostsSection{}
	for _, s := range sections {
		byName[s.name] = s
	}
	bad := map[string]bool{}
	for _, t := range types {
		block, ok := t12PostsFirstJSON(byName[t.Name].lines)
		if !ok || contract.Validate(t.Name, []byte(block)) != nil {
			bad[t.Name] = true
			continue
		}
		var rec struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(block), &rec) != nil || rec.Type != t.Name {
			bad[t.Name] = true
		}
	}
	return t12PostsKeys(bad)
}

func t12PostsFirstJSON(lines []string) (string, bool) {
	start := -1
	for i, line := range lines {
		if line == "```json" {
			start = i + 1
			continue
		}
		if start >= 0 && line == "```" {
			return strings.Join(lines[start:i], "\n"), true
		}
	}
	return "", false
}

func t12PostsKeys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func t12PostsPage(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("docs/feed-posts.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// t12PostsSynthetic builds a page from the registrations, one valid-looking
// section per type, so the negative cases need no file change.
func t12PostsSynthetic(types []contract.PostType) string {
	var b strings.Builder
	for _, t := range types {
		fmt.Fprintf(&b, "## %s\n%s\n\nBody text.\n\n```json\n{\"type\": %q}\n```\n\n", t.Name, t12PostsLine(t), t.Name)
	}
	return b.String()
}

func TestT12PostsEveryType(t *testing.T) {
	types := contract.PostTypes()
	if len(types) == 0 {
		t.Fatal("contract.PostTypes() is empty")
	}
	page := t12PostsPage(t)
	if bad := t12PostsStructure(page, types); len(bad) > 0 {
		t.Fatalf("sections missing, repeated, with a wrong line or not a post type: %v", bad)
	}
	_, sections := t12PostsSplit(page)
	var names []string
	for _, s := range sections {
		names = append(names, s.name)
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("sections are not sorted by name: %v", names)
	}
}

func TestT12PostsPreface(t *testing.T) {
	preface, _ := t12PostsSplit(t12PostsPage(t))
	head := strings.Join(preface, "\n")
	for _, want := range []string{"grid feed post", "feed_post", "bodies/<ulid>.json"} {
		if !strings.Contains(head, want) {
			t.Errorf("page opening does not mention %q", want)
		}
	}
}

func TestT12PostsExamplesValidate(t *testing.T) {
	if bad := t12PostsExamples(t12PostsPage(t), contract.PostTypes()); len(bad) > 0 {
		t.Fatalf("first json block does not validate as its type: %v", bad)
	}
}

func TestT12PostsNegative(t *testing.T) {
	types := contract.PostTypes()
	if len(types) < 2 {
		t.Fatal("need at least two post types")
	}
	whole := t12PostsSynthetic(types)
	if bad := t12PostsStructure(whole, types); len(bad) != 0 {
		t.Fatalf("synthetic whole page reported %v", bad)
	}

	// One section left out.
	gone := types[1]
	var rest []contract.PostType
	for _, pt := range types {
		if pt.Name != gone.Name {
			rest = append(rest, pt)
		}
	}
	if bad := t12PostsStructure(t12PostsSynthetic(rest), types); len(bad) != 1 || bad[0] != gone.Name {
		t.Errorf("left-out section: got %v, want [%s]", bad, gone.Name)
	}

	// A section line that names the wrong class.
	wrong := types[0]
	other := "chatter"
	if wrong.Class == "chatter" {
		other = "state"
	}
	bent := strings.Replace(whole, t12PostsLine(wrong), strings.Replace(t12PostsLine(wrong), "Class "+wrong.Class, "Class "+other, 1), 1)
	if bad := t12PostsStructure(bent, types); len(bad) != 1 || bad[0] != wrong.Name {
		t.Errorf("wrong class: got %v, want [%s]", bad, wrong.Name)
	}

	// A heading that is not a post type, and a repeated heading.
	if bad := t12PostsStructure(whole+"## envelope\nText.\n", types); len(bad) != 1 || bad[0] != "envelope" {
		t.Errorf("extra heading: got %v, want [envelope]", bad)
	}
	if bad := t12PostsStructure(whole+"## "+wrong.Name+"\n"+t12PostsLine(wrong)+"\n", types); len(bad) != 1 || bad[0] != wrong.Name {
		t.Errorf("repeated heading: got %v, want [%s]", bad, wrong.Name)
	}

	// The synthetic example blocks hold only a type field, so none validates.
	if bad := t12PostsExamples(whole, types); len(bad) != len(types) {
		t.Errorf("incomplete example blocks: got %d reported, want %d", len(bad), len(types))
	}
}
