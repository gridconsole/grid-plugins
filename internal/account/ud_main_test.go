package account

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMain keeps a test that sets no PATH of its own from starting a real
// provider. windowsFor runs the direct read, which starts `claude` or `codex`
// from PATH; a test that builds its own Env (the stream tests) used to reach the
// installed Claude Code and wait for its answer. Two stand-ins that refuse at
// once go in front of the real PATH. A test that wants a provider sets PATH
// itself (machine does), and then these are out of the way. The helper processes
// of the TestGjRegistry tests are this binary run again with the PATH their
// parent chose, so they are left as they are.
func TestMain(m *testing.M) {
	os.Exit(runWithoutProviders(m))
}

func runWithoutProviders(m *testing.M) int {
	if runtime.GOOS != "windows" && os.Getenv("GJ_REGISTRY_CHILD") == "" {
		dir, err := os.MkdirTemp("", "grid-no-provider-")
		if err != nil {
			return 1
		}
		defer os.RemoveAll(dir)
		for _, name := range []string{"claude", "codex"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 2\n"), 0o755); err != nil {
				return 1
			}
		}
		os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return m.Run()
}
