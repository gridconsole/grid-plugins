package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

func TestPersonalSettingsRefused(t *testing.T) {
	f := setup(t)
	personal := filepath.Join(f.grid, "personal", "tasks", "T-01-private")
	f.write(filepath.Join(personal, "task.md"), "private")
	f.write(filepath.Join(f.grid, "settings.json"), `{"hooks":{"home-allow":"off"}}`)
	f.env["GRID_ROLE"], f.env["GRID_MEMBER"], f.env["GRID_TASK"] = " guest ", " gus ", personal
	got := configuredLevel("home-allow", hookio.Input{Cwd: f.repo}, f.root, f.env, hookio.Deny("original"))
	if got.Behavior != "deny" || !strings.Contains(got.Reason, "task not found") || strings.Contains(got.Reason, personal) {
		t.Fatalf("decision = %+v", got)
	}
}
