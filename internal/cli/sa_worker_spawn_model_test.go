package cli

import (
	"path/filepath"
	"testing"
)

func saSpawn(t *testing.T, h *harness, task, role, model, effort string) {
	t.Helper()
	// The worker runs inside a registered folder of the project, as the settings resolver needs.
	env := h.env
	env.Cwd = filepath.Join(h.root, "repo")
	m, e, err := workerSpawnModel(env, task, role)
	if err != nil || m != model || e != effort {
		t.Fatalf("workerSpawnModel(%q) = %q/%q, %v; want %q/%q", role, m, e, err, model, effort)
	}
}

func TestSaWorkerSpawnModel(t *testing.T) {
	h, repo := goUnsetFixture(t)
	task := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-1")
	agents := filepath.Join(h.gridHome, "projects", "example", "knowledge", "agents")
	// No brain agent: the catalog's opus/medium (catalog.go: grid-researcher).
	saSpawn(t, h, task, "researcher", "opus", "medium")
	// A project agent with a model: the model is the brain's, the effort inherits.
	h.write(filepath.Join(agents, "grid-researcher.md"), "---\nname: grid-researcher\ndescription: d\nmodel: fable\n---\nBody.\n")
	saSpawn(t, h, task, "researcher", "fable", "medium")
	// Both values, passed through as written.
	h.write(filepath.Join(agents, "grid-researcher.md"), "---\nname: grid-researcher\nmodel: fable\neffort: high\n---\nBody.\n")
	saSpawn(t, h, task, "researcher", "fable", "high")
	saSpawn(t, h, task, "grid-researcher", "fable", "high")
	// A file with a key outside the agent fields is skipped, as brain check flags it.
	h.write(filepath.Join(agents, "grid-researcher.md"), "---\nname: grid-researcher\nmodel: fable\nhooks: {}\n---\nBody.\n")
	saSpawn(t, h, task, "researcher", "opus", "medium")
	// A model that is not a plain string is no value.
	h.write(filepath.Join(agents, "grid-researcher.md"), "---\nmodel: [a, b]\neffort: high\n---\nBody.\n")
	saSpawn(t, h, task, "researcher", "opus", "medium")
	// A personal value beats the brain's.
	h.write(filepath.Join(agents, "grid-researcher.md"), "---\nmodel: fable\neffort: high\n---\nBody.\n")
	if code := goUnsetRun(t, h, repo, "agent", "set", "grid-researcher", "--model", "haiku"); code != 0 {
		t.Fatalf("agent set: %d %s", code, h.errb)
	}
	saSpawn(t, h, task, "researcher", "haiku", "high")
	// A role with no catalog agent: the brain's values, or nothing.
	saSpawn(t, h, task, "person", "", "")
	h.write(filepath.Join(h.gridHome, "knowledge", "agents", "person.md"), "---\nmodel: fable\neffort: low\n---\nBody.\n")
	saSpawn(t, h, task, "person", "fable", "low")
}
