package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// The two reads a worker makes before a spawn, `grid lease status task --json` and
// `grid where --json`, are held to the output schemas cmd-lease-status and cmd-where.
// The app reads holder, gen and fence from the first and folder and task_path from the
// second (app internal/kernel/worker/epoch.go, agentbuild.go, judge.go, sharepush.go).

// ygLwRun runs grid in cwd, fails the test on a non-zero exit and returns stdout.
func ygLwRun(t *testing.T, h *harness, cwd string, args ...string) []byte {
	t.Helper()
	if code := h.run(cwd, args...); code != 0 {
		t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
	}
	return append([]byte(nil), h.out.Bytes()...)
}

// ygLwValid fails unless stdout passes the named schema, and returns it decoded.
func ygLwValid(t *testing.T, schema string, stdout []byte) map[string]any {
	t.Helper()
	if err := contract.Validate(schema, stdout); err != nil {
		t.Fatalf("%s rejects stdout: %v\n%s", schema, err, stdout)
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// ygLwEdit decodes a copy of stdout, lets edit change it and encodes it again.
func ygLwEdit(t *testing.T, stdout []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(stdout, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ygLwRenameFails renames each key in a copy of stdout, one at a time, and fails when
// the schema still accepts the copy: a plugin that renamed the field would be caught.
func ygLwRenameFails(t *testing.T, schema string, stdout []byte, keys ...string) {
	t.Helper()
	for _, key := range keys {
		t.Run("rename "+key, func(t *testing.T) {
			// The copy must pass before the rename, or a failure proves nothing.
			ygLwValid(t, schema, stdout)
			renamed := ygLwEdit(t, stdout, func(doc map[string]any) {
				if _, ok := doc[key]; !ok {
					t.Fatalf("stdout has no %q: %s", key, stdout)
				}
				doc[key+"_renamed"] = doc[key]
				delete(doc, key)
			})
			if err := contract.Validate(schema, renamed); err == nil {
				t.Fatalf("%s accepts %s renamed: %s", schema, key, renamed)
			}
		})
	}
}

func ygLwSchemaDoc(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := contract.Schema(name)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func ygLwKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// A task never leased, a task held through `lease take task --hold` (as hold_test.go
// does), and a task whose lease was released each answer lease status with output the
// schema accepts, and the fields the app reads cannot be renamed unnoticed.
func TestYgLeaseStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the task lease needs a Unix host")
	}
	h, task := feedHarness(t)
	who := []string{"--member", "member", "--role", "member"}
	run := func(args ...string) []byte {
		return ygLwRun(t, h, task, append(append([]string{}, who...), args...)...)
	}
	holder := "member@" + feedMachine
	var held []byte

	t.Run("free", func(t *testing.T) {
		doc := ygLwValid(t, "cmd-lease-status", run("lease", "status", "task", "--json"))
		if doc["holder"] != nil || doc["process"] != "free" || doc["fence"] != float64(0) {
			t.Fatalf("a task never leased: %v", doc)
		}
	})
	t.Run("held", func(t *testing.T) {
		var tenure lease.Tenure
		if err := json.Unmarshal(run("lease", "take", "task", "--hold", "--json"), &tenure); err != nil {
			t.Fatal(err)
		}
		held = run("lease", "status", "task", "--json")
		doc := ygLwValid(t, "cmd-lease-status", held)
		if doc["holder"] != holder || doc["fence"] != float64(tenure.Fence) || doc["gen"] != float64(tenure.Gen) || tenure.Fence < 1 {
			t.Fatalf("lease status disagrees with lease take %+v: %v", tenure, doc)
		}
	})
	if held == nil {
		t.Fatal("no held lease status to test further")
	}
	t.Run("app fields", func(t *testing.T) {
		ygLwRenameFails(t, "cmd-lease-status", held, "holder", "gen", "fence")
	})
	t.Run("every field", func(t *testing.T) {
		var doc map[string]any
		if err := json.Unmarshal(held, &doc); err != nil {
			t.Fatal(err)
		}
		ygLwRenameFails(t, "cmd-lease-status", held, ygLwKeys(doc)...)
	})
	t.Run("process", func(t *testing.T) {
		states := []string{"free", "remote"}
		for _, state := range processStates {
			states = append(states, state)
		}
		for _, state := range states {
			out := ygLwEdit(t, held, func(doc map[string]any) { doc["process"] = state })
			if err := contract.Validate("cmd-lease-status", out); err != nil {
				t.Errorf("process %q: %v", state, err)
			}
		}
		out := ygLwEdit(t, held, func(doc map[string]any) { doc["process"] = "running" })
		if contract.Validate("cmd-lease-status", out) == nil {
			t.Error("process outside the states is accepted")
		}
	})
	t.Run("released", func(t *testing.T) {
		var doc map[string]any
		if err := json.Unmarshal(held, &doc); err != nil {
			t.Fatal(err)
		}
		epoch := fmt.Sprintf("%d.%d", int64(doc["gen"].(float64)), int64(doc["fence"].(float64)))
		run("lease", "release", "task", "--epoch", epoch)
		doc = ygLwValid(t, "cmd-lease-status", run("lease", "status", "task", "--json"))
		if doc["holder"] != nil || doc["process"] != "free" || doc["fence"].(float64) < 1 {
			t.Fatalf("a released task keeps its fence and has no holder: %v", doc)
		}
	})
}

// cmd-lease-status cannot $ref lease.json: the validator checks the target's
// additionalProperties false against the whole value, which also holds `process`. So
// the fields are written out, and this keeps them equal to the task branch of
// lease.json plus process. fence and since are the two fields cmd-lease-status
// loosens, for a task never leased (task_lease.go writes fence 0 and since "").
func TestYgLeaseStatusFieldsMatchLeaseJSON(t *testing.T) {
	branches, _ := ygLwSchemaDoc(t, "lease")["oneOf"].([]any)
	if len(branches) == 0 {
		t.Fatal("lease.json has no oneOf")
	}
	task, _ := branches[0].(map[string]any)
	leaseProps, _ := task["properties"].(map[string]any)
	leaseRequired, _ := task["required"].([]any)
	cmd := ygLwSchemaDoc(t, "cmd-lease-status")
	cmdProps, _ := cmd["properties"].(map[string]any)
	cmdRequired, _ := cmd["required"].([]any)

	want := append(ygLwKeys(leaseProps), "process")
	sort.Strings(want)
	if got := ygLwKeys(cmdProps); !reflect.DeepEqual(got, want) {
		t.Errorf("cmd-lease-status fields %v; lease.json task branch plus process %v", got, want)
	}
	required := []string{}
	for _, key := range cmdRequired {
		required = append(required, key.(string))
	}
	sort.Strings(required)
	if !reflect.DeepEqual(required, want) {
		t.Errorf("cmd-lease-status required %v; every field %v", required, want)
	}
	if len(leaseRequired) != len(leaseProps) {
		t.Errorf("lease.json task branch requires %d of %d fields; recheck the choice in cmd-lease-status", len(leaseRequired), len(leaseProps))
	}
	if cmd["additionalProperties"] != false {
		t.Error("cmd-lease-status allows unknown fields")
	}
	for _, key := range ygLwKeys(leaseProps) {
		if key == "fence" || key == "since" {
			continue
		}
		if !reflect.DeepEqual(cmdProps[key], leaseProps[key]) {
			t.Errorf("%s differs between cmd-lease-status and lease.json: %v / %v", key, cmdProps[key], leaseProps[key])
		}
	}
}

// where --json in a project folder with a task, without a task, and outside any
// project answers with output the schema accepts; folder and task_path, which the
// app reads, cannot be renamed unnoticed, and every rule where can name is accepted.
func TestYgWhere(t *testing.T) {
	h := newHarness(t)
	for _, key := range strings.Fields("GRID_AGENT GRID_WORKER_ID GRID_WORKER GRID_LEASE GRID_LEASE_EPOCH GRID_WAKE GRID_FEED_TASK GRID_JOB GRID_ATTEMPT GRID_SESSION_ID CLAUDE_SESSION_ID GRID_OWNER") {
		t.Setenv(key, "")
	}
	for _, key := range []string{"GRID_MEMBER", "GRID_ACTOR", "GRID_WHO"} {
		t.Setenv(key, "member")
	}
	t.Setenv("GRID_ROLE", "owner")
	checkout := filepath.Join(h.root, "checkout")
	h.mkdir(checkout)
	testgit.Init(t, checkout, "https://example.com/org/example.git", true)
	ygLwRun(t, h, checkout, "project", "add", "--name", "example")
	h.env.Machine = gridhome.Machine(h.gridHome)
	var made struct{ ID, Path string }
	if err := json.Unmarshal(ygLwRun(t, h, checkout, "task", "new", "Example task", "--json"), &made); err != nil {
		t.Fatal(err)
	}
	var withTask []byte

	t.Run("task", func(t *testing.T) {
		withTask = ygLwRun(t, h, checkout, "where", "--json", "--task", made.ID)
		doc := ygLwValid(t, "cmd-where", withTask)
		if doc["task"] == "" || doc["folder"] == "" || doc["record"] != true || filepath.Base(doc["task_path"].(string)) != filepath.Base(made.Path) {
			t.Fatalf("where --task %s: %v", made.ID, doc)
		}
	})
	t.Run("no task", func(t *testing.T) {
		doc := ygLwValid(t, "cmd-where", ygLwRun(t, h, checkout, "where", "--json"))
		if doc["task"] != "" || doc["task_path"] != "" || doc["folder"] == "" || doc["record"] != true {
			t.Fatalf("where in the project folder: %v", doc)
		}
	})
	t.Run("no project", func(t *testing.T) {
		plain := filepath.Join(h.root, "plain")
		h.mkdir(plain)
		doc := ygLwValid(t, "cmd-where", ygLwRun(t, h, plain, "where", "--json"))
		if doc["record"] != false || doc["task"] != "" {
			t.Fatalf("where outside any project: %v", doc)
		}
	})
	if withTask == nil {
		t.Fatal("no where --task stdout to test further")
	}
	t.Run("app fields", func(t *testing.T) {
		ygLwRenameFails(t, "cmd-where", withTask, "folder", "task_path")
	})
	t.Run("every field", func(t *testing.T) {
		var doc map[string]any
		if err := json.Unmarshal(withTask, &doc); err != nil {
			t.Fatal(err)
		}
		ygLwRenameFails(t, "cmd-where", withTask, ygLwKeys(doc)...)
	})
	t.Run("rules", func(t *testing.T) {
		for _, rule := range []where.Rule{where.RuleExplicit, where.RuleTask, where.RuleRecord, where.RuleMultiRepo, where.RuleGit, where.RuleFolder, where.RulePersonal} {
			out := ygLwEdit(t, withTask, func(doc map[string]any) { doc["rule"] = string(rule) })
			if err := contract.Validate("cmd-where", out); err != nil {
				t.Errorf("rule %q: %v", rule, err)
			}
		}
		out := ygLwEdit(t, withTask, func(doc map[string]any) { doc["rule"] = "guess" })
		if contract.Validate("cmd-where", out) == nil {
			t.Error("a rule where does not have is accepted")
		}
	})
}
