package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/intent"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func TestSessionIntentCommands(t *testing.T) {
	for _, tc := range []struct {
		args                     []string
		verb, target, key, value string
	}{
		{[]string{"start"}, "start", "T-01-example", "", ""},
		{[]string{"start", "--agent", "planner"}, "start", "T-01-example", "agent", "planner"},
		{[]string{"stop", "--target", "session-example"}, "stop", "session-example", "", ""},
		{[]string{"send", "a message with spaces"}, "send", "T-01-example", "text", "a message with spaces"},
		{[]string{"send", "--text", "--literal text"}, "send", "T-01-example", "text", "--literal text"},
		{[]string{"pause"}, "pause", "T-01-example", "", ""},
		{[]string{"resume"}, "resume", "T-01-example", "", ""},
		{[]string{"handback"}, "handback", "T-01-example", "", ""},
		{[]string{"switch", "builder"}, "switch", "T-01-example", "agent", "builder"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			h := newHarness(t)
			task := livenessProject(h)
			args := append([]string{"session"}, tc.args...)
			args = append(args, "--who", "member", "--json")
			if code := h.run(task, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			var r intent.Record
			if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
				t.Fatal(err)
			}
			if r.Verb != tc.verb || r.Target != tc.target || r.Who != "member" || r.Args[tc.key] != tc.value {
				t.Fatalf("record: %+v", r)
			}
			entries, err := os.ReadDir(task)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "intents" {
				t.Fatalf("wrote outside intents: %v", entries)
			}
			rows, err := (intent.Store{TaskDir: task}).List(false)
			if err != nil || len(rows) != 1 || rows[0].ID != r.ID {
				t.Fatal(rows, err)
			}
		})
	}
}

