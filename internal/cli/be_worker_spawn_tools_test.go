package cli

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

var beCheckerFileTools = []string{"Read", "Grep", "Glob", "Bash", "mcp__claude-in-chrome__*", "mcp__Claude_Browser__*"}

func beSpawnTools(t *testing.T, h *harness, role string) []string {
	t.Helper()
	task := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-1")
	got, err := workerSpawnTools(h.env, task, role)
	if err != nil {
		t.Fatalf("workerSpawnTools(%q): %v", role, err)
	}
	return got
}

func beWantTools(t *testing.T, role string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("workerSpawnTools(%q) = %q, want %q", role, got, want)
	}
}

// Row 2: with no brain agent the tools are the ones agents/<role>.md names, in order.
func TestBeWorkerSpawnToolsFromAgentFile(t *testing.T) {
	h, _ := goUnsetFixture(t)
	beWantTools(t, "checker", beSpawnTools(t, h, "checker"), beCheckerFileTools)
	// agents/builder.md has a ": " in its description, which is not strict YAML; its tools are still read.
	for _, role := range []string{"builder", "orchestrator", "curator"} {
		if got := beSpawnTools(t, h, role); !slices.Contains(got, "Skill") {
			t.Errorf("workerSpawnTools(%q) = %q, want it to hold Skill", role, got)
		}
	}
	// The catalog name of a role is the same role.
	beWantTools(t, "grid-checker", beSpawnTools(t, h, "grid-checker"), beCheckerFileTools)
	// A role with no agent file has no tools key.
	beWantTools(t, "writer", beSpawnTools(t, h, "writer"), nil)
}

// Row 3: the brain agent of the role is the agent the Job runs, so its tools win, as a list.
func TestBeBrainAgentToolsWin(t *testing.T) {
	h, _ := goUnsetFixture(t)
	h.write(filepath.Join(h.gridHome, "knowledge", "agents", "grid-checker.md"),
		"---\nname: grid-checker\ndescription: d\ntools: [Read, mcp__claude-in-chrome__*]\n---\nBody.\n")
	beWantTools(t, "checker", beSpawnTools(t, h, "checker"), []string{"Read", "mcp__claude-in-chrome__*"})
}

// A project agent is found before the home agent, as AgentFor finds it.
func TestBeProjectBrainAgentToolsBeatHome(t *testing.T) {
	h, _ := goUnsetFixture(t)
	h.write(filepath.Join(h.gridHome, "knowledge", "agents", "grid-checker.md"), "---\nname: grid-checker\ntools: Read\n---\nBody.\n")
	h.write(filepath.Join(h.gridHome, "projects", "example", "knowledge", "agents", "grid-checker.md"), "---\nname: grid-checker\ntools: Grep, Glob\n---\nBody.\n")
	beWantTools(t, "checker", beSpawnTools(t, h, "checker"), []string{"Grep", "Glob"})
}

// A brain agent with no tools: line is the agent the Job runs, and it names no tools. The catalog file is not
// laid under it, so nothing is added that the agent the Job runs does not name.
func TestBeBrainAgentWithoutToolsAnswersNil(t *testing.T) {
	h, _ := goUnsetFixture(t)
	h.write(filepath.Join(h.gridHome, "knowledge", "agents", "grid-checker.md"), "---\nname: grid-checker\ndescription: d\nmodel: sonnet\n---\nBody.\n")
	beWantTools(t, "checker", beSpawnTools(t, h, "checker"), nil)
}

// A brain agent of a role that has no catalog file still gives its tools.
func TestBeBrainAgentOfRoleWithoutCatalogFile(t *testing.T) {
	h, _ := goUnsetFixture(t)
	h.write(filepath.Join(h.gridHome, "knowledge", "agents", "writer.md"), "---\nname: writer\ntools: Read, Write\n---\nBody.\n")
	beWantTools(t, "writer", beSpawnTools(t, h, "writer"), []string{"Read", "Write"})
}

// The tools: line is read as a comma-separated string or a list; entries are trimmed, empties and repeats
// dropped, order kept; a value of any other shape is no value.
func TestBeAgentToolsForms(t *testing.T) {
	for name, tc := range map[string]struct {
		md   string
		want []string
	}{
		"string":                    {"---\ntools: Read, Grep ,Glob\n---\nB\n", []string{"Read", "Grep", "Glob"}},
		"string repeats":            {"---\ntools: Read, , Grep,Read,\n---\nB\n", []string{"Read", "Grep"}},
		"list":                      {"---\ntools:\n  - Read\n  - ' Grep '\n  - Read\n  - ''\n  - Bash\n---\nB\n", []string{"Read", "Grep", "Bash"}},
		"flow list":                 {"---\ntools: [Read, mcp__x__*]\n---\nB\n", []string{"Read", "mcp__x__*"}},
		"list at col 0":             {"---\ntools:\n- Read\n# a note\n- Grep\nmodel: x\n---\nB\n", []string{"Read", "Grep"}},
		"stops at next key":         {"---\ntools: Read\nmodel: sonnet, opus\n---\nB\n", []string{"Read"}},
		"other key not strict YAML": {"---\ndescription: Runs one Job: reads it\ntools: Read, Skill\n---\nB\n", []string{"Read", "Skill"}},
		"order kept":                {"---\ntools: Write, Read\n---\nB\n", []string{"Write", "Read"}},
		"no tools key":              {"---\nname: a\n---\nB\n", nil},
		"empty tools":               {"---\ntools:\n---\nB\n", nil},
		"blank string":              {"---\ntools: ' , '\n---\nB\n", nil},
		"map":                       {"---\ntools: {a: b}\n---\nB\n", nil},
		"number":                    {"---\ntools: 3\n---\nB\n", nil},
		"list with a map":           {"---\ntools: [Read, {a: b}]\n---\nB\n", nil},
		"no front matter":           {"Body only.\n", nil},
		"duplicate key":             {"---\ntools: Read\ntools: Grep\n---\nB\n", nil},
	} {
		if got := beAgentTools(tc.md); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: beAgentTools = %q, want %q", name, got, tc.want)
		}
	}
}
