package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// checkHomeSource says where the Grid home came from: a flag, GRID_HOME, the shared
// config's current workspace, its older shape, or the default ~/.grid. It names a
// config that could not be used and the folder ~/.grid when it also holds data while
// the config points elsewhere. The row only informs: it always passes.
func checkHomeSource(add func(string, bool, string), o Options) {
	r := o.Resolved
	if r.Source == "" {
		return
	}
	tilde := func(p string) string { return gridhome.Tilde(p, o.Home) }
	var parts []string
	switch r.Source {
	case gridhome.SourceFlag:
		parts = append(parts, "from --grid-home")
	case gridhome.SourceEnv:
		parts = append(parts, "from GRID_HOME")
	case gridhome.SourceConfig:
		parts = append(parts, "from "+tilde(r.Config)+", workspace "+r.Workspace)
	case gridhome.SourceLegacy:
		parts = append(parts, "from the older shape of "+tilde(r.Config)+", workspace "+r.Workspace+"; it is rewritten when setup saves")
	default:
		parts = append(parts, "the default "+tilde(r.Dir))
	}
	if r.Member != "" {
		parts = append(parts, "member "+r.Member+" from the config")
	}
	if r.ConfigProblem != "" {
		parts = append(parts, tilde(gridconfig.Path(o.Home))+" not used: "+r.ConfigProblem)
	}
	if r.Machine != "" && filepath.Clean(o.GridHome) != filepath.Clean(r.Machine) && holdsData(r.Machine) {
		parts = append(parts, tilde(r.Machine)+" also holds data; "+tilde(o.GridHome)+" is used")
	}
	add("grid home source", true, strings.Join(parts, "; "))
}

// holdsData reports a folder with a VERSION file or a projects folder.
func holdsData(dir string) bool {
	for _, name := range []string{"VERSION", "projects"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}
