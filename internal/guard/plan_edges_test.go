package guard

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

func TestRecallLineForms(t *testing.T) {
	for _, tt := range []struct {
		text string
		want bool
	}{
		{"Recall: nothing found", true}, {" Used: note", true}, {"- **Used:** note", true}, {"__Recall__: nothing found", true}, {"**Recall:** nothing found", true}, {"recall: Nothing Found", true},
		{"Recall: none", false}, {"    Used: note", false}, {"```\nUsed: note\n```", false}, {"<!-- Used: note -->", false},
		{"Recall:", false}, {"Recall: **", false}, {"prefix Recall: none", false}, {"Used: ___", false},
	} {
		if got := plan.HasRecall(tt.text); got != tt.want {
			t.Errorf("%q: %v", tt.text, got)
		}
	}
}
