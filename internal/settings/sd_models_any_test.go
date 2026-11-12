package settings

import (
	"strings"
	"testing"
)

func TestSdModelValues(t *testing.T) {
	for _, m := range []string{"haiku", "sonnet", "opus", "fable", "sonnet[1m]", "claude-opus-5-5", "claude-sonnet-4-5[1m]", "Opus", "FABLE"} {
		if err := validateAgent(Agent{Model: m}); err != nil {
			t.Errorf("%q refused: %v", m, err)
		}
	}
	for _, m := range []string{"gpt", "claude-", "claude-x[2m]", "opus --bad", "claude-Opus"} {
		err := validateAgent(Agent{Model: m})
		want := `unknown model "` + m + `"; allowed: haiku, sonnet, opus, fable, <name>[1m] or a claude- model id`
		if err == nil || err.Error() != want {
			t.Errorf("%q: got %v, want %s", m, err, want)
		}
	}
	if err := validateAgent(Agent{}); err == nil || !strings.Contains(err.Error(), "agent settings require model or effort") {
		t.Errorf("empty: %v", err)
	}
	s := Store{Home: t.TempDir()}
	for in, want := range map[string]string{"Opus": "opus", "FABLE": "fable", "Sonnet[1m]": "sonnet[1m]", "claude-opus-5-5": "claude-opus-5-5"} {
		if err := s.SetAgent("grid-builder", Agent{Model: in}); err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		d, err := s.Effective()
		if err != nil || d.Agents["grid-builder"].Model != want {
			t.Errorf("%q stored %q, want %q (%v)", in, d.Agents["grid-builder"].Model, want, err)
		}
	}
}
