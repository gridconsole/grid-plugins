package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// The shell runs backticks and `$` inside double quotes, so a Job body whose
// acceptance starts with a backticked `grid deliver ...` lost its command (round 16,
// J-3 of T-01). The orchestrator text, the plan-jobs skill and the --body help say
// to put the JSON body in single quotes.

func TestR16OrchestratorTextSaysSingleQuotesForTheBody(t *testing.T) {
	text := orchestratorText(t)
	step5 := ""
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "5. Post each Job with") {
			step5 = l
		}
	}
	if step5 == "" {
		t.Fatal("agents/orchestrator.md has no step 5 line \"5. Post each Job with\"")
	}
	for _, want := range []string{"single quotes", "backticks", `'`, "--body -"} {
		if !strings.Contains(step5, want) {
			t.Errorf("step 5 lacks %q", want)
		}
	}
}

func TestR16PlanJobsSkillSaysSingleQuotesForTheBody(t *testing.T) {
	skill, ok := gridplugins.SkillFor("plan-jobs")
	if !ok {
		t.Fatal("no skills/plan-jobs/SKILL.md")
	}
	for _, want := range []string{"grid job post", "single quotes", "backticks", `'`} {
		if !strings.Contains(skill, want) {
			t.Errorf("skills/plan-jobs/SKILL.md lacks %q", want)
		}
	}
}

func TestR16BodyFlagHelpSaysSingleQuotes(t *testing.T) {
	h, path := feedHarness(t)
	if code := h.run(path, "job", "post", "--help"); code != 0 {
		t.Fatalf("help: %d %s %s", code, h.out, h.errb)
	}
	help := h.errb.String() + h.out.String()
	for _, want := range []string{"single quotes", "backticks"} {
		if !strings.Contains(help, want) {
			t.Errorf("job post --help lacks %q: %s", want, help)
		}
	}
}

// A body piped in with --body - and a ' escape reaches the Job intact, so the
// orchestrator text may name that route.
func TestR16JobPostBodyFromStdinKeepsBackticksAndQuote(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = "J-2"
	body["acceptance"] = []any{map[string]any{"id": "A1", "text": "`grid deliver --task T-01 --json` exit 0, the owner's text", "traces": "item"}}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.ReplaceAll(string(data), "owner's", `owner's`)
	h.env.Stdin = strings.NewReader(raw)
	if code := h.run(path, "job", "post", "J-2", "--type", "job", "--body", "-"); code != 0 {
		t.Fatalf("job post --body -: %d %s", code, h.errb)
	}
	read, err := store.Read(feed.Filter{Type: "job", Job: "J-2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) == 0 {
		t.Fatal("J-2 not posted")
	}
	out, _ := json.Marshal(read.Records)
	if !strings.Contains(string(out), "`grid deliver --task T-01 --json` exit 0, the owner's text") {
		t.Errorf("the body lost its backticks or quote: %s", out)
	}
}
