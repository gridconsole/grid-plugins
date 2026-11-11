package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// lyULID is a well-formed ULID that no fixture task holds.
const lyULID = "01ARZ3NDEKTSV4RRFFQ69G5FAZ"

// lyStableID reads the stable id of a task folder, as an agent's GRID_TASK holds it.
func lyStableID(t *testing.T, dir string) string {
	t.Helper()
	id, err := feed.TaskID(dir)
	if err != nil || !feed.IsID(id) {
		t.Fatalf("task %s has no stable id: %q, %v", dir, id, err)
	}
	return id
}

// lyShow runs `task show <operand> --json` from cwd and returns id and folder.
func lyShow(t *testing.T, h *harness, cwd, operand string) (string, string) {
	t.Helper()
	if code := h.run(cwd, "task", "show", operand, "--json"); code != 0 {
		t.Fatalf("task show %s from %s: exit %d: %s", operand, cwd, code, h.norm(h.errb.String()))
	}
	var shown map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil {
		t.Fatal(err)
	}
	id, _ := shown["id"].(string)
	folder, _ := shown["folder"].(string)
	return id, folder
}

// A migrated task's ULID names the task for task show and history add, from
// outside the task folder, as the T-NN does.
func TestLyTaskOperandULIDShowAndHistory(t *testing.T) {
	h, f := operandFixture(t, "show")
	ulid := lyStableID(t, f.Path)
	for _, cwd := range []string{h.root, h.home} {
		wantID, wantFolder := lyShow(t, h, cwd, f.ID)
		gotID, gotFolder := lyShow(t, h, cwd, ulid)
		if gotID != wantID || gotID != f.ID || gotFolder != wantFolder || gotFolder != filepath.Base(f.Path) {
			t.Errorf("from %s: ULID gave id %q folder %q, T-NN gave id %q folder %q", cwd, gotID, gotFolder, wantID, wantFolder)
		}
		if code := h.run(cwd, "history", "add", "Checked by ULID from "+filepath.Base(cwd), "--task", ulid); code != 0 {
			t.Fatalf("history add from %s: exit %d: %s", cwd, code, h.norm(h.errb.String()))
		}
		text := readTaskTestFile(t, filepath.Join(f.Path, "history.md"))
		if !strings.Contains(text, "Checked by ULID from "+filepath.Base(cwd)) {
			t.Errorf("history line missing from %s/history.md:\n%s", f.Path, text)
		}
	}
}

// proof add and deliver take the ULID an agent finds in GRID_TASK, from outside
// the task folder, and write into that task's folder.
func TestLyDeliverAndProofByULID(t *testing.T) {
	h, dir := deliveryFixture(t)
	ulid := lyStableID(t, dir)
	before := treeBytes(t, dir)
	if code := h.run(h.home, "proof", "add", "1", "--text", "All checks pass", "--kind", "check", "--task", ulid, "--json"); code != 0 {
		t.Fatalf("proof add: exit %d: %s", code, h.norm(h.errb.String()))
	}
	after := treeBytes(t, dir)
	changed := 0
	for name, text := range after {
		if before[name] != text {
			changed++
		}
	}
	if changed == 0 {
		t.Fatalf("proof add by ULID wrote nothing in %s", dir)
	}
	proofText := ""
	for name, text := range after {
		if strings.Contains(text, "All checks pass") {
			proofText = name
		}
	}
	if proofText == "" {
		t.Fatalf("proof text not found in the task folder: %v", after)
	}
	if code := h.run(h.home, "deliver", "--task", ulid, "--outcome", "Result ready"); code != 0 {
		t.Fatalf("deliver: exit %d: %s", code, h.norm(h.errb.String()))
	}
	text := readTaskTestFile(t, filepath.Join(dir, "task.md"))
	if !strings.Contains(text, "## Outcome") || !strings.Contains(text, "Result ready") || !strings.Contains(text, "state: waiting") {
		t.Errorf("task.md holds no Outcome after deliver by ULID:\n%s", text)
	}
}

// resolveTaskOperand gives the same task for the ULID as for the T-NN, for a
// migrated task and for a legacy one (id T-NN, ulid <ULID>).
func TestLyResolveTaskOperandULIDMigratedAndLegacy(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "migrated"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			h, f := operandFixture(t, "show")
			ulid := lyStableID(t, f.Path)
			if legacy {
				if err := os.Remove(filepath.Join(h.gridHome, "VERSION")); err != nil {
					t.Fatal(err)
				}
				h.write(filepath.Join(f.Path, "task.md"), "---\nid: "+f.ID+"\nulid: "+ulid+"\ntitle: Old title\nproject: "+f.Project+"\nsize: small\nworkflow: \"\"\nstate: queued\ncreated: '2026-01-02T03:04:05.000Z'\ncontract: \"1\"\n---\n")
			}
			h.env.Cwd = h.home
			want, err := resolveTaskOperand(h.env, f.ID)
			if err != nil {
				t.Fatalf("T-NN: %v", err)
			}
			got, err := resolveTaskOperand(h.env, ulid)
			if err != nil {
				t.Fatalf("ULID: %v", err)
			}
			if got.ID != want.ID || got.Path != want.Path || got.ID != f.ID || got.Path != f.Path {
				t.Errorf("ULID gave %q at %q, T-NN gave %q at %q, fixture %q at %q", got.ID, got.Path, want.ID, want.Path, f.ID, f.Path)
			}
		})
	}
}

