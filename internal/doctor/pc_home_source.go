package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// checkHomeSource says where the Grid home came from: a flag, GRID_HOME, the shared
// config's current workspace, its older shape, or the default ~/.grid; which config
// file was read and whether it was copied from ~/.grid/config.json; the machine
// folder, the brain folder and the member. It names a config that could not be used
// and the folder ~/.grid when it also holds data while another folder is used. The
// row only informs: it always passes.
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
	if r.ConfigCopiedFrom != "" {
		parts = append(parts, tilde(r.ConfigCopiedFrom)+" copied to "+tilde(r.Config)+"; the old file is kept")
	} else if r.Config != "" && r.Config == gridconfig.OldPath(o.Home) {
		parts = append(parts, "config read from the older "+tilde(r.Config))
	}
	if r.Machine != "" {
		from := "the OS standard place"
		switch r.MachineSource {
		case gridhome.SourceConfig:
			from = "the config"
		case gridhome.SourceEnv:
			from = "the environment"
		}
		parts = append(parts, "machine folder "+tilde(r.Machine)+" from "+from)
	}
	if r.Brain != "" {
		parts = append(parts, "brain "+tilde(r.Brain)+" from the config")
	}
	if r.Member != "" {
		parts = append(parts, "member "+r.Member+" from the config")
	} else if name, src := gridhome.DefaultMember(os.Getenv, o.Home); name != "" {
		// The name itself stays out of the row: doctor bundles are shared.
		from := "the OS login name"
		if src == gridhome.SourceAccount {
			from = "the Claude account's display name"
		}
		parts = append(parts, "no member in the config; setup offers "+from)
	}
	if r.ConfigProblem != "" {
		file := r.Config
		if file == "" {
			file = gridconfig.Path(os.Getenv, o.Home)
		}
		parts = append(parts, tilde(file)+" not used: "+r.ConfigProblem)
	}
	if r.DataRoot != "" && filepath.Clean(o.GridHome) != filepath.Clean(r.DataRoot) && holdsData(r.DataRoot) {
		parts = append(parts, tilde(r.DataRoot)+" also holds data; "+tilde(o.GridHome)+" is used")
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
