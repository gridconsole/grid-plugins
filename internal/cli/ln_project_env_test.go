package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/projectenv"
)

// lnEnvCliHarness is a temp home with project alpha registered, run for the
// given GRID_ROLE and GRID_MEMBER.
func lnEnvCliHarness(t *testing.T, role, member string) *harness {
	t.Helper()
	h := newHarness(t)
	t.Setenv("HOME", h.home)
	t.Setenv("GRID_HOME", h.gridHome)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(h.home, ".codex"))
	h.record(project.Record{Name: "alpha", ID: "01ALPHA"})
	h.env.Getenv = func(k string) string { return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": member}[k] }
	return h
}

func lnEnvCliPath(h *harness, id string) string {
	return filepath.Join(h.gridHome, "projects", "alpha", "knowledge", "environments", id+".md")
}

// lnEnvCliRaw is the entry file as bytes; ok is false when it does not exist.
func lnEnvCliRaw(t *testing.T, h *harness, id string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(lnEnvCliPath(h, id))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

// lnEnvCliChanges is project alpha's audit lines, newest first.
func lnEnvCliChanges(t *testing.T, h *harness) []brain.Change {
	t.Helper()
	changes, err := (brain.Store{GridHome: h.gridHome, Project: "alpha"}).Changes("project", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return changes
}

// lnEnvCliRun runs grid project env and fails the test on a non-zero exit or an
// answer the contract schema refuses.
func lnEnvCliRun(t *testing.T, h *harness, args ...string) map[string]any {
	t.Helper()
	if code := h.run(h.tmp, append([]string{"project", "env"}, args...)...); code != 0 {
		t.Fatalf("%v: exit %d: %s", args, code, h.errb)
	}
	if err := contract.Validate("cmd-project-env", h.out.Bytes()); err != nil {
		t.Fatalf("%v: answer %s does not match cmd-project-env.json: %v", args, h.out, err)
	}
	var answer map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &answer); err != nil {
		t.Fatalf("%v: %v: %s", args, err, h.out)
	}
	return answer
}

func lnEnvCliRender(t *testing.T, e projectenv.Entry) string {
	t.Helper()
	b, err := projectenv.Render(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLnEnvCliRoundTrip(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	if got := lnEnvCliRun(t, h, "list", "alpha", "--json"); !reflect.DeepEqual(got, map[string]any{"items": []any{}}) {
		t.Fatalf("list before add = %v", got)
	}
	answer := lnEnvCliRun(t, h, "add", "alpha", "--id", "dev", "--name", "Development", "--kind", "local", "--where", "this computer",
		"--runs", "app", "--test", "make test", "--how", "Run locally", "--from", "template", "--who", "ann", "--who", "bo",
		"--secret", "API_TOKEN:set", "--secret", "DB_URL:missing", "--approval", "yes", "--json")
	want := map[string]any{"id": "dev", "name": "Development", "kind": "local", "where": "this computer", "runs": "app",
		"test": "make test", "how": "Run locally", "from": "template", "who": []any{"ann", "bo"},
		"secrets": []any{map[string]any{"name": "API_TOKEN", "set": true}, map[string]any{"name": "DB_URL", "set": false}}, "approval": true}
	if !reflect.DeepEqual(answer, map[string]any{"state": "applied", "item": want}) {
		t.Fatalf("add answer = %v", answer)
	}
	yes := true
	entry := projectenv.Entry{ID: "dev", Name: "Development", Kind: "local", Where: "this computer", Runs: "app", Test: "make test",
		How: "Run locally", From: "template", Who: []string{"ann", "bo"},
		Secrets: []projectenv.Secret{{Name: "API_TOKEN", Set: true}, {Name: "DB_URL"}}, Approval: &yes}
	added, ok := lnEnvCliRaw(t, h, "dev")
	if !ok || added != lnEnvCliRender(t, entry) {
		t.Fatalf("entry file after add:\n%s\nwant projectenv.Render:\n%s", added, lnEnvCliRender(t, entry))
	}
	if got := lnEnvCliRun(t, h, "list", "alpha", "--json"); !reflect.DeepEqual(got, map[string]any{"items": []any{want}}) {
		t.Fatalf("list after add = %v", got)
	}

	// An empty --test clears that line and nothing else.
	answer = lnEnvCliRun(t, h, "edit", "alpha", "--id", "dev", "--test", "", "--json")
	delete(want, "test")
	if !reflect.DeepEqual(answer, map[string]any{"state": "applied", "item": want}) {
		t.Fatalf("edit answer = %v", answer)
	}
	entry.Test = ""
	edited, _ := lnEnvCliRaw(t, h, "dev")
	if edited != lnEnvCliRender(t, entry) {
		t.Fatalf("entry file after edit:\n%s", edited)
	}
	if strings.Replace(added, "test: make test\n", "", 1) != edited {
		t.Fatalf("edit changed more than the test line:\n%s\nwas:\n%s", edited, added)
	}

	if got := lnEnvCliRun(t, h, "remove", "alpha", "--id", "dev", "--json"); !reflect.DeepEqual(got, map[string]any{"state": "applied"}) {
		t.Fatalf("remove answer = %v", got)
	}
	if _, ok := lnEnvCliRaw(t, h, "dev"); ok {
		t.Fatal("remove left the entry file")
	}
	if got := lnEnvCliRun(t, h, "list", "alpha", "--json"); !reflect.DeepEqual(got, map[string]any{"items": []any{}}) {
		t.Fatalf("list after remove = %v", got)
	}

	changes := lnEnvCliChanges(t, h)
	if len(changes) != 3 {
		t.Fatalf("audit lines = %+v, want add, edit and remove", changes)
	}
	for i, status := range []string{"dropped", "accepted", "accepted"} {
		c := changes[i]
		if c.Status != status || c.OriginKind != "person" || c.OriginID != "bo" || c.Topic != "environments" || c.Name != "dev" {
			t.Fatalf("audit line %d = %+v, want %s by person:bo", i, c, status)
		}
	}
}

func TestLnEnvCliEditKeepsWhatHasNoFlag(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	no := false
	seed := projectenv.Entry{ID: "ci", Name: "CI", Kind: "cloud", Runs: "tests", Who: []string{"ann"},
		Secrets: []projectenv.Secret{{Name: "CI_TOKEN", Set: true}}, Steps: []projectenv.Step{{WF: "build", Step: "test", Text: "Check"}}, Approval: &no}
	if err := projectenv.Save(brain.Store{GridHome: h.gridHome, Project: "alpha"}, seed); err != nil {
		t.Fatal(err)
	}
	lnEnvCliRun(t, h, "edit", "01ALPHA", "--id", "ci", "--name", "Builds", "--who", "", "--json")
	seed.Name, seed.Who = "Builds", nil
	if got, _ := lnEnvCliRaw(t, h, "ci"); got != lnEnvCliRender(t, seed) {
		t.Fatalf("after edit:\n%s\nwant:\n%s", got, lnEnvCliRender(t, seed))
	}
	answer := lnEnvCliRun(t, h, "edit", "alpha", "--id", "ci", "--approval", "", "--secret", "CI_TOKEN:missing", "--json")
	seed.Approval, seed.Secrets = nil, []projectenv.Secret{{Name: "CI_TOKEN"}}
	if got, _ := lnEnvCliRaw(t, h, "ci"); got != lnEnvCliRender(t, seed) {
		t.Fatalf("after the second edit:\n%s\nwant:\n%s", got, lnEnvCliRender(t, seed))
	}
	item, _ := answer["item"].(map[string]any)
	if _, has := item["approval"]; has || !reflect.DeepEqual(item["steps"], []any{map[string]any{"wf": "build", "step": "test", "text": "Check"}}) {
		t.Fatalf("edit answer item = %v", item)
	}
}

func TestLnEnvCliRefusalsWriteNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"add with kind moon", []string{"add", "alpha", "--id", "x", "--name", "X", "--kind", "moon"}},
		{"edit to kind moon", []string{"edit", "alpha", "--id", "dev", "--kind", "moon"}},
		{"add with a secret value", []string{"add", "alpha", "--id", "x", "--name", "X", "--kind", "local", "--secret", "API_TOKEN=abc"}},
		{"edit with a secret value", []string{"edit", "alpha", "--id", "dev", "--secret", "API_TOKEN=abc"}},
		{"edit with a secret state that is a value", []string{"edit", "alpha", "--id", "dev", "--secret", "API_TOKEN:abc"}},
		{"edit to a test that holds a password", []string{"edit", "alpha", "--id", "dev", "--test", "password: x"}},
		{"add to an unknown project", []string{"add", "absent", "--id", "x", "--name", "X", "--kind", "local"}},
		{"edit in an unknown project", []string{"edit", "absent", "--id", "dev", "--name", "Y"}},
		{"remove in an unknown project", []string{"remove", "absent", "--id", "dev"}},
		{"add of a taken id", []string{"add", "alpha", "--id", "dev", "--name", "Other", "--kind", "local"}},
		{"edit of a missing id", []string{"edit", "alpha", "--id", "nope", "--name", "Y"}},
		{"remove of a missing id", []string{"remove", "alpha", "--id", "nope"}},
		{"add without a kind", []string{"add", "alpha", "--id", "x", "--name", "X"}},
		{"edit that changes nothing", []string{"edit", "alpha", "--id", "dev"}},
		{"remove with a field flag", []string{"remove", "alpha", "--id", "dev", "--name", "Y"}},
		{"approval that is not yes or no", []string{"edit", "alpha", "--id", "dev", "--approval", "maybe"}},
		{"a bad id", []string{"add", "alpha", "--id", "Dev!", "--name", "X", "--kind", "local"}},
		{"unknown verb", []string{"show", "alpha"}},
	} {
		for _, mode := range []string{"plain", "json"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				h := lnEnvCliHarness(t, "owner", "bo")
				lnEnvCliRun(t, h, "add", "alpha", "--id", "dev", "--name", "Development", "--kind", "local", "--test", "make test", "--json")
				before, _ := lnEnvCliRaw(t, h, "dev")
				lines := len(lnEnvCliChanges(t, h))
				args := append([]string{"project", "env"}, tc.args...)
				if mode == "json" {
					args = append(args, "--json")
				}
				if code := h.run(h.tmp, args...); code == 0 {
					t.Fatalf("exit 0: stdout %q", h.out)
				}
				if h.errb.Len() == 0 {
					t.Fatal("a refusal says why on stderr")
				}
				if strings.Contains(h.errb.String(), "abc") || strings.Contains(h.out.String(), "abc") {
					t.Fatalf("the refused value was printed: stdout %q stderr %q", h.out, h.errb)
				}
				if after, _ := lnEnvCliRaw(t, h, "dev"); after != before {
					t.Fatalf("dev changed:\n%s\nwas:\n%s", after, before)
				}
				if _, ok := lnEnvCliRaw(t, h, "x"); ok {
					t.Fatal("a refused add wrote x")
				}
				if got := lnEnvCliChanges(t, h); len(got) != lines {
					t.Fatalf("a refusal wrote an audit line: %+v", got[0])
				}
				if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "absent")); !os.IsNotExist(err) {
					t.Fatalf("the unknown project got a folder: %v", err)
				}
			})
		}
	}
}

