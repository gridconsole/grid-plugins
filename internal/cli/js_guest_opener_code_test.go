package cli

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// jsGoCase is one refused write: its argv without --json, the stderr it prints
// today and the envelope code, guest_refused when empty.
type jsGoCase struct {
	name   string
	args   []string
	stderr string
	code   string
}

// jsGoRefusal is the settings refusal line under the prefix the command prints.
func jsGoRefusal(prefix string) string { return prefix + ": " + guestSettingsRefusal + "\n" }

// jsGoRefused runs tc plain and with --json: exit 1 both times, empty plain stdout,
// the same stderr as today in both modes, the guest_refused envelope with --json,
// and the HOME and GRID_HOME trees unchanged.
func jsGoRefused(t *testing.T, h *harness, cwd string, tc jsGoCase) {
	t.Helper()
	before, homeBefore := bypassTree(t, h.gridHome), bypassTree(t, h.home)
	code := h.run(cwd, tc.args...)
	plainOut, plainErr := h.out.String(), h.errb.String()
	jsonCode := h.run(cwd, append(slices.Clone(tc.args), "--json")...)
	if code != 1 || jsonCode != 1 {
		t.Fatalf("exit plain=%d json=%d, want 1: %s", code, jsonCode, h.errb)
	}
	if plainOut != "" || plainErr != tc.stderr || h.errb.String() != tc.stderr {
		t.Fatalf("plain stdout %q, plain stderr %q, json stderr %q; want empty stdout and stderr %q", plainOut, plainErr, h.errb, tc.stderr)
	}
	want := tc.code
	if want == "" {
		want = CodeGuestRefused
	}
	assertErrorOutput(t, h.out.String(), h.errb.String(), want)
	if !reflect.DeepEqual(before, bypassTree(t, h.gridHome)) || !reflect.DeepEqual(homeBefore, bypassTree(t, h.home)) {
		t.Fatal("HOME or GRID_HOME tree changed after a refused write")
	}
}

// jsGoWrites runs args with --json for a member and wants exit 0 and a change
// under root, so the unchanged trees of the refused runs are not vacuous.
func jsGoWrites(t *testing.T, args []string, root func(*harness) string) {
	t.Helper()
	h, cwd := guestLeakFixture(t, "member")
	before := bypassTree(t, root(h))
	if code := h.run(cwd, append(slices.Clone(args), "--json")...); code != 0 || strings.Contains(h.out.String(), CodeGuestRefused) {
		t.Fatalf("member %v: exit %d, stdout %q, stderr %q; want exit 0", args, code, h.out, h.errb)
	}
	if reflect.DeepEqual(before, bypassTree(t, root(h))) {
		t.Fatalf("member %v wrote nothing", args)
	}
}

// The refusing callers of settingsStore: the runSettings kinds (settings.go:104),
// grid setup <project> (settings.go:216) and agent budget apply (budget.go:59).
func TestJsGoOpenerCallers(t *testing.T) {
	ilGuestNoLease(t)
	for _, role := range []string{"guest", "external"} {
		// Why: grid setup <project> never reaches the opener for these roles; role
		// guest is refused at the dispatch gate (cli.go:198), external at runSetup.
		setupPrefix := "setup"
		if role == "guest" {
			setupPrefix = "grid"
		}
		for _, tc := range []jsGoCase{
			{"agent_set", []string{"agent", "set", "grid-builder", "--model", "opus", "--project", "alpha"}, jsGoRefusal("agent"), ""},
			{"agent_unset", []string{"agent", "unset", "grid-builder", "--project", "alpha"}, jsGoRefusal("agent"), ""},
			{"skill_enable", []string{"skill", "enable", "research", "--project", "alpha"}, jsGoRefusal("skill"), ""},
			{"skill_disable", []string{"skill", "disable", "research", "--project", "alpha"}, jsGoRefusal("skill"), ""},
			{"hook_level", []string{"hook", "level", "home-allow", "off", "--project", "alpha"}, jsGoRefusal("hook"), ""},
			{"setup_project", []string{"setup", "alpha", "--answers", "-"}, jsGoRefusal(setupPrefix), ""},
			// Why: budget apply opens the Personal store, which a guest cannot
			// resolve, so the opener answers no_such_project before its guest check.
			{"budget_apply", []string{"agent", "budget", "apply", "balanced"}, "agent: project \"personal\" is not registered\n", CodeNoSuchProject},
		} {
			t.Run(role+"/"+tc.name, func(t *testing.T) {
				h, cwd := guestLeakFixture(t, role)
				jsGoRefused(t, h, cwd, tc)
			})
		}
	}
}

func TestJsGoClaudeSet(t *testing.T) {
	ilGuestNoLease(t)
	args := []string{"claude", "settings", "set", "autoCompactEnabled", "true"}
	for _, role := range []string{"guest", "external"} {
		t.Run(role, func(t *testing.T) {
			h, cwd := guestLeakFixture(t, role)
			jsGoRefused(t, h, cwd, jsGoCase{"claude_set", args, jsGoRefusal("claude"), ""})
		})
	}
	t.Run("member_writes", func(t *testing.T) { jsGoWrites(t, args, func(h *harness) string { return h.home }) })
}

func TestJsGoProjectRepo(t *testing.T) {
	ilGuestNoLease(t)
	for _, role := range []string{"guest", "external"} {
		for _, args := range [][]string{
			{"add", "alpha", "--where", "/w/x"},
			{"edit", "alpha", "--id", "x", "--branch", "b"},
			{"remove", "alpha", "--id", "x"},
		} {
			t.Run(role+"/"+args[0], func(t *testing.T) {
				h, cwd := guestLeakFixture(t, role)
				jsGoRefused(t, h, cwd, jsGoCase{args[0], append([]string{"project", "repo"}, args...), jsGoRefusal("grid"), ""})
			})
		}
	}
	t.Run("member_writes", func(t *testing.T) {
		jsGoWrites(t, []string{"project", "repo", "add", "alpha", "--where", "/w/x"}, func(h *harness) string { return h.gridHome })
	})
}
