package jobgraph

import (
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"strings"
)

// CheckPlan validates the one checklist model without resolving or executing Jobs.
func CheckPlan(text string) []string {
	return CheckPlanReport(text).Problems
}

// PlanReport separates invalid instructions from advisory verification warnings.
type PlanReport struct {
	Problems []string `json:"problems"`
	Warnings []string `json:"warnings"`
}

// CheckPlanReport checks the same plan as CheckPlan and flags common commands
// whose default output cannot establish a positive verification test count.
func CheckPlanReport(text string) PlanReport {
	report := PlanReport{Problems: []string{}, Warnings: []string{}}
	problems := []string{}
	items, err := plan.Items(text)
	if err != nil {
		report.Problems = []string{err.Error()}
		return report
	}
	jobs := []job.Job{}
	for _, item := range items {
		for index, raw := range item.Jobs {
			j, issues := job.DecodePlanned(raw, item.Kind)
			label := fmt.Sprintf("item %d Job %d", item.N, index+1)
			if j.ID != "" {
				label = j.ID
			}
			for _, issue := range issues {
				problems = append(problems, label+": "+issue)
			}
			if err := CheckPlanJob(j); err != nil {
				problems = append(problems, label+": "+err.Error())
			}
			if warning := verifyCountWarning(j); warning != "" {
				report.Warnings = append(report.Warnings, label+": "+warning)
			}
			jobs = append(jobs, j)
		}
	}
	graph, err := New(jobs)
	if err != nil {
		problems = append(problems, err.Error())
	} else if cycle := graph.Cycle(); len(cycle) > 0 {
		problems = append(problems, (&CycleError{Path: cycle}).Error())
	}
	report.Problems = problems
	report.Warnings = append(report.Warnings, nqNeedsWarnings(items)...)
	return report
}

// nqNeedsWarnings checks each step's prose `needs:` line. None of these is a
// problem. A missing line is reported only when some step of the plan has one.
func nqNeedsWarnings(items []plan.Item) []string {
	type step struct {
		needs  []int
		value  string
		found  bool
		parsed bool
	}
	steps := map[int]step{}
	anyLine := false
	for _, item := range items {
		needs, value, found := plan.NqNeedsRead(item.Prose)
		steps[item.N] = step{needs: needs, value: value, found: found, parsed: needs != nil}
		anyLine = anyLine || found
	}
	warnings := []string{}
	for _, item := range items {
		s := steps[item.N]
		switch {
		case !s.found:
			if anyLine {
				warnings = append(warnings, fmt.Sprintf("item %d: no needs: line", item.N))
			}
		case !s.parsed:
			warnings = append(warnings, fmt.Sprintf("item %d: needs: %s is not step numbers or none", item.N, s.value))
		}
		for _, n := range s.needs {
			if n == item.N {
				warnings = append(warnings, fmt.Sprintf("item %d: needs itself", item.N))
			} else if _, ok := steps[n]; !ok {
				warnings = append(warnings, fmt.Sprintf("item %d: needs step %d, which is not in the plan", item.N, n))
			}
		}
	}
	// Cycles over the edges that name another step of the plan.
	const (
		onPath = 1
		done   = 2
	)
	mark := map[int]int{}
	path := []int{}
	var visit func(n int)
	visit = func(n int) {
		mark[n] = onPath
		path = append(path, n)
		for _, m := range steps[n].needs {
			if _, ok := steps[m]; !ok || m == n {
				continue
			}
			switch mark[m] {
			case onPath:
				from := 0
				for path[from] != m {
					from++
				}
				parts := []string{}
				for _, p := range append(append([]int{}, path[from:]...), m) {
					parts = append(parts, fmt.Sprint(p))
				}
				warnings = append(warnings, fmt.Sprintf("item %d: needs cycle %s", m, strings.Join(parts, " -> ")))
			case 0:
				visit(m)
			}
		}
		path = path[:len(path)-1]
		mark[n] = done
	}
	for _, item := range items {
		if mark[item.N] == 0 {
			visit(item.N)
		}
	}
	return warnings
}

func verifyCountWarning(j job.Job) string {
	if j.Kind == "non-code" || j.Kind == "research" || j.Kind == "scout" {
		return ""
	}
	// This is a warning for common shell commands, not a shell parser or an
	// inspection of arbitrary Makefiles. Check each command independently.
	for _, part := range strings.FieldsFunc(j.Verify.Command, func(r rune) bool {
		return r == ';' || r == '&' || r == '|' || r == '\n'
	}) {
		words := strings.Fields(part)
		for len(words) > 0 && (strings.Contains(words[0], "=") || words[0] == "env") {
			words = words[1:]
		}
		if len(words) < 2 || (words[0] != "go" && words[0] != "make") || words[1] != "test" {
			continue
		}
		counts := false
		for _, word := range words[2:] {
			// GOFLAGS and Make variables often carry the test output flags.
			if _, value, ok := strings.Cut(word, "="); ok && !strings.HasPrefix(word, "-") {
				word = value
			}
			word = strings.Trim(word, `"'`)
			if word == "-v" || word == "-json" || word == "-v=true" || word == "-json=true" {
				counts = true
			}
		}
		if !counts {
			return "verify test totals may be unknown: use go test -v or -json, or ensure make test emits verbose/JSON test summaries"
		}
	}
	return ""
}
