package migrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginSessionsCrashMatrix(t *testing.T) {
	for _, step := range applySteps {
		t.Run(step, func(t *testing.T) {
			from, _ := fixture(t)
			home := from
			data := filepath.Join(filepath.Dir(from), "plugin-data")
			t.Setenv("CLAUDE_PLUGIN_DATA", data)
			oldTask := filepath.Join(from, "alpha/projects/widget/tasks/T-07-first")
			for _, ext := range []string{"", ".attached", ".tasks", ".tasks-agent", ".planner"} {
				put(t, filepath.Join(data, "sessions/hook"+ext), oldTask+"\r\n")
			}
			for _, ext := range []string{".usage", ".agents", ".briefs", ".gates", ".bash-tool"} {
				put(t, filepath.Join(data, "sessions/hook"+ext), "opaque data\r\n")
			}
			before := view(t, data)
			crashAt(t, step, from, home)
			finishAfterCrash(t, from, home)
			for _, ext := range []string{"", ".attached", ".tasks", ".tasks-agent", ".planner"} {
				if read(t, home, "sessions/hook"+ext) != filepath.Join(home, "projects/widget/tasks/T-07-first")+"\r\n" {
					t.Fatal("pointer not mapped", ext)
				}
			}
			if read(t, home, "sessions/hook.usage") != "opaque data\r\n" {
				t.Fatal("usage changed")
			}
			if !sameView(before, view(t, data)) {
				t.Fatal("plugin data changed")
			}
		})
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
