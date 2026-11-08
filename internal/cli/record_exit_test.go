package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

const exitRecordID = "01ARZ3NDEKTSV4RRFFQ69G5FAX"
const exitMessage = `{"ref":"task","kind":"message","text":"example"}`
const exitBodyError = "$.body: oneOf matched 0 branches: $.body: missing required ref"

func assignedExitHarness(t *testing.T, ack bool) (*harness, string, feed.Store, string, string) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("assign: %d %s", code, h.errb)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := state.Jobs["J-1"]
	epoch := fmt.Sprintf("%d.%d", j.Generation, j.Assign.Fence)
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE_EPOCH", epoch)
	t.Setenv("GRID_FEED_TASK", path)
	if ack {
		if code := h.run(path, "job", "ack", "J-1", "--attempt", "1"); code != 0 {
			t.Fatalf("ack: %d %s", code, h.errb)
		}
	}
	return h, path, store, epoch, j.AssignPost
}

func TestRecordExitFeed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		setup  func(*testing.T, *harness, string)
		code   int
		stderr string
	}{
		{name: "success", code: 0},
		{name: "resolution", args: []string{"--task", "T-999-missing"}, code: 1},
		{name: "storage", setup: func(t *testing.T, h *harness, path string) { h.write(filepath.Join(path, "feed"), "obstruction") }, code: 1},
		{name: "usage", args: []string{"--unknown"}, code: 2},
		{name: "unregistered", args: []string{"--type", "example"}, code: 3, stderr: `unregistered post type "example"`},
		{name: "schema", args: []string{"--body", "{}"}, code: 3, stderr: exitBodyError},
		{name: "array", args: []string{"--body", "[]"}, code: 3, stderr: "--body must be a JSON object"},
		{name: "null", args: []string{"--body", "null"}, code: 3, stderr: "--body must be a JSON object"},
		{name: "malformed", args: []string{"--body", "{"}, code: 3, stderr: "--body must be a JSON object"},
		{name: "reused id", args: []string{"--id", exitRecordID, "--body", `{"ref":"task","kind":"message","text":"changed"}`}, setup: func(t *testing.T, h *harness, path string) {
			if code := h.run(path, "feed", "post", "--type", "post", "--body", exitMessage, "--id", exitRecordID); code != 0 {
				t.Fatalf("seed: %d %s", code, h.errb)
			}
		}, code: 3, stderr: "record id already used for a different post"},
		{name: "guest answer", args: []string{"--type", "answer", "--re", exitRecordID, "--body", `{"answer":"yes"}`}, setup: func(t *testing.T, h *harness, path string) { t.Setenv("GRID_ROLE", "guest") }, code: 3, stderr: "guest answer must name a question addressed to that guest"},
		{name: "oversized envelope", args: []string{"--after", oversizedExitAfter()}, code: 3, stderr: "envelope exceeds 32 KB even with body reference"},
		// The CLI stamps matching identities; a conflicting assertion remains a session error.
		{name: "writer assertion", setup: func(t *testing.T, h *harness, path string) { t.Setenv("GRID_WORKER", "other@"+feedMachine) }, code: 1, stderr: "GRID_WORKER must match GRID_MEMBER and the local machine id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path := feedHarness(t)
			if tc.setup != nil {
				tc.setup(t, h, path)
			}
			args := append([]string{"feed", "post", "--type", "post", "--body", exitMessage, "--json"}, tc.args...)
			code := h.run(path, args...)
			if tc.stderr != "" && h.errb.String() != "grid feed: "+tc.stderr+"\n" {
				t.Errorf("stderr = %q, want %q", h.errb.String(), "grid feed: "+tc.stderr+"\n")
			}
			if code != tc.code {
				t.Errorf("exit %d, want %d: %s", code, tc.code, h.errb)
			}
		})
	}
}

func oversizedExitAfter() string {
	ids := make([]string, 1200)
	for i := range ids {
		ids[i] = fmt.Sprintf("%026d", i)
	}
	return strings.Join(ids, ",")
}

