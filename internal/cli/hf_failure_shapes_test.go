package cli

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// hfFsSection returns the "CLI JSON failures" section of contract/README.md.
func hfFsSection(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	_, rest, ok := strings.Cut(text, "\n## CLI JSON failures\n")
	if !ok {
		t.Fatal("contract/README.md has no CLI JSON failures section")
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	return section
}

var (
	hfFsExample = regexp.MustCompile("(?s)<!-- shape:(\\S+) -->\n```json\n(.*?)\n```")
	hfFsRow     = regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\|")
)

// hfFsKeys returns the sorted top-level keys of a JSON object.
func hfFsKeys(t *testing.T, data []byte) []string {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("not a JSON object: %v: %s", err, data)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// hfFsExamples reads the README's example object of each shape, by name.
func hfFsExamples(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, m := range hfFsExample.FindAllStringSubmatch(hfFsSection(t), -1) {
		out[m[1]] = []byte(m[2])
	}
	return out
}

// hfFsFixture is the actFixture harness with the checkout project of
// TestStableErrorCodes, and GRID_ROLE set to role.
func hfFsFixture(t *testing.T, role string) (*harness, string) {
	t.Helper()
	h, _, _ := actFixture(t, "new")
	h.env.Getenv = func(k string) string {
		if k == "GRID_ROLE" {
			return role
		}
		return ""
	}
	cwd := filepath.Join(h.root, "checkout")
	h.mkdir(cwd)
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	return h, cwd
}

// hfFsDocCodes is the code table of the README section.
func hfFsDocCodes(t *testing.T) []string {
	t.Helper()
	var codes []string
	for _, m := range hfFsRow.FindAllStringSubmatch(hfFsSection(t), -1) {
		codes = append(codes, m[1])
	}
	return codes
}

// hfFsCodeLiterals names the files whose code is written as a literal in a
// struct with a Code field, not as a constant.
var hfFsCodeLiterals = []string{"role_policy.go", "person_check.go", "task_write.go"}

// hfFsSourceCodes collects the codes the plugin can print: Code*/code* string
// constants in packages cli, settings and braincmd, and the Code value of the anonymous
// struct written in each of hfFsCodeLiterals.
func hfFsSourceCodes(t *testing.T) map[string]string {
	t.Helper()
	found := map[string]string{}
	fset := token.NewFileSet()
	root := filepath.Join("..", "..", "internal")
	literalFiles := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		pkg := file.Name.Name
		dir := filepath.Base(filepath.Dir(path))
		if pkg == "cli" || pkg == "settings" || pkg == "braincmd" {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, name := range vs.Names {
						if !strings.HasPrefix(name.Name, "Code") && !strings.HasPrefix(name.Name, "code") || i >= len(vs.Values) {
							continue
						}
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							v, _ := strconv.Unquote(lit.Value)
							found[v] = filepath.Base(path) + ":" + name.Name
						}
					}
				}
			}
		}
		if dir == "cli" && pkg == "cli" && slices.Contains(hfFsCodeLiterals, filepath.Base(path)) {
			literalFiles[filepath.Base(path)] = true
			for _, v := range hfFsLiteralCodes(file) {
				found[v] = filepath.Base(path) + ":literal"
			}
			if len(hfFsLiteralCodes(file)) == 0 {
				t.Errorf("%s: no struct literal with a Code field and a string value", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range hfFsCodeLiterals {
		if !literalFiles[name] {
			t.Errorf("internal/cli/%s not found", name)
		}
	}
	return found
}

// hfFsLiteralCodes returns the string values at the Code field of every
// unkeyed anonymous-struct composite literal in file.
func hfFsLiteralCodes(file *ast.File) []string {
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		st, ok := lit.Type.(*ast.StructType)
		if !ok {
			return true
		}
		index := 0
		for _, f := range st.Fields.List {
			width := max(1, len(f.Names))
			if len(f.Names) > 0 && f.Names[0].Name == "Code" && index < len(lit.Elts) {
				if bl, ok := lit.Elts[index].(*ast.BasicLit); ok && bl.Kind == token.STRING {
					v, _ := strconv.Unquote(bl.Value)
					out = append(out, v)
				}
			}
			index += width
		}
		return true
	})
	return out
}

func TestHfFsCodeTable(t *testing.T) {
	source := hfFsSourceCodes(t)
	doc := map[string]bool{}
	for _, c := range hfFsDocCodes(t) {
		if doc[c] {
			t.Errorf("README code table lists %q twice", c)
		}
		doc[c] = true
		if _, ok := source[c]; !ok {
			t.Errorf("README lists code %q that no Go file prints", c)
		}
	}
	for c, where := range source {
		if !doc[c] {
			t.Errorf("code %q (%s) is missing from the README code table", c, where)
		}
	}
	if len(doc) == 0 {
		t.Fatal("README code table is empty")
	}
}

