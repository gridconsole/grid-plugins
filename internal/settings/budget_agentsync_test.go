package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// ---- validateBudget ----

func goodFront() budgetFront {
	d := BuiltinBudget()
	return budgetFront{Default: d.Default, Presets: d.Presets, Codex: d.Codex}
}

func TestValidateBudgetDefaultsPass(t *testing.T) {
	if err := validateBudget(goodFront()); err != nil {
		t.Fatalf("the built-in table must pass: %v", err)
	}
	o, _ := budgetFixture(t)
	d, note, _ := ReadBudgetNote(o.GridHome)
	if note.State != "ok" {
		t.Fatalf("note: %+v", note)
	}
	if err := validateBudget(budgetFront{Default: d.Default, Presets: d.Presets, Codex: d.Codex}); err != nil {
		t.Fatalf("the note's front matter must pass: %v", err)
	}
}

func TestValidateBudgetRefusals(t *testing.T) {
	withPreset := func(name string, edit func(p *BudgetPreset)) func(f *budgetFront) {
		return func(f *budgetFront) {
			p := f.Presets[name]
			edit(&p)
			f.Presets[name] = p
		}
	}
	cases := []struct {
		name   string
		mutate func(f *budgetFront)
		want   string
	}{
		{"missing preset", func(f *budgetFront) { delete(f.Presets, "high") }, "presets must be exactly economic, balanced, high"},
		{"extra preset name", func(f *budgetFront) { f.Presets["turbo"] = f.Presets["high"] }, "presets must be exactly"},
		{"wrong preset name", func(f *budgetFront) { f.Presets["turbo"] = f.Presets["high"]; delete(f.Presets, "high") }, `unknown preset "turbo"`},
		{"compact 99999", withPreset("economic", func(p *BudgetPreset) { p.Compact = 99999 }), "preset economic: compact must be"},
		{"compact 1000001", withPreset("economic", func(p *BudgetPreset) { p.Compact = 1000001 }), "preset economic: compact must be"},
		{"advisor gpt", withPreset("economic", func(p *BudgetPreset) { p.Advisor = "gpt" }), `preset economic: unknown advisor "gpt"`},
		{"no agents", withPreset("economic", func(p *BudgetPreset) { p.Agents = map[string]Agent{} }), "preset economic lists no agents"},
		{"unknown agent", withPreset("economic", func(p *BudgetPreset) { p.Agents["grid-ghost"] = Agent{Model: "opus", Effort: "medium"} }), `unknown agent "grid-ghost"`},
		{"empty effort", withPreset("economic", func(p *BudgetPreset) { p.Agents["grid-planner"] = Agent{Model: "opus"} }), "grid-planner needs model and effort"},
		{"empty model", withPreset("economic", func(p *BudgetPreset) { p.Agents["grid-planner"] = Agent{Effort: "medium"} }), "grid-planner needs model and effort"},
		{"model refused by validateAgent", withPreset("economic", func(p *BudgetPreset) { p.Agents["grid-planner"] = Agent{Model: "gpt", Effort: "medium"} }), `grid-planner: unknown model "gpt"`},
		{"effort refused by validateAgent", withPreset("economic", func(p *BudgetPreset) { p.Agents["grid-planner"] = Agent{Model: "opus", Effort: "turbo"} }), `grid-planner: unknown effort "turbo"`},
		{"default turbo", func(f *budgetFront) { f.Default = "turbo" }, `unknown default preset "turbo"`},
		{"codex preset turbo", func(f *budgetFront) { f.Codex["turbo"] = CodexPreset{Compact: 200000} }, `unknown codex preset "turbo"`},
		{"codex compact 5", func(f *budgetFront) { f.Codex["economic"] = CodexPreset{Compact: 5} }, "codex economic: compact must be"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := goodFront()
			c.mutate(&f)
			err := validateBudget(f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
		})
	}
}

