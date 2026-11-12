package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// ymStVariant is one run of a command on a fresh harness home; fixture names the
// valid fixture file the run writes under UPDATE_YM_FIXTURES=1.
type ymStVariant struct {
	fixture string
	invalid bool // also write the invalid fixtures from this run
	setup   func(t *testing.T, h *harness)
	args    []string
}

// ymStCase is one command, the schema its stdout must pass, and the fields the
// app reads: renaming or retyping the last element of each path must fail Validate.
type ymStCase struct {
	name       string
	schema     string
	read       [][]string
	renameOnly [][]string // any JSON value: only a rename can fail
	variants   []ymStVariant
}

func ymStHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	for _, key := range strings.Fields("GRID_AGENT GRID_WORKER_ID GRID_WORKER GRID_LEASE GRID_LEASE_EPOCH GRID_WAKE GRID_FEED_TASK GRID_JOB GRID_ATTEMPT GRID_SESSION_ID CLAUDE_SESSION_ID GRID_OWNER GRID_APP_VERSION GRID_RELEASE_MANIFEST_URL") {
		t.Setenv(key, "")
	}
	for _, key := range strings.Fields("CLAUDE_CODE_EFFORT_LEVEL CLAUDE_CODE_SUBAGENT_MODEL_FORCE CLAUDE_CODE_SUBAGENT_MODEL CLAUDE_CODE_AUTO_COMPACT_WINDOW DISABLE_TELEMETRY DO_NOT_TRACK CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC DISABLE_GROWTHBOOK CLAUDE_CODE_USE_BEDROCK CLAUDE_CODE_USE_VERTEX") {
		t.Setenv(key, "")
	}
	for _, key := range []string{"GRID_MEMBER", "GRID_ACTOR", "GRID_WHO"} {
		t.Setenv(key, "member")
	}
	t.Setenv("GRID_ROLE", "owner")
	// The Claude settings are read from the test's own folder, never the real home.
	t.Setenv("HOME", h.home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, ".claude"))
	h.env.Machine = feedMachine
	return h
}

func ymStRun(t *testing.T, h *harness, args ...string) []byte {
	t.Helper()
	if code := h.run(h.home, args...); code != 0 {
		t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
	}
	return append([]byte(nil), h.out.Bytes()...)
}

