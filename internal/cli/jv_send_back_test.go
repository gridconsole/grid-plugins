package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// jvSendBackEnv replaces the harness Getenv whole: vars is what every command
// reads, so a test switches between the person and the holder by editing it.
func jvSendBackEnv(t *testing.T, h *harness) (vars map[string]string, person, holder func()) {
	t.Helper()
	epoch := os.Getenv("GRID_LEASE_EPOCH")
	vars = map[string]string{}
	h.env.Getenv = func(k string) string { return vars[k] }
	person = func() {
		clear(vars)
		vars["GRID_MEMBER"], vars["GRID_ROLE"], vars["GRID_ACTOR"] = "dana", "member", "dana"
	}
	holder = func() {
		clear(vars)
		vars["GRID_MEMBER"], vars["GRID_ROLE"], vars["GRID_LEASE_EPOCH"] = "member", "member", epoch
	}
	return vars, person, holder
}

// jvSendBackTask is approveJobHarness with J-3 waiting on the approve Job J-2,
// run as the person dana.
func jvSendBackTask(t *testing.T) (h *harness, path string, store feed.Store, text string, holder func()) {
	t.Helper()
	h, path, store, text = approveJobHarness(t)
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-3", "--body", personBody(t, "J-3", []any{"J-2"})); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	_, person, holder := jvSendBackEnv(t, h)
	person()
	return h, path, store, text, holder
}

// jvSendBackFiles is every file of the task folder but the lock files and the
// Feed's read cache: the Feed records, history.md, task.md and the rest.
func jvSendBackFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(p, ".lock") || strings.Contains(filepath.ToSlash(p), "/.cache/") {
			return err
		}
		b, err := os.ReadFile(p)
		files[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func jvSendBackHistory(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

// jvSendBackRecord is the newest record of type typ whose body says send_back.
func jvSendBackRecord(t *testing.T, store feed.Store, typ string) (feed.Entry, bool) {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: typ})
	if err != nil {
		t.Fatal(err)
	}
	for i := len(r.Records) - 1; i >= 0; i-- {
		if r.Records[i].Body["send_back"] == true {
			return r.Records[i], true
		}
	}
	return feed.Entry{}, false
}

// Row 2: the person sends the plan back; the holder's pass rejects J-2 with the
// words, J-3 stays held, the re-plan Job is posted, and history gains one
// send_back line with who and the hash and none of the words.
func TestJvSendBackRejects(t *testing.T) {
	h, path, store, text, holder := jvSendBackTask(t)
	before := jvSendBackHistory(t, path)
	if code := h.run(path, "approve", "T-07", "--plan-hash", plan.Hash(text), "--send-back", "split the parser", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	out := h.out.Bytes()
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil || got["what"] != "send_back" || got["plan_hash"] != plan.Hash(text) {
		t.Fatalf("output %s %v", out, err)
	}
	if err := contract.Validate("cmd-approve", out); err != nil {
		t.Fatalf("output is not a cmd-approve: %v", err)
	}
	holder()
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j2 := st.Jobs["J-2"]
	if j2.Status != contract.Rejected || j2.CloseReason == nil || *j2.CloseReason != "sent back by dana" {
		t.Fatalf("J-2 is %s close_reason %v", j2.Status, j2.CloseReason)
	}
	ctl, ok := jvSendBackRecord(t, store, "control")
	if !ok || ctl.Body["target"] != "J-2" || ctl.Body["answer"] != "split the parser" || ctl.Body["sent_back_hash"] != plan.Hash(text) || ctl.Body["plan_hash"] != nil {
		t.Fatalf("control %v", ctl.Body)
	}
	if s := st.Jobs["J-3"].Status; s == contract.Assigned {
		t.Fatalf("J-3 is %s after J-2 was sent back", s)
	}
	jobs, err := store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	replans := 0
	for _, e := range jobs.Records {
		if title, _ := e.Body["title"].(string); strings.HasPrefix(title, "Re-plan after J-2 was rejected") {
			replans++
		}
	}
	if replans != 1 {
		t.Fatalf("%d re-plan Jobs", replans)
	}
	after := jvSendBackHistory(t, path)
	added := strings.TrimPrefix(after, before)
	if !strings.HasPrefix(after, before) || strings.Count(added, "\n") != 1 || !strings.HasPrefix(added, "- ") {
		t.Fatalf("history gained %q", added)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(added, "- "))), &line); err != nil {
		t.Fatal(err)
	}
	who, _ := line["who"].(map[string]any)
	if line["what"] != "send_back" || line["plan_hash"] != plan.Hash(text) || who["id"] != "dana" {
		t.Fatalf("history line %v", line)
	}
	if _, ok := line["note"]; ok || strings.Contains(added, "split") || strings.Contains(added, "parser") {
		t.Fatalf("history line holds the words: %s", added)
	}
	if strings.Contains(after, `"what":"approve"`) {
		t.Fatalf("history holds an approve line:\n%s", after)
	}
	md, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil || !strings.Contains(string(md), "state: waiting") {
		t.Fatalf("task.md %s %v", md, err)
	}
	if body := approvalResult(t, store); body != nil {
		t.Fatalf("J-2 was accepted: %v", body)
	}
}

