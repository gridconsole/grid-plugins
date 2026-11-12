package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	legacyHome = "/srv/gridwork/home"
	legacyPath = "export PATH='/srv/gridwork/plugin-dist/bin':\"$PATH\"\n"
)

// legacyCopy is one Grid export block as builds from before the markers wrote it:
// a blank separator and the four lines, with no marker lines around them.
func legacyCopy() string {
	return "\nexport GRID_HOME='" + legacyHome + "'\n" +
		"export GRID_PROJECTS='" + legacyHome + "/projects'\n" +
		"unset GRID_TASK\n" + legacyPath
}

func writeSeed(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sessionstart-hook-0.sh")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A long-lived session's file: many unmarked copies, then one marked block.
func TestSessionEnvLegacyUnmarkedBlocksAreDropped(t *testing.T) {
	const n = 110
	path := writeSeed(t, strings.Repeat(legacyCopy(), n))
	// First write adds the marked block after the legacy copies, as a marker-era build did.
	if err := writeEnv(path, legacyHome, "", legacyPath); err != nil {
		t.Fatal(err)
	}
	// Re-seed so the legacy copies sit before one marked block.
	marked := readEnvFile(t, path)
	if err := os.WriteFile(path, []byte(strings.Repeat(legacyCopy(), n)+marked), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeEnv(path, legacyHome, "", legacyPath); err != nil {
		t.Fatal(err)
	}
	got := readEnvFile(t, path)
	want := envBlockStart + "\nexport GRID_HOME='" + legacyHome + "'\n" +
		"export GRID_PROJECTS='" + legacyHome + "/projects'\n" +
		"unset GRID_TASK\n" + legacyPath + envBlockEnd + "\n"
	if strings.TrimLeft(got, "\n") != want {
		t.Fatalf("want exactly one marked block, got %d bytes:\n%s", len(got), got)
	}
	if c := strings.Count(got, envBlockStart); c != 1 {
		t.Errorf("start markers: %d, want 1", c)
	}
	// Legacy copies and no marked block at all: the file ends with one block too.
	path = writeSeed(t, strings.Repeat(legacyCopy(), 3))
	if err := writeEnv(path, legacyHome, "", legacyPath); err != nil {
		t.Fatal(err)
	}
	got = readEnvFile(t, path)
	if n := countLines(got, "export GRID_HOME="); n != 1 {
		t.Errorf("GRID_HOME lines without a marked block: %d, want 1\n%s", n, got)
	}
	if n := countLines(got, "export PATH="); n != 1 {
		t.Errorf("export PATH lines without a marked block: %d, want 1\n%s", n, got)
	}
}

func TestSessionEnvLegacyCleanupKeepsUserLines(t *testing.T) {
	seed := "export BEFORE=1\n" + legacyCopy() + "alias ll='ls -l'\n" + legacyCopy() +
		"export GRID_HOME='/elsewhere'\n\nexport AFTER=2\n"
	path := writeSeed(t, seed)
	if err := writeEnv(path, legacyHome, "", legacyPath); err != nil {
		t.Fatal(err)
	}
	got := readEnvFile(t, path)
	for _, keep := range []string{"export BEFORE=1\n", "alias ll='ls -l'\n", "export GRID_HOME='/elsewhere'\n", "export AFTER=2\n"} {
		if !strings.Contains(got, keep) {
			t.Errorf("user line %q is gone:\n%s", keep, got)
		}
	}
	if n := countLines(got, "export GRID_HOME='"+legacyHome+"'"); n != 1 {
		t.Errorf("writeEnv GRID_HOME lines: %d, want 1\n%s", n, got)
	}
	if n := countLines(got, "export PATH="); n != 1 {
		t.Errorf("export PATH lines: %d, want 1\n%s", n, got)
	}
	// The user lines keep their order.
	i, j, k := strings.Index(got, "export BEFORE=1"), strings.Index(got, "alias ll="), strings.Index(got, "export AFTER=2")
	if !(i < j && j < k) {
		t.Errorf("user lines were reordered:\n%s", got)
	}
}

func TestSessionEnvWithoutLegacyLinesIsUnchanged(t *testing.T) {
	path := writeSeed(t, "export BEFORE=1\n\n\nalias x=y\n")
	if err := writeEnv(path, legacyHome, "", legacyPath); err != nil {
		t.Fatal(err)
	}
	first := readEnvFile(t, path)
	seed := "export BEFORE=1\n\n\n" + first[len("export BEFORE=1\n\n\n"):] + "\n\nexport AFTER=2\n\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeEnv(path, legacyHome, "", legacyPath); err != nil {
		t.Fatal(err)
	}
	if got := readEnvFile(t, path); got != seed {
		t.Errorf("file without legacy lines changed:\nwant %q\ngot  %q", seed, got)
	}
}
