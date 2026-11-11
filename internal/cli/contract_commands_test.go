package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/intent"
	"github.com/gridconsole/grid-plugins/internal/lease"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/testgit"

	"github.com/gridconsole/grid-plugins/internal/version"
)

type contractCommandFlag struct {
	Name     string `json:"name"`
	Takes    string `json:"takes"`
	Required bool   `json:"required"`
}
type contractCommandSub struct {
	Name   string                `json:"name"`
	Args   []string              `json:"args"`
	Flags  []contractCommandFlag `json:"flags"`
	JSON   bool                  `json:"json"`
	Output string                `json:"output"`
	Stream bool                  `json:"stream,omitempty"`
}
type contractCommandEntry struct {
	Name        string               `json:"name"`
	Summary     string               `json:"summary"`
	Built       bool                 `json:"built"`
	Subcommands []contractCommandSub `json:"subcommands"`
}
type contractCommandDocument struct {
	Contract string                 `json:"contract"`
	Commands []contractCommandEntry `json:"commands"`
}

func contractCommandWrite(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestContractCommandsInventory(t *testing.T) {
	var doc contractCommandDocument
	dec := json.NewDecoder(bytes.NewReader(contractdata.Commands))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t)
	if code := h.run(h.home, "version", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	var v map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if doc.Contract != v["contract"] {
		t.Fatalf("contract %q, version reports %q", doc.Contract, v["contract"])
	}
	table := commands()
	if len(doc.Commands) != len(table) {
		t.Fatalf("inventory has %d commands, table has %d", len(doc.Commands), len(table))
	}
	outputs := map[string]bool{}
	for i, c := range table {
		got := doc.Commands[i]
		if got.Name != c.name || got.Summary != c.summary || got.Built != (c.run != nil) {
			t.Errorf("entry %d does not match commands(): %+v", i, got)
		}
		if !got.Built && len(got.Subcommands) != 0 {
			t.Errorf("%s is unbuilt but has subcommands", c.name)
		}
		if got.Built && len(got.Subcommands) == 0 {
			t.Errorf("%s is built but has no invocation", c.name)
		}
		seen := map[string]bool{}
		for _, sub := range got.Subcommands {
			if seen[sub.Name] {
				t.Errorf("duplicate %s %s", c.name, sub.Name)
			}
			seen[sub.Name] = true
			hasJSON := false
			flags := map[string]bool{}
			for _, f := range sub.Flags {
				if flags[f.Name] || !strings.HasPrefix(f.Name, "--") || (f.Takes != "value" && f.Takes != "none") {
					t.Errorf("%s %s: invalid flag %+v", c.name, sub.Name, f)
				}
				flags[f.Name] = true
				hasJSON = hasJSON || f.Name == "--json"
			}
			if hasJSON != sub.JSON {
				t.Errorf("%s %s: json disagrees with flags", c.name, sub.Name)
			}
			if sub.Output != "" {
				if !sub.JSON {
					t.Errorf("%s %s: output without JSON", c.name, sub.Name)
				}
				if _, err := contract.Schema(sub.Output); err != nil {
					t.Error(err)
				}
				outputs[strings.TrimSuffix(sub.Output, ".json")] = true
			}
		}
	}
	for _, name := range contract.SchemaNames() {
		if strings.HasPrefix(name, "cmd-") && !outputs[name] {
			t.Errorf("output schema %s is not in commands.json", name)
		}
	}
}

func TestContractCommandsApp(t *testing.T) {
	names := []string{"version", "task-show", "task-list", "task-move", "plan-hash", "feed-state", "feed-post", "job-next", "approve", "session-list", "session-show", "session-start", "session-stop", "session-send", "session-pause", "session-resume", "session-handback", "session-switch", "intent-list", "worker-list"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for _, variant := range []string{"basic", "populated"} {
				t.Run(variant, func(t *testing.T) {
					populated := variant == "populated"
					h := newHarness(t)
					for _, key := range strings.Fields("GRID_AGENT GRID_WORKER_ID GRID_WORKER GRID_LEASE GRID_LEASE_EPOCH GRID_WAKE GRID_FEED_TASK GRID_JOB GRID_ATTEMPT GRID_SESSION_ID CLAUDE_SESSION_ID GRID_OWNER GRID_APP_VERSION GRID_RELEASE_MANIFEST_URL") {
						t.Setenv(key, "")
					}
					for _, key := range []string{"GRID_MEMBER", "GRID_ACTOR", "GRID_WHO"} {
						t.Setenv(key, "member")
					}
					t.Setenv("GRID_ROLE", "owner")
					h.env.Machine = feedMachine
					// Use the shared harness; tasks and plans are made by their CLI writers.
					run := func(cwd string, args ...string) []byte {
						t.Helper()
						if code := h.run(cwd, args...); code != 0 {
							t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
						}
						return append([]byte(nil), h.out.Bytes()...)
					}
					cwd := h.home
					var out []byte
					if name == "version" {
						if populated {
							old := version.Version
							version.Version = "1.2.3"
							t.Cleanup(func() { version.Version = old })
						}
						out = run(cwd, "version", "--json")
						if populated {
							checked := run(cwd, "version", "--check", "--json")
							if err := contract.Validate("cmd-version", checked); err != nil {
								t.Fatal(err)
							}
							if os.Getenv("UPDATE_CONTRACT_COMMAND_FIXTURES") == "1" {
								var value any
								if err := json.Unmarshal(checked, &value); err != nil {
									t.Fatal(err)
								}
								contractCommandWrite(t, filepath.Join("..", "..", "contract", "fixtures", "cmd-version", "valid", "check.json"), value)
							}
						}
					} else if name == "worker-list" {
						if populated {
							run(cwd, "task", "new", "Example worker task", "--json")
							// Presence is app-owned input; the plugin has no worker writer. A status file with no held lock is a stopped worker.
							h.write(filepath.Join(h.gridHome, "worker-status.json"), `{"pid":42,"version":"1.2.3","started_at":"2026-01-01T00:00:00Z","uptime_seconds":1,"address":"127.0.0.1:43111","token":"x"}`)
						}
						out = run(cwd, "worker", "list", "--json")
					} else if name == "feed-state" || name == "feed-post" || name == "job-next" {
						h, out = contractCommandFeed(t, name, populated)
					} else {
						if name == "task-move" {
							cwd = filepath.Join(h.root, "checkout")
							h.mkdir(cwd)
							testgit.Init(t, cwd, "https://example.com/org/example.git", true)
							run(cwd, "project", "add", "--name", "example")
							h.env.Machine = gridhome.Machine(h.gridHome)
						}
						created := run(cwd, "task", "new", "Example task", "--json")
						var made struct{ ID, Path string }
						if err := json.Unmarshal(created, &made); err != nil {
							t.Fatal(err)
						}
						cwd = made.Path
						writePlan := func() {
							source := filepath.Join(h.root, "proposal.md")
							h.write(source, planFixture(t, "plan.md"))
							run(cwd, "plan", "write", source, "--json")
						}
						switch name {
						case "task-show":
							if populated {
								writePlan()
							}
							out = run(cwd, "task", "show", "--json")
						case "task-list":
							args := []string{"task", "list", "--json"}
							if !populated {
								args = append(args, "--state", "closed")
							}
							out = run(cwd, args...)
						case "task-move":
							out = run(cwd, "task", "move", "--to", "worker-example", "--json")
						case "plan-hash":
							writePlan()
							if populated {
								source := filepath.Join(h.root, "revision.md")
								h.write(source, strings.Replace(planFixture(t, "plan.md"), "tag filter", "example filter", 1))
								run(cwd, "plan", "write", source)
							}
							out = run(cwd, "plan", "hash", "--json")
						case "approve":
							writePlan()
							run(cwd, "task", "status", made.ID, "waiting")
							args := []string{"approve", made.ID, "--json"}
							if populated {
								args = append(args, "--note", "Reviewed")
							}
							out = run(cwd, args...)
						case "session-list", "session-show":
							if populated || name == "session-show" {
								run(cwd, "task", "attach", made.ID, "--session", "session-example", "--json")
							}
							args := []string{"session", strings.TrimPrefix(name, "session-")}
							if name == "session-show" {
								args = append(args, "session-example")
							}
							out = run(cwd, append(args, "--json")...)
						case "intent-list":
							if populated {
								record := run(cwd, "session", "send", "Example message", "--idempotency-key", "example-request", "--json")
								var r intent.Record
								if err := json.Unmarshal(record, &r); err != nil {
									t.Fatal(err)
								}
								claim := run(cwd, "intent", "claim", r.ID, "--json")
								var c intent.Claim
								if err := json.Unmarshal(claim, &c); err != nil {
									t.Fatal(err)
								}
								run(cwd, "intent", "done", r.ID, "--token", c.Token, "--error=example failure", "--json")
							}
							out = run(cwd, "intent", "list", "--json")
						default:
							verb := strings.TrimPrefix(name, "session-")
							args := []string{"session", verb}
							if verb == "send" {
								args = append(args, "Example message")
							}
							if verb == "switch" {
								args = append(args, "builder")
							}
							if populated {
								args = append(args, "--target", "session-example", "--who", "app")
							}
							if populated && verb == "send" {
								args = append(args, "--idempotency-key", "example-request")
							}
							if populated && verb == "start" {
								args = append(args, "--agent", "builder")
							}
							out = run(cwd, append(args, "--json")...)
						}
					}
					schema := "cmd-" + name
					// Validate raw stdout before normalization; every stream line is a value.
					lines := bufio.NewScanner(bytes.NewReader(out))
					count := 0
					for lines.Scan() {
						count++
						if err := contract.Validate(schema, lines.Bytes()); err != nil {
							t.Fatalf("%s: %v\n%s", schema, err, out)
						}
					}
					if err := lines.Err(); err != nil {
						t.Fatal(err)
					}
					if count != 1 {
						t.Fatalf("expected one current output line, got %d", count)
					}
					if os.Getenv("UPDATE_CONTRACT_COMMAND_FIXTURES") == "1" {
						var value any
						if err := json.Unmarshal(out, &value); err != nil {
							t.Fatal(err)
						}
						value = contractCommandNormalize(h, value)
						contractCommandWrite(t, filepath.Join("..", "..", "contract", "fixtures", schema, "valid", variant+".json"), value)
					}
				})
			}
		})
	}
}

