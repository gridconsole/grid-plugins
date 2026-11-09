package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

func (h *harness) put(t *testing.T, rel, data string) {
	t.Helper()
	p := filepath.Join(h.env.GridHome, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCheckExitCodes(t *testing.T) {
	h := newHarness(t)
	h.propose(t)
	if code := h.run("check", "--level", "home"); code != 0 || !strings.Contains(h.out.String(), "findings: 0") {
		t.Fatalf("clean: %d %s %s", code, &h.out, &h.err)
	}
	if code := h.run("check", "--level", "home", "--json"); code != 0 || !strings.Contains(h.out.String(), `"findings":[]`) {
		t.Fatalf("clean json: %d %s", code, &h.out)
	}
	h.put(t, "knowledge/skills/demo/SKILL.md", "---\nname: demo\ndescription: d\nallowed-tools: Bash\n---\n!`id`\n")
	if code := h.run("check", "--level", "home"); code != 3 {
		t.Fatalf("findings: %d %s %s", code, &h.out, &h.err)
	}
	out := h.out.String()
	if !strings.Contains(out, "skill_field brain:home/knowledge/skills/demo/SKILL.md:4") || !strings.Contains(out, "```\nallowed-tools\n```") || !strings.Contains(out, "```\n!`id`\n```") {
		t.Fatalf("plain output:\n%s", out)
	}
	if code := h.run("check", "--level", "home", "--json"); code != 3 {
		t.Fatalf("json findings: %d", code)
	}
	var res struct {
		Checked  int `json:"checked"`
		Findings []struct {
			Code string `json:"code"`
			Line int    `json:"line"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &res); err != nil || res.Checked != 2 || len(res.Findings) != 2 || res.Findings[0].Code != "skill_field" {
		t.Fatalf("json: %v %+v %s", err, res, &h.out)
	}
}

func TestCheckProposalByID(t *testing.T) {
	h := newHarness(t)
	h.put(t, "brain-proposals/dep.md", "---\nname: dep\ntopic: deploy\ntype: reference\ndescription: d\nsummary: s\nupdated: \"\"\nsource: T-01\nsuperseded_by: \"\"\nlinks: []\nlevel: home\nstatus: proposed\nreason: \"\"\norigin:\n  kind: session\n  id: x\n  tier: agent\n---\nBody\n")
	if code := h.run("check", "brain:home/dep"); code != 3 || !strings.Contains(h.out.String(), "config: run grid deploy") {
		t.Fatalf("%d %s %s", code, &h.out, &h.err)
	}
	if code := h.run("check", "--level", "home", "dep"); code != 3 {
		t.Fatalf("%d", code)
	}
}

func TestCheckFencesQuotedTextAndUsage(t *testing.T) {
	h := newHarness(t)
	h.put(t, "knowledge/skills/demo/SKILL.md", "---\nname: demo\ndescription: d\n---\n!`echo ```` hi`\n")
	if code := h.run("check", "--level", "home"); code != 3 || !strings.Contains(h.out.String(), "`````\n!`echo ```` hi`\n`````") {
		t.Fatalf("%d\n%s", code, &h.out)
	}
	if code := h.run("check", "a", "b"); code != 2 {
		t.Fatalf("two ids: %d", code)
	}
	if code := h.run("check", "--bogus"); code != 2 {
		t.Fatalf("bad flag: %d", code)
	}
	h.put(t, "brain-proposals/bad.md", "nope\n")
	if code := h.run("check", "--level", "home"); code != 1 {
		t.Fatalf("unreadable: %d %s", code, &h.err)
	}
}

// failMark makes the proposal check report a finding for a proposal whose
// bytes hold the mark, on top of the real check.
func failMark(t *testing.T, mark string) {
	t.Helper()
	real := brain.ProposalCheck
	brain.ProposalCheck = func(level, name string, n brain.Note, b []byte, res *brain.CheckResult) error {
		if err := real(level, name, n, b, res); err != nil {
			return err
		}
		if strings.Contains(string(b), mark) {
			res.Findings = append(res.Findings, brain.Finding{Code: "test_finding", File: "brain:" + level + "/brain-proposals/" + name + ".md", Line: 7, Message: "marked to fail", Text: mark})
		}
		return nil
	}
	t.Cleanup(func() { brain.ProposalCheck = real })
}

func TestAcceptRefusedByCheckExitsThree(t *testing.T) {
	failMark(t, "FAILMARK")
	h := newHarness(t)
	h.propose(t, "--body", "FAILMARK\n")
	entry := filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md")

	if code := h.run("accept", "checks", "--level", "home", "--json"); code != 3 {
		t.Fatalf("json refusal: %d %s %s", code, &h.out, &h.err)
	}
	if h.out.Len() != 0 {
		t.Fatalf("stdout should be empty: %s", &h.out)
	}
	var got struct {
		Error    string `json:"error"`
		Code     string `json:"code"`
		Findings []struct {
			Code string `json:"code"`
			File string `json:"file"`
			Line int    `json:"line"`
			Text string `json:"text"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(h.err.Bytes(), &got); err != nil || got.Error != "check_failed" || got.Code != "check_failed" || len(got.Findings) != 1 || got.Findings[0].Code != "test_finding" || got.Findings[0].Text != "FAILMARK" {
		t.Fatalf("json: %v %+v %s", err, got, &h.err)
	}

	if code := h.run("accept", "checks", "--level", "home"); code != 3 {
		t.Fatalf("plain refusal: %d %s", code, &h.err)
	}
	if e := h.err.String(); !strings.Contains(e, "findings: 1") || !strings.Contains(e, "test_finding brain:home/brain-proposals/checks.md:7: marked to fail") || !strings.Contains(e, "```\nFAILMARK\n```") || h.out.Len() != 0 {
		t.Fatalf("plain stderr:\n%s", e)
	}
	if _, err := os.Stat(entry); err == nil {
		t.Fatal("the refused accept published the entry")
	}

	// Reject of the same proposal is never gated.
	if code := h.run("accept", "checks", "--level", "home", "--reject", "Not wanted."); code != 0 {
		t.Fatalf("reject: %d %s", code, &h.err)
	}
}

func TestAcceptCleanProposalStillExitsZero(t *testing.T) {
	failMark(t, "FAILMARK")
	h := newHarness(t)
	h.propose(t, "--body", "accept this as a workspace skill\n")
	if code := h.run("accept", "checks", "--level", "home"); code != 0 {
		t.Fatalf("clean accept: %d %s", code, &h.err)
	}
}

// A hand edit of a published entry is an audit_hash finding, exit 3.
func TestCheckReportsEntryEditedOutsideTheKeeper(t *testing.T) {
	h := newHarness(t)
	h.propose(t)
	if code := h.run("accept", "checks", "--level", "home"); code != 0 {
		t.Fatalf("accept: %d %s", code, &h.err)
	}
	if code := h.run("check", "--level", "home"); code != 0 {
		t.Fatalf("clean check: %d %s", code, &h.out)
	}
	entry := filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md")
	b, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, append(b, "Edited by hand.\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := h.run("check", "--level", "home"); code != 3 || !strings.Contains(h.out.String(), "audit_hash brain:home/knowledge/decisions/checks.md: entry changed outside the keeper: v1 was sha256:") {
		t.Fatalf("drift: %d %s %s", code, &h.out, &h.err)
	}
}