func TestValidateBudgetAccepts(t *testing.T) {
	t.Run("codex compact 0 means not set", func(t *testing.T) {
		f := goodFront()
		f.Codex["economic"] = CodexPreset{Compact: 0, SubagentEffort: "medium"}
		if err := validateBudget(f); err != nil {
			t.Fatal(err)
		}
	})
	for _, n := range []int{100000, 1000000} {
		f := goodFront()
		p := f.Presets["high"]
		p.Compact = n
		f.Presets["high"] = p
		if err := validateBudget(f); err != nil {
			t.Fatalf("compact %d is inside the range: %v", n, err)
		}
	}
	f := goodFront()
	f.Default = ""
	if err := validateBudget(f); err != nil {
		t.Fatalf("an empty default is allowed: %v", err)
	}
}

// ---- BudgetUndo ----

// highOtherThanBuilder puts four agent files already on the High values, so an Apply of
// High rewrites only grid-builder; the overlay does not exist, so Apply creates it.
func highOtherThanBuilder(t *testing.T, agents string) {
	t.Helper()
	for _, n := range []string{"grid-planner", "grid-reviewer", "grid-researcher", "grid-designer"} {
		put(t, filepath.Join(agents, n+".md"), agentText(n, "opus", "high", true), 0o644)
	}
}

func TestBudgetUndoRestoresChangedAndRemovesCreated(t *testing.T) {
	o, agents := budgetFixture(t)
	highOtherThanBuilder(t, agents)
	builder := filepath.Join(agents, "grid-builder.md")
	overlay := filepath.Join(o.GridHome, "settings.json")
	before := contents(t, builder)
	if _, err := os.Stat(overlay); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the fixture must have no overlay: %v", err)
	}
	o.Preset = "high"
	if res, err := BudgetApply(o); err != nil || res.Files != 1 {
		t.Fatalf("apply: %+v %v", res, err)
	}
	if string(contents(t, builder)) == string(before) {
		t.Fatal("apply must change grid-builder")
	}
	if _, err := os.Stat(snapshotPath(o)); err != nil {
		t.Fatalf("apply must leave the undo snapshot: %v", err)
	}
	res, err := BudgetUndo(o)
	must(t, err)
	if string(contents(t, builder)) != string(before) {
		t.Fatalf("the changed file must be back to its pre-Apply bytes:\n%s", contents(t, builder))
	}
	if _, err := os.Stat(overlay); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the overlay Apply created must be removed: %v", err)
	}
	if _, err := os.Stat(snapshotPath(o)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the snapshot must be deleted: %v", err)
	}
	if !slices.Equal(res.Restored, []string{"grid-builder"}) || len(res.Skipped) != 0 {
		t.Fatalf("undo: %+v", res)
	}
	var ce *CodedError
	if _, err := BudgetUndo(o); !errors.As(err, &ce) || ce.Code != codeNothingToUndo || ce.Error() != "There is no Apply to undo." {
		t.Fatalf("a second undo must be nothing_to_undo: %v", err)
	}
}

// BudgetApply never creates an agent file (a missing one is skipped), so a snapshot
// that records one as not existing before is written by hand.
func TestBudgetUndoRemovesAnAgentFileTheSnapshotSaysWasCreated(t *testing.T) {
	o, agents := budgetFixture(t)
	created := filepath.Join(agents, "grid-planner.md")
	changed := filepath.Join(agents, "grid-builder.md")
	overlay := filepath.Join(o.GridHome, "settings.json")
	createdNow, changedNow, overlayNow := []byte("created by apply\n"), []byte("changed by apply\n"), []byte("{}\n")
	changedBefore := []byte("before apply\n")
	put(t, created, createdNow, 0o644)
	put(t, changed, changedNow, 0o644)
	put(t, overlay, overlayNow, 0o644)
	snap := budgetSnapshot{Preset: "high", Overlay: snapFile{Path: overlay, AfterSHA: shaOf(overlayNow)},
		Agents: []snapAgent{
			{Agent: "grid-planner", File: snapFile{Path: created, Existed: false, AfterSHA: shaOf(createdNow)}},
			{Agent: "grid-builder", File: snapFile{Path: changed, Existed: true, Before: changedBefore, AfterSHA: shaOf(changedNow)}},
		}}
	raw, err := json.Marshal(snap)
	must(t, err)
	put(t, snapshotPath(o), raw, 0o600)
	res, err := BudgetUndo(o)
	must(t, err)
	if _, err := os.Stat(created); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the created file must be gone: %v", err)
	}
	if string(contents(t, changed)) != string(changedBefore) {
		t.Fatalf("the changed file must be back: %s", contents(t, changed))
	}
	if !slices.Equal(res.Restored, []string{"grid-planner", "grid-builder"}) {
		t.Fatalf("restored: %+v", res)
	}
	if _, err := os.Stat(snapshotPath(o)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the snapshot must be deleted: %v", err)
	}
}