func TestRecordExitWorkerArgv(t *testing.T) {
	for _, tc := range []struct {
		name, kind, body, after, epoch string
		ack                            bool
		code                           int
		stderr                         string
	}{
		{name: "ack", kind: "ack", body: "{}", after: "assign", code: 0},
		{name: "wrong after", kind: "ack", body: "{}", after: exitRecordID, code: 3, stderr: "ack requires its counted assignment in after"},
		{name: "missing after", kind: "ack", body: "{}", code: 3, stderr: "$.after: array length"},
		{name: "stale", kind: "ack", body: "{}", after: "assign", epoch: "0.999", code: 3, stderr: "stale lease"},
		{name: "first record", kind: "worker-exit", body: "{}", code: 3, stderr: "first record of a tenure must be ack"},
		{name: "invalid worker exit", kind: "worker-exit", body: `{"reason":"exploded"}`, ack: true, code: 3, stderr: exitBodyError},
		{name: "idempotent worker exit", kind: "worker-exit", body: `{"reason":"done"}`, ack: true, code: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path, store, epoch, assign := assignedExitHarness(t, tc.ack)
			if tc.epoch != "" {
				epoch = tc.epoch
			}
			args := []string{"feed", "post", "--type", tc.kind, "--id", exitRecordID, "--job", "J-1", "--attempt", "1", "--lease", "J-1", "--lease-epoch", epoch, "--body", tc.body, "--json"}
			after := tc.after
			if after == "assign" {
				after = assign
			}
			if after != "" {
				args = append(args, "--after", after)
			}
			before, err := store.Read(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			for n := 0; n < 2; n++ {
				code := h.run(path, args...)
				if tc.stderr != "" && h.errb.String() != "grid feed: "+tc.stderr+"\n" {
					t.Errorf("stderr = %q, want %q", h.errb.String(), "grid feed: "+tc.stderr+"\n")
				}
				if code != tc.code {
					t.Errorf("exit %d, want %d: %s", code, tc.code, h.errb)
				}
			}
			records, err := store.Read(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			want := len(before.Records)
			if tc.code == 0 {
				want++
			}
			if len(records.Records) != want {
				t.Errorf("records = %d, want %d", len(records.Records), want)
			}
		})
	}
}

func TestRecordExitJob(t *testing.T) {
	for _, verb := range []string{"ack", "post", "result"} {
		for _, name := range []string{"success", "resolution", "storage", "usage", "stale", "schema", "array", "null", "malformed", "reused id", "first record", "exited", "unknown type"} {
			if verb == "ack" && (name == "first record" || name == "exited") || verb != "post" && name == "unknown type" {
				continue
			}
			t.Run(verb+"/"+name, func(t *testing.T) {
				h, path, store, _, _ := assignedExitHarness(t, verb != "ack" && name != "first record")
				body := "{}"
				extra := []string{}
				if verb == "post" {
					body = exitMessage
					extra = []string{"--type", "post"}
				}
				if verb == "result" {
					raw, err := os.ReadFile("../../contract/fixtures/result/valid/basic.json")
					if err != nil {
						t.Fatal(err)
					}
					var r struct {
						Body json.RawMessage `json:"body"`
					}
					if err := json.Unmarshal(raw, &r); err != nil {
						t.Fatal(err)
					}
					body = string(r.Body)
				}
				args := append([]string{"job", verb, "J-1", "--attempt", "1", "--id", exitRecordID, "--json", "--body", body}, extra...)
				want := 3
				stderr := ""
				switch name {
				case "success":
					want = 0
				case "resolution":
					args = append(args, "--task", "T-999-missing")
					want = 1
				case "storage":
					if err := os.Remove(filepath.Join(store.Dir(), ".orchestrator.lock")); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					h.mkdir(filepath.Join(store.Dir(), ".orchestrator.lock"))
					want = 1
				case "usage":
					args = append(args, "--unknown")
					want = 2
				case "stale":
					args = append(args, "--lease-epoch", "0.999")
					stderr = "stale lease"
				case "schema":
					args = append(args, "--body", `{"unexpected":true}`)
					stderr = exitBodyError
				case "array":
					args = append(args, "--body", "[]")
					stderr = "--body must be a JSON object"
				case "null":
					args = append(args, "--body", "null")
					stderr = "--body must be a JSON object"
				case "malformed":
					args = append(args, "--body", "{")
					stderr = "--body must be a JSON object"
				case "reused id":
					if code := h.run(path, args...); code != 0 {
						t.Fatalf("seed: %d %s", code, h.errb)
					}
					args = append(args, "--re", feedTaskID)
					stderr = "record id already used for a different post"
				case "first record":
					stderr = "Job tenure must start with ack"
				case "exited":
					if code := h.run(path, "job", "post", "J-1", "--attempt", "1", "--type", "worker-exit", "--body", `{"reason":"done"}`); code != 0 {
						t.Fatalf("exit seed: %d %s", code, h.errb)
					}
					stderr = "attempt already exited"
				case "unknown type":
					args = append(args, "--type", "example")
					stderr = `unregistered post type "example"`
				}
				code := h.run(path, args...)
				if stderr != "" && h.errb.String() != stderr+"\n" {
					t.Errorf("stderr = %q, want %q", h.errb.String(), stderr+"\n")
				}
				if code != want {
					t.Errorf("exit %d, want %d: %s", code, want, h.errb)
				}
			})
		}
	}
}

func TestRecordExitCancel(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
		text string
	}{
		{"success", nil, 0, ""},
		{"resolution", []string{"--task", "T-999-missing"}, 1, ""},
		{"usage", []string{"--unknown"}, 2, ""},
		{"stale", []string{"--lease-epoch", "0.999"}, 3, "stale lease"},
		{"unknown Job", []string{"--job", "J-999"}, 3, "unknown Job J-999"},
		{"array", []string{"--body", "[]"}, 3, "--body must be a JSON object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path, _ := orchestratorHarness(t)
			args := append([]string{"job", "cancel", "J-1", "--reason", "scope withdrawn", "--json"}, tc.args...)
			code := h.run(path, args...)
			if code != tc.code {
				t.Errorf("exit %d, want %d: %s", code, tc.code, h.errb)
			}
			if tc.text != "" && h.errb.String() != tc.text+"\n" {
				t.Errorf("stderr = %q, want %q", h.errb.String(), tc.text+"\n")
			}
		})
	}
}

