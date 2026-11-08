package packaging

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestChecksumsCoverPackage(t *testing.T) {
	dist := t.TempDir()
	plugin := filepath.Join(dist, "plugin")
	writeTestFile(t, filepath.Join(plugin, "bin", "grid-linux-amd64"), "binary", 0o755)
	writeTestFile(t, filepath.Join(plugin, ".claude-plugin", "plugin.json"), "manifest", 0o644)
	writeTestFile(t, filepath.Join(dist, "stale"), "not shipped", 0o644)
	if err := writeChecksums(dist, plugin); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%x  plugin/.claude-plugin/plugin.json\n%x  plugin/bin/grid-linux-amd64\n", sha256.Sum256([]byte("manifest")), sha256.Sum256([]byte("binary")))
	if string(b) != want {
		t.Fatalf("%s", b)
	}
	if err := writeChecksums(dist, plugin); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if err != nil || string(again) != want {
		t.Fatalf("checksum output changed: %v", err)
	}
}

func TestCopySkillsPreservesTextAndScripts(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "source"), filepath.Join(root, "dest")
	const prose = "---\nname: example\n---\nUse `grid task list`.\n"
	writeTestFile(t, filepath.Join(source, "example", "SKILL.md"), prose, 0o644)
	writeTestFile(t, filepath.Join(source, "example", "scripts", "check"), "#!/bin/sh\nexit 0\n", 0o755)
	if err := copyTree(source, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "example", "SKILL.md"))
	if err != nil || string(b) != prose {
		t.Fatalf("skill changed: %q %v", b, err)
	}
	info, err := os.Stat(filepath.Join(dest, "example", "scripts", "check"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("script mode: %v", info.Mode())
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(source, "example", "SKILL.md"), filepath.Join(source, "link")); err != nil {
			t.Fatal(err)
		}
		if err := copyTree(source, dest); err == nil {
			t.Fatal("packaged a symlink")
		}
	}
}

func TestBuildRejectsInvalidVersionBeforeWriting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	for _, version := range []string{"", "v1\nextra", "v1 -X anything=bad", "../v1"} {
		if err := Build(root, version, io.Discard); err == nil {
			t.Errorf("accepted %q", version)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("created output for invalid version: %v", err)
	}
}

func TestBuildEnvironmentOverridesHost(t *testing.T) {
	t.Setenv("GOFLAGS", "-race")
	t.Setenv("GOAMD64", "v4")
	t.Setenv("CGO_ENABLED", "1")
	seen := make(map[string]string)
	for _, entry := range buildEnv("linux", "amd64") {
		key, value, _ := strings.Cut(entry, "=")
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("duplicate %s", key)
		}
		seen[key] = value
	}
	for key, want := range map[string]string{"GOOS": "linux", "GOARCH": "amd64", "GOFLAGS": "", "GOAMD64": "v1", "CGO_ENABLED": "0", "GOWORK": "off"} {
		if seen[key] != want {
			t.Errorf("%s=%q want %q", key, seen[key], want)
		}
	}
}
