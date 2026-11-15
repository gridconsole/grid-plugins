package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// showJSON runs `grid job show <task> <id> --json` and decodes the answer.
func showJSON(t *testing.T, h *harness, task, id string, want int) map[string]any {
	t.Helper()
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(task, "job", "show", task, id, "--json"); code != want {
		t.Fatalf("job show %s --json: exit %d want %d: %s %s", id, code, want, h.out, h.errb)
	}
	var r map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatalf("job show %s --json: %v: %s", id, err, h.out)
	}
	return r
}

// A Job with no saved folder shows from its Feed record: a person question Job
// (grid question ask writes no folder) and a posted Job that has not started.
// No folder is written for either, and an id on neither the Feed nor disk fails.
func TestJobShowWithoutSavedFolder(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	q := askJob(t, h, path)
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-3", "--body", personBody(t, "J-3", []any{q.Job})); code != 0 {
		t.Fatalf("post J-3 %d: %s", code, h.errb)
	}
	for _, id := range []string{q.Job, "J-3"} {
		if _, err := os.Stat(filepath.Join(path, "jobs", id)); !os.IsNotExist(err) {
			t.Fatalf("%s has a saved folder before show: %v", id, err)
		}
	}

	// The person question Job: text and --json carry its question, role, kind and state.
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(path, "job", "show", path, q.Job); code != 0 {
		t.Fatalf("job show %s: exit %d: %s %s", q.Job, code, h.out, h.errb)
	}
	for _, want := range []string{"id: " + q.Job + "\n", "role: Person\n", "kind: ask\n", "state: Not started\n", "question: Which export format?\n", "title: Q-1: Which export format?\n"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("text show of the question Job lacks %q:\n%s", want, h.out)
		}
	}
	r := showJSON(t, h, path, q.Job, 0)
	job, _ := r["job"].(map[string]any)
	if r["ok"] != true || r["id"] != q.Job || r["state"] != "Not started" || r["record"] == "" || job == nil ||
		job["role"] != "Person" || job["kind"] != "ask" || job["title"] != "Q-1: Which export format?" ||
		job["intent"].(map[string]any)["goal"] != "Which export format?" {
		t.Errorf("--json show of the question Job: %v", r)
	}

	// A posted Job that has not started, with the question Job in its after list.
	h.out.Reset()
	if code := h.run(path, "job", "show", path, "J-3"); code != 0 {
		t.Fatalf("job show J-3: exit %d: %s %s", code, h.out, h.errb)
	}
	for _, want := range []string{"id: J-3\n", "state: Not started\n", "after: " + q.Job + "\n"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("text show of the posted Job lacks %q:\n%s", want, h.out)
		}
	}
	r = showJSON(t, h, path, "J-3", 0)
	if job, _ = r["job"].(map[string]any); r["ok"] != true || r["id"] != "J-3" || r["state"] != "Not started" || job == nil || len(job["after"].([]any)) != 1 {
		t.Errorf("--json show of the posted Job: %v", r)
	}

	// Show wrote no folder.
	for _, id := range []string{q.Job, "J-3"} {
		if _, err := os.Stat(filepath.Join(path, "jobs", id)); !os.IsNotExist(err) {
			t.Errorf("show wrote jobs/%s: %v", id, err)
		}
	}

	// An id on neither the Feed nor disk is still an error.
	if code := h.run(path, "job", "show", path, "J-99"); code != 1 {
		t.Errorf("job show of an unknown id: exit %d: %s", code, h.out)
	}
	r = showJSON(t, h, path, "J-99", 1)
	if r["ok"] == true || r["error"] == "" {
		t.Errorf("--json show of an unknown id: %v", r)
	}
}