func TestLnEnvCliGuestIsRefused(t *testing.T) {
	for _, role := range []string{"guest", "grid-guest"} {
		t.Run(role, func(t *testing.T) {
			h := lnEnvCliHarness(t, role, "gus")
			for _, args := range [][]string{
				{"list", "alpha"},
				{"add", "alpha", "--id", "dev", "--name", "D", "--kind", "local"},
				{"edit", "alpha", "--id", "dev", "--name", "D"},
				{"remove", "alpha", "--id", "dev"},
			} {
				if code := h.run(h.tmp, append([]string{"project", "env"}, args...)...); code != 1 {
					t.Fatalf("%v: exit %d, want 1: %s", args, code, h.errb)
				}
				if got := h.errb.String(); got != "grid: "+guestSettingsRefusal+"\n" {
					t.Fatalf("%v: stderr %q", args, got)
				}
				if _, ok := lnEnvCliRaw(t, h, "dev"); ok {
					t.Fatalf("%v: a guest wrote the entry", args)
				}
				if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "alpha", "knowledge")); !os.IsNotExist(err) {
					t.Fatalf("%v: a guest touched the brain: %v", args, err)
				}
			}
		})
	}
}

func TestLnEnvCliUnknownProjectCode(t *testing.T) {
	for _, args := range [][]string{
		{"list", "absent"},
		{"add", "absent", "--id", "x", "--name", "X", "--kind", "local"},
		{"edit", "absent", "--id", "x", "--name", "X"},
		{"remove", "absent", "--id", "x"},
	} {
		h := lnEnvCliHarness(t, "owner", "bo")
		if code := h.run(h.tmp, append(append([]string{"project", "env"}, args...), "--json")...); code != 1 {
			t.Fatalf("%v: exit %d: %s", args, code, h.errb)
		}
		assertErrorOutput(t, h.out.String(), h.errb.String(), CodeNoSuchProject)
	}
}

// Without a person the author is the session, as grid brain drop falls back.
func TestLnEnvCliSessionAuthorWithoutAPerson(t *testing.T) {
	h := lnEnvCliHarness(t, "", "")
	lnEnvCliRun(t, h, "add", "alpha", "--id", "dev", "--name", "D", "--kind", "remote", "--json")
	if c := lnEnvCliChanges(t, h); len(c) != 1 || c[0].OriginKind != "session" || c[0].OriginID == "" {
		t.Fatalf("audit lines = %+v, want one by session", c)
	}
}
