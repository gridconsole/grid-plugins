package formats_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// hfRiKnownOpen names a difference between the doc, the schema and job.Inputs
// that is known and left as it is. The key is the message the comparison
// produces; the value says where each side is wrong. Each test checks the list
// both ways: a difference not listed fails, and a listed one that no longer
// differs fails too. It is empty: the doc was fixed to match.
var hfRiKnownOpen = map[string]string{}

// hfRiSchema is the part of fmt-job-render.json the tests compare: the
// properties and the required list of the `inputs` object.
type hfRiSchema struct {
	Props    []string
	Required []string
}

func hfRiLoadSchema(t *testing.T) hfRiSchema {
	t.Helper()
	raw, err := contract.Schema("fmt-job-render")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Properties struct {
			Inputs struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			} `json:"inputs"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	in := doc.Properties.Inputs
	if len(in.Properties) == 0 {
		t.Fatal("fmt-job-render.json has no inputs properties")
	}
	s := hfRiSchema{Required: append([]string(nil), in.Required...)}
	for name := range in.Properties {
		s.Props = append(s.Props, name)
	}
	sort.Strings(s.Props)
	sort.Strings(s.Required)
	return s
}

// hfRiDocRow is one `inputs.*` row of the field table: the field name without
// the prefix and the text of its presence column.
type hfRiDocRow struct {
	Name, Presence string
}

// hfRiDoc is what the "Saved Job render" section says about `inputs`.
type hfRiDoc struct {
	Rows      []hfRiDocRow
	InputsRow string // the meaning cell of the `inputs` row
}

var hfRiName = regexp.MustCompile("^`inputs\\.([A-Za-z_]+)`$")

func hfRiLoadDoc(t *testing.T) hfRiDoc {
	t.Helper()
	b, err := os.ReadFile("../formats.md")
	if err != nil {
		t.Fatal(err)
	}
	_, rest, ok := strings.Cut(string(b), "\n## Saved Job render\n")
	if !ok {
		t.Fatal("formats.md has no Saved Job render section")
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	var d hfRiDoc
	seen := map[string]bool{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) < 3 {
			continue
		}
		if cells[0] == "`inputs`" {
			d.InputsRow = cells[2]
		}
		m := hfRiName.FindStringSubmatch(cells[0])
		if m == nil {
			continue
		}
		if seen[m[1]] {
			t.Errorf("formats.md lists inputs.%s twice", m[1])
		}
		seen[m[1]] = true
		d.Rows = append(d.Rows, hfRiDocRow{Name: m[1], Presence: cells[1]})
	}
	if d.InputsRow == "" || len(d.Rows) == 0 {
		t.Fatalf("formats.md: inputs row %q, %d inputs.* rows; the table is not where the test reads it", d.InputsRow, len(d.Rows))
	}
	return d
}

func hfRiNames(rows []hfRiDocRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}

// hfRiDiff lists the names found on one side only, as messages.
func hfRiDiff(aName string, a []string, bName string, b []string) []string {
	in := func(list []string, s string) bool {
		for _, x := range list {
			if x == s {
				return true
			}
		}
		return false
	}
	var out []string
	for _, x := range a {
		if !in(b, x) {
			out = append(out, x+": in "+aName+", not in "+bName)
		}
	}
	for _, x := range b {
		if !in(a, x) {
			out = append(out, x+": in "+bName+", not in "+aName)
		}
	}
	return out
}

// hfRiCheck fails for each difference that is not known open, and for each
// known-open entry of this test (prefix) that no longer differs.
func hfRiCheck(t *testing.T, prefix string, got []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, g := range got {
		key := prefix + g
		seen[key] = true
		if _, ok := hfRiKnownOpen[key]; !ok {
			t.Errorf("%s", key)
		}
	}
	for key, why := range hfRiKnownOpen {
		if strings.HasPrefix(key, prefix) && !seen[key] {
			t.Errorf("known-open entry no longer differs, remove it: %s (%s)", key, why)
		}
	}
}

// TestHfRiRows: the `inputs.*` rows of the doc table and the properties of
// `inputs` in the schema are the same set.
func TestHfRiRows(t *testing.T) {
	s, d := hfRiLoadSchema(t), hfRiLoadDoc(t)
	hfRiCheck(t, "rows: ", hfRiDiff("formats.md", hfRiNames(d.Rows), "schema", s.Props))
}

var hfRiCount = regexp.MustCompile(`\b(\w+) required fields\b`)

var hfRiWords = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12}

// TestHfRiRequired: each row says required or optional, the required rows are
// the schema's `inputs.required` list, and the `inputs` row states its count.
func TestHfRiRequired(t *testing.T) {
	s, d := hfRiLoadSchema(t), hfRiLoadDoc(t)
	var req []string
	for _, r := range d.Rows {
		switch r.Presence {
		case "required":
			req = append(req, r.Name)
		case "optional":
		default:
			t.Errorf("inputs.%s: presence %q, want required or optional", r.Name, r.Presence)
		}
	}
	sort.Strings(req)
	hfRiCheck(t, "required: ", hfRiDiff("formats.md", req, "schema", s.Required))

	m := hfRiCount.FindStringSubmatch(d.InputsRow)
	if m == nil {
		t.Fatalf("the inputs row has no sentence of the form \"<count> required fields\": %s", d.InputsRow)
	}
	n, ok := hfRiWords[strings.ToLower(m[1])]
	if !ok {
		t.Fatalf("the inputs row count %q is not a number word", m[1])
	}
	if want := len(s.Required); n != want {
		hfRiCheck(t, "count: ", []string{m[1] + " in formats.md, " + strconv.Itoa(want) + " in schema"})
	} else {
		hfRiCheck(t, "count: ", nil)
	}
}

// TestHfRiStruct: the json tags of job.Inputs are the properties of `inputs`.
func TestHfRiStruct(t *testing.T) {
	s := hfRiLoadSchema(t)
	var tags []string
	rt := reflect.TypeOf(job.Inputs{})
	for i := 0; i < rt.NumField(); i++ {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			t.Errorf("job.Inputs.%s has no json name", rt.Field(i).Name)
			continue
		}
		tags = append(tags, name)
	}
	sort.Strings(tags)
	hfRiCheck(t, "struct: ", hfRiDiff("job.Inputs", tags, "schema", s.Props))
}
