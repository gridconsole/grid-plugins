package cli

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

// ilGuestEnvelope is the stdout line finish writes for a guest refusal: the
// envelope of contract/README.md shape 1, encoded the way writeJSON encodes it.
func ilGuestEnvelope(stderr string) string {
	type detail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(struct {
		Error detail `json:"error"`
	}{detail{CodeGuestRefused, strings.TrimSuffix(stderr, "\n")}})
	return b.String()
}

// ilGuestArgs fills the guest parity placeholders the way TestGuestProjectParity does.
func ilGuestArgs(h *harness, args []string) []string {
	out := slices.Clone(args)
	for i, arg := range out {
		switch arg {
		case "{project}":
			out[i] = "alpha"
		case "{task}":
			out[i] = "T-01"
		case "{file}":
			out[i] = filepath.Join(h.root, "input.md")
		case "{cwd}":
			out[i] = h.root
		}
	}
	return out
}

// ilGuestNoLease clears the lease variables some Feed adapters read directly.
func ilGuestNoLease(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GRID_WORKER", "GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_JOB", "GRID_WAKE", "GRID_ATTEMPT", "GRID_OWNER"} {
		t.Setenv(key, "")
	}
}

// The gate set comes from guestParityCommands and guestWriteVerb, never a hand list.
func TestIlGuestGateWalk(t *testing.T) {
	ilGuestNoLease(t)
	keys := make([]string, 0, len(guestParityCommands))
	for key := range guestParityCommands {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seen := map[string]int{}
	for _, key := range keys {
		fields := strings.Fields(key)
		argv := slices.Concat(fields, guestParityCommands[key])
		name, rest := argv[0], argv[1:]
		if !guestWriteVerb(name, rest) {
			continue
		}
		seen[name]++
		// Why: question withdraw and task delete/restore exit 3, the others exit 1.
		wantExit := 1
		if name == "question" || name == "task" && (rest[0] == "delete" || rest[0] == "restore") {
			wantExit = 3
		}
		t.Run(key, func(t *testing.T) {
			h := newHarness(t)
			guestParityFixture(t, h, "invited")
			args := ilGuestArgs(h, argv)
			code := h.run(h.root, args...)
			plainOut, plainErr := h.out.String(), h.errb.String()
			guestParityFixture(t, h, "invited")
			jsonCode := h.run(h.root, append(slices.Clone(args), "--json")...)
			if code != wantExit || jsonCode != wantExit {
				t.Fatalf("exit plain=%d json=%d, want %d", code, jsonCode, wantExit)
			}
			if plainOut != "" {
				t.Fatalf("plain stdout %q, want empty", plainOut)
			}
			if plainErr == "" || h.errb.String() != plainErr {
				t.Fatalf("json stderr %q, plain stderr %q; want the same refusal", h.errb.String(), plainErr)
			}
			assertErrorOutput(t, h.out.String(), h.errb.String(), CodeGuestRefused)
		})
	}
	for _, name := range []string{"setup", "task", "question", "session"} {
		if seen[name] == 0 {
			t.Errorf("the walk found no %s key that guestWriteVerb refuses", name)
		}
	}
	for _, role := range []string{"member", "owner"} {
		t.Run(role+"/task close", func(t *testing.T) {
			h := newHarness(t)
			guestParityFixture(t, h, "invited")
			h.env.Getenv = func(key string) string { return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "gus"}[key] }
			h.run(h.root, "task", "close", "T-01", "--json")
			if strings.Contains(h.out.String(), CodeGuestRefused) || strings.Contains(h.errb.String(), `role "guest"`) {
				t.Fatalf("%s got the guest refusal: stdout %q, stderr %q", role, h.out, h.errb)
			}
		})
		t.Run(role+"/setup", func(t *testing.T) {
			h, cwd := guestLeakFixture(t, role)
			if code := h.run(cwd, "setup", "--yes", "--json"); code != 0 || strings.Contains(h.out.String(), CodeGuestRefused) {
				t.Fatalf("%s setup: exit %d, stdout %q, stderr %q; want exit 0 and no guest refusal", role, code, h.out, h.errb)
			}
		})
	}
	t.Run("guest_with_task_lease/task new", func(t *testing.T) {
		h, fixture := guestTaskFixture(t, "guest", "task", false)
		if code := h.run(h.home, "task", "new", "another task", "--project", "example", "--json"); code != 0 {
			t.Fatalf("exit %d, want 0: %s", code, h.errb)
		}
		var made struct{ ID, Path string }
		if err := json.Unmarshal(h.out.Bytes(), &made); err != nil || made.ID == "" || made.ID == fixture.ID {
			t.Fatalf("stdout %q (%v), want a new task", h.out, err)
		}
	})
}

