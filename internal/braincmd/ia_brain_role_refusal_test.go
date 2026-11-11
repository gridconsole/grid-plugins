package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// iaBrVerbs holds one call of each brain verb that checks the caller's role,
// with the stderr line its refusal prints (without the newline).
var iaBrVerbs = []struct {
	verb string
	args []string
	line string
}{
	{"changes", []string{"changes"}, "grid brain changes: caller role is not allowed to read brain changes"},
	{"revert", []string{"revert", "decisions/checks", "--to", "1"}, "grid brain revert: caller role is not allowed to revert brain entries"},
	{"supersede", []string{"supersede", "parity", "--by", "parity-v2", "--reason", "parity reason here"}, "grid brain supersede: caller role is not allowed to supersede brain entries"},
	{"drop", []string{"drop", "decisions/checks", "--expect", "0123456789abcdef"}, "grid brain drop: caller role is not allowed to drop brain entries"},
	{"restore", []string{"restore", "decisions/checks", "--expect", "0123456789abcdef"}, "grid brain restore: caller role is not allowed to restore brain entries"},
}

// iaBrObject requires stdout to be one line holding exactly the person_required
// object for the refusal line, keys in the contract's order.
func iaBrObject(t *testing.T, verb, stdout, line string) {
	t.Helper()
	hint := strings.TrimPrefix(line, "grid brain "+verb+": ")
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout %q is not one JSON object: %v", stdout, err)
	}
	want := map[string]any{"error": "person_required", "reason": "role_not_allowed", "hint": hint, "code": "person_required", "message": line}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stdout object = %v, want %v", got, want)
	}
	exact := `{"error":"person_required","reason":"role_not_allowed","hint":"` + hint + `","code":"person_required","message":"` + line + "\"}\n"
	if stdout != exact {
		t.Fatalf("stdout = %q, want %q", stdout, exact)
	}
}

// iaBrProject writes a project record under the Grid home, and its roles file
// when roles is not empty.
func iaBrProject(t *testing.T, h *harness, name, id, roles string) {
	t.Helper()
	dir := filepath.Join(h.env.GridHome, "projects", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.md"), []byte("---\nid: "+id+"\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if roles != "" {
		if err := os.WriteFile(filepath.Join(dir, "project-roles.json"), []byte(roles), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIaBrRoleRefusal(t *testing.T) {
	for _, role := range []string{"guest", "other"} {
		for _, v := range iaBrVerbs {
			t.Run(role+"/"+v.verb, func(t *testing.T) {
				h := newHarness(t)
				h.env.Role = role
				plain := h.run(v.args...)
				plainOut, plainErr := h.out.String(), h.err.String()
				if plain != 3 || plainOut != "" || plainErr != v.line+"\n" {
					t.Fatalf("plain: exit=%d stdout=%q stderr=%q, want exit 3, no stdout, stderr %q", plain, plainOut, plainErr, v.line+"\n")
				}
				code := h.run(append(slices.Clone(v.args), "--json")...)
				if code != 3 || h.err.String() != plainErr {
					t.Fatalf("--json: exit=%d stderr=%q, want exit 3 and the plain stderr %q", code, &h.err, plainErr)
				}
				iaBrObject(t, v.verb, h.out.String(), v.line)
				if _, err := os.Stat(h.env.GridHome); !os.IsNotExist(err) {
					t.Fatalf("a refused caller touched the Grid home: %v", err)
				}
			})
		}
	}
}

// A guest asking for a registered project it is not granted and for one that
// does not exist gets the same role refusal, byte for byte.
func TestIaBrHiddenProjectSame(t *testing.T) {
	for _, v := range iaBrVerbs[:2] {
		t.Run(v.verb, func(t *testing.T) {
			h := newHarness(t)
			h.env.Resolver = where.Resolver{GridHome: h.env.GridHome, Home: h.env.Home, Caller: where.Caller{Role: "guest", Member: "gus"}}
			iaBrProject(t, h, "alpha", "01ALPHA", `{"people":[{"id":"gus","perm":"Client"}]}`)
			iaBrProject(t, h, "beta", "01BETA", "")
			// The fixture hides beta: a role the verb allows still cannot resolve it.
			h.env.Role = "member"
			if code := h.run(append(slices.Clone(v.args), "--project", "beta", "--json")...); code != 1 || !strings.Contains(h.err.String(), `\"beta\" is not registered`) {
				t.Fatalf("member --project beta: exit=%d stdout=%q stderr=%q, want beta hidden", code, &h.out, &h.err)
			}
			h.env.Role = "guest"
			var outs, errs []string
			for _, asked := range []string{"beta", "absent"} {
				code := h.run(append(slices.Clone(v.args), "--project", asked, "--json")...)
				if code != 3 || h.err.String() != v.line+"\n" {
					t.Fatalf("--project %s: exit=%d stderr=%q, want exit 3 and %q", asked, code, &h.err, v.line+"\n")
				}
				iaBrObject(t, v.verb, h.out.String(), v.line)
				outs, errs = append(outs, h.out.String()), append(errs, h.err.String())
			}
			if outs[0] != outs[1] || errs[0] != errs[1] {
				t.Fatalf("beta and absent differ: stdout %q / %q, stderr %q / %q", outs[0], outs[1], errs[0], errs[1])
			}
		})
	}
}

func TestIaBrReadme(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "contract", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	start := strings.Index(text, "Both verbs check `GRID_ROLE`")
	if start < 0 {
		t.Fatal("contract/README.md has no paragraph starting \"Both verbs check `GRID_ROLE`\"")
	}
	para := text[start:]
	if end := strings.Index(para, "\n\n"); end >= 0 {
		para = para[:end]
	}
	for _, word := range []string{"person_required", "role_not_allowed", "supersede"} {
		if !strings.Contains(para, word) {
			t.Errorf("the brain role paragraph does not name %s:\n%s", word, para)
		}
	}
}
