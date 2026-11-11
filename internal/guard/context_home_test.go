package guard

import (
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// The guards judge the same Grid home the CLI uses: GRID_HOME when set (spelled
// the way gridhome.Dir spells it), else <HOME>/.grid.
func TestContextGridHomeFollowsGridhome(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name         string
		env          map[string]string
		wantHome     string
		wantProjects string
	}{
		{"moved home, spelled loosely", map[string]string{"GRID_HOME": root + "/data/../data/grid/", "HOME": "/home/example"},
			filepath.Join(root, "data", "grid"), filepath.Join(root, "data", "grid", "projects")},
		{"default home", map[string]string{"HOME": "/home/example"},
			filepath.Join("/home/example", ".grid"), filepath.Join("/home/example", ".grid", "projects")},
	}
	for _, tc := range tests {
		c := newContext(hookio.Input{}, root, tc.env)
		if c.home != tc.wantHome || c.projects != tc.wantProjects {
			t.Errorf("%s: home %q projects %q, want %q and %q", tc.name, c.home, c.projects, tc.wantHome, tc.wantProjects)
		}
	}
}
