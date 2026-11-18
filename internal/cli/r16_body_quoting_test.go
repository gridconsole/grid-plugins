package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// The shell runs backticks and `$` inside double quotes, so a Job body whose
// acceptance starts with a backticked `grid deliver ...` lost its command (round 16,
// J-3 of T-01). The orchestrator text, the plan-jobs skill and the --body help say
// to put the JSON body in single quotes.

// r16Escape is the six characters backslash, u, 0, 0, 2, 7: the JSON escape for an
// apostrophe. It is a raw string on purpose; a literal apostrophe in the prose ends
// the agent's own single-quoted body.
const r16Escape = `\u0027`

func TestR16EscapeIsSixCharacters(t *testing.T) {
	if len(r16Escape) != 6 || r16Escape[0] != '\\' || strings.Contains(r16Escape, "'") {
		t.Fatalf("r16Escape is %q, want backslash u 0 0 2 7", r16Escape)
	}
}

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
	for _, want := range []string{"single quotes", "backticks", r16Escape, "--body-file"} {
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
	for _, want := range []string{"grid job post", "single quotes", "backticks", r16Escape} {
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

// A body piped in with --body - and a ' escape reaches the Job intact.
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
	raw := strings.ReplaceAll(string(data), "owner's", `owner`+r16Escape+`s`)
	if !strings.Contains(raw, `owner\u0027s`) {
		t.Fatalf("the raw body holds no escape: %s", raw)
	}
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
	if strings.Contains(string(out), "u0027") {
		t.Errorf("the escape was stored, not decoded: %s", out)
	}
}

// The shell line an agent writes: the JSON body in single quotes, the apostrophe as
// the six-character escape. bash exits 0 and hands the body on untouched; a literal
// apostrophe in its place ends the quoted body and bash fails.
func TestR16SingleQuotedBodyWithEscapeSurvivesTheShellAndPosts(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on this machine")
	}
	h, path, store := orchestratorHarness(t)
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = "J-3"
	body["acceptance"] = []any{map[string]any{"id": "A1", "text": "`grid deliver --task T-01 --json` exit 0, the owner's text", "traces": "item"}}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	escaped := strings.ReplaceAll(string(data), "owner's", `owner`+r16Escape+`s`)

	shell := func(body string) (string, error) {
		cmd := exec.Command(bash, "-c", "printf '%s' '"+body+"'")
		out, err := cmd.Output()
		return string(out), err
	}
	piped, err := shell(escaped)
	if err != nil {
		t.Fatalf("bash with the escape: %v", err)
	}
	if piped != escaped {
		t.Fatalf("bash changed the body:\n got %s\nwant %s", piped, escaped)
	}
	if _, err := shell(string(data)); err == nil {
		t.Errorf("bash accepted a literal apostrophe inside the single-quoted body; the control proves nothing")
	}

	h.env.Stdin = strings.NewReader(piped)
	if code := h.run(path, "job", "post", "J-3", "--type", "job", "--body", "-"); code != 0 {
		t.Fatalf("job post --body -: %d %s", code, h.errb)
	}
	read, err := store.Read(feed.Filter{Type: "job", Job: "J-3"})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(read.Records)
	if !strings.Contains(string(out), "the owner's text") {
		t.Errorf("the posted text lost the apostrophe: %s", out)
	}
}