func TestIntentCommandsLifecycle(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	for range 2 {
		if code := h.run(h.tmp, "session", "send", "hello", "--task", task, "--idempotency-key", "request-example", "--json"); code != 0 {
			t.Fatal(code, h.errb)
		}
	}
	rows, err := (intent.Store{TaskDir: task}).List(false)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	id := rows[0].ID
	if code := h.run(task, "intent", "claim", id, "--who", "worker-example", "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	var claim intent.Claim
	if err := json.Unmarshal(h.out.Bytes(), &claim); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "intent", "claim", id); code != 5 {
		t.Fatal("second claim", code)
	}
	if code := h.run(task, "intent", "list", "--pending", "--json"); code != 0 || h.out.String() != "[]\n" {
		t.Fatal(code, h.out, h.errb)
	}
	if code := h.run(task, "intent", "done", id); code != 2 {
		t.Fatal("missing token", code)
	}
	if code := h.run(task, "intent", "done", id, "--claim", claim.Token, "--error", "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	var done intent.Completion
	if err := json.Unmarshal(h.out.Bytes(), &done); err != nil || done.Error != "worker reported an error" {
		t.Fatal(done, err)
	}
	if code := h.run(task, "intent", "claim", id); code != 5 {
		t.Fatal("completed claim", code)
	}
	if code := h.run(task, "intent", "list"); code != 0 || !strings.Contains(h.out.String(), "state: done") {
		t.Fatal(code, h.out, h.errb)
	}
	// Successful completion and explicit failure reason, including the env token.
	for _, failure := range []string{"", "agent unavailable"} {
		r, err := (intent.Store{TaskDir: task}).Enqueue("resume", "task", nil, "cli", "")
		if err != nil {
			t.Fatal(err)
		}
		c, err := (intent.Store{TaskDir: task}).Claim(r.ID, "worker")
		if err != nil {
			t.Fatal(err)
		}
		h.env.Getenv = func(key string) string {
			if key == "GRID_INTENT_CLAIM" {
				return c.Token
			}
			return ""
		}
		args := []string{"intent", "done", r.ID, "--json"}
		if failure != "" {
			args = append(args, "--error="+failure)
		}
		if code := h.run(task, args...); code != 0 {
			t.Fatal(code, h.errb)
		}
		done = intent.Completion{}
		if err := json.Unmarshal(h.out.Bytes(), &done); err != nil || done.Error != failure {
			t.Fatal(done, err)
		}
	}
}

func TestIntentRenewCommand(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	if code := h.run(task, "session", "start", "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	var r intent.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "intent", "renew", r.ID, "--token", "01ARZ3NDEKTSV4RRFFQ69G5FAV"); code != 3 {
		t.Fatal("renew without a claim", code, h.errb)
	}
	if code := h.run(task, "intent", "claim", r.ID, "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	var claim intent.Claim
	if err := json.Unmarshal(h.out.Bytes(), &claim); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--token", "--claim"} {
		if code := h.run(task, "intent", "renew", r.ID, flag, claim.Token, "--json"); code != 0 {
			t.Fatal(flag, code, h.errb)
		}
		var renewed intent.Claim
		if err := json.Unmarshal(h.out.Bytes(), &renewed); err != nil {
			t.Fatal(err)
		}
		if renewed.Token != claim.Token || renewed.Until.Before(claim.Until) {
			t.Fatalf("%s renewed %+v from %+v", flag, renewed, claim)
		}
	}
	if code := h.run(task, "intent", "renew", r.ID, "--token", "01ARZ3NDEKTSV4RRFFQ69G5FAV"); code != 3 || !strings.Contains(h.errb.String(), "claim token") {
		t.Fatal("renew with a stale token", code, h.errb)
	}
	if code := h.run(task, "intent", "renew", r.ID); code != 2 {
		t.Fatal("renew without a token", code)
	}
	h.env.Getenv = func(key string) string {
		if key == "GRID_INTENT_CLAIM" {
			return claim.Token
		}
		return ""
	}
	if code := h.run(task, "intent", "renew", r.ID); code != 0 || !strings.Contains(h.out.String(), "claim: "+claim.Token) {
		t.Fatal("renew with the environment token", code, h.out, h.errb)
	}
	if code := h.run(task, "intent", "done", r.ID, "--token", claim.Token); code != 0 {
		t.Fatal("done with --token", code, h.errb)
	}
	if code := h.run(task, "intent", "renew", r.ID); code != 5 {
		t.Fatal("renew after done", code)
	}
}

// The desktop passes --who <person> and one --idempotency-key per click; without
// a key, only a repeat from the same caller inside the window coalesces.
func TestSendWhoAndKeys(t *testing.T) {
	send := func(t *testing.T, args ...string) (intent.Record, int) {
		t.Helper()
		h := newHarness(t)
		task := livenessProject(h)
		var last intent.Record
		for _, a := range args {
			argv := append([]string{"session", "send", "yes", "--json"}, strings.Fields(a)...)
			if code := h.run(task, argv...); code != 0 {
				t.Fatal(a, code, h.errb)
			}
			if err := json.Unmarshal(h.out.Bytes(), &last); err != nil {
				t.Fatal(err)
			}
		}
		rows, err := (intent.Store{TaskDir: task}).List(false)
		if err != nil {
			t.Fatal(err)
		}
		return last, len(rows)
	}
	for _, tc := range []struct {
		name    string
		calls   []string
		records int
		who     string
	}{
		{"same caller repeats", []string{"--who alice", "--who alice"}, 1, "alice"},
		{"different callers", []string{"--who alice", "--who bob"}, 2, "bob"},
		{"default caller repeats", []string{"", ""}, 1, "cli"},
		{"one key per click", []string{"--who alice --idempotency-key click-1", "--who alice --idempotency-key click-2"}, 2, "alice"},
		{"retried click", []string{"--who alice --idempotency-key click-1", "--who alice --idempotency-key click-1"}, 1, "alice"},
		{"same key, different callers", []string{"--who alice --idempotency-key click-1", "--who bob --idempotency-key click-1"}, 2, "bob"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			last, records := send(t, tc.calls...)
			if records != tc.records || last.Who != tc.who {
				t.Fatalf("records: %d who: %s, want %d and %s", records, last.Who, tc.records, tc.who)
			}
		})
	}
}

func TestTaskUsageListsMove(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.tmp, "task", "--help"); code != 0 || !strings.Contains(h.out.String(), "grid task move <machine>") {
		t.Fatal(code, h.out, h.errb)
	}
}

