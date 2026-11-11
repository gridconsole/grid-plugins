package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

func fixHint(name, detail string) string {
	switch name {
	case "grid home":
		return "Run grid setup, or set GRID_HOME to your existing Grid data folder."
	case "grid home writable":
		return "Give your user write permission on the Grid home, or set GRID_HOME to a writable data folder."
	case "format version":
		if strings.Contains(detail, "Update grid.") {
			return "Update grid before opening this data."
		}
		return "Run grid migrate; if VERSION is missing or invalid, restore it from a known backup."
	case "git":
		return "Install git and add its executable directory to PATH."
	case "plugin hooks":
		return "Reinstall the Grid plugin, or pass --hooks-file with its hooks/hooks.json path."
	case "grid on PATH":
		return "Run grid setup and add the directory containing the grid launcher to PATH."
	case "allow block", "deny block":
		if strings.HasPrefix(detail, "missing rules: ") || detail == "Claude settings file is missing" {
			return "Run grid setup"
		}
		if strings.HasPrefix(detail, "Grid home ") {
			return "Fix the Grid home location, then run grid setup."
		}
		return "Fix or move the Claude settings file, then run grid setup."
	}
	return ""
}

func checkAllowBlock(add func(string, bool, string), o Options) {
	status := settings.ReadClaudeAllowStatus(o.SettingsPath, o.Home, o.GridHome)
	switch {
	case status.Reason != "":
		add("allow block", false, status.Reason)
	case len(status.Missing) > 0:
		add("allow block", false, "missing rules: "+strings.Join(status.Missing, ", "))
	default:
		add("allow block", true, "all setup allow rules are present")
	}
}

// checkDenyBlock reports whether setup's deny rules are in the Claude settings.
// It only reports; it changes nothing.
func checkDenyBlock(add func(string, bool, string), o Options) {
	status := settings.ReadClaudeDenyStatus(o.SettingsPath, o.Home, o.GridHome, o.Projects)
	switch {
	case status.Reason != "":
		add("deny block", false, status.Reason)
	case len(status.Missing) > 0:
		add("deny block", false, "missing rules: "+strings.Join(status.Missing, ", "))
	default:
		add("deny block", true, "all setup deny rules are present")
	}
}

// checkSandbox says whether Claude Code's sandbox is on. It is always ok: the
// sandbox is optional, so it never fails the report.
func checkSandbox(add func(string, bool, string), o Options) {
	status := settings.ReadClaudeSandbox(o.SettingsPath)
	switch status.State {
	case settings.ClaudeSandboxOn:
		add("sandbox", true, "on (grid setup --sandbox)")
	case settings.ClaudeSandboxOff:
		add("sandbox", true, "off; grid setup --sandbox turns on Claude Code's sandbox")
	default:
		add("sandbox", true, "unknown: "+status.Reason)
	}
}

func checkInstallation(add func(string, bool, string), o Options) {
	lookPath := o.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if _, err := lookPath("grid"); err != nil {
		add("grid on PATH", false, "grid was not found on PATH")
	} else {
		add("grid on PATH", true, "grid is on PATH")
	}
	if o.HooksFile == "" {
		add("plugin hooks", false, "cannot locate the plugin hooks file")
		return
	}
	b, err := os.ReadFile(o.HooksFile)
	if err != nil {
		add("plugin hooks", false, "cannot read the plugin hooks file")
		return
	}
	if err := ValidateHooks(b); err != nil {
		add("plugin hooks", false, err.Error())
		return
	}
	add("plugin hooks", true, "plugin hooks file is valid")
}

// ValidateHooks checks the command-hook structure without executing commands.
// Unknown fields are allowed for forward-compatible host metadata.
func ValidateHooks(b []byte) error {
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string   `json:"type"`
				Command string   `json:"command"`
				Timeout *float64 `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("plugin hooks file is too large")
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("plugin hooks file is not valid JSON or has an invalid structure")
	}
	if len(doc.Hooks) == 0 || len(doc.Hooks["SessionStart"]) == 0 {
		return fmt.Errorf("plugin hooks file has no SessionStart hook")
	}
	for event, groups := range doc.Hooks {
		if strings.TrimSpace(event) == "" || len(groups) == 0 {
			return fmt.Errorf("plugin hooks file has an empty event")
		}
		for _, group := range groups {
			if _, err := regexp.Compile(group.Matcher); err != nil {
				return fmt.Errorf("plugin hooks file has an invalid matcher")
			}
			if len(group.Hooks) == 0 {
				return fmt.Errorf("plugin hooks file has an empty hook group")
			}
			for _, h := range group.Hooks {
				if h.Type != "command" || strings.TrimSpace(h.Command) == "" {
					return fmt.Errorf("plugin hooks file needs nonempty command hooks")
				}
				if h.Timeout != nil && *h.Timeout <= 0 {
					return fmt.Errorf("plugin hooks file has an invalid timeout")
				}
			}
		}
	}
	return nil
}