// Row 3: each refusal exits with its code and leaves the Feed and history.md as
// they were.
func TestJvSendBackRefusals(t *testing.T) {
	long := strings.Repeat("a", 3501)
	for _, c := range []struct {
		name  string
		after func(t *testing.T, h *harness, path, text string, holder func())
		args  func(text string) []string
		code  int
		say   string
	}{
		{"stale hash", nil, func(string) []string { return []string{"--plan-hash", "000000", "--send-back", "split it"} }, 1, "plan hash no longer matches"},
		{"no open approve Job", func(t *testing.T, h *harness, path, text string, holder func()) {
			holder()
			if code := h.run(path, "job", "cancel", "J-2", "--reason", "scope withdrawn"); code != 0 {
				t.Fatalf("%d %s", code, h.errb)
			}
		}, func(text string) []string { return []string{"--plan-hash", plan.Hash(text), "--send-back", "split it"} }, 3, "no open plan approve Job to send back"},
		{"second send-back", func(t *testing.T, h *harness, path, text string, holder func()) {
			if code := h.run(path, "approve", "T-07", "--plan-hash", plan.Hash(text), "--send-back", "split the parser"); code != 0 {
				t.Fatalf("%d %s", code, h.errb)
			}
			holder()
			if code := h.run(path, "job", "next", "--json"); code != 0 {
				t.Fatalf("%d %s", code, h.errb)
			}
		}, func(text string) []string { return []string{"--plan-hash", plan.Hash(text), "--send-back", "split it"} }, 3, "no open plan approve Job to send back"},
		{"note", nil, func(text string) []string {
			return []string{"--plan-hash", plan.Hash(text), "--send-back", "split it", "--note", "why"}
		}, 2, "--note"},
		{"empty words", nil, func(text string) []string { return []string{"--plan-hash", plan.Hash(text), "--send-back", "  "} }, 2, "--send-back"},
		{"3501 characters", nil, func(text string) []string { return []string{"--plan-hash", plan.Hash(text), "--send-back", long} }, 3, "the answer is longer than 3500 characters"},
		{"no plan hash", nil, func(string) []string { return []string{"--send-back", "split it"} }, 2, "--plan-hash"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h, path, _, text, holder := jvSendBackTask(t)
			if c.after != nil {
				c.after(t, h, path, text, holder)
			}
			vars := map[string]string{"GRID_MEMBER": "dana", "GRID_ROLE": "member"}
			h.env.Getenv = func(k string) string { return vars[k] }
			before := jvSendBackFiles(t, path)
			code := h.run(path, append([]string{"approve", "T-07"}, c.args(text)...)...)
			if code != c.code || !strings.Contains(h.errb.String(), c.say) {
				t.Fatalf("exit %d, want %d with %q: %s", code, c.code, c.say, h.errb)
			}
			after := jvSendBackFiles(t, path)
			if len(after) != len(before) {
				t.Fatalf("files %d -> %d", len(before), len(after))
			}
			for p, b := range before {
				if after[p] != b {
					t.Fatalf("%s changed", p)
				}
			}
		})
	}
}

// Row 1: an answer and a control with send_back and sent_back_hash pass the
// schema on post and read back counted (the verdict of an accepted record), and
// the answer of contract/fixtures/answer/valid/basic.json still does.
func TestJvSendBackOldReader(t *testing.T) {
	h, path, store, text, holder := jvSendBackTask(t)
	if code := h.run(path, "approve", "T-07", "--plan-hash", plan.Hash(text), "--send-back", "split the parser"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	holder()
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	b, err := os.ReadFile("../../contract/fixtures/answer/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Body map[string]any `json:"body"`
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	dana := feed.Writer{Author: feed.Author{ID: "dana", Role: "member"}, Worker: "dana@" + feedMachine}
	basic, err := store.Post(feed.Input{Type: "answer", Re: st.Jobs["J-2"].History[0], Body: fixture.Body}, dana)
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, e := range r.Records {
		if e.Body["send_back"] == true {
			seen[e.Type] = e.Verdict
		}
		if e.ID == basic.ID {
			seen["basic"] = e.Verdict
		}
	}
	for _, k := range []string{"answer", "control", "basic"} {
		if seen[k] != "counted" {
			t.Fatalf("%s reads %q, want counted: %v", k, seen[k], seen)
		}
	}
}
