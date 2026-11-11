package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// Arguments are explicit so a new inventory route cannot silently get a vacuous
// usage-error check. Selectors are added separately for every supported form.
var guestParityCommands = map[string][]string{
	"agent set":            {"grid-builder", "--model", "opus"},
	"agent unset":          {"grid-builder"},
	"claude settings show": {},
	"answer":               {"{task}", "Q-01", "parity"},
	"approve":              {"{task}"},
	"artifact import":      {"{file}"},
	"artifact link":        {"https://example.invalid/parity"},
	"artifact list":        {},
	"artifact publish":     {"A-01"},
	"brain accept":         {"parity"},
	"brain changes":        {},
	"brain check":          {},
	"brain decide":         {"{task}", "parity", "accept"},
	"brain index":          {"--read-only"},
	"brain keep":           {},
	"brain revert":         {"parity/parity", "--to", "1"},
	"brain supersede":      {"parity", "--by", "parity-v2", "--reason", "parity reason here"},
	"brain propose":        {"--name", "parity", "--topic", "general", "--description", "parity", "--summary", "parity", "--body", "parity", "--external"},
	"brain search":         {"parity"},
	"brain show":           {"parity"},
	"brain topics":         {},
	"deliver":              {"--outcome", "parity"},

	"feed fold":     {},
	"feed handover": {},
	"feed post":     {"--type", "post", "--body", "{}"},
	"feed read":     {},
	"feed state":    {},
	"feed tick":     {},
	"feed types":    {},
	"history add":   {"parity"},
	"hook level":    {"home-allow", "off"},

	"intent claim":       {"parity"},
	"intent done":        {"parity", "--claim", "parity"},
	"intent list":        {},
	"intent release":     {"parity", "--claim", "parity"},
	"intent renew":       {"parity", "--claim", "parity"},
	"job ack":            {},
	"job blocked":        {},
	"job cancel":         {},
	"job lint":           {"{file}"},
	"job order":          {},
	"job next":           {},
	"job post":           {},
	"job render":         {"{task}", "I-01"},
	"job result":         {},
	"job show":           {"J-01"},
	"job trace":          {"parity.md"},
	"job tree":           {},
	"job verify":         {"J-01"},
	"land":               {"{task}", "--dry-run"},
	"lease release":      {"task", "--epoch", "0.1"},
	"lease status":       {"task"},
	"lease take":         {"task", "--session", "parity"},
	"person approvers":   {"--area", "code"},
	"person list":        {},
	"person show":        {"gus"},
	"plan check":         {},
	"plan diff":          {"1", "2"},
	"plan hash":          {},
	"plan items":         {},
	"plan migrate":       {"--dry-run"},
	"plan write":         {"{file}"},
	"project relink":     {"{project}", "{cwd}"},
	"project remove":     {"{project}"},
	"project repo":       {"add", "{project}", "--where", "/parity/app"},
	"project-md get":     {"{project}", "id"},
	"project-md set":     {"{project}", "title", "parity"},
	"proof add":          {"I-01", "--text", "parity"},
	"proof index":        {},
	"question add":       {"parity?", "--applies", "2030-01-01T00:00:00Z", "--default", "defer"},
	"question ask":       {"parity?", "--applies", "2030-01-01T00:00:00Z", "--default", "defer"},
	"question list":      {},
	"question sync":      {},
	"question tick":      {},
	"question withdraw":  {"Q-01", "--reason", "parity"},
	"routine add":        {"parity", "--every", "1h", "--run", "grid version"},
	"routine list":       {},
	"routine remove":     {"parity"},
	"routine run":        {"parity"},
	"routine schedule":   {},
	"routine unschedule": {},
	"session handback":   {},
	"session list":       {},
	"session pause":      {},
	"session resume":     {},
	"session send":       {"parity"},
	"session show":       {"parity"},
	"session start":      {},
	"session stop":       {},
	"session switch":     {"grid-builder"},
	"setup":              {"{project}", "--answers", "-"},
	"signal act":         {"S-01ARZ3NDEKTSV4RRFFQ69G5FAV", "--act", "note"},
	"signal add":         {"--title", "parity"},
	"signal dismiss":     {"S-01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	"signal move":        {"S-01ARZ3NDEKTSV4RRFFQ69G5FAV", "--to", "alpha"},
	"skill disable":      {"research"},
	"skill enable":       {"research"},
	"task attach":        {"{task}", "--session", "parity", "--runtime", "claude"},
	"task close":         {"{task}"},
	"task decline":       {"{task}", "--reason", "parity"},
	"task delete":        {"{task}"},
	"task history":       {"{task}"},
	"task lane":          {"{task}", "doing"},
	"task list":          {},
	"task move":          {"parity-worker"},
	"task new":           {"parity"},
	"task rename":        {"{task}", "--title", "parity"},
	"task reopen":        {"{task}"},
	"task restore":       {"{task}"},
	"task show":          {"{task}"},
	"task status":        {"{task}", "waiting"},
	"task stop":          {"{task}"},
	"usage":              {},
	"usage post":         {"--job", "J-01", "--tokens", "1"},
	"usage show":         {},
	"open":               {},
	"where":              {},
	"workflow show":      {},
	"workflow write":     {"--file", "{file}", "--rev", "none"},
}

// These --note options are text/reasons, not note locators. They still get a
// separate row with the task selector, so their project resolution is exercised.
// Any future --note flag needs an explicit classification here.
var guestParityNoteText = map[string]bool{
	"approve":      true,
	"brain decide": true, "proof add": true,
}

type guestParityRow struct {
	command, form string
	args          []string
	json          bool
}

type guestParityOutput struct {
	Code           int
	Stdout, Stderr string
}

func guestParityRows(t *testing.T) []guestParityRow {
	t.Helper()
	var doc contractCommandDocument
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	// The generated inventory misses these operand-dependent parsers. Keep the
	// supplement explicit rather than dropping the real project-taking forms.
	for i := range doc.Commands {
		c := &doc.Commands[i]
		if c.Name == "setup" {
			c.Subcommands[0].Args = []string{"<project>"}
		}
		if c.Name == "hook" {
			for j := range c.Subcommands {
				sub := &c.Subcommands[j]
				if sub.Name == "level" {
					sub.Flags = append(sub.Flags, contractCommandFlag{Name: "--project"})
					sub.JSON = true
				}
			}
		}
	}
	var rows []guestParityRow
	used := map[string]bool{}
	for _, c := range doc.Commands {
		for _, sub := range c.Subcommands {
			command := strings.TrimSpace(c.Name + " " + sub.Name)
			flags := map[string]bool{}
			for _, f := range sub.Flags {
				flags[f.Name] = true
			}
			projectArg, taskArg := false, false
			for _, arg := range sub.Args {
				projectArg = projectArg || strings.Contains(arg, "project") || c.Name == "project" && strings.Contains(arg, "name-or-id")
				taskArg = taskArg || strings.Contains(arg, "task") || strings.Contains(arg, "T-NN")
			}
			// eval run's "task" is an evaluation suite case, not a Grid task.
			if command == "eval run" {
				continue
			}
			if !flags["--project"] && !flags["--task"] && !flags["--note"] && !projectArg && !taskArg {
				continue
			}
			base, ok := guestParityCommands[command]
			if !ok {
				t.Errorf("missing guest parity command: %s", command)
				continue
			}
			used[command] = true
			add := func(form string, args []string) {
				rows = append(rows, guestParityRow{command, form, append(strings.Fields(command), args...), sub.JSON})
			}
			if flags["--project"] {
				args := append(slices.Clone(base), "--project", "{project}")
				add("project", args)
				if flags["--task"] && !slices.Contains(base, "{task}") {
					add("project-task", append(slices.Clone(args), "--task", "{task}"))
				}
			}
			if flags["--task"] && command != "job render" {
				args := slices.Clone(base)
				if !slices.Contains(args, "{task}") {
					args = append(args, "--task", "{task}")
				}
				add("task", args)
			}
			// The lease operand is the word task, not a task id; the task-lease rows below cover it.
			if (projectArg || taskArg) && c.Name != "lease" {
				args := slices.Clone(base)
				if !slices.Contains(args, "{task}") && taskArg {
					// Task-first shorthand is independent of the --task form.
					if command == "task move" {
						args = []string{"{task}", "--to", "parity-worker"}
					} else if command != "task restore" {
						args = append([]string{"{task}"}, args...)
					}
				}
				add("operand", args)
			}
			if command == "signal act" {
				for _, act := range []string{"turn", "add", "snooze", "dismiss", "wake", "reopen"} {
					args := []string{"S-01ARZ3NDEKTSV4RRFFQ69G5FAV", "--act", act, "--project", "{project}"}
					if act == "add" {
						args = append(args, "--task", "{task}")
					}
					if act == "snooze" {
						args = append(args, "--until", "2030-01-01T00:00:00Z")
					}
					add("act-"+act, args)
				}
			}
			if c.Name == "lease" {
				// The task lease dispatch has its own flags, and also resolves through a
				// project.
				args := []string{"task", "--task", "{task}"}
				if sub.Name == "take" {
					args = append(args, "--session", "parity")
				}
				if sub.Name == "release" {
					args = append(args, "--epoch", "0.1")
				}
				add("task-lease", args)
				add("task-lease-project", append(slices.Clone(args), "--project", "{project}"))
			}
			if flags["--note"] {
				if !guestParityNoteText[command] {
					t.Errorf("classify --note resolution for %s", command)
					continue
				}
				args := append(slices.Clone(base), "--note", "parity")
				if flags["--task"] {
					args = append(args, "--task", "{task}")
				}
				add("note", args)
			}
		}
	}
	for command := range guestParityCommands {
		if !used[command] {
			t.Errorf("stale guest parity command: %s", command)
		}
	}
	// job show and job lint also take a task file by path, with no --task: the
	// task folder the file lies in decides who may read it.
	for _, command := range []string{"job show", "job lint"} {
		rows = append(rows, guestParityRow{command, "file", append(strings.Fields(command), "{task-file}"), true})
	}
	// Name/ID and task ID/path enter different resolver branches.
	originals := slices.Clone(rows)
	for _, row := range originals {
		for _, variant := range []struct{ from, to, suffix string }{
			{"{project}", "{project-id}", "-project-id"},
			{"{task}", "{task-path}", "-task-path"},
		} {
			if !slices.Contains(row.args, variant.from) {
				continue
			}
			args := slices.Clone(row.args)
			for i := range args {
				if args[i] == variant.from {
					args[i] = variant.to
				}
			}
			rows = append(rows, guestParityRow{row.command, row.form + variant.suffix, args, row.json})
		}
	}
	return rows
}

func guestParityFixture(t *testing.T, h *harness, state string) {
	t.Helper()
	if err := os.RemoveAll(h.gridHome); err != nil {
		t.Fatal(err)
	}
	h.mkdir(h.gridHome)
	h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
	h.write(filepath.Join(h.root, "input.md"), approvedPlan)
	h.env.Getenv = func(key string) string {
		return map[string]string{"GRID_ROLE": "guest", "GRID_MEMBER": "gus"}[key]
	}
	h.env.Stdin = strings.NewReader(`{"test":"make test"}`)
	if state == "unknown" {
		return
	}
	h.record(project.Record{Name: "alpha", ID: "id-alpha", Folders: []project.Folder{{Machine: h.env.Machine, Path: h.root}}})
	dir := filepath.Join(h.gridHome, "projects", "alpha")
	if state != "no-roles" {
		member := "someone-else"
		if state == "invited" {
			member = "gus"
		}
		h.write(filepath.Join(dir, "project-roles.json"), fmt.Sprintf(`{"people":[{"id":%q,"perm":"Client"}]}`, member))
	}
	taskDir := filepath.Join(dir, "tasks", "T-01-parity")
	h.write(filepath.Join(taskDir, "task.md"), "---\nid: T-01\ntask_id: 01ARZ3NDEKTSV4RRFFQ69G5FAV\nproject: alpha\ntitle: parity\nstate: working\nsize: small\n---\n")
	h.write(filepath.Join(taskDir, "plan.md"), approvedPlan)
	h.write(filepath.Join(dir, "workflows.md"), cliWorkflow)
}

// Run is the real CLI entry called by cmd/grid/main.go. All dispatch, parsing,
// origin gates and project resolution run here; no command handler is called.
// Reusing the same selector and paths while changing only project visibility
// permits byte-for-byte comparisons without normalizing paths, IDs or times.
func TestGuestProjectParity(t *testing.T) {
	// Some Feed adapters read these directly rather than through Env.Getenv.
	for _, key := range []string{"GRID_WORKER", "GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_JOB", "GRID_WAKE", "GRID_ATTEMPT", "GRID_OWNER"} {
		t.Setenv(key, "")
	}
	t.Setenv("GRID_ROLE", "guest")
	t.Setenv("GRID_MEMBER", "gus")
	for _, row := range guestParityRows(t) {
		modes := []string{"plain"}
		if row.json {
			modes = append(modes, "json")
		}
		for _, mode := range modes {
			t.Run(row.command+"/"+row.form+"/"+mode, func(t *testing.T) {
				h := newHarness(t)
				args := slices.Clone(row.args)
				for i, arg := range args {
					switch arg {
					case "{project}":
						args[i] = "alpha"
					case "{task}":
						args[i] = "T-01"
					case "{project-id}":
						args[i] = "id-alpha"
					case "{task-path}":
						args[i] = filepath.Join(h.gridHome, "projects", "alpha", "tasks", "T-01-parity")
					case "{task-file}":
						args[i] = filepath.Join(h.gridHome, "projects", "alpha", "tasks", "T-01-parity", "plan.md")
					case "{file}":
						args[i] = filepath.Join(h.root, "input.md")
					case "{cwd}":
						args[i] = h.root
					}
				}
				if mode == "json" {
					args = append(args, "--json")
				}
				outputs := map[string]guestParityOutput{}
				for _, state := range []string{"uninvited", "no-roles", "unknown", "invited"} {
					guestParityFixture(t, h, state)
					code := h.run(h.root, args...)
					outputs[state] = guestParityOutput{code, h.out.String(), h.errb.String()}
				}
				unknown := outputs["unknown"]
				t.Run("invited", func(t *testing.T) {
					if outputs["invited"] == unknown && !guestParityEarlyAnswer(row.command, mode, unknown) {
						t.Fatalf("invited guest still gets unknown answer: %+v", unknown)
					}
					// The file is outside the Grid home: lint --task must not read it.
					if row.command == "job lint" && slices.Contains(row.args, "--task") && outputs["invited"] != unknown {
						t.Fatalf("invited guest lint --task read a file outside the Grid home: %+v", outputs["invited"])
					}
				})
				t.Run("parity", func(t *testing.T) {
					if outputs["uninvited"] == unknown && outputs["no-roles"] == unknown {
						return
					}
					t.Fatalf("visibility leak: unknown=%+v uninvited=%+v no-roles=%+v", unknown, outputs["uninvited"], outputs["no-roles"])
				})
			})
		}
	}
}

// Only documented pre-resolution answers may coincide with the invited case.
// Usage errors and ordinary "not found" errors never qualify.
func guestParityEarlyAnswer(command, mode string, out guestParityOutput) bool {
	want := guestParityOutput{}
	// The guest refusals that record guest_refused print its envelope with --json.
	guestRefused := false
	{
		switch command {
		case "routine add", "routine remove", "routine run", "routine schedule", "routine unschedule",
			"project relink", "project remove", "project repo",
			"setup":
			want = guestParityOutput{1, "", "grid: guests cannot change project settings\n"}
			guestRefused = true
		case "signal add":
			// Known open (TestIlGuestSiteList): no code, so no stdout.
			want = guestParityOutput{1, "", "grid: guests cannot change project settings\n"}
		case "task delete", "task restore":
			want = guestParityOutput{3, "", "grid " + command + ": task edit rejected: a person with an allowed role must perform this task move\n"}
			guestRefused = true
		case "task new", "task close", "task reopen", "task stop", "task move":
			want = guestParityOutput{1, "", "grid " + command + ": role \"guest\" may not " + strings.TrimPrefix(command, "task ") + " a task; a guest answers and comments\n"}
			guestRefused = true
		case "task lane":
			want = guestParityOutput{1, "", "grid task lane: role \"guest\" may not move a task to a lane; a guest answers and comments\n"}
			guestRefused = true
		case "session start", "session stop", "session send", "session pause", "session resume", "session handback", "session switch":
			want = guestParityOutput{1, "", "grid " + command + ": role \"guest\" may not " + strings.TrimPrefix(command, "session ") + " a session; a guest answers and comments\n"}
			guestRefused = true
		case "question withdraw":
			want = guestParityOutput{3, "", "question: Q-01 role \"guest\" may not withdraw\n"}
			guestRefused = true
		case "signal act", "signal dismiss", "signal move":
			want = guestParityOutput{3, "", "grid: guests cannot act on signals\n"}
			if mode == "json" {
				want.Stdout = "{\"error\":\"refused\",\"message\":\"guests cannot act on signals\"}\n"
			}
		case "workflow write":
			want = guestParityOutput{3, "", "grid workflow write: caller role is not allowed to write workflows\n"}
			if mode == "json" {
				want.Stdout = "{\"error\":\"person_required\",\"reason\":\"role_not_allowed\",\"hint\":\"caller role is not allowed to write workflows\",\"code\":\"person_required\",\"message\":\"grid workflow write: caller role is not allowed to write workflows\"}\n"
			}
		case "job lint":
			// The row lints a file outside the Grid home with --task: the by-path
			// gate answers as an unknown task whatever the invitation.
			want = guestParityOutput{1, "", "job: task not found\n"}
			if mode == "json" {
				want = guestParityOutput{1, "{\"ok\":false,\"findings\":[],\"error\":\"task not found\"}\n", ""}
			}
		case "brain changes":
			want = guestParityOutput{3, "", "grid brain changes: caller role is not allowed to read brain changes\n"}
			if mode == "json" {
				want.Stdout = `{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to read brain changes","code":"person_required","message":"grid brain changes: caller role is not allowed to read brain changes"}` + "\n"
			}
		case "brain revert":
			want = guestParityOutput{3, "", "grid brain revert: caller role is not allowed to revert brain entries\n"}
			if mode == "json" {
				want.Stdout = `{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to revert brain entries","code":"person_required","message":"grid brain revert: caller role is not allowed to revert brain entries"}` + "\n"
			}
		case "brain supersede":
			want = guestParityOutput{3, "", "grid brain supersede: caller role is not allowed to supersede brain entries\n"}
			if mode == "json" {
				want.Stdout = `{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to supersede brain entries","code":"person_required","message":"grid brain supersede: caller role is not allowed to supersede brain entries"}` + "\n"
			}
		case "feed types":
			// This command returns the public schema without resolving any scope.
			return out.Code == 0 && out.Stderr == "" && strings.Contains(out.Stdout, "knowledge")
		default:
			return false
		}
	}
	if guestRefused && mode == "json" {
		want.Stdout = ilGuestEnvelope(want.Stderr)
	}
	return out == want
}
