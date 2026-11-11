package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGoUnsetAgentRemovesOnlyThatEntry(t *testing.T) {
	home := t.TempDir()
	s := Store{Home: home}
	if err := s.SetAgent("grid-builder", Agent{Model: "opus", Effort: "high"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgent("grid-scout", Agent{Model: "haiku"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UnsetAgent("grid-builder"); err != nil {
		t.Fatal(err)
	}
	doc, err := s.Effective()
	if err != nil {
		t.Fatal(err)
	}
	if doc.Agents["grid-scout"].Model != "haiku" {
		t.Fatalf("other agent lost: %+v", doc.Agents["grid-scout"])
	}
	if doc.Agents["grid-builder"].Model == "opus" && doc.Agents["grid-builder"].Effort == "high" {
		t.Fatalf("entry still set: %+v", doc.Agents["grid-builder"])
	}
	path := filepath.Join(home, "settings.json")
	before, _ := os.ReadFile(path)
	if err := s.UnsetAgent("grid-builder"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("second unset changed the file")
	}
	if err := s.UnsetAgent("nobody"); err == nil {
		t.Fatal("unknown agent accepted")
	}
}