func TestRecordExitDuplicateWake(t *testing.T) {
	h, path, _, _, _ := assignedExitHarness(t, true)
	args := []string{"job", "post", "J-1", "--attempt", "1", "--type", "post", "--body", exitMessage, "--id", exitRecordID, "--wake", feedTaskID + "/1"}
	if code := h.run(path, args...); code != 0 {
		t.Fatalf("first: %d %s", code, h.errb)
	}
	args = append(args, "--id", feedMachine, "--wake", feedTaskID+"/2")
	if code := h.run(path, args...); code != 1 || h.errb.String() != "wake write already exists\n" || h.out.String() != exitRecordID+"\n" {
		t.Fatalf("retry: %d stdout=%s stderr=%s", code, h.out, h.errb)
	}
}

func TestRecordExitFeedAdditionalRejections(t *testing.T) {
	for _, name := range []string{"other tenure id", "spilled body conflict", "first task record", "handover precondition"} {
		t.Run(name, func(t *testing.T) {
			h, path := feedHarness(t)
			args := []string{"feed", "post", "--type", "post", "--body", exitMessage, "--id", exitRecordID}
			want := ""
			switch name {
			case "other tenure id":
				if code := h.run(path, args...); code != 0 {
					t.Fatalf("seed: %d %s", code, h.errb)
				}
				t.Setenv("GRID_MEMBER", "other")
				args = append(args, "--wake", feedTaskID+"/1")
				want = "record id already used for a different post"
			case "spilled body conflict":
				h.write(filepath.Join(path, "feed", "bodies", exitRecordID+".json"), "{}")
				args = append(args, "--type", "knowledge", "--body", `{"title":"Example","text":"`+strings.Repeat("x", feed.MaxLine)+`"}`)
				want = "body id already used"
			case "first task record":
				h.write(filepath.Join(path, "feed", "lease", "task.json"), `{"lease":"task","holder":"member@`+feedMachine+`","fence":1,"gen":0,"attempt":null}`)
				args = append(args, "--lease", "task", "--lease-epoch", "0.1")
				want = "first record of a tenure must be handover"
			case "handover precondition":
				args = []string{"feed", "handover"}
				want = "handover requires an already granted task lease"
			}
			if code := h.run(path, args...); code != 3 || h.errb.String() != "grid feed: "+want+"\n" {
				t.Fatalf("exit %d: %s, want 3: %s", code, h.errb, want)
			}
		})
	}
}

func TestRecordExitHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"feed", "--help"}, {"job", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h, path := feedHarness(t)
			if code := h.run(path, args...); code != 0 || !strings.Contains(h.out.String(), exitStatusHelp) {
				t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
			}
		})
	}
}