func TestIntentJSONGolden(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	store := intent.Store{TaskDir: task, Now: func() time.Time { return now }}
	var output strings.Builder
	for i, verb := range []string{"start", "pause", "send"} {
		args := map[string]string{}
		if verb == "send" {
			args["text"] = "Continue the task"
		}
		now = now.Add(time.Second)
		r, err := store.Enqueue(verb, "session-example", args, "member", "")
		if err != nil {
			t.Fatal(err)
		}
		// Stable IDs keep golden fixtures readable and retain real JSON rendering.
		fixed := []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAW", "01ARZ3NDEKTSV4RRFFQ69G5FAX"}[i]
		oldPath := filepath.Join(task, "intents", r.ID+".json")
		r.ID = fixed
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(task, "intents", fixed+".json"), b, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(oldPath); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			c := intent.Claim{Token: "01ARZ3NDEKTSV4RRFFQ69G5FAY", Who: "worker", Created: now, Until: time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)}
			b, err = json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(task, "intents", fixed+".claim"), b, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if i == 2 {
			c, err := store.Claim(fixed, "worker")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Done(fixed, c.Token, "agent unavailable"); err != nil {
				t.Fatal(err)
			}
			// Normalize only the random token in the completion fixture.
			p := filepath.Join(task, "intents", fixed+".done.json")
			b, err = os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(strings.ReplaceAll(string(b), c.Token, "01ARZ3NDEKTSV4RRFFQ69G5FAZ")), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, args := range [][]string{{"intent", "list", "--json"}, {"intent", "list", "--json", "--pending"}} {
		if code := h.run(task, args...); code != 0 {
			t.Fatal(code, h.errb)
		}
		output.WriteString(h.out.String())
	}
	golden.Assert(t, "intent-list", output.String())
}

func TestIntentUsage(t *testing.T) {
	for _, args := range [][]string{
		{"session", "send"}, {"session", "switch"}, {"session", "pause", "--agent", "builder"},
		{"session", "send", "hello", "--text", "again"}, {"session", "switch", "builder", "--agent", "planner"},
		{"session", "start", "--who", " "}, {"session", "send", ""}, {"task", "move"},
		{"intent"}, {"intent", "unknown"}, {"intent", "claim"}, {"intent", "done"}, {"intent", "renew"}, {"intent", "renew", "bad"},
		{"intent", "list", "--claim", "bad"}, {"intent", "done", "bad"}, {"session", "unknown"},
	} {
		h := newHarness(t)
		task := livenessProject(h)
		if code := h.run(task, args...); code != 2 {
			t.Errorf("%v: %d %s", args, code, h.errb)
		}
		if _, err := os.Stat(filepath.Join(task, "intents")); !os.IsNotExist(err) {
			t.Fatal("usage wrote intents")
		}
	}
	h := newHarness(t)
	if code := h.run(h.tmp, "session", "start"); code != 1 {
		t.Fatal("outside task", code)
	}
	for _, args := range [][]string{{"intent", "claim", "--help"}, {"session", "start", "--help"}} {
		if code := h.run(h.tmp, args...); code != 0 {
			t.Fatal(args, code)
		}
	}
}

func TestTaskMoveIntents(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "no remote", true: "remote"}[remote], func(t *testing.T) {
			h := newHarness(t)
			task := livenessProject(h)
			repo := filepath.Join(h.root, "repo")
			h.mkdir(repo)
			url := ""
			if remote {
				url = "https://example.com/org/repo.git"
			}
			testgit.Init(t, repo, url, false)
			h.record(project.Record{Name: "example", ID: "EXAMPLE", Folders: []project.Folder{{Machine: h.env.Machine, Path: repo}}, Repos: []project.Repo{{Path: "."}}})
			for _, args := range [][]string{
				{"task", "move", "worker-example", "--json"},
				{"task", "move", "T-01", "--to", "worker-example", "--project", "example", "--json"},
				{"task", "move", "--to", "worker-example", "--json"},
			} {
				code := h.run(task, args...)
				if !remote {
					if code != 1 || !strings.Contains(h.errb.String(), noMoveRemote) {
						t.Fatal(code, h.errb)
					}
					if _, err := os.Stat(filepath.Join(task, "intents")); !os.IsNotExist(err) {
						t.Fatal("queued move without remote")
					}
					continue
				}
				if code != 0 {
					t.Fatal(code, h.errb)
				}
				var r intent.Record
				if err := json.Unmarshal(h.out.Bytes(), &r); err != nil || r.Verb != "move" || r.Target != "worker-example" || len(r.Args) != 0 {
					t.Fatal(r, err)
				}
			}
		})
	}
}

func TestTaskMoveChecksEveryRepository(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	folder := filepath.Join(h.root, "workspace")
	first, second := filepath.Join(folder, "one"), filepath.Join(folder, "two")
	h.mkdir(first, second)
	testgit.Init(t, first, "https://example.com/org/one.git", false)
	testgit.Init(t, second, "", false)
	h.record(project.Record{Name: "example", ID: "EXAMPLE", Folders: []project.Folder{{Machine: h.env.Machine, Path: folder}}, Repos: []project.Repo{{Path: "one"}, {Path: "two"}}})
	if code := h.run(task, "task", "move", "worker-example"); code != 1 || !strings.Contains(h.errb.String(), noMoveRemote) {
		t.Fatal(code, h.errb)
	}
	if _, err := os.Stat(filepath.Join(task, "intents")); !os.IsNotExist(err) {
		t.Fatal("queued incomplete move")
	}
	testgit.Run(t, second, "remote", "add", "origin", "https://example.com/org/two.git")
	if code := h.run(task, "task", "move", "worker-example"); code != 0 {
		t.Fatal(code, h.errb)
	}
}
