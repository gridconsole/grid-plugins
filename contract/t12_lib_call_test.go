package contractdata_test

import (
	"bytes"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/lib"
)

const t12LibPage = "docs/lib-call.md"

func t12LibRead(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(t12LibPage)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// t12LibSection returns the lines under the "## <title>" heading, up to the next
// "## " heading.
func t12LibSection(t *testing.T, page, title string) []string {
	t.Helper()
	var out []string
	in := false
	for _, line := range strings.Split(page, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimPrefix(line, "## ") == title
			continue
		}
		if in {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		t.Fatalf("section %q is missing or empty in %s", title, t12LibPage)
	}
	return out
}

// t12LibRows returns the body rows of the markdown table in lines, each as its
// trimmed cells. The header and the separator are skipped.
func t12LibRows(lines []string) [][]string {
	var rows [][]string
	seen := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		seen++
		if seen <= 2 {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

// t12LibFieldDiff compares the first cells of a Call fields table, in backticks,
// with the field names: missing names have no row, extra names have a row and
// no field.
func t12LibFieldDiff(lines, fields []string) (missing, extra []string) {
	have := map[string]bool{}
	for _, cells := range t12LibRows(lines) {
		name := strings.Trim(cells[0], "`")
		have[name] = true
	}
	want := map[string]bool{}
	for _, f := range fields {
		want[f] = true
		if !have[f] {
			missing = append(missing, f)
		}
	}
	for name := range have {
		if !want[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	return missing, extra
}

func t12LibExportedFields() []string {
	var names []string
	rt := reflect.TypeOf(lib.Call{})
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).IsExported() {
			names = append(names, rt.Field(i).Name)
		}
	}
	return names
}

func TestT12LibFieldRows(t *testing.T) {
	lines := t12LibSection(t, t12LibRead(t), "Call fields")
	fields := t12LibExportedFields()
	if len(fields) == 0 {
		t.Fatal("lib.Call has no exported fields")
	}
	missing, extra := t12LibFieldDiff(lines, fields)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("Call fields table: no row for %v; row for a field the struct lacks %v", missing, extra)
	}
	rows := t12LibRows(lines)
	if len(rows) != len(fields) {
		t.Fatalf("Call fields table has %d rows for %d fields", len(rows), len(fields))
	}
	rt := reflect.TypeOf(lib.Call{})
	for i, cells := range rows {
		if len(cells) != 4 {
			t.Fatalf("row %d has %d cells, want 4: %v", i, len(cells), cells)
		}
		f, _ := rt.FieldByName(strings.Trim(cells[0], "`"))
		if got, want := strings.Trim(cells[1], "`"), f.Type.String(); got != want && got != strings.ReplaceAll(want, "interface {}", "any") {
			t.Errorf("field %s: page says type %s, struct has %s", f.Name, got, want)
		}
	}
}

func TestT12LibExitCodes(t *testing.T) {
	lines := t12LibSection(t, t12LibRead(t), "Exit codes")
	var got []int
	for _, cells := range t12LibRows(lines) {
		n, err := strconv.Atoi(cells[0])
		if err != nil {
			t.Fatalf("exit code cell %q: %v", cells[0], err)
		}
		got = append(got, n)
	}
	if want := []int{0, 1, 2, 3, 4, 5, 130}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Exit codes table holds %v, want %v", got, want)
	}

	code := lib.Run(lib.Call{Args: []string{"t12-no-such-verb"}, GridHome: t.TempDir(), Home: t.TempDir()})
	if code != 2 {
		t.Fatalf("lib.Run of an unknown verb returned %d, the page gives usage as 2", code)
	}
}

var (
	t12LibQuoted = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
	t12LibArgs   = regexp.MustCompile(`Args:\s*\[\]string\{([^}]*)\}`)
	t12LibEnv    = regexp.MustCompile(`Environ:\s*\[\]string\{([^}]*)\}`)
)

func t12LibStrings(t *testing.T, re *regexp.Regexp, block string) []string {
	t.Helper()
	m := re.FindStringSubmatch(block)
	if m == nil {
		t.Fatalf("the example has no %v", re)
	}
	var out []string
	for _, q := range t12LibQuoted.FindAllString(m[1], -1) {
		s, err := strconv.Unquote(q)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestT12LibExample(t *testing.T) {
	page := t12LibRead(t)
	_, rest, ok := strings.Cut(page, "```go\n")
	if !ok {
		t.Fatal("the page has no go example")
	}
	block, _, _ := strings.Cut(rest, "```")
	args := t12LibStrings(t, t12LibArgs, block)
	environ := t12LibStrings(t, t12LibEnv, block)
	if !reflect.DeepEqual(args, []string{"version", "--json"}) {
		t.Fatalf("the example runs %v, want version --json", args)
	}

	var out, errOut bytes.Buffer
	code := lib.Run(lib.Call{Args: args, GridHome: t.TempDir(), Environ: environ, Stdout: &out, Stderr: &errOut})
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if err := contract.Validate("cmd-version", out.Bytes()); err != nil {
		t.Fatalf("Stdout %q does not validate against cmd-version: %v", out.String(), err)
	}
}

func TestT12LibNegative(t *testing.T) {
	synthetic := []string{
		"| Field | Type | Meaning | Empty or nil means |",
		"| --- | --- | --- | --- |",
	}
	for _, f := range t12LibExportedFields() {
		if f != "OwnerStart" {
			synthetic = append(synthetic, "| `"+f+"` | x | x | x |")
		}
	}
	synthetic = append(synthetic, "| `NoSuchField` | x | x | x |")
	missing, extra := t12LibFieldDiff(synthetic, t12LibExportedFields())
	if !reflect.DeepEqual(missing, []string{"OwnerStart"}) {
		t.Fatalf("missing = %v, want [OwnerStart]", missing)
	}
	if !reflect.DeepEqual(extra, []string{"NoSuchField"}) {
		t.Fatalf("extra = %v, want [NoSuchField]", extra)
	}
}

var t12LibCite = regexp.MustCompile(`lib/lib\.go:(\d+)(?:-(\d+))?`)

// TestT12LibCitations holds the page to its rule that every statement cites a
// line that exists in lib/lib.go.
func TestT12LibCitations(t *testing.T) {
	page := t12LibRead(t)
	src, err := os.ReadFile("../lib/lib.go")
	if err != nil {
		t.Fatal(err)
	}
	total := strings.Count(string(src), "\n")
	for _, title := range []string{"Call fields", "What a call reads and what stays with the host", "Exit codes"} {
		for _, line := range t12LibSection(t, page, title) {
			s := strings.TrimSpace(line)
			isRow := strings.HasPrefix(s, "|") && !strings.HasPrefix(s, "| Field") && !strings.HasPrefix(s, "| Code") && !strings.HasPrefix(s, "| ---")
			if !isRow && !strings.HasPrefix(s, "- ") {
				continue
			}
			cites := t12LibCite.FindAllStringSubmatch(s, -1)
			if len(cites) == 0 {
				t.Errorf("%s: no lib/lib.go:<line> citation in %q", title, s)
			}
			for _, c := range cites {
				lo, _ := strconv.Atoi(c[1])
				hi := lo
				if c[2] != "" {
					hi, _ = strconv.Atoi(c[2])
				}
				if lo < 1 || hi < lo || hi > total {
					t.Errorf("%s: citation %s is outside lib/lib.go (1-%d)", title, c[0], total)
				}
			}
		}
	}
}
