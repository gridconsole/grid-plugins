package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

func runPlanCheck(text string, asJSON bool, env Env) int {
	report := jobgraph.CheckPlanReport(text)
	problems := report.Problems
	for i, problem := range problems {
		problems[i] = planFormatError(text, errors.New(problem)).Error()
	}
	code := 0
	if len(problems) > 0 {
		code = 1
	}
	if asJSON {
		if writeJSON(env.Stdout, struct {
			OK       bool     `json:"ok"`
			Problems []string `json:"problems"`
			Warnings []string `json:"warnings"`
		}{code == 0, problems, report.Warnings}) != 0 {
			return 1
		}
		return code
	}
	for _, problem := range problems {
		fmt.Fprintln(env.Stdout, strings.Join(strings.Fields(problem), " "))
	}
	for _, warning := range report.Warnings {
		fmt.Fprintln(env.Stdout, "warning: "+strings.Join(strings.Fields(warning), " "))
	}
	return code
}
