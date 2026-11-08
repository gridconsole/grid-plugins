package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func fixture(t *testing.T) (Store, Store) {
	t.Helper()
	home := gridhome.Real(t.TempDir())
	if err := project.Write(home, project.Record{Name: "example", ID: "project-id"}); err != nil {
		t.Fatal(err)
	}
	return Store{Home: home}, Store{Home: home, Project: "example"}
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func contents(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	must(t, err)
	return b
}

func TestProjectOverrides(t *testing.T) {
	personal, proj := fixture(t)
	must(t, personal.SetAgent("grid-builder", Agent{Model: "Opus", Effort: "high"}))
	must(t, proj.SetAgent("grid-builder", Agent{Effort: "low"}))
	must(t, personal.SetSkill("research", false))
	must(t, proj.SetSkill("research", true))
	must(t, personal.SetHook("plan-guard", "off"))
	must(t, proj.SetHook("plan-guard", "block"))
	for _, tc := range []struct {
		name  string
		store Store
		agent Agent
		skill bool
		level string
	}{
		{"personal", personal, Agent{"opus", "high"}, false, "off"},
		{"project", proj, Agent{"opus", "low"}, true, "block"},
		{"project id", Store{personal.Home, "project-id"}, Agent{"opus", "low"}, true, "block"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := tc.store.Effective()
			must(t, err)
			if d.Agents["grid-builder"] != tc.agent || d.Skills["research"] != tc.skill || d.Hooks["plan-guard"] != tc.level {
				t.Fatalf("unexpected effective settings: %+v", d)
			}
		})
	}
	// An inherited field must still follow subsequent changes to personal values.
	must(t, personal.SetAgent("grid-builder", Agent{Model: "haiku"}))
	d, err := proj.Effective()
	must(t, err)
	if d.Agents["grid-builder"] != (Agent{"haiku", "low"}) {
		t.Fatal(d.Agents)
	}
}

func TestInvalidChangesLeaveBytes(t *testing.T) {
	s, _ := fixture(t)
	must(t, s.SetAgent("grid-builder", Agent{Model: "opus"}))
	path := filepath.Join(s.Home, "settings.json")
	before := contents(t, path)
	for _, tc := range []struct {
		name   string
		change func() error
	}{
		{"agent", func() error { return s.SetAgent("unknown", Agent{Model: "opus"}) }},
		{"model", func() error { return s.SetAgent("grid-builder", Agent{Model: "unknown"}) }},
		{"effort", func() error { return s.SetAgent("grid-builder", Agent{Effort: "unknown"}) }},
		{"skill", func() error { return s.SetSkill("routing", false) }},
		{"hook", func() error { return s.SetHook("unknown", "off") }},
		{"level", func() error { return s.SetHook("plan-guard", "never") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.change()
			if err == nil || !strings.Contains(err.Error(), "allowed:") {
				t.Fatalf("missing allowed list: %v", err)
			}
			if !bytes.Equal(before, contents(t, path)) {
				t.Fatal("invalid write changed bytes")
			}
		})
	}
}

func TestInvalidDocumentsWriteNothing(t *testing.T) {
	for _, data := range []string{
		`{"unknown":1}`, `{"agents":{"unknown":{"model":"opus"}}}`, `{"agents":{"grid-builder":{"tools":"Bash"}}}`,
		`{"agents":{"grid-builder":{"model":"unknown"}}}`, `{"agents":{"grid-builder":{"model":""}}}`,
		`{"agents":{"grid-builder":{"effort":null}}}`, `{"skills":{"research":"false"}}`,
		`{"hooks":{"plan-guard":"never"}}`, `{"hooks":null}`, `{"hooks":{},"hooks":{}}`,
		`{"agents":{"grid-builder":{"model":"opus","model":"haiku"}}}`, `null`, `{} {}`, `{`,
	} {
		t.Run(data, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "settings.json")
			must(t, os.WriteFile(path, []byte(data), 0o600))
			err := (Store{Home: home}).SetSkill("research", false)
			if err == nil {
				t.Fatal("accepted invalid document")
			}
			if string(contents(t, path)) != data {
				t.Fatal("changed invalid document")
			}
			files, err := os.ReadDir(home)
			must(t, err)
			if len(files) != 1 {
				t.Fatalf("wrote files on invalid document: %v", files)
			}
		})
	}
}

