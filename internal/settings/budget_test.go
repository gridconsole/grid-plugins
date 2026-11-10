package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

const userFile = `{"zeta":{"b":1.50,"a":[1e3,{"z":null,"y":true}]},"modèle":"é","permissions":{"allow":["Bash(ls)"],"defaultMode":"plan"},"alpha":"first","unknownFuture":{"deep":{"x":[1,2]}}}`

func reindent(t *testing.T, raw string) []byte {
	t.Helper()
	b, err := encodeClaudeJSON(json.RawMessage(raw))
	must(t, err)
	return b
}

func put(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	must(t, os.WriteFile(path, data, mode))
	must(t, os.Chmod(path, mode))
}

func keysAndValues(t *testing.T, data []byte) ([]string, map[string]string) {
	t.Helper()
	obj, err := parseClaudeObject(data)
	must(t, err)
	keys, vals := []string{}, map[string]string{}
	for _, f := range obj {
		keys = append(keys, f.key)
		vals[f.key] = canon(f.value)
	}
	return keys, vals
}

func setOpts(t *testing.T) (ClaudeSetOptions, string) {
	t.Helper()
	home := t.TempDir()
	return ClaudeSetOptions{Home: home}, filepath.Join(home, ".claude", "settings.json")
}

func TestClaudeSetKeepsUnknownKeysAndOrder(t *testing.T) {
	o, path := setOpts(t)
	original := reindent(t, userFile)
	put(t, path, original, 0o600)
	wantKeys, wantVals := keysAndValues(t, original)

	o.Key, o.Value = "autoCompactWindow", json.RawMessage("250000")
	res, err := ClaudeSet(o)
	must(t, err)
	if !res.Written {
		t.Fatal("expected a write")
	}
	keys, vals := keysAndValues(t, contents(t, path))
	if !slices.Equal(keys, append(slices.Clone(wantKeys), "autoCompactWindow")) {
		t.Fatalf("key order after set: %v", keys)
	}
	for k, v := range wantVals {
		if vals[k] != v {
			t.Fatalf("value of %q changed: %s != %s", k, vals[k], v)
		}
	}
	if vals["autoCompactWindow"] != "250000" || !strings.Contains(vals["zeta"], "1.50") || !strings.Contains(vals["zeta"], "1e3") {
		t.Fatalf("numbers were not kept: %v", vals)
	}

	o.Value = nil // set it back to unset
	if _, err := ClaudeSet(o); err != nil {
		t.Fatal(err)
	}
	if got := contents(t, path); string(got) != string(original) {
		t.Fatalf("round trip changed the file:\n%s\n!=\n%s", got, original)
	}

	// a nested key keeps its siblings and goes away without a trace
	o.Key, o.Value = "permissions.defaultMode", json.RawMessage(`"auto"`)
	if _, err := ClaudeSet(o); err != nil {
		t.Fatal(err)
	}
	if _, vals := keysAndValues(t, contents(t, path)); !strings.Contains(vals["permissions"], `"allow":["Bash(ls)"]`) || !strings.Contains(vals["permissions"], `"auto"`) {
		t.Fatalf("permissions: %s", vals["permissions"])
	}
}

func TestClaudeSetRefusesInvalid(t *testing.T) {
	for name, content := range map[string]string{
		"invalid":   `{"a":`,
		"duplicate": `{"a":1,"a":2}`,
		"trailing":  `{"a":1} {"b":2}`,
		"comments":  "{\n // keep\n \"a\": 1\n}",
	} {
		t.Run(name, func(t *testing.T) {
			o, path := setOpts(t)
			put(t, path, []byte(content), 0o600)
			o.Key, o.Value = "autoCompactWindow", json.RawMessage("250000")
			_, err := ClaudeSet(o)
			var ce *CodedError
			if !errors.As(err, &ce) || ce.Code != CodeInvalidSettings {
				t.Fatalf("want invalid_settings, got %v", err)
			}
			if string(contents(t, path)) != content {
				t.Fatal("the invalid file was changed")
			}
		})
	}
}

