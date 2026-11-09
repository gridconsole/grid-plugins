package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const foldInlineHead = "---\nid: T-01\ntitle: Inline\nproject: alpha\nsize: normal\n"
const foldInlineTail = "state: queued\ncreated: 2026-11-01T09:00:00Z\n---\n"

func foldInlineRead(t *testing.T, text string) (Task, error) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "T-01-inline")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"task.md": text, "intent.md": IntentHead + "Keep it.\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return Read(dir)
}

func TestFoldInlineWorkflowList(t *testing.T) {
	text := foldInlineHead + "workflow:\n  - {step: plan, kind: approve}\n  - {step: build, kind: run}\n" + foldInlineTail
	got, err := foldInlineRead(t, text)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "T-01" || got.Title != "Inline" || got.Project != "alpha" || got.Size != "normal" || got.Workflow != "" ||
		got.State != "queued" || got.Created != "2026-11-01T09:00:00Z" || got.Intent != "Keep it.\n" {
		t.Fatalf("%+v", got)
	}
}

func TestFoldNamedWorkflowUnchanged(t *testing.T) {
	got, err := foldInlineRead(t, foldInlineHead+"workflow: Standard\n"+foldInlineTail)
	if err != nil || got.Workflow != "Standard" || got.Size != "normal" || got.State != "queued" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestFoldDuplicateWorkflowStillFails(t *testing.T) {
	text := foldInlineHead + "workflow: Standard\nworkflow:\n  - {step: build, kind: run}\n" + foldInlineTail
	if _, err := foldInlineRead(t, text); err == nil || !strings.Contains(err.Error(), "already defined") {
		t.Fatalf("err=%v", err)
	}
}
