package hook

import (
	"os"
	"path/filepath"
	"runtime"
)

// voLauncherPathLine returns the line that puts the Grid home's bin folder on PATH, or ""
// when home is empty, <home>/bin/grid is missing or a directory (a link that resolves to a
// file counts), the folder is already on PATH, or the host is Windows (its env output is
// unchanged). The folder outlives plugin updates, so a session started before an update
// keeps a working `grid`. SessionStart falls back to pluginPathLine when this returns "".
func voLauncherPathLine(home, pathEnv string) string {
	if home == "" || runtime.GOOS == "windows" {
		return ""
	}
	bin := filepath.Join(home, "bin")
	if st, err := os.Stat(filepath.Join(bin, "grid")); err != nil || st.IsDir() {
		return ""
	}
	for _, p := range filepath.SplitList(pathEnv) {
		if p != "" && filepath.Clean(p) == bin {
			return ""
		}
	}
	return "export PATH=" + shellQuote(bin) + ":\"$PATH\"\n"
}
