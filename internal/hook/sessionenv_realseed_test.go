package hook

import (
	"os"
	"strings"
	"testing"
)

// The session-env file of a long-lived session, copied as it was: 111 unmarked Grid
// copies (110 with the plugin-dist PATH line) and one marked block.
func TestSessionEnvRealSeedKeepsOnlyTheMarkedBlock(t *testing.T) {
	seed, err := os.ReadFile("testdata/sessionstart-hook-0.sh")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(seed), "export PATH='/srv/gridwork/plugin-dist/bin':\"$PATH\"\n"); n != 110 {
		t.Fatalf("seed has %d legacy PATH lines, want 110", n)
	}
	path := writeSeed(t, string(seed))
	const home = "/root/.grid"
	pathLine := "export PATH='/root/.grid/bin':\"$PATH\"\n"
	if err := writeEnv(path, home, "", pathLine); err != nil {
		t.Fatal(err)
	}
	got := readEnvFile(t, path)
	want := envBlockStart + "\nexport GRID_HOME='/root/.grid'\n" +
		"export GRID_PROJECTS='/root/.grid/projects'\n" +
		"unset GRID_TASK\n" + pathLine + envBlockEnd + "\n"
	if got != want {
		t.Fatalf("want exactly one marked block, got %d lines (%d PATH lines):\n%s",
			strings.Count(got, "\n"), countLines(got, "export PATH="), got)
	}
}

// Only PATH lines that point at a plugin-dist or Grid bin folder are Grid's.
func TestSessionEnvLegacyPathCleanupKeepsOtherPathLines(t *testing.T) {
	keep := []string{
		"export PATH='/opt/x/bin':\"$PATH\"\n",
		"export PATH='/opt/plugin-dist-x/bin':\"$PATH\"\n",
		"export PATH='/srv/gridwork/plugin-dist/sbin':\"$PATH\"\n",
		"export PATH=\"$HOME/go/bin:$PATH\"\n",
	}
	grid := []string{
		"export PATH='/srv/gridwork/plugin-dist/bin':\"$PATH\"\n",
		"export PATH='/root/.grid/bin':\"$PATH\"\n",
		"export PATH='/home/ann/.grid/bin':\"$PATH\"\n",
	}
	seed := strings.Join(keep, "") + "\n" +
		"export GRID_HOME='/root/.grid'\nexport GRID_PROJECTS='/root/.grid/projects'\nunset GRID_TASK\n" +
		strings.Join(grid, "")
	path := writeSeed(t, seed)
	if err := writeEnv(path, "/root/.grid", "", "export PATH='/root/.grid/bin':\"$PATH\"\n"); err != nil {
		t.Fatal(err)
	}
	got := readEnvFile(t, path)
	for _, k := range keep {
		if !strings.Contains(got, k) {
			t.Errorf("user PATH line %q is gone:\n%s", k, got)
		}
	}
	if n := countLines(got, "export PATH='/srv/gridwork/plugin-dist/bin'"); n != 0 {
		t.Errorf("plugin-dist PATH lines left: %d\n%s", n, got)
	}
	if n := countLines(got, "export PATH='/home/ann/.grid/bin'"); n != 0 {
		t.Errorf("Grid bin PATH lines left: %d\n%s", n, got)
	}
	if n := countLines(got, "export PATH='/root/.grid/bin'"); n != 1 {
		t.Errorf("block PATH lines: %d, want 1\n%s", n, got)
	}
}
