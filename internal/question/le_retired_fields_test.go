package question

import "testing"

func TestLeRwQuestionBody(t *testing.T) {
	body := jobBody("Q-1", Input{Question: "Which?", Who: "dana"})
	for _, k := range []string{"review", "max_rounds"} {
		if _, ok := body[k]; ok {
			t.Errorf("jobBody writes the retired key %q", k)
		}
	}
}
