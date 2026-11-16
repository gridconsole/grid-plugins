package task

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/proof"
	"go.yaml.in/yaml/v4"
)

func deliverFixture(t *testing.T, header string) (Store, Task) {
	t.Helper()
	s, task := writeFixture(t)
	path := filepath.Join(task.Path, "task.md")
	data, _ := os.ReadFile(path)
	text := strings.Replace(string(data), "state: queued", header+"state: running", 1)
	write(t, path, text+"\n## Notes\n\nKeep these.\n\n## Outcome\n\nOld result.\n\n## Follow-up\n\nKeep this too.\n")
	write(t, filepath.Join(task.Path, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  jobs: []\n")
	approveFixturePlan(t, s, task)
	if _, err := (proof.Store{TaskDir: task.Path}).Add(proof.Input{Item: "1", Text: "ok", Kind: "check", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	return s, task
}

func keyOrder(t *testing.T, text string) []string {
	t.Helper()
	var doc yaml.Node
	front := strings.SplitN(strings.TrimPrefix(text, "---\n"), "\n---\n", 2)[0]
	if err := yaml.Load([]byte(front), &doc); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for i := 0; i+1 < len(doc.Content[0].Content); i += 2 {
		keys = append(keys, doc.Content[0].Content[i].Value)
	}
	return keys
}

// Deliver edits the header the way Change does: key order, comments and unknown
// keys survive, and the Outcome, state, gate and receipt land in one commit.
func TestDeliverKeepsHeaderAndCommitsOnce(t *testing.T) {
	s, task := deliverFixture(t, "# Keep this comment\nextra: [one, two]\nstatus: running\n")
	path := filepath.Join(task.Path, "task.md")
	before, _ := os.ReadFile(path)
	beforeKeys := keyOrder(t, string(before))
	resultBefore, err := planJobResult(task.Path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(task.Path, "history.md"), "- legacy entry\n")

	summary := `Done: "ready" # not a comment`
	d, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: summary + "\nSecond line.", By: "grid"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Task != task.ID || d.DryRun {
		t.Fatalf("%+v", d)
	}
	after, _ := os.ReadFile(path)
	text := string(after)
	for _, keep := range []string{"# Keep this comment", "extra: [one, two]", "## Notes\n\nKeep these.", "## Follow-up\n\nKeep this too."} {
		if !strings.Contains(text, keep) {
			t.Fatalf("lost %q:\n%s", keep, text)
		}
	}
	if strings.Contains(text, "Old result") || strings.Count(text, "## Outcome\n") != 1 || !strings.Contains(text, "Second line.") {
		t.Fatalf("outcome not replaced:\n%s", text)
	}
	afterKeys := keyOrder(t, text)
	if len(afterKeys) < len(beforeKeys) {
		t.Fatalf("lost keys: %v -> %v", beforeKeys, afterKeys)
	}
	for i, k := range beforeKeys {
		if afterKeys[i] != k {
			t.Fatalf("reordered keys: %v -> %v", beforeKeys, afterKeys)
		}
	}
	var fm map[string]any
	if err := yaml.Load([]byte(strings.SplitN(strings.TrimPrefix(text, "---\n"), "\n---\n", 2)[0]), &fm); err != nil {
		t.Fatal(err)
	}
	if fm["state"] != "waiting" || fm["status"] != "waiting" || fm["gate"] != nil || fm["step"] != "ship" || fm["outcome"] != summary || fm["updated"] != "2026-01-02T03:04:05Z" {
		t.Fatalf("%v", fm)
	}
	got, err := Read(task.Path)
	if err != nil || got.State != "waiting" {
		t.Fatalf("%+v %v", got, err)
	}
	lines := historyLines(t, task.Path)
	if len(lines) != 2 || lines[0] != "- legacy entry" {
		t.Fatalf("%v", lines)
	}
	var receipt Event
	if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "- ")), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.What != "deliver" || receipt.Note != summary || receipt.Who.ID != "grid" || receipt.FromState != "running" || receipt.ToState != "waiting" {
		t.Fatalf("%+v", receipt)
	}
	if _, err := os.Stat(filepath.Join(task.Path, journalName)); !os.IsNotExist(err) {
		t.Fatal("journal was not removed")
	}
	// The fixture's approval of the plan is the only one: delivery adds none.
	if got, err := planJobResult(task.Path); err != nil || got != resultBefore {
		t.Fatalf("delivery changed the plan Job result: %+v -> %+v (%v)", resultBefore, got, err)
	}
	noReceiptFile(t, task.Path)
	if _, err := os.Stat(filepath.Join(task.Path, "approval-ship.json")); !os.IsNotExist(err) {
		t.Fatal("delivery wrote approval-ship.json")
	}
}

// Delivery cannot get around the state table: from paused, or from a task that
// is already waiting, it is refused like a status move and writes nothing.
func TestDeliverFollowsTheStateTable(t *testing.T) {
	for _, state := range []string{"paused", "waiting", "closed"} {
		t.Run(state, func(t *testing.T) {
			s, task := deliverFixture(t, "")
			path := filepath.Join(task.Path, "task.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "state: running", "state: "+state, 1))
			snapshot := map[string]string{}
			entries, _ := os.ReadDir(task.Path)
			for _, e := range entries {
				if !e.IsDir() {
					b, _ := os.ReadFile(filepath.Join(task.Path, e.Name()))
					snapshot[e.Name()] = string(b)
				}
			}
			_, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"})
			if err == nil {
				t.Fatal("delivered")
			}
			if state != "closed" && !strings.Contains(err.Error(), state) {
				t.Fatal(err)
			}
			for name, want := range snapshot {
				if name == ".task.lock" {
					continue
				}
				if got, _ := os.ReadFile(filepath.Join(task.Path, name)); string(got) != want {
					t.Fatalf("refused delivery changed %s", name)
				}
			}
		})
	}
}

// A journal left by a crashed writer replays before delivery writes, so the
// delivered Outcome and gate are not overwritten by the older image later.
func TestDeliverReplaysPendingJournalFirst(t *testing.T) {
	s, task := deliverFixture(t, "")
	path := filepath.Join(task.Path, "task.md")
	data, _ := os.ReadFile(path)
	stale := strings.Replace(string(data), "state: running", "state: stuck", 1)
	journal, _ := json.Marshal(map[string][]byte{"task.md": []byte(stale), "history.md": []byte("- crashed status move\n")})
	write(t, filepath.Join(task.Path, journalName), string(journal))
	if _, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(task.Path)
	if err != nil || got.State != "waiting" {
		t.Fatalf("%+v %v", got, err)
	}
	lines := historyLines(t, task.Path)
	if len(lines) != 2 || lines[0] != "- crashed status move" {
		t.Fatalf("%v", lines)
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "Ready") {
		t.Fatal("delivered Outcome missing")
	}
	var receipt Event
	json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "- ")), &receipt)
	if receipt.FromState != "stuck" {
		t.Fatalf("delivery did not start from the replayed state: %+v", receipt)
	}
}

func TestDeliverDryRunWritesNothing(t *testing.T) {
	s, task := deliverFixture(t, "")
	before := treeSnapshot(t, task.Path)
	d, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid", DryRun: true})
	if err != nil || !d.DryRun || len(d.Actions) != 3 {
		t.Fatalf("%+v %v", d, err)
	}
	if after := treeSnapshot(t, task.Path); len(before) != len(after) {
		t.Fatalf("dry run changed the folder: %v -> %v", before, after)
	} else {
		for name, b := range before {
			if after[name] != b {
				t.Fatalf("dry run changed %s", name)
			}
		}
	}
}

func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		rel, _ := filepath.Rel(root, path)
		out[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