func TestBudgetUndoSkipsTheFileThePersonEditedAfterApply(t *testing.T) {
	o, agents := budgetFixture(t)
	before := map[string]string{}
	for _, n := range AgentNames() {
		if p := filepath.Join(agents, n+".md"); fileExists(p) {
			before[n] = string(contents(t, p))
		}
	}
	o.Preset = "high"
	_, err := BudgetApply(o)
	must(t, err)
	builder := filepath.Join(agents, "grid-builder.md")
	mine := append(contents(t, builder), []byte("Edited by hand.\n")...)
	put(t, builder, mine, 0o644)
	res, err := BudgetUndo(o)
	must(t, err)
	if len(res.Skipped) != 1 || res.Skipped[0].Agent != "grid-builder" || res.Skipped[0].Reason != "changed after apply" {
		t.Fatalf("skipped: %+v", res.Skipped)
	}
	if string(contents(t, builder)) != string(mine) {
		t.Fatal("the edited file must be untouched")
	}
	if slices.Contains(res.Restored, "grid-builder") || len(res.Restored) != 4 {
		t.Fatalf("restored: %+v", res.Restored)
	}
	for _, n := range []string{"grid-planner", "grid-reviewer", "grid-researcher", "grid-designer"} {
		if got := string(contents(t, filepath.Join(agents, n+".md"))); got != before[n] {
			t.Fatalf("%s must be restored:\n%s", n, got)
		}
	}
}

func TestBudgetUndoWithNoSnapshot(t *testing.T) {
	o, _ := budgetFixture(t)
	_, err := BudgetUndo(o)
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != codeNothingToUndo || ce.Msg != "There is no Apply to undo." {
		t.Fatalf("got %v", err)
	}
}

func fileExists(p string) bool { _, err := os.Lstat(p); return err == nil }

// ---- projectFile, tilde, fileSHA ----

func TestProjectFile(t *testing.T) {
	st := &budgetState{}
	if st.projectFile(BudgetOptions{ProjectDir: ""}, "grid-builder") {
		t.Fatal("no project dir: false")
	}
	project := t.TempDir()
	o := BudgetOptions{ProjectDir: project}
	if st.projectFile(o, "grid-builder") {
		t.Fatal("no file there: false")
	}
	put(t, filepath.Join(project, ".claude", "agents", "grid-builder.md"), []byte("x"), 0o644)
	if !st.projectFile(o, "grid-builder") {
		t.Fatal("a regular agents/<name>.md: true")
	}
	if err := os.MkdirAll(filepath.Join(project, ".claude", "agents", "grid-designer.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if st.projectFile(o, "grid-designer") {
		t.Fatal("a directory with that name: false")
	}
}

func TestTildeAndFileSHA(t *testing.T) {
	home := t.TempDir()
	if got := tilde(home, filepath.Join(home, ".claude", "agents", "a.md")); got != "~/.claude/agents/a.md" {
		t.Fatalf("under home: %q", got)
	}
	other := filepath.Join(t.TempDir(), "x", "y.md")
	if got := tilde(home, other); got != filepath.ToSlash(other) {
		t.Fatalf("outside home: %q", got)
	}
	if got := tilde(home, home); got != filepath.ToSlash(home) {
		t.Fatalf("home itself is not rewritten: %q", got)
	}
	if got := tilde("", other); got != filepath.ToSlash(other) {
		t.Fatalf("empty home: %q", got)
	}
	if got := fileSHA(claudeFile{}); got != "" {
		t.Fatalf("a missing file has no sha: %q", got)
	}
	f := claudeFile{exists: true, data: []byte("abc")}
	if got := fileSHA(f); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha256 of abc: %q", got)
	}
}

// ---- writeRendered ----