func TestAllowedValues(t *testing.T) {
	s, _ := fixture(t)
	for _, model := range models {
		for _, effort := range efforts {
			t.Run(model+"/"+effort, func(t *testing.T) { must(t, s.SetAgent("grid-builder", Agent{model, effort})) })
		}
	}
	for _, name := range hooks {
		for _, level := range levels {
			must(t, s.SetHook(name, level))
		}
	}
	for _, name := range skills {
		must(t, s.SetSkill(name, false))
		must(t, s.SetSkill(name, true))
	}
}

func TestConcurrentWritersPreserveUpdates(t *testing.T) {
	s, _ := fixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, len(skills)+len(agents))
	for _, name := range skills {
		wg.Go(func() { errs <- s.SetSkill(name, false) })
	}
	for _, a := range agents {
		wg.Go(func() { errs <- s.SetAgent(a.name, Agent{Model: "opus", Effort: "max"}) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		must(t, err)
	}
	d, err := s.Effective()
	must(t, err)
	for name, on := range d.Skills {
		if on {
			t.Errorf("lost disable for %s", name)
		}
	}
	for name, a := range d.Agents {
		if a != (Agent{"opus", "max"}) {
			t.Errorf("lost agent update for %s", name)
		}
	}
}

func TestSetupValidationAndAtomicReplacement(t *testing.T) {
	_, s := fixture(t)
	good := []byte(`{"model":"Sonnet","effort":"xhigh","deploys":"Deploy from this machine","deployCommand":"make deploy","test":"make test"}`)
	answers, err := s.RecordSetup(good)
	must(t, err)
	if answers["model"] != "sonnet" {
		t.Fatal(answers)
	}
	path := filepath.Join(s.Home, "projects", "example", "setup-answers.json")
	before := contents(t, path)
	for _, data := range []string{`{"unknown":true}`, `{"effort":"urgent"}`, `{"test":1}`, `{"test":null}`, `{"model":"other"}`, `{"deploys":"CI/CD deploys","deployCommand":"make deploy"}`, `{"test":"a","test":"b"}`, `null`, `{`} {
		t.Run(data, func(t *testing.T) {
			if _, err := s.RecordSetup([]byte(data)); err == nil {
				t.Fatal("accepted invalid answers")
			}
			if !bytes.Equal(before, contents(t, path)) {
				t.Fatal("invalid answers changed recorded file")
			}
		})
	}
	must(t, os.Remove(filepath.Join(s.Home, "VERSION")))
	must(t, os.Remove(filepath.Join(s.Home, ".settings.lock")))
	if _, err := s.RecordSetup([]byte(`{"bogus":"value"}`)); err == nil || !strings.Contains(err.Error(), "allowed:") {
		t.Fatal(err)
	}
	for _, name := range []string{"VERSION", ".settings.lock"} {
		if _, err := os.Stat(filepath.Join(s.Home, name)); !os.IsNotExist(err) {
			t.Fatalf("invalid setup created %s", name)
		}
	}
}

func TestSymlinkAndUnknownProject(t *testing.T) {
	s, _ := fixture(t)
	target := filepath.Join(s.Home, "untouched.json")
	must(t, os.WriteFile(target, []byte("{}\n"), 0o600))
	must(t, os.Symlink(target, filepath.Join(s.Home, "settings.json")))
	if err := s.SetSkill("research", false); err == nil {
		t.Fatal("followed settings symlink")
	}
	if string(contents(t, target)) != "{}\n" {
		t.Fatal("changed symlink target")
	}
	must(t, os.Remove(filepath.Join(s.Home, "settings.json")))
	s.Project = "../outside"
	if err := s.SetSkill("research", false); err == nil {
		t.Fatal("accepted unknown project")
	}
	s.Project = ""
	must(t, os.Symlink(target, filepath.Join(s.Home, ".settings.lock")))
	if err := s.SetSkill("research", false); err == nil {
		t.Fatal("followed lock symlink")
	}
}