func TestClaudeSetChangedSinceRead(t *testing.T) {
	o, path := setOpts(t)
	original := reindent(t, userFile)
	put(t, path, original, 0o600)
	o.Key, o.Value = "autoCompactWindow", json.RawMessage("250000")
	o.HasExpect, o.Expect = true, json.RawMessage("300000") // the panel saw another value
	_, err := ClaudeSet(o)
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != CodeChanged || string(contents(t, path)) != string(original) {
		t.Fatalf("expect mismatch: %v", err)
	}
	// a change between the read and the rename
	o.HasExpect = false
	other := reindent(t, `{"other":1}`)
	o.beforeRename = func() { put(t, path, other, 0o600) }
	_, err = ClaudeSet(o)
	if !errors.As(err, &ce) || ce.Code != CodeChanged || string(contents(t, path)) != string(other) {
		t.Fatalf("seam change: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".grid-claude-*")); len(left) != 0 {
		t.Fatalf("staged file left behind: %v", left)
	}
}

func TestClaudeSetFollowsSymlinkKeepsMode(t *testing.T) {
	o, path := setOpts(t)
	target := filepath.Join(t.TempDir(), "real.json")
	put(t, target, reindent(t, userFile), 0o640)
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	must(t, os.Symlink(target, path))
	o.Key, o.Value = "autoCompactEnabled", json.RawMessage("false")
	if _, err := ClaudeSet(o); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v", err)
	}
	info, err := os.Stat(target)
	must(t, err)
	if info.Mode().Perm() != 0o640 || !strings.Contains(string(contents(t, target)), `"autoCompactEnabled": false`) {
		t.Fatalf("mode %v or content wrong", info.Mode().Perm())
	}
}

func TestClaudeSetCreatesMissingFileAndChecksValues(t *testing.T) {
	o, path := setOpts(t)
	o.Key, o.Value = "autoCompactWindow", json.RawMessage("50")
	if _, err := ClaudeSet(o); err == nil {
		t.Fatal("a window under 100000 must be refused")
	}
	o.Value = json.RawMessage("250000")
	if _, err := ClaudeSet(o); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	must(t, err)
	if info.Mode().Perm() != 0o600 || string(contents(t, path)) != "{\n  \"autoCompactWindow\": 250000\n}\n" {
		t.Fatalf("created file: %v %q", info.Mode().Perm(), contents(t, path))
	}
}

func TestClaudeShowReadsEnvAndLayers(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	put(t, filepath.Join(home, ".claude", "settings.json"), reindent(t, `{"autoCompactWindow":300000,"modelSettings":{"opus":{"autoCompactWindow":400000}},"permissions":{"defaultMode":"auto"}}`), 0o600)
	put(t, filepath.Join(project, ".claude", "settings.json"), reindent(t, `{"permissions":{"defaultMode":"plan"}}`), 0o600)
	env := map[string]string{"CLAUDE_CODE_EFFORT_LEVEL": "high", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "haiku", "DISABLE_TELEMETRY": "1"}
	res, err := ClaudeShow(ClaudeShowOptions{Home: home, ProjectDir: project, ManagedPath: filepath.Join(home, "none.json"),
		Getenv: func(k string) string { return env[k] }, Now: func() time.Time { return time.Unix(0, 0) }})
	must(t, err)
	if res.State != "ok" || res.Env["CLAUDE_CODE_EFFORT_LEVEL"] == nil || *res.Env["CLAUDE_CODE_EFFORT_LEVEL"] != "high" ||
		*res.Env["CLAUDE_CODE_SUBAGENT_MODEL_FORCE"] != "haiku" || res.Env["CLAUDE_CODE_SUBAGENT_MODEL"] != nil {
		t.Fatalf("env: %+v", res.Env)
	}
	mode := res.Rows["permissions.defaultMode"]
	if string(mode.Now) != `"auto"` || string(mode.InForce) != `"plan"` || mode.SetBy == nil || *mode.SetBy != "project" {
		t.Fatalf("a project value must win over the user file: %+v", mode)
	}
	if w := res.Rows["autoCompactWindow"]; string(w.Now) != "300000" || w.ShadowedBy == nil || *w.ShadowedBy != "modelSettings.opus.autoCompactWindow" {
		t.Fatalf("window row: %+v", w)
	}
	if adv := res.Rows["advisorModel"]; len(adv.Blocked) != 1 || adv.Blocked[0] != "telemetry" {
		t.Fatalf("advisor row: %+v", adv)
	}
}

