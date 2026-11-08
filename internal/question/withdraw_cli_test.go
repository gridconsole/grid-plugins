package question_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

func TestPersonWithdrawCLI(t *testing.T) {
	for _, tc := range []struct {
		name, role, agent, reason, prior, stderr string
		marker, workflow                         string
		code                                     int
	}{
		{name: "owner", role: "owner", reason: "Resolved elsewhere"},
		{name: "member", role: "member"},
		{name: "guest", role: "guest", code: 3, stderr: "question: Q-1 role \"guest\" may not withdraw\n"},
		{name: "agent shell", role: "member", agent: "builder", code: 3, stderr: "question: Q-1 agent shell requires a task lease to withdraw\n"},
		{name: "worker id", role: "owner", marker: "GRID_WORKER_ID", agent: "builder", code: 3, stderr: "question: Q-1 agent shell requires a task lease to withdraw\n"},
		{name: "worker", role: "owner", marker: "GRID_WORKER", agent: "builder", code: 3, stderr: "question: Q-1 agent shell requires a task lease to withdraw\n"},
		{name: "agent role", role: "agent", code: 3, stderr: "question: Q-1 role \"agent\" may not withdraw\n"},
		{name: "cli role", role: "cli", code: 3, stderr: "question: Q-1 role \"cli\" may not withdraw\n"},
		{name: "workflow member refused", role: "member", workflow: "owner", code: 3, stderr: "question: Q-1 role \"member\" may not withdraw\n"},
		{name: "workflow owner allowed", role: "owner", workflow: "owner"},
		{name: "defaulted", role: "owner", prior: "default", code: 3, stderr: "question: Q-1 is defaulted\n"},
		{name: "answered", role: "owner", prior: "answer", code: 3, stderr: "question: Q-1 is answered\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "grid")
			task := filepath.Join(home, "projects", "example", "tasks", "T-01-example")
			if err := os.MkdirAll(task, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "projects", "example", "project-roles.json"), []byte(`{"people":[{"id":"person","perm":"Client"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			const taskID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
			for path, data := range map[string]string{filepath.Join(home, "VERSION"): "1\n", filepath.Join(home, "machine"): "01ARZ3NDEKTSV4RRFFQ69G5FAW\n", filepath.Join(task, "task.md"): "---\nid: " + taskID + "\n---\n"} {
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, key := range []string{"GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_AGENT", "GRID_WORKER_ID", "GRID_WORKER", "GRID_FEED_TASK", "GRID_JOB", "GRID_ATTEMPT"} {
				t.Setenv(key, "")
			}
			t.Setenv("GRID_MEMBER", "person")
			t.Setenv("GRID_ROLE", "owner")
			var out, stderr bytes.Buffer
			env := cli.Env{Cwd: root, Home: root, GridHome: home, Ceiling: root, Stdout: &out, Stderr: &stderr}
			run := func(args ...string) int { out.Reset(); stderr.Reset(); return cli.Run(args, env) }
			if tc.workflow != "" {
				workflowText := "---\ngate_post_roles: {withdraw: [" + tc.workflow + "]}\n---\n"
				if err := os.WriteFile(filepath.Join(task, "workflow.md"), []byte(workflowText), 0600); err != nil {
					t.Fatal(err)
				}
			}
			deadline := "2099-01-01T00:00:00Z"
			if tc.prior == "default" {
				deadline = "2000-01-01T00:00:00Z"
			}
			if code := run("question", "add", "Proceed?", "--task", task, "--default", "Wait", "--applies", deadline, "--json"); code != 0 {
				t.Fatalf("add: %d %s", code, &stderr)
			}
			var q question.Record
			if err := json.Unmarshal(out.Bytes(), &q); err != nil {
				t.Fatal(err)
			}
			if tc.prior == "answer" {
				if code := run("answer", task, q.ID, "Yes"); code != 0 {
					t.Fatalf("answer: %d %s", code, &stderr)
				}
			}
			f := feed.Store{TaskDir: task, TaskID: taskID}
			before, err := f.ReadOnly(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(task, "questions", q.ID+".md")
			original, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("GRID_ROLE", tc.role)
			marker := tc.marker
			if marker == "" {
				marker = "GRID_AGENT"
			}
			t.Setenv(marker, tc.agent)
			args := []string{"question", "withdraw", q.ID, "--task", task, "--json"}
			if tc.reason != "" {
				args = append(args, "--reason", tc.reason)
			}
			if code := run(args...); code != tc.code || stderr.String() != tc.stderr {
				t.Fatalf("withdraw: exit=%d stderr=%q; want exit=%d stderr=%q", code, stderr.String(), tc.code, tc.stderr)
			}
			after, err := f.ReadOnly(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.code != 0 {
				current, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if len(after.Records) != len(before.Records) || out.Len() != 0 || !bytes.Equal(original, current) {
					t.Fatal("refused withdraw wrote data")
				}
				return
			}
			var got question.Record
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			reason := tc.reason
			if reason == "" {
				reason = "No longer needed"
			}
			if got.State != "withdrawn" || len(got.History) != 1 || got.History[0].Text != reason || got.History[0].Who != "person" || got.History[0].Role != tc.role {
				t.Fatalf("withdraw projection: %+v", got)
			}
			if len(after.Records) != len(before.Records)+1 {
				t.Fatal("withdraw did not append exactly one record")
			}
			p := after.Records[len(after.Records)-1]
			if p.Type != "control" || p.Lease != nil || p.Fence != 0 || p.Re == nil || *p.Re != q.Post || p.Body["target"] != q.Post || p.Body["question_event"] != "withdrawn" || p.Body["text"] != reason {
				t.Fatalf("withdraw envelope: %+v", p)
			}
			state, err := f.State()
			if err != nil {
				t.Fatal(err)
			}
			if state.Questions[q.Post].Open || len(state.NeedsYou) != 0 {
				t.Fatal("withdrawn question remains in needs-you")
			}
			if code := run(args...); code != 0 {
				t.Fatalf("retry: %d %s", code, &stderr)
			}
			retry, err := f.ReadOnly(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if len(retry.Records) != len(after.Records) || !bytes.Contains(out.Bytes(), []byte(`"state":"withdrawn"`)) {
				t.Fatal("retry must print the record without appending")
			}
		})
	}
}