func contractCommandNormalize(h *harness, v any) any {
	switch x := v.(type) {
	case string:
		return strings.ReplaceAll(filepath.ToSlash(x), filepath.ToSlash(h.root), "<ROOT>")
	case []any:
		for i, item := range x {
			x[i] = contractCommandNormalize(h, item)
		}
	case map[string]any:
		for k, item := range x {
			if k == "pid" {
				x[k] = 42
				continue
			}
			if k == "pid_start" {
				x[k] = "example-start"
				continue
			}
			x[k] = contractCommandNormalize(h, item)
		}
	}
	return v
}

// Feed calls use the existing stable-ID task fixture until task new owns that ID.
func contractCommandFeed(t *testing.T, name string, populated bool) (*harness, []byte) {
	t.Helper()
	h, task := feedHarness(t)
	h.env.Machine = feedMachine
	run := func(args ...string) []byte {
		t.Helper()
		if code := h.run(task, args...); code != 0 {
			t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
		}
		return append([]byte(nil), h.out.Bytes()...)
	}
	if name == "feed-post" {
		args := []string{"feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"Example message"}`, "--json"}
		if populated {
			args = append(args, "--job", "J-1", "--attempt", "1")
		}
		return h, run(args...)
	}
	if populated || name == "job-next" {
		store := feed.Store{TaskDir: task, TaskID: feedTaskID}
		writer := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
		// Task tenure has no CLI writer; all Feed records below go through the CLI.
		grant, err := (lease.TaskStore{Feed: store}).TakeTask(writer, os.Getpid(), contract.FormatTime(time.Now()))
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", grant.Fence))
		run("feed", "handover", "--reason", "Example handover", "--lease", "task", "--json")
		if populated {
			body, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
			if err != nil {
				t.Fatal(err)
			}
			run("job", "post", "J-1", "--type", "job", "--body", string(body), "--json")
			run("feed", "post", "--type", "question", "--body", `{"question":"Continue?","who":"owner"}`, "--job", "J-1", "--lease", "task", "--json")
		}
	}
	if name == "feed-state" {
		return h, run("feed", "state", "--json")
	}
	if populated {
		// The app's waiting call emits the assignment immediately and exits.
		return h, run("job", "next", "--wait", "--json")
	}
	return h, run("job", "next", "--json")
}

// This byte-for-byte baseline was captured before adding any cmd- schema.
// It deliberately does not derive its expected registrations from the schemas.
func TestContractCommandsFeedTypes(t *testing.T) {
	const want = `{"types":[{"type":"ack","version":1,"class":"state","roles":["worker","agent","cli"]},{"type":"answer","version":1,"class":"state","roles":["owner","member","lead","agent","worker","reviewer","curator","host","cli","guest","external"]},{"type":"checkpoint","version":1,"class":"state","roles":["worker","agent","cli"]},{"type":"control","version":1,"class":"state","roles":["lead","owner","member","cli","guest"]},{"type":"deploy-failed","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"found","version":1,"class":"chatter","roles":["reviewer","lead","worker","agent","cli"]},{"type":"handover","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"head","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"host","version":1,"class":"state","roles":["host","cli","lead"]},{"type":"job","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"knowledge","version":1,"class":"chatter","roles":["owner","member","lead","agent","worker","reviewer","curator","host","cli","guest","external"]},{"type":"loop-detected","version":1,"class":"state","roles":["cli","worker","lead"]},{"type":"post","version":1,"class":"chatter","roles":["owner","member","lead","agent","worker","reviewer","curator","host","cli","guest","external"]},{"type":"question","version":1,"class":"state","roles":["lead","owner","member","worker","agent","cli"]},{"type":"result","version":2,"class":"state","roles":["worker","agent","cli"]},{"type":"submit","version":1,"class":"state","roles":["worker","agent","cli"]},{"type":"transfer","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"usage-notice","version":1,"class":"chatter","roles":["cli"]},{"type":"worker-exit","version":1,"class":"state","roles":["worker","agent","cli"]}]}
`
	h := newHarness(t)
	if code := h.run(h.home, "feed", "types", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	if h.out.String() != want {
		t.Fatalf("feed types changed:\n%s", h.out)
	}
	var baseline struct {
		Types []contract.PostType `json:"types"`
	}
	if err := json.Unmarshal([]byte(want), &baseline); err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(contract.PostTypes())
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(baseline.Types)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("PostTypes changed: %s", got)
	}
	for _, name := range contract.SchemaNames() {
		if !strings.HasPrefix(name, "cmd-") {
			continue
		}
		data, err := contract.Schema(name)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err = json.Unmarshal(data, &schema); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"x-class", "x-roles"} {
			if _, ok := schema[key]; ok {
				t.Errorf("%s has %s", name, key)
			}
		}
	}
}