func TestCodexKeysReadOnly(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".codex", "config.toml"), []byte("model_auto_compact_token_limit = 200000 # mine\n[agents]\ndefault_subagent_model = \"gpt-x\"\n"), 0o644)
	res, err := ClaudeShow(ClaudeShowOptions{Home: home, ManagedPath: filepath.Join(home, "none.json"), Getenv: func(string) string { return "" }})
	must(t, err)
	if !res.Codex.Exists || res.Codex.Confirmed || res.Codex.Rows["model_auto_compact_token_limit"] != int64(200000) || res.Codex.Rows["agents.default_subagent_model"] != "gpt-x" {
		t.Fatalf("codex rows: %+v", res.Codex)
	}
	_, err = ClaudeSet(ClaudeSetOptions{Home: home, Key: "codex.model_auto_compact_token_limit", Value: json.RawMessage("250000")})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != CodeUnconfirmedKey {
		t.Fatalf("want unconfirmed_key, got %v", err)
	}
	before := contents(t, filepath.Join(home, ".codex", "config.toml"))
	o, _ := budgetFixture(t)
	o.With = []string{"codex"}
	o.Preset = "high"
	if _, err := BudgetApply(o); !errors.As(err, &ce) || ce.Code != CodeUnconfirmedKey {
		t.Fatalf("apply --with codex: %v", err)
	}
	if string(contents(t, filepath.Join(home, ".codex", "config.toml"))) != string(before) {
		t.Fatal("codex file changed")
	}
}

// ---- budget ----

const noteText = `---
name: agent-budget
title: Agent budget
updated: 2026-11-10
default: balanced
presets:
  economic:
    compact: 200000
    advisor: off
    agents:
      grid-planner:    {model: opus,   effort: medium}
      grid-reviewer:   {model: opus,   effort: medium}
      grid-researcher: {model: opus,   effort: medium}
      grid-builder:    {model: sonnet, effort: medium}
      grid-designer:   {model: sonnet, effort: medium}
  balanced:
    compact: 250000
    advisor: opus
    agents:
      grid-planner:    {model: opus,   effort: medium}
      grid-reviewer:   {model: opus,   effort: medium}
      grid-researcher: {model: opus,   effort: medium}
      grid-builder:    {model: sonnet, effort: high}
      grid-designer:   {model: sonnet, effort: high}
  high:
    compact: 351000
    advisor: fable
    agents:
      grid-planner:    {model: opus, effort: high}
      grid-reviewer:   {model: opus, effort: high}
      grid-researcher: {model: opus, effort: high}
      grid-builder:    {model: opus, effort: high}
      grid-designer:   {model: opus, effort: high}
codex:
  economic: {compact: 200000, subagent_model: null, subagent_effort: medium}
  balanced: {compact: 250000, subagent_model: null, subagent_effort: medium}
  high:     {compact: 350000, subagent_model: null, subagent_effort: high}
---

Preset values.
`

func agentText(name, model, effort string, owned bool) []byte {
	meta := ""
	if owned {
		meta = "metadata: {grid: {owner: grid, template: abc}}\n"
	}
	return []byte("---\nname: " + name + "\ndescription: Does the work.\nmodel: " + model + "\neffort: " + effort + "\n" + meta + "tools: Read, Edit\n---\nBody of " + name + ".\n")
}

