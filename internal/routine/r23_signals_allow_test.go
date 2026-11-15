package routine

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
)

// r23AllowedBashPrefixes returns the command prefixes of the Bash(<prefix>:*)
// rules in the --allowedTools value of a routine's claude command.
func r23AllowedBashPrefixes(t *testing.T, args []string) []string {
	t.Helper()
	for i, a := range args {
		if a != "--allowedTools" || i+1 >= len(args) {
			continue
		}
		var out []string
		for _, rule := range strings.Split(args[i+1], ",") {
			if p, ok := strings.CutPrefix(rule, "Bash("); ok {
				if p, ok = strings.CutSuffix(p, ":*)"); ok {
					out = append(out, p)
				}
			}
		}
		return out
	}
	t.Fatalf("no --allowedTools in %q", args)
	return nil
}

// A routine runs claude with --permission-mode dontAsk, and a Bash allow rule
// matches the start of the command. Every `grid signal` command the signals
// skill gives must therefore start with an allowed prefix and be one command:
// a leading printf or cd, or a pipe or && , is refused and no signal is saved.
// The routine already runs in the Grid home, so the Personal form needs no cd.
func TestR23SignalsSkillCommandsPassTheRoutineAllowList(t *testing.T) {
	var args []string
	var dir string
	p := ProcessExecutor{runCommand: func(cmd *exec.Cmd) error {
		args, dir = cmd.Args, cmd.Dir
		return nil
	}}
	home := t.TempDir()
	if err := p.Execute(context.Background(), Store{GridHome: home}, Entry{Run: "signals"}); err != nil {
		t.Fatal(err)
	}
	if dir != home {
		t.Fatalf("the routine runs in %q, want the Grid home %q", dir, home)
	}
	prefixes := r23AllowedBashPrefixes(t, args)

	text, ok := gridplugins.SkillFor("signals")
	if !ok {
		t.Fatal("the plugin has no signals skill")
	}
	placeholder := regexp.MustCompile(`<[^<>]*>`)
	quoted := regexp.MustCompile(`"[^"]*"|'[^']*'`)
	seen := 0
	for n, line := range strings.Split(text, "\n") {
		for i, span := range strings.Split(line, "`") {
			if i%2 == 0 || !strings.Contains(span, "grid signal ") {
				continue
			}
			seen++
			allowed := false
			for _, prefix := range prefixes {
				if strings.HasPrefix(span, prefix+" ") {
					allowed = true
				}
			}
			if !allowed {
				t.Errorf("SKILL.md:%d: `%s` does not start with an allowed prefix %q, so dontAsk refuses it", n+1, span, prefixes)
			}
			bare := quoted.ReplaceAllString(placeholder.ReplaceAllString(span, ""), "")
			for _, bad := range []string{"|", "&&", ";", "printf", "cd "} {
				if strings.Contains(bare, bad) {
					t.Errorf("SKILL.md:%d: `%s` holds %q, which is not one allowed command", n+1, span, bad)
				}
			}
		}
	}
	if seen < 4 {
		t.Fatalf("only %d grid signal commands in the skill", seen)
	}
}
