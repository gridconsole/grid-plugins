package hook

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// A guarded tool call over the input limit follows the plan-guard level: block
// denies, warn (the default) warns and lets the call through, off is silent.
func TestOverLimitDenyFollowsThePlanGuardLevel(t *testing.T) {
	huge := strings.Repeat("x", hookio.InputLimit)
	stdin := `{"hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{"file_path":"/repo/a.go","content":"` + huge + `"}}`
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, tt := range []struct {
			level               string // "" stores nothing: the catalog default, warn
			deny, warned, quiet bool
		}{
			{"", false, true, false},
			{"warn", false, true, false},
			{"off", false, false, true},
			{"block", true, false, false},
		} {
			t.Run(event+"/"+tt.level, func(t *testing.T) {
				root := t.TempDir()
				grid := filepath.Join(root, "home", ".grid")
				if tt.level != "" {
					if err := (settings.Store{Home: grid}).SetHook("plan-guard", tt.level); err != nil {
						t.Fatal(err)
					}
				}
				var out bytes.Buffer
				Run(Options{Event: event, Stdin: strings.NewReader(stdin), Stdout: &out, Cwd: root,
					Resolver: where.Resolver{GridHome: grid, Home: filepath.Join(root, "home")},
					Getenv:   func(string) string { return "" }})
				s := out.String()
				if got := strings.Contains(s, `"deny"`); got != tt.deny {
					t.Fatalf("plan-guard %q: deny=%v want %v: %.300s", tt.level, got, tt.deny, s)
				}
				if got := strings.Contains(s, "larger than"); got != (tt.deny || tt.warned) {
					t.Fatalf("plan-guard %q: reason said=%v: %.300s", tt.level, got, s)
				}
				// warn and off give no decision at all, for PermissionRequest too:
				// at most a systemMessage, never an allow or a behavior.
				if !tt.deny && (strings.Contains(s, "behavior") || strings.Contains(s, "permissionDecision")) {
					t.Fatalf("plan-guard %q: a decision was written: %.300s", tt.level, s)
				}
				if tt.quiet && s != "" {
					t.Fatalf("plan-guard off: expected silence: %.300s", s)
				}
			})
		}
	}
}
