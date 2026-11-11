package contract

import (
	"strings"
	"testing"
)

// feReceiptsSection is the "Plan approval receipts" section of contract/README.md,
// up to the next second-level heading.
func feReceiptsSection(t *testing.T) string {
	t.Helper()
	text := readPluginFile(t, "contract/README.md")
	_, rest, ok := strings.Cut(text, "\n## Plan approval receipts\n")
	if !ok {
		t.Fatal("contract/README.md has no \"## Plan approval receipts\" section")
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	return section
}

// The plan Job's result is the only approval. The per-gate receipts, --gate,
// grid preview and the workflow digest hold are gone from the code, so the
// contract must not describe them.
func TestReceiptsSectionDropsGoneBehaviour(t *testing.T) {
	section := feReceiptsSection(t)
	for _, gone := range []string{"--gate", "grid preview", "workflow_changed", "workflow_digest", "Deliver blocked", "preview gate", "ship gate", "what: preview"} {
		if strings.Contains(section, gone) {
			t.Errorf("Plan approval receipts section still names %q, which no code has", gone)
		}
	}
	flat := strings.Join(strings.Fields(section), " ")
	for _, receipt := range []string{"approval-preview.json", "approval-ship.json"} {
		if strings.Contains(flat, receipt) && !strings.Contains(flat, "no longer written or read") {
			t.Errorf("section names %s without saying it is no longer written", receipt)
		}
	}
}

func TestReceiptsSectionStatesWhatHolds(t *testing.T) {
	section := feReceiptsSection(t)
	flat := strings.Join(strings.Fields(section), " ")
	for _, want := range []string{
		"plan Job's accepted result is the only approval",
		"not a fallback",
		"warns by default",
		"denies only at level `block`",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("Plan approval receipts section lacks %q", want)
		}
	}
	if strings.Contains(flat, "denies by default") {
		t.Error("section says the host guard denies by default; it warns by default")
	}
}

func TestReceiptsErrorTableDropsWorkflowChanged(t *testing.T) {
	text := readPluginFile(t, "contract/README.md")
	if strings.Contains(text, "`workflow_changed`") {
		t.Error("contract/README.md still lists the workflow_changed error code, which no code returns")
	}
}
