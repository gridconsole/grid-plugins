package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

var ErrPersonVerbs = errors.New("invalid person_verbs")

// PersonVerbs reads additive task-local person policy. A list gates verbs for
// all person roles; a map additionally restricts each verb to listed roles.
func PersonVerbs(dir string) (map[string][]string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "workflow.md"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	// Legacy workflow prose carries no policy.
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, nil
	}
	end := 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "---" {
		end++
	}
	if end == len(lines) {
		return nil, fmt.Errorf("workflow front matter is not closed")
	}
	var front map[string]any
	if err = yaml.Load([]byte(strings.Join(lines[1:end], "\n")), &front); err != nil {
		return nil, err
	}
	value, ok := front["person_verbs"]
	if !ok {
		return nil, nil
	}
	out := map[string][]string{}
	switch v := value.(type) {
	case []any:
		for _, entry := range v {
			verb, ok := entry.(string)
			if !ok || strings.TrimSpace(verb) == "" {
				return nil, fmt.Errorf("%w: needs verb names", ErrPersonVerbs)
			}
			out[verb] = nil
		}
	case map[string]any:
		for verb, entry := range v {
			list, ok := entry.([]any)
			if !ok {
				return nil, fmt.Errorf("%w: needs role lists", ErrPersonVerbs)
			}
			roles := []string{}
			for _, r := range list {
				role, ok := r.(string)
				if !ok || (role != "owner" && role != "member" && role != "cli") {
					return nil, fmt.Errorf("%w: invalid role", ErrPersonVerbs)
				}
				roles = append(roles, role)
			}
			out[verb] = roles
		}
	default:
		return nil, fmt.Errorf("%w: needs a verb list or role map", ErrPersonVerbs)
	}
	return out, nil
}