// ymStMutate decodes out, applies f to the object that holds the last path element
// and returns the changed document.
func ymStMutate(t *testing.T, out []byte, path []string, f func(m map[string]any, key string)) []byte {
	t.Helper()
	var doc any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	at := doc
	for _, key := range path[:len(path)-1] {
		m, ok := at.(map[string]any)
		if !ok || m[key] == nil {
			t.Fatalf("no %s on the way to %s in %s", key, strings.Join(path, "."), out)
		}
		at = m[key]
	}
	m, ok := at.(map[string]any)
	if !ok {
		t.Fatalf("%s is not under an object in %s", strings.Join(path, "."), out)
	}
	if _, ok := m[path[len(path)-1]]; !ok {
		t.Fatalf("%s is missing from %s", strings.Join(path, "."), out)
	}
	f(m, path[len(path)-1])
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func ymStRename(m map[string]any, key string) { m[key+"Renamed"] = m[key]; delete(m, key) }
// ymStRetype swaps the value for one of another JSON type: a string for a number, a number otherwise.
func ymStRetype(m map[string]any, key string) {
	if _, number := m[key].(float64); number {
		m[key] = "retyped"
		return
	}
	m[key] = 42
}

func ymStPersonalSettings(t *testing.T, h *harness) {
	t.Helper()
	h.write(filepath.Join(h.home, ".claude", "settings.json"), `{"autoCompactWindow":200000,"advisorModel":"opus","permissions":{"defaultMode":"auto"}}`)
}

func TestYmSettingsSchemas(t *testing.T) {
	cases := []ymStCase{
		{name: "skill list", schema: "cmd-skill-list", read: [][]string{{"run-job"}, {"task"}},
			variants: []ymStVariant{
				{fixture: "basic", invalid: true, args: []string{"skill", "list", "--json"}},
				{fixture: "populated", setup: func(t *testing.T, h *harness) { ymStRun(t, h, "skill", "disable", "run-job") }, args: []string{"skill", "list", "--json"}},
			}},
		{name: "skill enable", schema: "cmd-settings-receipt", read: [][]string{{"name"}, {"kind"}, {"project"}},
			variants: []ymStVariant{{fixture: "skill-enable", invalid: true, args: []string{"skill", "enable", "run-job", "--json"}}}},
		{name: "agent set", schema: "cmd-settings-receipt", read: [][]string{{"name"}},
			variants: []ymStVariant{{fixture: "agent-set", args: []string{"agent", "set", "grid-builder", "--model", "opus", "--effort", "high", "--json"}}}},
		{name: "claude settings show", schema: "cmd-claude-settings-show", read: [][]string{{"path"}, {"state"}, {"mainModel", "window"}, {"rows", "advisorModel", "setBy"}, {"codex", "exists"}},
			renameOnly: [][]string{{"rows", "autoCompactWindow", "inForce"}, {"rows", "autoCompactWindow", "now"}},
			variants: []ymStVariant{
				{fixture: "basic", invalid: true, args: []string{"claude", "settings", "show", "--json"}},
				{fixture: "populated", setup: func(t *testing.T, h *harness) {
					ymStPersonalSettings(t, h)
					t.Setenv("DISABLE_TELEMETRY", "1")
				}, args: []string{"claude", "settings", "show", "--json"}},
			}},
		{name: "claude settings set", schema: "cmd-claude-settings-set", read: [][]string{{"key"}, {"written"}},
			renameOnly: [][]string{{"to"}, {"from"}},
			variants: []ymStVariant{
				{fixture: "basic", invalid: true, args: []string{"claude", "settings", "set", "autoCompactWindow", "300000", "--json"}},
				{fixture: "populated", setup: ymStPersonalSettings, args: []string{"claude", "settings", "set", "autoCompactWindow", "300000", "--expect", "200000", "--json"}},
			}},
		{name: "agent budget show", schema: "cmd-agent-budget-show", read: [][]string{{"agents"}, {"presets"}, {"env"}, {"undo"}, {"note", "state"}},
			variants: []ymStVariant{
				{fixture: "basic", invalid: true, args: []string{"agent", "budget", "show", "--json"}},
				{fixture: "populated", setup: func(t *testing.T, h *harness) {
					ymStRun(t, h, "agent", "budget", "apply", "economic", "--json")
					t.Setenv("CLAUDE_CODE_SUBAGENT_MODEL", "sonnet")
				}, args: []string{"agent", "budget", "show", "--json"}},
			}},
	}
	for _, c := range cases {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			for _, v := range c.variants {
				t.Run(v.fixture, func(t *testing.T) {
					h := ymStHarness(t)
					if v.setup != nil {
						v.setup(t, h)
					}
					out := ymStRun(t, h, v.args...)
					if err := contract.Validate(c.schema, out); err != nil {
						t.Fatalf("%s: %v\n%s", c.schema, err, out)
					}
					// The fields the app reads are required and typed: a renamed or retyped copy fails.
					for _, path := range c.read {
						for name, f := range map[string]func(map[string]any, string){"renamed": ymStRename, "retyped": ymStRetype} {
							if err := contract.Validate(c.schema, ymStMutate(t, out, path, f)); err == nil {
								t.Errorf("%s accepts %s %s", c.schema, name, strings.Join(path, "."))
							}
						}
					}
					for _, path := range c.renameOnly {
						if err := contract.Validate(c.schema, ymStMutate(t, out, path, ymStRename)); err == nil {
							t.Errorf("%s accepts renamed %s", c.schema, strings.Join(path, "."))
						}
					}
					if os.Getenv("UPDATE_YM_FIXTURES") != "1" {
						return
					}
					dir := filepath.Join("..", "..", "contract", "fixtures", c.schema)
					write := func(sub, name string, b []byte) {
						var value any
						if err := json.Unmarshal(b, &value); err != nil {
							t.Fatal(err)
						}
						contractCommandWrite(t, filepath.Join(dir, sub, name+".json"), contractCommandNormalize(h, value))
					}
					write("valid", v.fixture, out)
					if v.invalid {
						first := c.read[0]
						write("invalid", "renamed-"+first[len(first)-1], ymStMutate(t, out, first, ymStRename))
						write("invalid", "wrong-type-"+first[len(first)-1], ymStMutate(t, out, first, ymStRetype))
					}
				})
			}
		})
	}
}

// The skill list is one boolean per skill Grid ships; a skill added to the catalog
// without a schema line fails here, not in the app.
func TestYmSkillListSchemaNamesTheCatalog(t *testing.T) {
	data, err := contract.Schema("cmd-skill-list")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
		Required []string `json:"required"`
		Extra    any      `json:"additionalProperties"`
	}
	if err = json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	want := settings.SkillNames()
	var props []string
	for name, p := range schema.Properties {
		props = append(props, name)
		if p.Type != "boolean" {
			t.Errorf("%s is %q, want boolean", name, p.Type)
		}
	}
	slices.Sort(props)
	slices.Sort(schema.Required)
	slices.Sort(want)
	if !slices.Equal(props, want) || !slices.Equal(schema.Required, want) || schema.Extra != false {
		t.Errorf("skill list schema properties %v, required %v, additionalProperties %v; catalog %v", props, schema.Required, schema.Extra, want)
	}
}
