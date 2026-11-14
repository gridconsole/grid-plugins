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
	"github.com/gridconsole/grid-plugins/internal/where"
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
	must(t, personal.SetHook("home-allow", "off"))
	must(t, proj.SetHook("home-allow", "block"))
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
			if d.Agents["grid-builder"] != tc.agent || d.Skills["research"] != tc.skill || d.Hooks["home-allow"] != tc.level {
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
		`{"agents":{"unknown":{"model":"opus"}}}`, `{"agents":{"grid-builder":{"tools":"Bash"}}}`,
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

// No hook has a fixed level. The three guards (host-guard,
// only-writer, plan-guard) default to block; off, warn and block are accepted
// from the command and from a settings file, a project's level wins over the
// personal one, and ResolveHookLevel returns the level that was set.
func TestGuardLevelsAreSettableAndResolved(t *testing.T) {
	personal, proj := fixture(t)
	guards := []string{"host-guard", "only-writer", "plan-guard"}
	query := where.Query{Cwd: t.TempDir()}
	resolver := where.Resolver{GridHome: personal.Home}
	for _, name := range guards {
		d, err := personal.Effective()
		must(t, err)
		def := "warn" // verdicts-11nov v-4; plan-guard: needed.md:47; host-guard: v-enforce
		if d.Hooks[name] != def {
			t.Fatalf("%s defaults to %s, want %s", name, d.Hooks[name], def)
		}
		if got, err := ResolveHookLevel(name, resolver, query); err != nil || got != def {
			t.Fatalf("%s default resolves to %q, %v", name, got, err)
		}
		for _, level := range levels {
			must(t, personal.SetHook(name, level))
			d, err := personal.Effective()
			must(t, err)
			if d.Hooks[name] != level {
				t.Fatalf("%s set to %s reads back %s", name, level, d.Hooks[name])
			}
			if got, err := ResolveHookLevel(name, resolver, query); err != nil || got != level {
				t.Fatalf("%s set to %s resolves to %q, %v", name, level, got, err)
			}
			// A settings file that names the level is read as it is, not refused.
			path := filepath.Join(personal.Home, "settings.json")
			must(t, os.WriteFile(path, []byte(`{"hooks":{"`+name+`":"`+level+`"}}`), 0o600))
			if levels, err := ReadHookLevels(personal.Home, ""); err != nil || levels[name] != level {
				t.Fatalf("file %s %s read as %q, %v", name, level, levels[name], err)
			}
		}
		// The project's level wins over the personal one.
		must(t, personal.SetHook(name, "off"))
		must(t, proj.SetHook(name, "warn"))
		d, err = proj.Effective()
		must(t, err)
		if d.Hooks[name] != "warn" {
			t.Fatalf("%s project level is %s, want warn", name, d.Hooks[name])
		}
		must(t, personal.SetHook(name, "block"))
		must(t, proj.SetHook(name, "block"))
	}
	// A level outside off, warn and block is still refused, with the allowed list.
	err := personal.SetHook("plan-guard", "never")
	if err == nil || !strings.Contains(err.Error(), "allowed: off, warn, block") {
		t.Fatalf("bad level: %v", err)
	}
}

// A settings.json written before the brief-lint hook was removed still loads: the
// retired key is ignored, the other hooks read as set, and a rewrite drops it.
// The command no longer accepts the name.
func TestRetiredBriefLintKeyIsIgnored(t *testing.T) {
	personal, _ := fixture(t)
	path := filepath.Join(personal.Home, "settings.json")
	must(t, os.WriteFile(path, []byte(`{"hooks":{"brief-lint":"block","host-guard":"warn"}}`), 0o600))
	d, err := personal.Effective()
	if err != nil {
		t.Fatalf("settings naming brief-lint refused: %v", err)
	}
	if _, ok := d.Hooks["brief-lint"]; ok {
		t.Fatalf("retired hook still listed: %v", d.Hooks)
	}
	if d.Hooks["host-guard"] != "warn" {
		t.Fatalf("host-guard = %q, want warn", d.Hooks["host-guard"])
	}
	if err := personal.SetHook("history-log", "off"); err != nil {
		t.Fatalf("rewrite of a file naming brief-lint refused: %v", err)
	}
	if strings.Contains(string(contents(t, path)), "brief-lint") {
		t.Fatalf("rewrite kept the retired key: %s", contents(t, path))
	}
	if err := personal.SetHook("brief-lint", "off"); err == nil {
		t.Fatal("the command accepted the removed hook")
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