func budgetFixture(t *testing.T) (BudgetOptions, string) {
	t.Helper()
	home, grid := t.TempDir(), gridhome.Real(t.TempDir())
	put(t, budgetNotePath(grid), []byte(noteText), 0o644)
	agents := filepath.Join(home, ".claude", "agents")
	for _, n := range []string{"grid-planner", "grid-reviewer", "grid-researcher"} {
		put(t, filepath.Join(agents, n+".md"), agentText(n, "opus", "medium", true), 0o644)
	}
	for _, n := range []string{"grid-builder", "grid-designer"} {
		put(t, filepath.Join(agents, n+".md"), agentText(n, "sonnet", "high", true), 0o644)
	}
	put(t, filepath.Join(home, ".claude", "settings.json"), reindent(t, userFile), 0o600)
	return BudgetOptions{Home: home, GridHome: grid, Getenv: func(string) string { return "" }, Now: func() time.Time { return time.Unix(1700000000, 0) }}, agents
}

func snapshotFiles(t *testing.T, o BudgetOptions) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, p := range []string{filepath.Join(o.GridHome, "settings.json"), filepath.Join(o.Home, ".claude", "settings.json")} {
		b, err := os.ReadFile(p)
		out[p] = string(b) + "|" + errString(err)
	}
	files, _ := filepath.Glob(filepath.Join(o.Home, ".claude", "agents", "*.md"))
	for _, p := range files {
		out[p] = string(contents(t, p))
	}
	return out
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return "absent"
}

func TestBudgetNoteStrict(t *testing.T) {
	o, _ := budgetFixture(t)
	put(t, budgetNotePath(o.GridHome), []byte(strings.Replace(noteText, "title: Agent budget\n", "title: Agent budget\nsurprise: yes\n", 1)), 0o644)
	_, note, _ := ReadBudgetNote(o.GridHome)
	if note.State != "invalid" || note.Error == nil {
		t.Fatalf("an unknown key must make the note invalid: %+v", note)
	}
	o.Preset = "high"
	var ce *CodedError
	if _, err := BudgetPreviewFor(o); !errors.As(err, &ce) || ce.Code != CodeInvalidSettings {
		t.Fatalf("never apply from an invalid note: %v", err)
	}
}

func TestBudgetNoteMissingUsesBuiltIn(t *testing.T) {
	o, _ := budgetFixture(t)
	must(t, os.Remove(budgetNotePath(o.GridHome)))
	res, err := BudgetShow(o)
	must(t, err)
	if res.Note.State != "missing" || res.Presets["balanced"].Compact != 250000 || len(res.Presets) != 3 {
		t.Fatalf("built-in values: %+v", res.Note)
	}
}

func TestBudgetPresetValuesComeFromTheNote(t *testing.T) {
	o, _ := budgetFixture(t)
	// the overlay is the source of truth: files and overlay agree on Balanced
	must(t, (Store{Home: o.GridHome}).SetAgents(map[string]Agent{"grid-builder": {Model: "sonnet", Effort: "high"}, "grid-designer": {Model: "sonnet", Effort: "high"}}, ""))
	res, err := BudgetShow(o)
	must(t, err)
	if res.Note.State != "ok" || res.Presets["high"].Compact != 351000 { // the note says 351000; the built-in says 350000
		t.Fatalf("values must come from the note: %+v", res.Presets["high"])
	}
	if res.Applied == nil || *res.Applied != "balanced" || res.Closest != "balanced" {
		t.Fatalf("balanced matches the files: %v %s", res.Applied, res.Closest)
	}
	o.Preset, o.With = "high", []string{"compact"}
	pv, err := BudgetPreviewFor(o)
	must(t, err)
	if len(pv.Settings) != 1 || string(pv.Settings[0].To) != "351000" || len(pv.Files) != 5 || pv.Digest == "" {
		t.Fatalf("preview: %+v", pv)
	}
}

