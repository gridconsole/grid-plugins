package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestSettingsJSONGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"agents", []string{"agent", "list", "--json"}},
		{"skills", []string{"skill", "list", "--json"}},
		{"hooks", []string{"hook", "levels", "--json"}},
		{"setup", []string{"setup", "example", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.record(project.Record{Name: "example", ID: "example-id", Folders: []project.Folder{{Machine: h.env.Machine, Path: h.home}}})
			h.write(filepath.Join(h.home, "setup-answers.json"), `{"test":"make test","model":"Sonnet","effort":"high"}`)
			if code := h.run(h.home, tc.args...); code != 0 {
				t.Fatalf("%d: %s", code, h.errb)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "settings", tc.name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(h.out.Bytes(), want) {
				t.Fatalf("got %s\nwant %s", h.out, want)
			}
			if h.errb.Len() != 0 {
				t.Fatal(h.errb)
			}
		})
	}
}

func TestSettingsCLIOverridesAndFailures(t *testing.T) {
	h := newHarness(t)
	repo := filepath.Join(h.root, "repo")
	h.mkdir(repo)
	h.record(project.Record{Name: "example", ID: "example-id", Folders: []project.Folder{{Machine: h.env.Machine, Path: repo}}})
	for _, args := range [][]string{
		{"agent", "set", "grid-builder", "--model", "opus", "--effort", "high"},
		{"agent", "set", "grid-builder", "--project", "example-id", "--effort", "low"},
		{"skill", "disable", "research"}, {"skill", "enable", "research", "--project", "example"},
		{"hook", "level", "plan-guard", "off"}, {"hook", "level", "plan-guard", "block", "--project", "example"},
	} {
		if code := h.run(repo, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, h.errb)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"agent", "list", "--json"}, `"grid-builder":{"model":"opus","effort":"low"}`},
		{[]string{"skill", "list", "--json"}, `"research":true`},
		{[]string{"hook", "levels", "--json"}, `"plan-guard":"block"`},
	} {
		if code := h.run(repo, tc.args...); code != 0 || !strings.Contains(h.out.String(), tc.want) {
			t.Fatalf("%v: %d %s %s", tc.args, code, h.out, h.errb)
		}
	}
	path := filepath.Join(h.gridHome, "settings.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"agent", "set", "grid-builder", "--model", "wrong"}, {"agent", "set", "grid-builder", "--model="},
		{"agent", "set", "grid-builder"}, {"agent", "set", "unknown", "--model", "opus"},
		{"skill", "disable", "unknown"}, {"hook", "level", "plan-guard", "never"},
		{"skill", "enable", "research", "--unknown"}, {"agent", "list", "extra"},
		{"skill", "disable", "research", "--project", "missing"},
	} {
		if code := h.run(repo, args...); code == 0 {
			t.Fatalf("accepted %v", args)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("%v changed file", args)
		}
	}
}

func TestSetupFileAndStdin(t *testing.T) {
	h := newHarness(t)
	h.record(project.Record{Name: "example"})
	h.env.Stdin = strings.NewReader(`{"test":"make test"}`)
	if code := h.run(h.home, "setup", "example", "--answers", "-", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	path := filepath.Join(h.gridHome, "projects", "example", "setup-answers.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{"unknown":"value"}`, `{"effort":"wrong"}`, `{`, strings.Repeat(" ", 1<<20) + "{}"} {
		h.write(filepath.Join(h.home, "setup-answers.json"), data)
		if code := h.run(h.home, "setup", "example"); code == 0 {
			t.Fatal("accepted invalid answers")
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("changed setup record")
		}
	}
}
