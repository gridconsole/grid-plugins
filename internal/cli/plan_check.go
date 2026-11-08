package cli

import (
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

func runPlanCheck(text string, asJSON bool, env Env) int {
	problems := jobgraph.CheckPlan(text)
	code := 0
	if len(problems) > 0 {
		code = 1
	}
	if asJSON {
		if writeJSON(env.Stdout, struct {
			OK       bool     `json:"ok"`
			Problems []string `json:"problems"`
		}{code == 0, problems}) != 0 {
			return 1
		}
		return code
	}
	for _, problem := range problems {
		fmt.Fprintln(env.Stdout, strings.Join(strings.Fields(problem), " "))
	}
	return code
}
