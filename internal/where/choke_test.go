package where

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonalChokePoint(t *testing.T) {
	// Keep exceptions narrow and document why each caller can bypass propagation.
	allowed := map[string]map[string]string{
		"gridhome.TasksDirs(": {
			"internal/gridhome/":       "primitive task-root enumeration",
			"internal/where/":          "caller-aware enumeration choke point",
			"internal/task/create.go":  "global id allocation and staging cleanup need every root",
			"internal/task/history.go": "FolderAt is used by hooks running in agent sessions",
		},
		"where.Resolver{": {
			"internal/where/":            "resolver implementation owns caller policy",
			"internal/cli/cli.go":        "shared CLI resolver stamps the caller",
			"internal/guard/settings.go": "guard adapter stamps the caller from its environment",
			"internal/workflow/files.go": "workflow file enumeration has no caller and remains owned by workflow lanes",
		},
	}
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for needle, exceptions := range allowed {
			if !strings.Contains(string(data), needle) {
				continue
			}
			permitted := false
			for entry, reason := range exceptions {
				if reason == "" {
					t.Fatalf("missing exception reason: %s", entry)
				}
				if rel == entry || strings.HasSuffix(entry, "/") && strings.HasPrefix(rel, entry) {
					permitted = true
				}
			}
			if !permitted {
				t.Errorf("%s uses %s outside the caller-policy choke point", rel, needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
