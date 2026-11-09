package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