func TestWriteRendered(t *testing.T) {
	rendered := agentText("grid-builder", "opus", "high", true)
	older := agentText("grid-builder", "sonnet", "medium", true)
	yours := agentText("grid-builder", "sonnet", "medium", false)

	t.Run("new file is written and its parent made", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "claude", "agents", "grid-builder.md")
		state, err := writeRendered(path, rendered)
		if err != nil || state != syncWritten {
			t.Fatalf("got %q %v", state, err)
		}
		if string(contents(t, path)) != string(rendered) {
			t.Fatal("bytes on disk differ")
		}
	})
	t.Run("same bytes are unchanged", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "grid-builder.md")
		put(t, path, rendered, 0o644)
		state, err := writeRendered(path, rendered)
		if err != nil || state != syncUnchanged {
			t.Fatalf("got %q %v", state, err)
		}
		if string(contents(t, path)) != string(rendered) {
			t.Fatal("bytes on disk differ")
		}
	})
	t.Run("a directory at the path is yours", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "grid-builder.md")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		state, err := writeRendered(path, rendered)
		if err != nil || state != syncYours {
			t.Fatalf("got %q %v", state, err)
		}
		if info, err := os.Lstat(path); err != nil || !info.IsDir() {
			t.Fatalf("the directory must be untouched: %v", err)
		}
	})
	t.Run("a file without the Grid marker is yours and untouched", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "grid-builder.md")
		put(t, path, yours, 0o644)
		state, err := writeRendered(path, rendered)
		if err != nil || state != syncYours {
			t.Fatalf("got %q %v", state, err)
		}
		if string(contents(t, path)) != string(yours) {
			t.Fatal("the person's file was changed")
		}
	})
	t.Run("a Grid-owned different file is rewritten", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "grid-builder.md")
		put(t, path, older, 0o644)
		state, err := writeRendered(path, rendered)
		if err != nil || state != syncWritten {
			t.Fatalf("got %q %v", state, err)
		}
		if string(contents(t, path)) != string(rendered) {
			t.Fatal("bytes on disk differ")
		}
	})
	t.Run("a parent that is a regular file is an error and nothing is written", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "agents")
		put(t, parent, []byte("not a folder"), 0o644)
		state, err := writeRendered(filepath.Join(parent, "grid-builder.md"), rendered)
		if err == nil || state != "" {
			t.Fatalf("got %q %v", state, err)
		}
		if info, err := os.Lstat(parent); err != nil || !info.Mode().IsRegular() || string(contents(t, parent)) != "not a folder" {
			t.Fatalf("the parent file must be untouched: %v", err)
		}
	})
	t.Run("a symlink at the path is yours and its target unchanged", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on Windows")
		}
		dir := t.TempDir()
		target := filepath.Join(dir, "target.md")
		put(t, target, older, 0o644)
		path := filepath.Join(dir, "grid-builder.md")
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		state, err := writeRendered(path, rendered)
		if err != nil || state != syncYours {
			t.Fatalf("got %q %v", state, err)
		}
		if string(contents(t, target)) != string(older) {
			t.Fatal("the symlink target was changed")
		}
		if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("the symlink must stay: %v", err)
		}
	})
}

// ---- splitFront ----

func TestSplitFront(t *testing.T) {
	t.Run("normal file", func(t *testing.T) {
		front, body, ok := splitFront("---\nname: a\nmodel: x\n---\nBody\n")
		if !ok || !slices.Equal(front, []string{"name: a", "model: x"}) || body != "\nBody\n" {
			t.Fatalf("got %q %q %v", front, body, ok)
		}
	})
	t.Run("no leading marker", func(t *testing.T) {
		for _, text := range []string{"name: a\n---\nBody\n", "", " ---\nname: a\n---\n"} {
			if front, body, ok := splitFront(text); ok || front != nil || body != "" {
				t.Fatalf("%q: got %q %q %v", text, front, body, ok)
			}
		}
	})
	t.Run("empty front matter", func(t *testing.T) {
		front, body, ok := splitFront("---\n---\n")
		if !ok || len(front) != 0 || body != "\n" {
			t.Fatalf("got %q %q %v", front, body, ok)
		}
	})
	t.Run("no closing marker", func(t *testing.T) {
		for _, text := range []string{"---\nname: a\n", "---\nname: a\n---", "---\n"} {
			if front, body, ok := splitFront(text); ok || front != nil || body != "" {
				t.Fatalf("%q: got %q %q %v", text, front, body, ok)
			}
		}
	})
	t.Run("CRLF is not recognised here", func(t *testing.T) {
		// Pinned: splitFront alone does not read CRLF; renderAgent and ownedByGrid
		// turn CRLF into LF before they call it (agentsync.go:155, :202).
		if front, body, ok := splitFront("---\r\nname: a\r\n---\r\nBody\r\n"); ok || front != nil || body != "" {
			t.Fatalf("got %q %q %v", front, body, ok)
		}
	})
}

