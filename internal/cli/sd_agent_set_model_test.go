package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSdAgentSetFable(t *testing.T) {
	h, repo := goUnsetFixture(t)
	for _, args := range [][]string{
		{"agent", "set", "grid-builder", "--model", "fable"},
		{"agent", "set", "grid-builder", "--project", "example", "--model", "opus[1m]"},
	} {
		if code := goUnsetRun(t, h, repo, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, h.errb)
		}
	}
	if code := goUnsetRun(t, h, repo, "agent", "list", "--json"); code != 0 {
		t.Fatalf("list: %d %s", code, h.errb)
	}
	if want := `"grid-builder":{"model":"opus[1m]","effort":"medium"}`; !strings.Contains(h.out.String(), want) {
		t.Fatalf("list %s lacks %s", h.out, want)
	}
	home := string(goUnsetRead(t, filepath.Join(h.gridHome, "settings.json")))
	if !strings.Contains(home, `"fable"`) {
		t.Fatalf("settings.json lacks fable: %s", home)
	}
}
