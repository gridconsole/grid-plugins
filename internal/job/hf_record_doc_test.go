package job_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// hfJrKnownOpen lists code that disagrees with docs/job-contract.md. Each entry
// names a file and the text that makes it wrong; the test fails when the text is
// gone (the entry is stale) and when the module has a writer of a retired field
// that is not listed (the entry is missing).
var hfJrKnownOpen = []struct{ file, text, why string }{
	{"internal/migrate/receipt.go", `"max_rounds": 2`, "writes retired review and max_rounds (job-contract.md says no Job declares them)"},
	{"internal/contract/fold.go", `j.ReviewRounds++`, "review_rounds counts worker exits with reason loop, not review rounds"},
}

func hfJrRoot() string { return filepath.Join("..", "..") }

func hfJrRead(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(hfJrRoot(), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hfJrTempHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GRID_HOME", home)
}

type hfJrRow struct{ field, required, writtenBy string }

// hfJrTable reads the rows of the "Job record fields" table.
func hfJrTable(t *testing.T) []hfJrRow {
	t.Helper()
	doc := hfJrRead(t, "docs/job-contract.md")
	_, section, ok := strings.Cut(doc, "\n## Job record fields\n")
	if !ok {
		t.Fatal("docs/job-contract.md has no section \"Job record fields\"")
	}
	if i := strings.Index(section, "\n## "); i >= 0 {
		section = section[:i]
	}
	var rows []hfJrRow
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), " | ")
		if len(cells) != 4 {
			t.Fatalf("table row has %d cells, want 4: %s", len(cells), line)
		}
		rows = append(rows, hfJrRow{strings.Trim(cells[0], "`"), cells[1], cells[2]})
	}
	return rows
}

func hfJrSchema(t *testing.T) (props map[string]json.RawMessage, required []string) {
	t.Helper()
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal([]byte(hfJrRead(t, "contract/schemas/job-record.json")), &s); err != nil {
		t.Fatal(err)
	}
	return s.Properties, s.Required
}

func TestHfJrFields(t *testing.T) {
	hfJrTempHome(t)
	props, required := hfJrSchema(t)
	isRequired := map[string]bool{}
	for _, f := range required {
		isRequired[f] = true
	}
	seen := map[string]bool{}
	for _, row := range hfJrTable(t) {
		if seen[row.field] {
			t.Errorf("field %s has more than one row", row.field)
		}
		seen[row.field] = true
		if _, ok := props[row.field]; !ok {
			t.Errorf("row %s is not a property of job-record.json", row.field)
			continue
		}
		if row.required != "required" && row.required != "optional" {
			t.Errorf("field %s: column is %q, want required or optional", row.field, row.required)
		} else if (row.required == "required") != isRequired[row.field] {
			t.Errorf("field %s: table says %s, schema required is %v", row.field, row.required, isRequired[row.field])
		}
	}
	var missing []string
	for name := range props {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		t.Errorf("property %s has no row in the Job record fields table", name)
	}
	for _, name := range required {
		if _, ok := props[name]; !ok {
			t.Errorf("schema requires %s but does not define it", name)
		}
	}
}

func TestHfJrPlanned(t *testing.T) {
	hfJrTempHome(t)
	doc := hfJrRead(t, "docs/job-contract.md")
	m := regexp.MustCompile("(?s)Every entry supplies (.*?)\\.\\n").FindStringSubmatch(doc)
	if m == nil {
		t.Fatal("docs/job-contract.md has no \"Every entry supplies ...\" sentence")
	}
	var got []string
	for _, n := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(m[1], -1) {
		got = append(got, n[1])
	}
	if !reflect.DeepEqual(got, job.PlannedFields) {
		t.Errorf("Planned fields sentence names %v, job.PlannedFields is %v", got, job.PlannedFields)
	}
}

func TestHfJrWriters(t *testing.T) {
	hfJrTempHome(t)
	rows := hfJrTable(t)
	if len(rows) == 0 {
		t.Fatal("the Job record fields table has no rows")
	}
	path := regexp.MustCompile("`([^`]*/[^`]*)`")
	for _, row := range rows {
		kinds := regexp.MustCompile(`\b(plan|render|run|none)\b`).FindAllString(row.writtenBy, -1)
		if len(kinds) == 0 {
			t.Errorf("field %s: written by %q names none of plan, render, run, none", row.field, row.writtenBy)
		}
		paths := path.FindAllStringSubmatch(row.writtenBy, -1)
		if len(paths) == 0 && row.writtenBy != "none" {
			t.Errorf("field %s: written by %q names no file", row.field, row.writtenBy)
		}
		for _, p := range paths {
			if _, err := os.Stat(filepath.Join(hfJrRoot(), filepath.FromSlash(p[1]))); err != nil {
				t.Errorf("field %s: written-by file %s does not exist", row.field, p[1])
			}
		}
	}
}

func TestHfJrNoRoundCount(t *testing.T) {
	hfJrTempHome(t)
	doc := strings.Join(strings.Fields(hfJrRead(t, "docs/job-contract.md")), " ")
	i := strings.Index(doc, "no round count")
	if i < 0 {
		return
	}
	start := strings.LastIndex(doc[:i], ". ") + 1
	end := len(doc)
	if e := strings.Index(doc[i:], ". "); e >= 0 {
		end = i + e
	}
	if !strings.Contains(doc[start:end], "plan") {
		t.Errorf("the sentence %q says \"no round count\" without saying it is the plan", doc[start:end])
	}
	if props, _ := hfJrSchema(t); props["review_rounds"] == nil {
		t.Error("job-record.json has no review_rounds")
	}
}

// TestHfJrKnownOpen checks the known-open list both ways.
func TestHfJrKnownOpen(t *testing.T) {
	hfJrTempHome(t)
	listed := map[string]bool{}
	for _, k := range hfJrKnownOpen {
		if !strings.Contains(hfJrRead(t, k.file), k.text) {
			t.Errorf("known-open entry %s is stale: %q is gone (%s)", k.file, k.text, k.why)
		}
		listed[k.file] = true
	}
	writer := regexp.MustCompile(`"max_rounds":\s*[0-9]`)
	err := filepath.WalkDir(filepath.Join(hfJrRoot(), "internal"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(hfJrRoot(), p)
		rel = filepath.ToSlash(rel)
		if writer.Match(b) && !listed[rel] {
			t.Errorf("%s writes the retired max_rounds and is not in the known-open list", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