// Probe every fixed object reached by a captured output, including array items,
// referenced envelopes and nested records. Dictionaries keep their dynamic keys.
func TestContractCommandsRejectUnknownFields(t *testing.T) {
	for _, name := range contract.SchemaNames() {
		if !strings.HasPrefix(name, "cmd-") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			data, err := contract.Schema(name)
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			if err = json.Unmarshal(data, &schema); err != nil {
				t.Fatal(err)
			}
			fixtures, err := filepath.Glob(filepath.Join("..", "..", "contract", "fixtures", name, "valid", "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			mutations := 0
			for _, path := range fixtures {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err = contract.Validate(name, data); err != nil {
					t.Fatal(err)
				}
				var root any
				if err = json.Unmarshal(data, &root); err != nil {
					t.Fatal(err)
				}
				var walk func(map[string]any, any)
				walk = func(s map[string]any, v any) {
					if ref, ok := s["$ref"].(string); ok {
						b, err := contract.Schema(ref)
						if err != nil {
							t.Fatal(err)
						}
						var target map[string]any
						if err = json.Unmarshal(b, &target); err != nil {
							t.Fatal(err)
						}
						walk(target, v)
					}
					for _, key := range []string{"allOf", "anyOf", "oneOf"} {
						branches, _ := s[key].([]any)
						for _, branch := range branches {
							walk(branch.(map[string]any), v)
						}
					}
					switch value := v.(type) {
					case map[string]any:
						if s["additionalProperties"] == false {
							value["__contract_extra"] = true
							b, err := json.Marshal(root)
							if err != nil {
								t.Fatal(err)
							}
							if contract.Validate(name, b) == nil {
								t.Errorf("%s accepts an undocumented object field", path)
							}
							delete(value, "__contract_extra")
							mutations++
						}
						props, _ := s["properties"].(map[string]any)
						for k, item := range value {
							child, ok := props[k].(map[string]any)
							if !ok {
								child, _ = s["additionalProperties"].(map[string]any)
							}
							if child != nil {
								walk(child, item)
							}
						}
					case []any:
						child, ok := s["items"].(map[string]any)
						if ok {
							for _, item := range value {
								walk(child, item)
							}
						}
					}
				}
				walk(schema, root)
			}
			if mutations == 0 {
				t.Fatal("no populated fixed object fixture was checked")
			}
		})
	}
}
