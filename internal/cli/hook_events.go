package cli

import (
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/hookevents"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// hookRuleEvent is one row of grid hook events --json: the last time a rule was
// consulted by a hook run. A rule with no run has null fields.
type hookRuleEvent struct {
	Rule      string  `json:"rule"`
	LastEvent *string `json:"lastEvent"`
	At        *string `json:"at"`
	Outcome   *string `json:"outcome"`
}

// runHookEvents answers grid hook events [--all] [--json]. It reads the log the
// hooks append to (internal/hookevents) and changes nothing. The log is Grid-home
// wide: a line names no project.
func runHookEvents(args []string, env Env) int {
	fs := newFlags("hook events", env)
	asJSON := fs.Bool("json", false, "print JSON")
	all := fs.Bool("all", false, "list the whole log, newest first")
	operands, code, stop := parseMixed(fs, args)
	if stop {
		return code
	}
	if len(operands) != 0 {
		fmt.Fprintln(env.Stderr, "hook events: expected 0 operands")
		return 2
	}
	log, err := hookevents.Read(env.GridHome)
	if err != nil {
		return commandError(env, "hook", err)
	}
	if *all {
		if log == nil {
			log = []hookevents.Record{}
		}
		if *asJSON {
			return writeJSON(env.Stdout, log)
		}
		for _, r := range log {
			fmt.Fprintf(env.Stdout, "%s %s %s %s\n", r.At, r.Rule, r.Event, r.Outcome)
		}
		return 0
	}
	last := hookevents.Last(log)
	rows := []hookRuleEvent{}
	for _, name := range settings.HookNames() {
		row := hookRuleEvent{Rule: name}
		if r, ok := last[name]; ok {
			row.LastEvent, row.At, row.Outcome = &r.Event, &r.At, &r.Outcome
		}
		rows = append(rows, row)
	}
	if *asJSON {
		return writeJSON(env.Stdout, rows)
	}
	dash := func(s *string) string {
		if s == nil {
			return "-"
		}
		return *s
	}
	for _, row := range rows {
		fmt.Fprintf(env.Stdout, "%s %s %s %s\n", row.Rule, dash(row.LastEvent), dash(row.At), dash(row.Outcome))
	}
	return 0
}
