package jobgraph

import (
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// CheckPlan validates the one checklist model without resolving or executing Jobs.
func CheckPlan(text string) []string {
	problems := []string{}
	items, err := plan.Items(text)
	if err != nil {
		return []string{err.Error()}
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
			floor, err := ReviewFloor(j, Policy{})
			if err != nil {
				problems = append(problems, label+": "+err.Error())
			} else {
				for _, issue := range floor.Errors {
					problems = append(problems, label+": "+issue)
				}
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
	return problems
}
