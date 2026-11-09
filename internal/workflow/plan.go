package workflow

import (
	"errors"
	"os"
	"path/filepath"
)

// PlanHashToken reads plan_hash_token from the task's workflow.md front matter.
// Missing files or keys default to any; malformed or unreadable policy fails closed.
func PlanHashToken(taskPath string) (string, error) {
	invalid := errors.New("workflow.md: plan_hash_token must be any or digest")
	raw, err := os.ReadFile(filepath.Join(taskPath, "workflow.md"))
	if errors.Is(err, os.ErrNotExist) {
		return "any", nil
	}
	if err != nil {
		return "", invalid
	}
	front, err := parseFrontmatter(raw)
	if err != nil {
		return "", invalid
	}
	value, exists := front["plan_hash_token"]
	if !exists {
		return "any", nil
	}
	token, ok := value.(string)
	if !ok || (token != "any" && token != "digest") {
		return "", invalid
	}
	return token, nil
}