func TestHfFsShapes(t *testing.T) {
	examples := hfFsExamples(t)
	for _, name := range []string{"envelope", "signal-refused", "signal-invalid", "person-gate", "job-waiting"} {
		if _, ok := examples[name]; !ok {
			t.Fatalf("README has no example for shape %q", name)
		}
	}
	missing := "S-01ARZ3NDEKTSV4RRFFQ69G5FAW"
	for _, tc := range []struct {
		shape, role string
		args        []string
		exit        int
	}{
		{"envelope", "", []string{"task", "show", "T-99"}, 1},
		{"signal-refused", "", []string{"signal", "act", missing, "--project", "example"}, 3},
		{"signal-invalid", "", []string{"signal", "move", actID}, 2},
		{"person-gate", "builder", []string{"person", "check"}, 3},
	} {
		t.Run(tc.shape, func(t *testing.T) {
			h, cwd := hfFsFixture(t, tc.role)
			if exit := h.run(cwd, append(tc.args, "--json")...); exit != tc.exit {
				t.Fatalf("exit = %d, want %d: %s", exit, tc.exit, h.errb)
			}
			if got, want := hfFsKeys(t, h.out.Bytes()), hfFsKeys(t, examples[tc.shape]); !slices.Equal(got, want) {
				t.Fatalf("stdout keys = %v, README example keys = %v: %s", got, want, h.out)
			}
			if tc.shape == "envelope" {
				var env, doc struct{ Error map[string]json.RawMessage }
				if err := json.Unmarshal(h.out.Bytes(), &env); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(examples[tc.shape], &doc); err != nil {
					t.Fatal(err)
				}
				if len(env.Error) != 2 || len(doc.Error) != 2 || env.Error["code"] == nil || env.Error["message"] == nil || doc.Error["code"] == nil || doc.Error["message"] == nil {
					t.Fatalf("envelope error keys: %s, README %s", h.out, examples[tc.shape])
				}
			}
		})
	}
	t.Run("job-waiting", func(t *testing.T) {
		h := newHarness(t)
		if !writeJobWaiting(h.env, &orchestrator.WaitingError{Job: "J-04", Target: "J-03", Reason: "waiting for J-03"}) {
			t.Fatal("writeJobWaiting printed nothing for a WaitingError")
		}
		if got, want := hfFsKeys(t, h.out.Bytes()), hfFsKeys(t, examples["job-waiting"]); !slices.Equal(got, want) {
			t.Fatalf("stdout keys = %v, README example keys = %v: %s", got, want, h.out)
		}
	})
}

// TestHfFsSilent: `task show` with no operand outside a task folder is the
// command of TestTaskUsageAndFailures (task_test.go:258), which exits 1 with
// no code; with --json it prints nothing on stdout.
func TestHfFsSilent(t *testing.T) {
	h, _ := hfFsFixture(t, "")
	if exit := h.run(h.home, "task", "show", "--json"); exit != 1 || h.errb.Len() == 0 {
		t.Fatalf("exit = %d, stderr %q", exit, h.errb)
	}
	if h.out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", h.out)
	}
	if !strings.Contains(strings.Join(strings.Fields(hfFsSection(t)), " "), "prints nothing on stdout") {
		t.Fatal("README does not say that an unclassified failure prints nothing on stdout")
	}
}

// hfFsRefusals probes refused signal objects. open marks a refusal that
// prints no code today (the doc says so: the code is optional); the test
// fails both when a probe without a code is not listed and when a listed one
// has gained a code.
var hfFsRefusals = []struct {
	name string
	args []string
	open bool // known open: signal.go:238 passes errorCode(err), "" per error_codes.go:44-45
}{
	{"no-such-note", []string{"signal", "act", "S-01ARZ3NDEKTSV4RRFFQ69G5FAW", "--project", "example"}, false},
	{"not-snoozed", []string{"signal", "act", actID, "--project", "example", "--act", "wake"}, true},
	{"already-there", []string{"signal", "move", actID, "--project", "example", "--to", "example"}, true},
}

func TestHfFsKnownOpen(t *testing.T) {
	for _, tc := range hfFsRefusals {
		t.Run(tc.name, func(t *testing.T) {
			h, cwd := hfFsFixture(t, "")
			if exit := h.run(cwd, append(tc.args, "--json")...); exit != 3 {
				t.Fatalf("exit = %d, want 3: %s", exit, h.errb)
			}
			var got struct{ Error, Message, Code string }
			if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Error != "refused" {
				t.Fatalf("stdout %s: %v", h.out, err)
			}
			switch {
			case tc.open && got.Code != "":
				t.Fatalf("code is now %q: remove %q from the known-open list and the README sentence", got.Code, tc.name)
			case !tc.open && got.Code == "":
				t.Fatalf("refusal %q prints no code: add it to the known-open list", tc.name)
			}
		})
	}
}
