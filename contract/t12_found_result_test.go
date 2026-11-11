package contractdata_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

const t12FoundDoc = "docs/found-and-result.md"

var (
	t12FoundRowRE   = regexp.MustCompile("^\\| `([^`]+)` \\|")
	t12FoundBlockRE = regexp.MustCompile("(?s)```json\n(.*?)\n```")
)

// t12FoundSchema is what the page must cover for one type: every property path
// of the inline body and every enum string inside it.
type t12FoundSchema struct {
	fields []string
	enums  []string
}

// t12FoundInline loads a schema and returns its inline body branch: the
// body.oneOf branch whose required list is not ["ref"].
func t12FoundInline(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := contract.Schema(name)
	if err != nil {
		t.Fatalf("schema %s: %v", name, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("schema %s: %v", name, err)
	}
	for _, part := range schema["allOf"].([]any) {
		props, _ := part.(map[string]any)["properties"].(map[string]any)
		body, ok := props["body"].(map[string]any)
		if !ok {
			continue
		}
		for _, branch := range body["oneOf"].([]any) {
			b := branch.(map[string]any)
			if req, _ := json.Marshal(b["required"]); string(req) != `["ref"]` {
				return b
			}
		}
	}
	t.Fatalf("schema %s: no inline body branch", name)
	return nil
}

// t12FoundWalk collects property paths and enum strings, recursively. Array
// items add no path segment. if/then branches only repeat paths, so they are skipped.
func t12FoundWalk(node map[string]any, path string, out *t12FoundSchema) {
	if enum, ok := node["enum"].([]any); ok {
		for _, v := range enum {
			if s, ok := v.(string); ok {
				out.enums = append(out.enums, s)
			}
		}
	}
	if props, ok := node["properties"].(map[string]any); ok {
		for name, child := range props {
			p := name
			if path != "" {
				p = path + "." + name
			}
			out.fields = append(out.fields, p)
			t12FoundWalk(child.(map[string]any), p, out)
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		t12FoundWalk(items, path, out)
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if list, ok := node[key].([]any); ok {
			for _, child := range list {
				t12FoundWalk(child.(map[string]any), path, out)
			}
		}
	}
}

func t12FoundCollect(t *testing.T, name string) t12FoundSchema {
	t.Helper()
	var out t12FoundSchema
	t12FoundWalk(t12FoundInline(t, name), "", &out)
	sort.Strings(out.fields)
	sort.Strings(out.enums)
	return out
}

// t12FoundSection returns the text of the "## <name>" section of the page.
func t12FoundSection(t *testing.T, doc, name string) string {
	t.Helper()
	var lines []string
	in := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = line == "## "+name
			continue
		}
		if in {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		t.Fatalf("page has no section %q", name)
	}
	return strings.Join(lines, "\n")
}

func t12FoundDocText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(t12FoundDoc))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// t12FoundRows lists the first-cell names of a section's table rows.
func t12FoundRows(section string) map[string]bool {
	rows := map[string]bool{}
	for _, line := range strings.Split(section, "\n") {
		if m := t12FoundRowRE.FindStringSubmatch(line); m != nil {
			rows[m[1]] = true
		}
	}
	return rows
}

// t12FoundFieldProblems reports every schema field with no row and every row
// naming a field the schema lacks.
func t12FoundFieldProblems(section string, want t12FoundSchema) []string {
	var out []string
	rows := t12FoundRows(section)
	if !strings.Contains(section, "| Field | Meaning | Who reads it |") {
		out = append(out, "missing table header")
	}
	known := map[string]bool{}
	for _, f := range want.fields {
		known[f] = true
		if !rows[f] {
			out = append(out, fmt.Sprintf("no row for field `%s`", f))
		}
	}
	for r := range rows {
		if !known[r] {
			out = append(out, fmt.Sprintf("row names `%s`, which the schema lacks", r))
		}
	}
	sort.Strings(out)
	return out
}

// t12FoundEnumProblems reports every enum string not written in backticks.
func t12FoundEnumProblems(section string, want t12FoundSchema) []string {
	var out []string
	for _, v := range want.enums {
		if !strings.Contains(section, "`"+v+"`") {
			out = append(out, fmt.Sprintf("enum value `%s` is not in backticks", v))
		}
	}
	return out
}

func t12FoundHas(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestT12FoundFieldRows(t *testing.T) {
	doc := t12FoundDocText(t)
	for _, name := range []string{"found", "result"} {
		want := t12FoundCollect(t, name)
		if len(want.fields) == 0 {
			t.Fatalf("%s: schema walk found no fields", name)
		}
		for _, p := range t12FoundFieldProblems(t12FoundSection(t, doc, name), want) {
			t.Errorf("%s: %s", name, p)
		}
	}
}

func TestT12FoundEnumValues(t *testing.T) {
	doc := t12FoundDocText(t)
	for _, name := range []string{"found", "result"} {
		want := t12FoundCollect(t, name)
		if len(want.enums) == 0 {
			t.Fatalf("%s: schema walk found no enums", name)
		}
		for _, p := range t12FoundEnumProblems(t12FoundSection(t, doc, name), want) {
			t.Errorf("%s: %s", name, p)
		}
	}
}

func TestT12FoundExamples(t *testing.T) {
	doc := t12FoundDocText(t)
	for _, name := range []string{"found", "result"} {
		m := t12FoundBlockRE.FindStringSubmatch(t12FoundSection(t, doc, name))
		if m == nil {
			t.Fatalf("%s: section has no json block", name)
		}
		raw, err := os.ReadFile(filepath.Join("fixtures", name, "valid", "basic.json"))
		if err != nil {
			t.Fatal(err)
		}
		var env map[string]any
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(m[1]), &body); err != nil {
			t.Fatalf("%s: example is not one JSON object: %v", name, err)
		}
		env["body"] = body
		data, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		if err := contract.Validate(name, data); err != nil {
			t.Errorf("%s: example does not validate: %v", name, err)
		}
		// The inline branch must be the one that matched: a bad enum fails.
		body["kind"], body["outcome"] = "nonsense", nil
		bad, _ := json.Marshal(env)
		if contract.Validate(name, bad) == nil {
			t.Errorf("%s: a broken body still validates, so the example proves nothing", name)
		}
	}
}

func TestT12FoundNegative(t *testing.T) {
	doc := t12FoundDocText(t)
	found := t12FoundSection(t, doc, "found")
	want := t12FoundCollect(t, "found")

	// The real section has no problems, so what follows is caused by the edit.
	if p := append(t12FoundFieldProblems(found, want), t12FoundEnumProblems(found, want)...); len(p) != 0 {
		t.Fatalf("found section already has problems: %v", p)
	}

	var kept []string
	for _, line := range strings.Split(found, "\n") {
		if m := t12FoundRowRE.FindStringSubmatch(line); m != nil && m[1] == "kind" {
			continue
		}
		kept = append(kept, line)
	}
	got := t12FoundFieldProblems(strings.Join(kept, "\n"), want)
	if !t12FoundHas(got, "no row for field `kind`") {
		t.Errorf("section without the kind row: reported %v, want `kind`", got)
	}

	got = t12FoundEnumProblems(strings.ReplaceAll(found, "`security`", "security"), want)
	if !t12FoundHas(got, "enum value `security` is not in backticks") {
		t.Errorf("section without `security`: reported %v, want `security`", got)
	}

	got = t12FoundFieldProblems(found+"\n| `bogus` | x | y |\n", want)
	if !t12FoundHas(got, "row names `bogus`, which the schema lacks") {
		t.Errorf("section with an extra row: reported %v, want `bogus`", got)
	}
}
