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
			"internal/where/personal.go": "caller-aware enumeration choke point", // Filters primitive roots using the caller's Personal policy.
			"internal/task/create.go":    "global allocation and cleanup",        // Global IDs and stale staging cleanup must include every root.
		},
		"where.Resolver{": {
			"internal/cli/cli.go":        "CLI boundary",   // The shared CLI resolver stamps the caller from its environment.
			"internal/guard/settings.go": "guard boundary", // Direct guard adapters stamp the caller from their environment.
		},
	}
	used := map[string]map[string]bool{}
	for needle, exceptions := range allowed {
		used[needle] = map[string]bool{}
		for entry, reason := range exceptions {
			if strings.TrimSpace(reason) == "" {
				t.Fatalf("missing exception reason: %s", entry)
			}
		}
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
			if _, permitted := exceptions[rel]; !permitted {
				t.Errorf("%s uses %s outside the caller-policy choke point", rel, needle)
			} else {
				used[needle][rel] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for needle, exceptions := range allowed {
		for entry := range exceptions {
			if !used[needle][entry] {
				t.Errorf("stale exception: %s no longer uses %s", entry, needle)
			}
		}
	}
}
