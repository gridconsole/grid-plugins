package task

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkBriefRefusesABriefOverTheReadLimit(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subagents")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	text := append([]byte("---\nrole: builder\nstatus: running\n---\n## Outcome\n"), bytes.Repeat([]byte("x"), 1<<20)...)
	path := filepath.Join(sub, "01-big.md")
	if err := os.WriteFile(path, text, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MarkBrief(dir, "01-big.md", "done"); err == nil {
		t.Fatal("an oversized brief was rewritten")
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, text) {
		t.Fatalf("brief changed: %d bytes, was %d", len(got), len(text))
	}
}