// ---- ClaudeAgentsDir, ClaudeRowNames, defaultManagedPath, CodedError ----

func TestClaudeAgentsDir(t *testing.T) {
	home := t.TempDir()
	got, err := ClaudeAgentsDir(home, "")
	if want := filepath.Join(home, ".claude", "agents"); err != nil || got != want {
		t.Fatalf("got %q %v, want %q", got, err, want)
	}
	cfg := filepath.Join(t.TempDir(), "cfg")
	got, err = ClaudeAgentsDir(home, cfg)
	if want := filepath.Join(cfg, "agents"); err != nil || got != want {
		t.Fatalf("configDir: got %q %v, want %q", got, err, want)
	}
	settingsPath, err := ClaudeSettingsPath(home, cfg)
	must(t, err)
	if got != filepath.Join(filepath.Dir(settingsPath), "agents") {
		t.Fatalf("must sit beside settings.json: %q vs %q", got, settingsPath)
	}
	got, err = ClaudeAgentsDir("", filepath.Join("rel", "cfg"))
	abs, absErr := filepath.Abs(filepath.Join("rel", "cfg", "agents"))
	must(t, absErr)
	if err != nil || !filepath.IsAbs(got) || got != abs {
		t.Fatalf("a relative configDir is made absolute: got %q %v, want %q", got, err, abs)
	}
	if got, err := ClaudeAgentsDir("", ""); err == nil || got != "" {
		t.Fatalf("no home and no configDir must fail: %q %v", got, err)
	}
}

func TestClaudeRowNamesIsACopy(t *testing.T) {
	want := []string{"autoCompactWindow", "autoCompactEnabled", "advisorModel", "permissions.defaultMode", "remoteControlAtStartup"}
	first := ClaudeRowNames()
	if !slices.Equal(first, want) {
		t.Fatalf("got %v", first)
	}
	first[0], first[4] = "changed", "changed"
	if second := ClaudeRowNames(); !slices.Equal(second, want) {
		t.Fatalf("mutating the result changed the list: %v", second)
	}
}

func TestDefaultManagedPath(t *testing.T) {
	want := "/etc/claude-code/managed-settings.json"
	switch runtime.GOOS {
	case "darwin":
		want = "/Library/Application Support/ClaudeCode/managed-settings.json"
	case "windows":
		want = `C:\Program Files\ClaudeCode\managed-settings.json`
	}
	if got := defaultManagedPath(); got != want {
		t.Fatalf("%s: got %q, want %q", runtime.GOOS, got, want)
	}
}

func TestCodedErrorMessageAndUnwrap(t *testing.T) {
	inner := errors.New("disk full")
	coded := &CodedError{Code: codeWriteFailed, Path: "p", Msg: "Could not write p.", Err: inner}
	if coded.Error() != "Could not write p." {
		t.Fatalf("Error: %q", coded.Error())
	}
	if !errors.Is(coded, inner) || errors.Unwrap(coded) != inner {
		t.Fatal("errors.Is must reach the inner error through Unwrap")
	}
	var wrapped error = coded
	var ce *CodedError
	if !errors.As(wrapped, &ce) || ce.Code != codeWriteFailed {
		t.Fatal("errors.As")
	}
	if plain := codedErr(codeNothingToUndo, "", "no %s", "undo"); plain.Error() != "no undo" || errors.Unwrap(plain) != nil {
		t.Fatalf("a coded error with no inner error: %v", plain)
	}
}
