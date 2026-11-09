package where

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// ErrPersonalRefused means the caller cannot resolve the machine owner's Personal project.
var ErrPersonalRefused = errors.New("this caller cannot use the Personal project")

// ErrProjectUnregistered identifies an unavailable explicit project.
var ErrProjectUnregistered = errors.New("project is not registered")

// Caller distinguishes host person roles from the owner's machine sessions.
type Caller struct{ Role, Member string }

// CallerFrom reads the host's caller identity, trimming both environment values.
func CallerFrom(getenv func(string) string) Caller {
	return Caller{Role: strings.TrimSpace(getenv("GRID_ROLE")), Member: strings.TrimSpace(getenv("GRID_MEMBER"))}
}

// DefaultPersonalAllowed is a default, replaceable through PersonalAllowed.
// The workflow key "personal roles:" will carry this policy once L60R and L68
// land; this default does not read workflow files.
func DefaultPersonalAllowed(c Caller) bool {
	role := contract.NormalizeRole(c.Role)
	switch contract.TierFor(c.Role) {
	case contract.PersonGuest, contract.External:
		return false
	case contract.PersonTeam:
		return role == "owner"
	default:
		// TierFor also maps unknown roles to Agent. Only known machine
		// sessions and plugin agents may use the owner's Personal scope.
		switch role {
		case "", "cli", "host", "worker", "agent":
			return true
		}
		switch role {
		case "builder", "checker", "curator", "designer", "library", "operator",
			"planner", "researcher", "reviewer", "runner", "scout":
			return true
		default:
			return false
		}
	}
}

// PersonalOK reports whether this caller may use the Personal project.
func (r Resolver) PersonalOK() bool {
	allowed := r.PersonalAllowed
	if allowed == nil {
		allowed = DefaultPersonalAllowed
	}
	return allowed(r.Caller)
}

// PersonalProjectAllowed applies the caller's Personal policy to a registry name.
// Guest invitations are checked separately by each project's discovery path.
func (r Resolver) PersonalProjectAllowed(name string) bool {
	return !strings.EqualFold(name, Personal) || r.PersonalOK()
}

// TasksDirs lists the task roots visible to this caller.
func (r Resolver) TasksDirs() ([]string, error) {
	dirs, err := gridhome.TasksDirs(r.GridHome)
	if err != nil {
		return nil, err
	}
	personalOK := r.PersonalOK()
	if personalOK && !r.guest() {
		return dirs, nil
	}
	personal := gridhome.TasksDir(r.GridHome, Personal)
	visible := dirs[:0]
	for _, dir := range dirs {
		name := filepath.Base(filepath.Dir(dir))
		if (personalOK || dir != personal) && r.GuestProjectAllowed(name) && r.guestTaskPathAllowed(gridhome.TasksDir(gridhome.Real(r.GridHome), name)) {
			visible = append(visible, dir)
		}
	}
	return visible, nil
}

func unregisteredProject(name string) error { return projectUnavailable{name} }

type projectUnavailable struct{ name string }

func (e projectUnavailable) Error() string {
	return "project " + strconv.Quote(e.name) + " is not registered"
}
func (e projectUnavailable) Unwrap() error { return ErrProjectUnregistered }