func TestBudgetApplyThenUndoGivesTheSameBytes(t *testing.T) {
	o, agents := budgetFixture(t)
	before := snapshotFiles(t, o)
	o.Preset, o.With = "high", []string{"compact"}
	pv, err := BudgetPreviewFor(o)
	must(t, err)
	o.Expect = pv.Digest
	res, err := BudgetApply(o)
	must(t, err)
	if res.Files != 5 || res.Settings != 1 || !res.Undo {
		t.Fatalf("apply: %+v", res)
	}
	got := string(contents(t, filepath.Join(agents, "grid-builder.md")))
	if !strings.Contains(got, "model: opus\neffort: high\n") || !strings.Contains(got, "metadata: {grid: {owner: grid, template: abc}}\ntools: Read, Edit\n---\nBody of grid-builder.\n") {
		t.Fatalf("only the model and effort lines may change:\n%s", got)
	}
	if overlay := string(contents(t, filepath.Join(o.GridHome, "settings.json"))); !strings.Contains(overlay, `"budget": "high"`) || !strings.Contains(overlay, `"effort": "high"`) {
		t.Fatalf("overlay: %s", overlay)
	}
	if _, vals := keysAndValues(t, contents(t, filepath.Join(o.Home, ".claude", "settings.json"))); vals["autoCompactWindow"] != "351000" || vals["alpha"] != `"first"` {
		t.Fatalf("settings.json: %v", vals)
	}
	show, err := BudgetShow(o)
	must(t, err)
	if show.Applied == nil || *show.Applied != "high" || show.Undo == nil || show.Undo.Preset != "high" {
		t.Fatalf("show after apply: %+v %+v", show.Applied, show.Undo)
	}
	back, err := BudgetUndo(o)
	must(t, err)
	if len(back.Restored) != 5 || len(back.Skipped) != 0 || len(back.Settings) != 1 {
		t.Fatalf("undo: %+v", back)
	}
	if after := snapshotFiles(t, o); !equalMaps(before, after) {
		t.Fatalf("apply then undo must give the same bytes:\nbefore %v\nafter  %v", before, after)
	}
	var ce *CodedError
	if _, err := BudgetUndo(o); !errors.As(err, &ce) || ce.Code != CodeNothingToUndo {
		t.Fatalf("undo is one level: %v", err)
	}
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestBudgetUndoSkipsChangedFile(t *testing.T) {
	o, agents := budgetFixture(t)
	o.Preset = "high"
	_, err := BudgetApply(o)
	must(t, err)
	mine := append(contents(t, filepath.Join(agents, "grid-designer.md")), []byte("My note.\n")...)
	put(t, filepath.Join(agents, "grid-designer.md"), mine, 0o644)
	res, err := BudgetUndo(o)
	must(t, err)
	if len(res.Skipped) != 1 || res.Skipped[0].Agent != "grid-designer" || res.Skipped[0].Reason != "changed after apply" || len(res.Restored) != 4 {
		t.Fatalf("undo: %+v", res)
	}
	if string(contents(t, filepath.Join(agents, "grid-designer.md"))) != string(mine) {
		t.Fatal("the changed file was overwritten")
	}
	if string(contents(t, filepath.Join(agents, "grid-builder.md"))) != string(agentText("grid-builder", "sonnet", "high", true)) {
		t.Fatal("an untouched file was not restored")
	}
}

func TestBudgetApplyLeavesPersonsOwnFile(t *testing.T) {
	o, agents := budgetFixture(t)
	mine := agentText("grid-builder", "haiku", "low", false)
	put(t, filepath.Join(agents, "grid-builder.md"), mine, 0o644)
	o.Preset = "high"
	pv, err := BudgetPreviewFor(o)
	must(t, err)
	if !slices.Contains(pv.Skipped, BudgetSkip{"grid-builder", "yours"}) {
		t.Fatalf("skipped: %+v", pv.Skipped)
	}
	_, err = BudgetApply(o)
	must(t, err)
	if string(contents(t, filepath.Join(agents, "grid-builder.md"))) != string(mine) {
		t.Fatal("the person's own file was changed")
	}
	if show, _ := BudgetShow(o); show.Agents["grid-builder"].State != "yours" {
		t.Fatalf("state: %+v", show.Agents["grid-builder"])
	}
}

func TestBudgetApplyAllOrNothing(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("a directory mode does not stop root")
	}
	o, agents := budgetFixture(t)
	before := snapshotFiles(t, o)
	must(t, os.Chmod(agents, 0o555)) // no stage file can be created beside the agent files
	t.Cleanup(func() { os.Chmod(agents, 0o755) })
	o.Preset, o.With = "high", []string{"compact"}
	var ce *CodedError
	if _, err := BudgetApply(o); !errors.As(err, &ce) || ce.Code != CodeWriteFailed {
		t.Fatalf("want write_failed, got %v", err)
	}
	must(t, os.Chmod(agents, 0o755))
	if after := snapshotFiles(t, o); !equalMaps(before, after) {
		t.Fatal("a failed apply changed something")
	}
	if _, err := os.Stat(snapshotPath(o)); err == nil {
		t.Fatal("an undo snapshot was left for an apply that did not happen")
	}
	if left, _ := filepath.Glob(filepath.Join(agents, ".grid-claude-*")); len(left) != 0 {
		t.Fatalf("staged files left: %v", left)
	}
}