func TestIlGuestSettingsSites(t *testing.T) {
	ilGuestNoLease(t)
	for _, role := range []string{"guest", "external"} {
		for _, tc := range []struct {
			name string
			args func(t *testing.T, cwd string) []string
		}{
			{"routine_add", func(*testing.T, string) []string {
				return []string{"routine", "add", "parity", "--every", "1h", "--run", "grid version", "--project", "alpha"}
			}},
			{"project_relink", func(_ *testing.T, cwd string) []string { return []string{"project", "relink", "alpha", cwd} }},
			{"claude_setup", func(*testing.T, string) []string { return []string{"setup", "--yes"} }},
			{"claude_setup_remove", func(*testing.T, string) []string { return []string{"setup", "--remove"} }},
			{"agent_sync", func(t *testing.T, _ string) []string {
				return []string{"agent", "sync", "--templates", agentTemplates(t, t.TempDir()), "--dir", filepath.Join(t.TempDir(), "agents")}
			}},
		} {
			t.Run(role+"/"+tc.name, func(t *testing.T) {
				h, cwd := guestLeakFixture(t, role)
				args := tc.args(t, cwd)
				before, homeBefore := bypassTree(t, h.gridHome), bypassTree(t, h.home)
				code := h.run(cwd, args...)
				plainOut, plainErr := h.out.String(), h.errb.String()
				jsonCode := h.run(cwd, append(slices.Clone(args), "--json")...)
				if code != 1 || jsonCode != 1 {
					t.Fatalf("exit plain=%d json=%d, want 1: %s", code, jsonCode, h.errb)
				}
				if plainOut != "" || !strings.Contains(plainErr, guestSettingsRefusal) || h.errb.String() != plainErr {
					t.Fatalf("plain stdout %q, plain stderr %q, json stderr %q; want empty stdout and the same settings refusal", plainOut, plainErr, h.errb)
				}
				assertErrorOutput(t, h.out.String(), h.errb.String(), CodeGuestRefused)
				if !reflect.DeepEqual(before, bypassTree(t, h.gridHome)) || !reflect.DeepEqual(homeBefore, bypassTree(t, h.home)) {
					t.Fatal("HOME or GRID_HOME tree changed after a refused write")
				}
			})
		}
	}
	// Known open (TestIlGuestSiteList): signal.go keeps errors.New(guestSettingsRefusal).
	t.Run("known_open/signal add", func(t *testing.T) {
		h, cwd := guestLeakFixture(t, "guest")
		if code := h.run(cwd, "signal", "add", "--title", "parity", "--json"); code != 1 || h.out.Len() != 0 || !strings.Contains(h.errb.String(), guestSettingsRefusal) {
			t.Fatalf("exit %d, stdout %q, stderr %q; want exit 1, empty stdout and the settings refusal", code, h.out, h.errb)
		}
	})
}

// ilGuestOpenSites are the settings refusals that still print no code with --json:
// signal.go:65 waits for C-ia-signal-refusal-codes, settings.go is the store opener
// (one call; its callers are not read yet) and budget.go and project_repo.go wait
// for their argv to be read.
var ilGuestOpenSites = map[string]int{"signal.go": 1, "settings.go": 1, "budget.go": 1, "project_repo.go": 1}

// ilGuestRefusalCalls counts the calls errors.New(guestSettingsRefusal) in the
// function bodies of file; the package-level sentinel's own declaration is no site.
func ilGuestRefusalCalls(file *ast.File) int {
	n := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "New" {
				return true
			}
			pkg, okPkg := sel.X.(*ast.Ident)
			arg, okArg := call.Args[0].(*ast.Ident)
			if okPkg && okArg && pkg.Name == "errors" && arg.Name == "guestSettingsRefusal" {
				n++
			}
			return true
		})
	}
	return n
}

func TestIlGuestSiteList(t *testing.T) {
	t.Run("package", func(t *testing.T) {
		paths, err := filepath.Glob("*.go")
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		found, total, want := map[string]int{}, 0, 0
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			if n := ilGuestRefusalCalls(file); n > 0 {
				found[path] = n
				total += n
			}
		}
		for name, n := range found {
			if open, ok := ilGuestOpenSites[name]; !ok {
				t.Errorf("%s: %d errors.New(guestSettingsRefusal) site(s) not in the known-open list; return the guest sentinel", name, n)
			} else if n > open {
				t.Errorf("%s: %d errors.New(guestSettingsRefusal) sites, %d known open", name, n, open)
			}
		}
		for name, open := range ilGuestOpenSites {
			want += open
			if found[name] < open {
				t.Errorf("%s: %d errors.New(guestSettingsRefusal) sites, %d known open: remove it from the known-open list", name, found[name], open)
			}
		}
		if total != want {
			t.Errorf("the walk finds %d sites, want the %d known open: %v", total, want, found)
		}
	})
	t.Run("reports_a_site", func(t *testing.T) {
		src := "package cli\n\nimport \"errors\"\n\nvar e = errors.New(guestSettingsRefusal)\n\nfunc f() error { return errors.New(guestSettingsRefusal) }\n"
		file, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := ilGuestRefusalCalls(file); got != 1 {
			t.Fatalf("walk reports %d sites in a source with one call in a function, want 1", got)
		}
	})
}

func TestIlGuestReadme(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "## CLI JSON failures")
	if start < 0 {
		t.Fatal("README has no CLI JSON failures section")
	}
	section := text[start:]
	if end := strings.Index(section[1:], "\n## "); end >= 0 {
		section = section[:end+1]
	}
	rows := 0
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "| `"+CodeGuestRefused+"` |") {
			rows++
		}
	}
	if rows != 1 {
		t.Errorf("README code table has %d %s rows, want 1", rows, CodeGuestRefused)
	}
	named := false
	for _, para := range strings.Split(section, "\n\n") {
		if !strings.HasPrefix(strings.TrimSpace(para), "|") && strings.Contains(para, "`"+CodeGuestRefused+"`") && strings.Contains(para, "exit 1") && strings.Contains(para, "exit 3") {
			named = true
		}
	}
	if !named {
		t.Errorf("README CLI JSON failures section names no %s next to exit 1 and exit 3", CodeGuestRefused)
	}
}
