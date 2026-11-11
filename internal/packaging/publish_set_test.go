package packaging

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// denyFolders must never appear, at any depth, in the public plugin tree.
var denyFolders = []string{"evals", "corpus", "recordings", "fixtures", "golden", "testdata", "lanes"}

// deniedPaths returns every directory or file in tree whose name is on the deny list.
func deniedPaths(t *testing.T, tree string) []string {
	t.Helper()
	var hits []string
	err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for _, name := range denyFolders {
			if entry.Name() == name {
				rel, _ := filepath.Rel(tree, path)
				hits = append(hits, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hits
}

func TestPluginTreeHasNoInternalFolders(t *testing.T) {
	root := filepath.Join("..", "..")
	plugin := filepath.Join(t.TempDir(), "plugin")
	if err := stage(root, plugin, "v0.0.0-test"); err != nil {
		t.Fatal(err)
	}
	if hits := deniedPaths(t, plugin); len(hits) != 0 {
		t.Fatalf("public plugin tree holds internal folders: %s", strings.Join(hits, ", "))
	}
	// The source repo does hold them, so the test is not passing on an empty set.
	for _, name := range []string{"evals", "corpus", "recordings", "fixtures", "golden"} {
		if info, err := os.Stat(filepath.Join(root, name)); err != nil || !info.IsDir() {
			t.Errorf("source folder %s missing: the deny list check has nothing to prove", name)
		}
	}
}

func TestStageLeavesDeniedSourceFoldersBehind(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "LICENSE"), "license", 0o644)
	writeTestFile(t, filepath.Join(root, "skills", "one", "SKILL.md"), "skill", 0o644)
	writeTestFile(t, filepath.Join(root, "agents", "a.md"), "agent", 0o644)
	writeTestFile(t, filepath.Join(root, "agent-templates", "t.md"), "template", 0o644)
	for _, name := range denyFolders {
		writeTestFile(t, filepath.Join(root, name, "case.txt"), "internal", 0o644)
	}
	plugin := filepath.Join(t.TempDir(), "plugin")
	if err := stage(root, plugin, "v0.0.0-test"); err != nil {
		t.Fatal(err)
	}
	if hits := deniedPaths(t, plugin); len(hits) != 0 {
		t.Fatalf("staged denied folders: %v", hits)
	}
	if _, err := os.Stat(filepath.Join(plugin, "skills", "one", "SKILL.md")); err != nil {
		t.Fatalf("shipped skill missing: %v", err)
	}
}

func TestDeniedPathsFindsAPlantedFolder(t *testing.T) {
	tree := t.TempDir()
	writeTestFile(t, filepath.Join(tree, "skills", "x", "evals", "case.md"), "internal", 0o644)
	hits := deniedPaths(t, tree)
	if len(hits) != 1 || hits[0] != "skills/x/evals" {
		t.Fatalf("hits = %v", hits)
	}
}

func TestShippedDirsAvoidTheDenyList(t *testing.T) {
	for _, dir := range shippedDirs {
		for _, name := range denyFolders {
			if dir == name {
				t.Errorf("shippedDirs lists denied folder %s", dir)
			}
		}
	}
}
