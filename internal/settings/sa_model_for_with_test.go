package settings

import "testing"

func saAnswer(t *testing.T, st Store, role string, brain Agent, model, effort string) {
	t.Helper()
	m, e, err := st.ModelForWith(role, brain)
	if err != nil || m != model || e != effort {
		t.Fatalf("ModelForWith(%q, %+v) = %q/%q, %v; want %q/%q", role, brain, m, e, err, model, effort)
	}
}

func TestSaModelForWith(t *testing.T) {
	personal, proj := fixture(t)
	brain := Agent{Model: "fable", Effort: "high"}
	// Nothing set: the brain's values sit over the catalog's (opus/medium).
	saAnswer(t, personal, "researcher", brain, "fable", "high")
	saAnswer(t, proj, "researcher", brain, "fable", "high")
	// A personal model beats the brain's; the brain's effort stays.
	must(t, personal.SetAgent("grid-researcher", Agent{Model: "sonnet"}))
	saAnswer(t, personal, "researcher", brain, "sonnet", "high")
	saAnswer(t, proj, "researcher", brain, "sonnet", "high")
	// A project value beats both.
	must(t, proj.SetAgent("grid-researcher", Agent{Model: "haiku", Effort: "low"}))
	saAnswer(t, proj, "researcher", brain, "haiku", "low")
	saAnswer(t, personal, "researcher", brain, "sonnet", "high")
}

func TestSaModelForWithLayers(t *testing.T) {
	personal, proj := fixture(t)
	// A personal value equal to the catalog's is still the personal value.
	must(t, personal.SetAgent("grid-scout", Agent{Model: "haiku"}))
	saAnswer(t, personal, "scout", Agent{Model: "fable", Effort: "high"}, "haiku", "high")
	// An empty brain field inherits.
	saAnswer(t, personal, "researcher", Agent{Effort: "max"}, "opus", "max")
	saAnswer(t, personal, "researcher", Agent{Model: "fable"}, "fable", "medium")
	// The orchestrator's agent is grid-planner.
	saAnswer(t, personal, "orchestrator", Agent{Model: "fable"}, "fable", "medium")
	// A role with no catalog agent answers the brain's values, and reads no settings file.
	saAnswer(t, personal, "person", Agent{Model: "fable", Effort: "high"}, "fable", "high")
	saAnswer(t, personal, "person", Agent{}, "", "")
	// The brain's values need no catalog check: they are passed through as written.
	saAnswer(t, proj, "researcher", Agent{Model: "claude-fable-5-2", Effort: "xhigh"}, "claude-fable-5-2", "xhigh")
}

// With an empty brain Agent every answer is ModelFor's.
func TestSaModelForWithEmptyBrainIsModelFor(t *testing.T) {
	personal, proj := fixture(t)
	must(t, personal.SetAgent("grid-builder", Agent{Model: "opus"}))
	must(t, proj.SetAgent("grid-reviewer", Agent{Effort: "high"}))
	for _, st := range []Store{personal, proj} {
		for _, role := range []string{"builder", "grid-builder", "Builder", "checker", "researcher", "reviewer", "curator", "orchestrator", "grid-orchestrator", "scout", "person", "", "no-such-role", "grid-"} {
			wm, we, werr := st.ModelFor(role)
			saAnswer(t, st, role, Agent{}, wm, we)
			if werr != nil {
				t.Fatalf("ModelFor(%q): %v", role, werr)
			}
		}
	}
	saAnswer(t, personal, "builder", Agent{}, "opus", "medium")
	saAnswer(t, proj, "reviewer", Agent{}, "opus", "high")
}
