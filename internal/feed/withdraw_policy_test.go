package feed

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateWithdrawalTaskPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, policy                  string
		wantOpen, warning, unreadable bool
	}{
		{name: "own owner policy", policy: "---\ngate_post_roles: {withdraw: [owner]}\n---\n", wantOpen: true},
		{name: "own member policy", policy: "---\ngate_post_roles: {withdraw: [member]}\n---\n"},
		{name: "empty policy", policy: "---\ngate_post_roles: {withdraw: []}\n---\n", wantOpen: true},
		{name: "missing policy"},
		{name: "malformed policy", policy: "---\ngate_post_roles: [\n---\n", warning: true},
		{name: "unreadable policy", unreadable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, "projects", "example", "tasks", "T-01-example")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			s := Store{TaskDir: dir, TaskID: testTask}
			q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"id": "Q-1", "kind": "person"}}}, testWriter())
			mustPost(t, s, Input{Type: "control", Re: q.ID, Body: map[string]any{"question_event": "withdrawn", "target": q.ID, "text": "No longer needed"}}, testWriter())
			// A broken, unrelated template must not affect this task's snapshot.
			for _, p := range []string{filepath.Join(home, "workflows.md"), filepath.Join(home, "projects", "example", "workflows.md")} {
				if err := os.WriteFile(p, []byte("# Workflows\n\n## Unrelated\napplies when: invalid\nreview rounds: not-a-number\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			policy := filepath.Join(dir, "workflow.md")
			if tc.unreadable {
				if err := os.Mkdir(policy, 0700); err != nil {
					t.Fatal(err)
				}
			} else if tc.policy != "" {
				if err := os.WriteFile(policy, []byte(tc.policy), 0600); err != nil {
					t.Fatal(err)
				}
			}
			stderr, err := os.CreateTemp(t.TempDir(), "stderr-")
			if err != nil {
				t.Fatal(err)
			}
			defer stderr.Close()
			old := os.Stderr
			os.Stderr = stderr
			defer func() { os.Stderr = old }()
			state, err := s.State()
			os.Stderr = old
			if tc.unreadable {
				if err == nil || !strings.Contains(err.Error(), "cannot read task withdrawal policy") {
					t.Fatalf("unreadable policy: %v", err)
				}
				if info, e := stderr.Stat(); e != nil || info.Size() != 0 {
					t.Fatalf("read failure emitted a fallback warning: %v", e)
				}
				return
			}
			if err != nil || state.Questions[q.ID].Open != tc.wantOpen {
				t.Fatalf("state=%+v error=%v", state, err)
			}
			if (len(state.NeedsYou) == 1) != tc.wantOpen {
				t.Fatalf("needs-you=%v", state.NeedsYou)
			}
			if _, err := stderr.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(stderr)
			if err != nil {
				t.Fatal(err)
			}
			if tc.warning {
				if strings.Count(string(raw), "warning:") != 1 || !strings.Contains(string(raw), "using defaults") || strings.Count(string(raw), "\n") != 1 {
					t.Fatalf("want one fallback warning: %q", raw)
				}
			} else if len(raw) != 0 {
				t.Fatalf("unrelated policy produced warning: %q", raw)
			}
		})
	}
}
