package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// pjVb2Remote is a bare remote in a temp folder with main pushed, and a work clone that pushes
// to it. It answers the remote, the clone and main's SHA.
func pjVb2Remote(t *testing.T) (remote, work, mainSHA string) {
	t.Helper()
	testgit.Isolate(t)
	root := t.TempDir()
	remote, work = filepath.Join(root, "remote.git"), filepath.Join(root, "work")
	testgit.Run(t, root, "init", "-q", "--bare", "-b", "main", remote)
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	testgit.Init(t, work, remote, true)
	return remote, work, pjVb2Push(t, work, "main", "refs/heads/main")
}

// pjVb2Push makes one empty commit in work, pushes it to ref on origin and answers its SHA.
func pjVb2Push(t *testing.T, work, msg, ref string) string {
	t.Helper()
	testgit.Run(t, work, "commit", "-q", "--allow-empty", "-m", msg)
	testgit.Run(t, work, "push", "-q", "origin", "HEAD:"+ref)
	return testgit.Run(t, work, "rev-parse", "HEAD")
}

// pjVb2Rework posts the review J-4.1 of the builder J-4 on a fresh task T-07, then runs
// `grid judge rework J-4 --from J-4.1` with args, and answers the rework Job's body on the Feed.
func pjVb2Rework(t *testing.T, args ...string) map[string]any {
	t.Helper()
	h, task, _ := pjVbFeed(t)
	pjVbRun(t, h, task, "job", "post", "J-4.1", "--type", "job", "--json", "--body", pjVbBody(t, "J-4.1", map[string]any{
		"role": "reviewer", "discovered_from": "J-4", "verify": map[string]any{"kind": "reviewer-check", "job": "J-4.1"},
	}))
	out := pjVbRun(t, h, task, append([]string{"judge", "rework", "J-4", "--from", "J-4.1", "--json"}, args...)...)
	var printed struct {
		Posted []struct{ ID, Type, Job string } `json:"posted"`
	}
	if err := json.Unmarshal([]byte(out), &printed); err != nil || len(printed.Posted) != 2 || printed.Posted[0].Type != "job" {
		t.Fatalf("rework printed %q (%v), want the rework Job and its control", out, err)
	}
	var rework *feed.Record
	records := pjVbRecords(t, task)
	for i, r := range records {
		if r.ID == printed.Posted[0].ID {
			rework = &records[i]
		}
	}
	if rework == nil || rework.Body["continues"] != "J-4" || rework.Body["discovered_from"] != "J-4.1" {
		t.Fatalf("rework record %+v, want a Job that continues J-4 from J-4.1", rework)
	}
	return rework.Body
}

// Row 8 (V27) on a real Feed: `grid judge rework --remote` bases the rework Job on the SHA
// refs/grid/T-07/head points at on the remote now, and on the target's tip there when the task
// has no head; a remote it cannot read fails the verb and posts nothing.
func TestPjVb2JudgeReworkBaseIsReadOnTheRemote(t *testing.T) {
	remote, work, mainSHA := pjVb2Remote(t)
	head := pjVb2Push(t, work, "task head", "refs/grid/T-07/head")
	if body := pjVb2Rework(t, "--remote", remote, "--target", "main"); body["base"] != head {
		t.Fatalf("rework base %v with a task head on the remote, want the head %s", body["base"], head)
	}

	testgit.Run(t, work, "push", "-q", "origin", ":refs/grid/T-07/head")
	if body := pjVb2Rework(t, "--remote", remote, "--target", "main"); body["base"] != mainSHA {
		t.Fatalf("rework base %v with no task head, want main's tip %s", body["base"], mainSHA)
	}

	h, task, _ := pjVbFeed(t)
	pjVbRun(t, h, task, "job", "post", "J-4.1", "--type", "job", "--json", "--body", pjVbBody(t, "J-4.1", map[string]any{
		"role": "reviewer", "discovered_from": "J-4", "verify": map[string]any{"kind": "reviewer-check", "job": "J-4.1"},
	}))
	before := len(pjVbRecords(t, task))
	absent := filepath.Join(t.TempDir(), "absent.git")
	if code := h.run(task, "judge", "rework", "J-4", "--from", "J-4.1", "--remote", absent, "--target", "main"); code == 0 || !strings.Contains(h.errb.String(), "git ls-remote") {
		t.Fatalf("rework with an unreadable remote exited %d: %s%s, want a failure naming git ls-remote", code, h.out, h.errb)
	}
	if after := len(pjVbRecords(t, task)); after != before {
		t.Fatalf("the Feed holds %d records after a failed rework, %d before", after, before)
	}
}
