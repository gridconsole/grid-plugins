package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const brainNote = "---\ntype: workflow\nname: flow\ngate_post_roles: {person: [agent]}\n---\n"

func writeBrainFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func brainTask(t *testing.T, home, rel string) string {
	t.Helper()
	dir := filepath.Join(home, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func wantPerson(t *testing.T, taskDir string, person []string) {
	t.Helper()
	got, err := GatePostRoles(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	want := defaultGatePostRoles()
	want["person"] = person
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("roles = %v, want %v", got, want)
	}
}

func TestGatePostRolesProjectNote(t *testing.T) {
	home := t.TempDir()
	writeBrainFile(t, filepath.Join(home, "projects/acme/knowledge/process/flow.md"), brainNote)
	task := brainTask(t, home, "projects/acme/tasks/T-01-x")
	wantPerson(t, task, []string{"agent"})
}

func TestGatePostRolesTaskWorkflowWins(t *testing.T) {
	home := t.TempDir()
	writeBrainFile(t, filepath.Join(home, "projects/acme/knowledge/process/flow.md"), brainNote)
	task := brainTask(t, home, "projects/acme/tasks/T-01-x")
	writeBrainFile(t, filepath.Join(task, "workflow.md"), "---\ngate_post_roles: {person: [owner]}\n---\n")
	wantPerson(t, task, []string{"owner"})
}

func TestGatePostRolesHomeNote(t *testing.T) {
	home := t.TempDir()
	writeBrainFile(t, filepath.Join(home, "knowledge/process/flow.md"), brainNote)
	wantPerson(t, brainTask(t, home, "projects/acme/tasks/T-01-x"), []string{"agent"})
	wantPerson(t, brainTask(t, home, "personal/tasks/T-02-y"), []string{"agent"})
}

func TestGatePostRolesProjectBeatsHome(t *testing.T) {
	home := t.TempDir()
	writeBrainFile(t, filepath.Join(home, "knowledge/process/flow.md"), brainNote)
	writeBrainFile(t, filepath.Join(home, "projects/acme/knowledge/process/flow.md"),
		"---\ntype: workflow\nname: flow\ngate_post_roles: {person: [lead]}\n---\n")
	wantPerson(t, brainTask(t, home, "projects/acme/tasks/T-01-x"), []string{"lead"})
	// A personal task has no project level, so only the home note applies.
	wantPerson(t, brainTask(t, home, "personal/tasks/T-02-y"), []string{"agent"})
}

func TestGatePostRolesDefaultsWithoutNote(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{
		t.TempDir(),
		brainTask(t, home, "projects/acme/tasks/T-01-x"),
		brainTask(t, home, "personal/tasks/T-02-y"),
		brainTask(t, home, "projects/acme/tasks/not-a-task"),
	} {
		got, err := GatePostRoles(dir)
		if err != nil || !reflect.DeepEqual(got, defaultGatePostRoles()) {
			t.Fatalf("%s: got %v, %v; want the default", dir, got, err)
		}
	}
}

func TestGatePostRolesOutsideLayoutIgnoresNotes(t *testing.T) {
	home := t.TempDir()
	writeBrainFile(t, filepath.Join(home, "knowledge/process/flow.md"), brainNote)
	// Not under projects/<p>/tasks or personal/tasks: no brain level applies.
	for _, rel := range []string{"tasks/T-01-x", "projects/acme/T-01-x", "other/acme/tasks/T-01-x"} {
		got, err := GatePostRoles(brainTask(t, home, rel))
		if err != nil || !reflect.DeepEqual(got, defaultGatePostRoles()) {
			t.Fatalf("%s: got %v, %v; want the default", rel, got, err)
		}
	}
}

func TestGatePostRolesBrokenOrDoubledProjectNoteFallsThrough(t *testing.T) {
	cases := map[string]func(t *testing.T, project string){
		"broken front matter": func(t *testing.T, project string) {
			writeBrainFile(t, filepath.Join(project, "knowledge/process/flow.md"),
				"---\ntype: workflow\nname: flow\ngate_post_roles: [unclosed\n---\n")
		},
		"bad roles": func(t *testing.T, project string) {
			writeBrainFile(t, filepath.Join(project, "knowledge/process/flow.md"),
				"---\ntype: workflow\nname: flow\ngate_post_roles: {person: typo}\n---\n")
		},
		"two notes": func(t *testing.T, project string) {
			for _, name := range []string{"one", "two"} {
				writeBrainFile(t, filepath.Join(project, "knowledge/process/"+name+".md"),
					"---\ntype: workflow\nname: "+name+"\ngate_post_roles: {person: [lead]}\n---\n")
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			setup(t, filepath.Join(home, "projects/acme"))
			task := brainTask(t, home, "projects/acme/tasks/T-01-x")
			// No home note: the default answers.
			got, err := GatePostRoles(task)
			if err != nil || !reflect.DeepEqual(got, defaultGatePostRoles()) {
				t.Fatalf("got %v, %v; want the default and no error", got, err)
			}
			// A home note answers in its place.
			writeBrainFile(t, filepath.Join(home, "knowledge/process/flow.md"), brainNote)
			wantPerson(t, task, []string{"agent"})
		})
	}
}

func TestGatePostRolesBrokenTaskWorkflowStillErrors(t *testing.T) {
	home := t.TempDir()
	writeBrainFile(t, filepath.Join(home, "projects/acme/knowledge/process/flow.md"), brainNote)
	task := brainTask(t, home, "projects/acme/tasks/T-01-x")
	writeBrainFile(t, filepath.Join(task, "workflow.md"), "---\ngate_post_roles: {person: typo}\n---\n")
	if got, err := GatePostRoles(task); err == nil {
		t.Fatalf("broken workflow.md fell back to a note: %v", got)
	}
}
