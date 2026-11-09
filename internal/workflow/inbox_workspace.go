package workflow

import (
	"errors"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// ErrInboxWorkspace deliberately omits workflow paths and policy values.
var ErrInboxWorkspace = errors.New("invalid inbox_workspace in workspace workflow (only personal is supported)")

// InboxWorkspace reads only the workspace policy through the bounded,
// symlink-refusing workflow reader. Missing files and unset keys use Personal.
func InboxWorkspace(home string) (string, error) {
	text, _, err := readFile(gridhome.Real(home), "workflows.md")
	if errors.Is(err, ErrLinkedFile) {
		// A linked file is not an invalid value: name the link so the person
		// knows to remove it. The error carries no machine path.
		return "", err
	}
	if err != nil {
		return "", ErrInboxWorkspace
	}
	first, _, _ := strings.Cut(text, "\n")
	if strings.TrimSpace(first) != "---" {
		return where.Personal, nil
	}
	front, err := parseFrontmatter([]byte(text))
	if err != nil {
		return "", ErrInboxWorkspace
	}
	value, set := front["inbox_workspace"]
	if !set {
		return where.Personal, nil
	}
	if value, ok := value.(string); ok && value == where.Personal {
		return value, nil
	}
	return "", ErrInboxWorkspace
}