// A well-formed ULID that no task holds is the refusal T-99 gets.
func TestLyTaskOperandULIDNotFound(t *testing.T) {
	h, f := operandFixture(t, "show")
	h.env.Cwd = h.home
	if !feed.IsID(lyULID) || lyStableID(t, f.Path) == lyULID {
		t.Fatal("the unknown ULID must be well formed and unused")
	}
	_, wantErr := resolveTaskOperand(h.env, "T-99")
	if !errors.Is(wantErr, where.ErrTaskNotFound) {
		t.Fatalf("T-99: %v", wantErr)
	}
	_, err := resolveTaskOperand(h.env, lyULID)
	if !errors.Is(err, where.ErrTaskNotFound) {
		t.Errorf("unknown ULID: %v, want ErrTaskNotFound", err)
	}
	if code := h.run(h.home, "task", "show", lyULID); code != 1 {
		t.Errorf("task show of an unknown ULID: exit %d, want 1: %s", code, h.errb)
	}
}

// The ULID lookup walks only the tasks directories the caller may see: the
// Personal task's ULID is refused for a member and a guest, the shared task's
// ULID resolves, and a link in the shared directory cannot reach the Personal task.
func TestLyTaskOperandULIDPolicyAndLink(t *testing.T) {
	h, personal := operandFixture(t, "show")
	projectDir := filepath.Join(h.root, "example-project")
	h.mkdir(projectDir)
	shared, err := taskWriter(h.env).New(task.NewOptions{Cwd: projectDir, Words: "Shared task"})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(h.gridHome, "projects", shared.Project, "project-roles.json"), `{"people":[{"id":"example-member","perm":"Client"}]}`)
	h.env.Cwd = h.home
	role := func(role string) func(string) string {
		return func(key string) string {
			switch key {
			case "GRID_ROLE":
				return role
			case "GRID_MEMBER":
				return "example-member"
			}
			return ""
		}
	}
	for _, r := range []string{"owner", "member", "guest"} {
		h.env.Getenv = role(r)
		for _, f := range []task.Task{personal, shared} {
			got, err := resolveTaskOperand(h.env, lyStableID(t, f.Path))
			allowed := f.ID == shared.ID || r == "owner"
			if allowed && (err != nil || got.ID != f.ID || got.Path != f.Path) {
				t.Errorf("%s resolving %s by ULID: %+v, %v", r, f.Path, got, err)
			}
			if !allowed && !errors.Is(err, where.ErrTaskNotFound) {
				t.Errorf("%s by Personal ULID: %v, want ErrTaskNotFound", r, err)
			}
		}
	}

	link := filepath.Join(filepath.Dir(shared.Path), "T-99-link")
	if err := os.Symlink(personal.Path, link); err != nil {
		t.Fatal(err)
	}
	personalULID := lyStableID(t, personal.Path)
	h.env.Getenv = role("member")
	if _, err := resolveTaskOperand(h.env, personalULID); !errors.Is(err, errTaskOperand) {
		t.Errorf("member through the cross-project link: %v, want errTaskOperand", err)
	}
	h.env.Getenv = role("owner")
	if _, err := resolveTaskOperand(h.env, personalULID); err == nil || !strings.Contains(err.Error(), "more than one folder") {
		t.Errorf("owner with two folders holding the id: %v", err)
	}
	// Beside the link the shared task answers its ULID as it answers its T-NN.
	h.env.Getenv = role("member")
	wantT, wantErr := resolveTaskOperand(h.env, shared.ID)
	gotT, gotErr := resolveTaskOperand(h.env, lyStableID(t, shared.Path))
	t.Logf("beside the link: T-NN %q, %v; ULID %q, %v", wantT.Path, wantErr, gotT.Path, gotErr)
	if (wantErr == nil) != (gotErr == nil) || gotT.Path != wantT.Path {
		t.Errorf("shared task beside the link: ULID %+v, %v; T-NN %+v, %v", gotT, gotErr, wantT, wantErr)
	}
}

// Two real folders whose task.md give the same ULID name no single task.
func TestLyTaskOperandULIDTwoFolders(t *testing.T) {
	h, f := operandFixture(t, "show")
	h.env.Cwd = h.home
	text := readTaskTestFile(t, filepath.Join(f.Path, "task.md"))
	h.write(filepath.Join(filepath.Dir(f.Path), "T-05-copy", "task.md"), text)
	_, err := resolveTaskOperand(h.env, lyStableID(t, f.Path))
	if err == nil || !strings.Contains(err.Error(), "task has more than one folder in the Grid home") {
		t.Errorf("two folders with one ULID: %v", err)
	}
	if code := h.run(h.home, "task", "show", lyStableID(t, f.Path)); code != 1 {
		t.Errorf("task show with two folders: exit %d, want 1", code)
	}
}
