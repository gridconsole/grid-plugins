package contract

import (
	"strings"
	"testing"
)

// The deliver skill and the curator agent say what `grid deliver` does on main:
// it raises no gate, it moves the task to waiting at step ship, and it posts the
// curator Job itself when brain proposals wait.

func TestSkillDeliverTextHasNoOldPhrases(t *testing.T) {
	text := readPluginFile(t, "skills/deliver/SKILL.md")
	for _, old := range []string{
		"ship gate", "raises the gate", "gate: ship", "an unapproved plan", "If no curator is available",
	} {
		if strings.Contains(text, old) {
			t.Errorf("skills/deliver/SKILL.md still contains %q", old)
		}
	}
}

func TestSkillDeliverTextNamesStepShip(t *testing.T) {
	text := readPluginFile(t, "skills/deliver/SKILL.md")
	if !strings.Contains(text, "step: ship") {
		t.Error("skills/deliver/SKILL.md does not name \"step: ship\"")
	}
}

func TestSkillDeliverTextDryRunNamesProof(t *testing.T) {
	text := readPluginFile(t, "skills/deliver/SKILL.md")
	found := false
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "--dry-run") {
			found = true
			if !strings.Contains(line, "proof") {
				t.Errorf("the dry-run line does not name the real refusal, missing proof: %q", line)
			}
		}
	}
	if !found {
		t.Error("skills/deliver/SKILL.md has no dry-run line")
	}
}

func TestSkillDeliverTextHasNoGateWord(t *testing.T) {
	for _, rel := range []string{"skills/deliver/SKILL.md", "agents/curator.md"} {
		if strings.Contains(strings.ToLower(readPluginFile(t, rel)), "gate") {
			t.Errorf("%s contains the word \"gate\"", rel)
		}
	}
}
