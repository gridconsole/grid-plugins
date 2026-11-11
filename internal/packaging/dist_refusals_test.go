package packaging

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// refusalNoBinaries asserts Build stopped before any go build ran: no
// dist/plugin/bin/grid-* file exists under root.
func refusalNoBinaries(t *testing.T, root string) {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(root, "dist", "plugin", "bin", "grid-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("a binary was built before the refusal: %v", found)
	}
}

func TestBuildRefusesDistThatIsAFile(t *testing.T) {
	root := t.TempDir()
	dist := filepath.Join(root, "dist")
	writeTestFile(t, dist, "keep me", 0o644)
	if err := Build(root, "1", io.Discard); err == nil {
		t.Fatal("Build succeeded with dist a regular file")
	}
	got, err := os.ReadFile(dist)
	if err != nil || string(got) != "keep me" {
		t.Fatalf("dist file changed: %q, %v", got, err)
	}
	refusalNoBinaries(t, root)
}

func TestBuildRefusesChecksumsThatIsANonEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "dist", "SHA256SUMS", "inner.txt")
	writeTestFile(t, inner, "keep me", 0o644)
	if err := Build(root, "1", io.Discard); err == nil {
		t.Fatal("Build succeeded with dist/SHA256SUMS a non-empty directory")
	}
	if _, err := os.Stat(inner); err != nil {
		t.Fatalf("file inside SHA256SUMS is gone: %v", err)
	}
	refusalNoBinaries(t, root)
}

func TestBuildRefusesMissingSkillsFolder(t *testing.T) {
	root := t.TempDir()
	if err := Build(root, "1", io.Discard); err == nil {
		t.Fatal("Build succeeded with no skills folder")
	}
	refusalNoBinaries(t, root)
}

func TestBuildRefusesMissingLicense(t *testing.T) {
	root := t.TempDir()
	for _, dir := range shippedDirs {
		writeTestFile(t, filepath.Join(root, dir, "one.txt"), "x", 0o644)
	}
	err := Build(root, "1", io.Discard)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Build with no LICENSE = %v, want an error that is os.ErrNotExist", err)
	}
	refusalNoBinaries(t, root)
}

func TestBuildRefusesInvalidVersionContrast(t *testing.T) {
	err := Build(t.TempDir(), "bad version!", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "invalid release version") {
		t.Fatalf("Build(bad version) = %v, want invalid release version", err)
	}
}

func TestStageRefusesPluginUnderAFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "afile")
	writeTestFile(t, file, "x", 0o644)
	if err := stage(root, filepath.Join(file, "plugin"), "1"); err == nil {
		t.Fatal("stage succeeded with the plugin path under a regular file")
	}
}

func TestWriteRefusesRootThatIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	writeTestFile(t, file, "x", 0o644)
	if err := Write(file, "1"); err == nil {
		t.Fatal("Write succeeded with root a regular file")
	}
}

func TestWriteRefusesLauncherThatIsADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin", "grid"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, "1"); err == nil {
		t.Fatal("Write succeeded with bin/grid a directory")
	}
}

func TestWriteChecksumsRefusesMissingPlugin(t *testing.T) {
	dist := t.TempDir()
	if err := writeChecksums(dist, filepath.Join(dist, "missing")); err == nil {
		t.Fatal("writeChecksums succeeded on a plugin folder that does not exist")
	}
	if _, err := os.Stat(filepath.Join(dist, "SHA256SUMS")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("SHA256SUMS was written: stat err %v", err)
	}
}
