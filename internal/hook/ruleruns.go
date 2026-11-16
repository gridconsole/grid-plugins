package hook

import (
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/hookevents"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// ruleRuns collects, for one Run, the rules that were consulted and what each
// one came to. A rule asked at several places in one run (history-log is) leaves
// one line: the last outcome noted for it. The zero value and nil are ready.
type ruleRuns struct {
	order   []string
	outcome map[string]string
}

func (r *ruleRuns) note(rule, outcome string) {
	if r == nil {
		return
	}
	if r.outcome == nil {
		r.outcome = map[string]string{}
	}
	if _, seen := r.outcome[rule]; !seen {
		r.order = append(r.order, rule)
	}
	r.outcome[rule] = outcome
}

// note records what a rule came to in this run. A call from a test that built
// Options without Run does nothing.
func (o Options) note(rule, outcome string) { o.runs.note(rule, outcome) }

// levelOutcome is what a rule whose level was just read has come to before it
// does anything: skipped at off, ok otherwise.
func levelOutcome(level string) string {
	if level == "off" {
		return hookevents.Skipped
	}
	return hookevents.OK
}

// decisionOutcome reads a guard's decision: a refusal is blocked, a warning or a
// question is warned, anything else is ok.
func decisionOutcome(d hookio.Decision) string {
	switch {
	case d.Behavior == "deny":
		return hookevents.Blocked
	case d.Behavior == "ask" || len(d.Warnings) > 0:
		return hookevents.Warned
	}
	return hookevents.OK
}

// flushRuns appends one line per rule this run consulted. It is the last thing a
// run does, writes nothing to Stdout or Stderr, and ignores every failure, so the
// hook's exit code and output are what they were without it.
func (o Options) flushRuns() {
	if o.runs == nil || len(o.runs.order) == 0 || o.Resolver.GridHome == "" {
		return
	}
	event := o.Event
	for _, e := range events {
		if strings.EqualFold(event, e.Name) {
			event = e.Name
		}
	}
	at := time.Now().UTC().Format(time.RFC3339)
	recs := make([]hookevents.Record, 0, len(o.runs.order))
	for _, rule := range o.runs.order {
		recs = append(recs, hookevents.Record{Rule: rule, Event: event, At: at, Outcome: o.runs.outcome[rule]})
	}
	hookevents.Append(o.Resolver.GridHome, recs)
}
