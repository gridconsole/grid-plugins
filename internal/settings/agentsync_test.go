package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAgentTemplates(t *testing.T, dir, version string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"grid-builder", "Explore"} {
		text := "---\nname: " + name + "\ndescription: " + name + " agent\nmodel: sonnet\neffort: medium\ntools: Read, Grep\n---\n\nBody " + version + " for " + name + ".\n"
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func syncFixture(t *testing.T) SyncOptions {
	t.Helper()
	root := t.TempDir()
	o := SyncOptions{Home: filepath.Join(root, "grid"), TemplatesDir: filepath.Join(root, "agent-templates"), AgentsDir: filepath.Join(root, "claude", "agents")}
	if err := os.MkdirAll(o.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	writeAgentTemplates(t, o.TemplatesDir, "v1")
	return o
}

func stateOf(results []SyncResult, agent string) string {
	for _, r := range results {
		if r.Agent == agent {
			return r.State
		}
	}
	return ""
}

func TestAgentSyncLeavesPersonsOwnFile(t *testing.T) {
	o := syncFixture(t)
	own := []byte("---\nname: grid-builder\ndescription: mine\nmodel: opus\n---\n\nMy own builder.\n")
	path := filepath.Join(o.AgentsDir, "grid-builder.md")
	if err := os.MkdirAll(o.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, own, 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := SyncAgents(o)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(own) {
		t.Fatalf("the person's file changed:\n%s", got)
	}
	if s := stateOf(results, "grid-builder"); s != SyncYours {
		t.Fatalf("grid-builder state = %q, want %q", s, SyncYours)
	}
	if s := stateOf(results, "Explore"); s != SyncWritten {
		t.Fatalf("Explore state = %q, want %q", s, SyncWritten)
	}
}

func TestAgentSyncNewTemplateKeepsOverlay(t *testing.T) {
	o := syncFixture(t)
	if err := (Store{Home: o.Home}).SetAgent("grid-builder", Agent{Model: "opus", Effort: "high"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncAgents(o); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(o.AgentsDir, "grid-builder.md")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"model: opus\n", "effort: high\n", "Body v1 for grid-builder.", "owner: grid\n", "template: "} {
		if !strings.Contains(string(first), want) {
			t.Fatalf("first render lacks %q:\n%s", want, first)
		}
	}
	// A plugin update replaces the templates with new bodies, and so a new sha.
	writeAgentTemplates(t, o.TemplatesDir, "v2")
	results, err := SyncAgents(o)
	if err != nil {
		t.Fatal(err)
	}
	if s := stateOf(results, "grid-builder"); s != SyncWritten {
		t.Fatalf("grid-builder state = %q, want %q", s, SyncWritten)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"model: opus\n", "effort: high\n", "Body v2 for grid-builder."} {
		if !strings.Contains(string(second), want) {
			t.Fatalf("second render lacks %q:\n%s", want, second)
		}
	}
	if strings.Contains(string(second), "Body v1") || strings.Contains(string(second), "model: sonnet") {
		t.Fatalf("second render keeps the old body or the template's model:\n%s", second)
	}
	if sha := func(b []byte) string { _, s, _ := strings.Cut(string(b), "template: "); return strings.Fields(s)[0] }; sha(first) == sha(second) {
		t.Fatal("the template sha did not change")
	}
}
