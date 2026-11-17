package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridconfig"
)

// pbVrEnv is decideFixture with a Grid config whose workspace names a vault as
// the brain folder. Home is the folder that holds the config.
func pbVrEnv(t *testing.T) (Env, string) {
	t.Helper()
	env, _ := decideFixture(t)
	env.Home = t.TempDir()
	vault := filepath.Join(t.TempDir(), "vault")
	for rel, text := range map[string]string{
		"Plain.md":             "No front matter here.\n",
		"Ideas/Café plan.md":   "---\ntags: [food]\naliases: [Cafe]\n---\nPlan the café with [[Plain|the plain one]].\n",
		"Ideas/Deep/Nested.md": "Nested.\n",
		".obsidian/Skipped.md": "x\n",
		"projects/sample/P.md": "A project note.\n",
	} {
		p := filepath.Join(vault, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: env.GridHome, Brain: vault, Member: "Sam"}}}
	if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, func(string) string { return "" }, env.Home), cfg); err != nil {
		t.Fatal(err)
	}
	return env, vault
}

func TestPbVrListJSONOfAVault(t *testing.T) {
	env, vault := pbVrEnv(t)
	code, out, errs := qfRun(env, "list", "--json", "--level", "home")
	if code != 0 {
		t.Fatalf("list: exit %d: %s", code, errs)
	}
	var l brain.Listing
	if err := json.Unmarshal([]byte(out), &l); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if l.Root != vault {
		t.Fatalf("root %q, want %q", l.Root, vault)
	}
	ids := map[string]brain.ListRow{}
	for _, r := range l.Notes {
		ids[r.ID] = r
	}
	for _, want := range []string{"brain:home/Plain", "brain:home/Ideas/Café plan", "brain:home/Ideas/Deep/Nested"} {
		if _, ok := ids[want]; !ok {
			t.Errorf("missing %s in %s", want, out)
		}
	}
	if len(ids) != 3 {
		t.Errorf("listed %d notes, want 3: %s", len(ids), out)
	}
	c := ids["brain:home/Ideas/Café plan"]
	if c.Author != "Sam" || len(c.Edges) != 1 || c.Edges[0].To != "brain:home/Plain" || !c.Edges[0].Resolved || c.Tags[0] != "food" {
		t.Errorf("café row: %+v", c)
	}
	if strings.Contains(out, "null") {
		t.Errorf("a null in the listing: %s", out)
	}
}

func TestPbVrShowVaultIDWithSpaces(t *testing.T) {
	env, _ := pbVrEnv(t)
	code, out, errs := qfRun(env, "show", "brain:home/Ideas/Café plan", "--json")
	if code != 0 {
		t.Fatalf("show: exit %d: %s", code, errs)
	}
	var e brain.Entry
	if err := json.Unmarshal([]byte(out), &e); err != nil || e.ID != "brain:home/Ideas/Café plan" || e.Path != "Ideas/Café plan.md" {
		t.Fatalf("entry %+v: %v", e, err)
	}
	if code, _, _ := qfRun(env, "show", "brain:home/../x", "--json"); code == 0 {
		t.Error("a .. id must fail")
	}
}

// With no brain key in the config, the old knowledge/ layout lists as before.
func TestPbVrListOfAnOldBrain(t *testing.T) {
	env, _ := decideFixture(t)
	env.Home = t.TempDir()
	s := brain.Store{GridHome: env.GridHome}
	if _, err := s.Propose(brain.Note{Name: "deploy", Topic: "decisions", Description: "When deploying", Summary: "How to deploy", Source: "T-01", Level: "home", Body: "Steps.\n"}, brain.Origin{Kind: "session", ID: "example"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept("home", "deploy"); err != nil {
		t.Fatal(err)
	}
	code, out, errs := qfRun(env, "list", "--json", "--level", "home")
	if code != 0 {
		t.Fatalf("list: exit %d: %s", code, errs)
	}
	var l brain.Listing
	if err := json.Unmarshal([]byte(out), &l); err != nil || len(l.Notes) != 1 {
		t.Fatalf("%v: %s", err, out)
	}
	if r := l.Notes[0]; r.ID != "brain:home/deploy" || r.Path != "knowledge/decisions/deploy.md" || r.Topic != "decisions" || r.Origin != "person" {
		t.Fatalf("row: %+v", r)
	}
	if l.Root != env.GridHome {
		t.Fatalf("root %q, want the Grid home %q", l.Root, env.GridHome)
	}
	if code, _, _ := qfRun(env, "list", "extra"); code != 2 {
		t.Errorf("list with an operand exits %d, want 2", code)
	}
}
