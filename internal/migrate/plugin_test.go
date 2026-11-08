package migrate

import (
	"path/filepath"
	"strings"
	"testing"
)

var pointerExts = []string{"", ".attached", ".tasks", ".tasks-agent", ".planner"}

// pluginFixture fills a plugin data folder, the folder the Go plugin reads its session
// pointers from, with pointers that name a task of the named home alpha.
func pluginFixture(t *testing.T, from string) (data, oldTask string) {
	t.Helper()
	data = filepath.Join(filepath.Dir(from), "plugin-data")
	oldTask = filepath.Join(from, "alpha/projects/widget/tasks/T-07-first")
	for _, ext := range pointerExts {
		put(t, filepath.Join(data, "sessions/hook"+ext), oldTask+"\r\n")
	}
	for _, ext := range []string{".usage", ".agents", ".briefs", ".gates", ".bash-tool"} {
		put(t, filepath.Join(data, "sessions/hook"+ext), "opaque data\r\n")
	}
	return data, oldTask
}

// checkPluginPointers reads the pointers where the guard reads them: sessions/<id><ext> in the
// plugin data folder itself.
func checkPluginPointers(t *testing.T, data, want string) {
	t.Helper()
	for _, ext := range pointerExts {
		if got := read(t, data, "sessions/hook"+ext); got != want+"\r\n" {
			t.Fatalf("plugin pointer %q names %q, want %q", ext, got, want)
		}
	}
	if read(t, data, "sessions/hook.usage") != "opaque data\r\n" {
		t.Fatal("usage changed")
	}
}

func TestPluginSessionsCrashMatrix(t *testing.T) {
	for _, step := range applySteps {
		t.Run(step, func(t *testing.T) {
			t.Parallel()
			from, _ := fixture(t)
			home := from
			data, _ := pluginFixture(t, from)
			crashAt(t, step, from, home, "CLAUDE_PLUGIN_DATA="+data)
			finishAfterCrash(t, from, home, WithPluginData(data))
			newTask := filepath.Join(home, "projects/widget/tasks/T-07-first")
			for _, ext := range pointerExts {
				if read(t, home, "sessions/hook"+ext) != newTask+"\r\n" {
					t.Fatal("pointer not copied into the home", ext)
				}
			}
			if read(t, home, "sessions/hook.usage") != "opaque data\r\n" {
				t.Fatal("usage changed")
			}
			checkPluginPointers(t, data, newTask)
		})
	}
}

// The plugin reads the pointers in its own folder, so migration rewrites them there,
// and a rollback puts the old paths back, also when it stops part way.
func TestPluginPointersRewrittenInPlaceAndRolledBack(t *testing.T) {
	for _, step := range []string{"", "rollback-published", "worktree-rewritten"} {
		t.Run(step, func(t *testing.T) {
			t.Parallel()
			from, _ := fixture(t)
			data, oldTask := pluginFixture(t, from)
			before := view(t, data)
			oldHome := view(t, from)
			if _, err := scan(t, from, from, WithPluginData(data)).Apply(); err != nil {
				t.Fatal(err)
			}
			checkPluginPointers(t, data, filepath.Join(from, "projects/widget/tasks/T-07-first"))
			if step != "" {
				crashAt(t, step, from, from, rollbackEnv, "CLAUDE_PLUGIN_DATA="+data)
			}
			finishRollback(t, from)
			if !sameView(oldHome, view(t, from)) {
				t.Fatal("old home not restored")
			}
			checkPluginPointers(t, data, oldTask)
			if !sameView(before, view(t, data)) {
				t.Fatal("plugin data differs from before the migration")
			}
		})
	}
}

func TestPluginPointerChangedBeforeRollbackIsRefused(t *testing.T) {
	from, _ := fixture(t)
	data, _ := pluginFixture(t, from)
	if _, err := scan(t, from, from, WithPluginData(data)).Apply(); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(data, "sessions/hook.tasks"), "somewhere else\r\n")
	before := view(t, filepath.Dir(from))
	if _, err := Rollback(from); err == nil || !strings.Contains(err.Error(), "plugin session pointer changed") {
		t.Fatal(err)
	}
	if !sameView(before, view(t, filepath.Dir(from))) {
		t.Fatal("refusal changed data")
	}
}

func TestPluginSessionsStaleAndConflict(t *testing.T) {
	from, home := fixture(t)
	data := filepath.Join(filepath.Dir(from), "plugin-data")
	put(t, filepath.Join(data, "sessions/hook.usage"), "original")
	p := scan(t, from, home, WithPluginData(data))
	put(t, filepath.Join(data, "sessions/hook.usage"), "new usage")
	if _, err := p.Apply(); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatal(err)
	}
	put(t, filepath.Join(from, "alpha/sessions/hook.usage"), "conflict")
	p = scan(t, from, home, WithPluginData(data))
	if len(p.Conflicts) == 0 {
		t.Fatal("missing conflict")
	}
}
