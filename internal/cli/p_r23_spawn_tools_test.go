package cli

import "testing"

// A role whose only definition is its template (scout, researcher, ...) gets its
// Instructions from the template, but no tool list: the template's `tools:` is that of
// an interactive subagent (grid-scout has no Bash), and a spawn that took it would leave
// the worker unable to run `grid job result`.
func TestTemplateOnlyRoleSpawnsWithoutTools(t *testing.T) {
	h, _ := goUnsetFixture(t)
	for _, role := range []string{"scout", "researcher", "designer", "operator", "runner", "library"} {
		beWantTools(t, role, beSpawnTools(t, h, role), nil)
	}
}