func TestBudgetApplyDigestMismatch(t *testing.T) {
	o, agents := budgetFixture(t)
	o.Preset = "high"
	pv, err := BudgetPreviewFor(o)
	must(t, err)
	put(t, filepath.Join(agents, "grid-planner.md"), agentText("grid-planner", "opus", "low", true), 0o644)
	o.Expect = pv.Digest
	var ce *CodedError
	if _, err := BudgetApply(o); !errors.As(err, &ce) || ce.Code != CodeChanged {
		t.Fatalf("want changed, got %v", err)
	}
	if got := string(contents(t, filepath.Join(agents, "grid-planner.md"))); !strings.Contains(got, "effort: low") {
		t.Fatalf("nothing may be written: %s", got)
	}
}

func TestBudgetEnvOverrideRead(t *testing.T) {
	o, _ := budgetFixture(t)
	env := map[string]string{"CLAUDE_CODE_EFFORT_LEVEL": "max", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "haiku"}
	o.Getenv = func(k string) string { return env[k] }
	res, err := BudgetShow(o)
	must(t, err)
	if len(res.Env) != 2 || res.Env[0] != (BudgetEnv{"CLAUDE_CODE_EFFORT_LEVEL", "max", "overrides-effort"}) || res.Env[1].Effect != "forces-model" {
		t.Fatalf("env: %+v", res.Env)
	}
}

// Apply writes only under the person's own folders, so replacing the plugin's
// folder (a plugin update) cannot reach what Apply wrote. The full check that
// the rendered files take the new template body lands with D-agents-sync.
func TestBudgetOverlayAndFilesLiveOutsideThePluginFolder(t *testing.T) {
	o, agents := budgetFixture(t)
	plugin := filepath.Join(t.TempDir(), "plugin", "1.0")
	put(t, filepath.Join(plugin, "agents", "builder.md"), []byte("old"), 0o644)
	o.Preset = "high"
	_, err := BudgetApply(o)
	must(t, err)
	must(t, os.RemoveAll(filepath.Dir(plugin))) // the update replaces the versioned folder
	put(t, filepath.Join(filepath.Dir(plugin), "1.1", "agents", "builder.md"), []byte("new"), 0o644)
	show, err := BudgetShow(o)
	must(t, err)
	if show.Applied == nil || *show.Applied != "high" || !strings.Contains(string(contents(t, filepath.Join(agents, "grid-builder.md"))), "model: opus") {
		t.Fatalf("the budget did not survive: %+v", show.Applied)
	}
}
