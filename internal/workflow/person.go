package workflow

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrPersonVerbs = errors.New("invalid person_verbs")

// PersonVerbs reads additive task-local person policy. A list gates verbs for
// all person roles; a map additionally restricts each verb to listed roles.
func PersonVerbs(dir string) (map[string][]string, error) {
	raw, err := readTaskPolicy(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var linked *os.PathError
	if errors.Is(err, ErrLinkedFile) && errors.As(err, &linked) {
		// Keep the link in the chain, without the task's path: the person gate
		// refuses instead of dropping the verbs this file adds.
		return nil, fmt.Errorf("%w: %w", ErrPersonVerbs, linked.Err)
	}
	if err != nil {
		return nil, errors.New("cannot read workflow.md")
	}
	return personVerbs(raw)
}

// A nil policy is unset; a non-nil empty map explicitly replaces defaults.
func personVerbs(raw []byte) (map[string][]string, error) {
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	// Legacy workflow prose carries no policy.
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, nil
	}
	front, err := parseFrontmatter(raw)
	if err != nil {
		return nil, errors.New("malformed workflow front matter")
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
				if !ok || (role != "owner" && role != "member" && role != "cli" && role != "lead" && role != "guest") {
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
