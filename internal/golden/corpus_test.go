package golden_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hook"
)

// legacyHooks maps a recorded script to the hook events whose Go handler
// replaces it. Unknown events silently return 0, so each pair is explicit and
// also has to be an event hook.Handles. Scripts with no Go counterpart on main
// (history-log, brain-read, brain-recall, permission-forward, ...) are absent.
var legacyHooks = map[string][]string{
	"grid-env":        {"SessionStart"},
	"agent-routing":   {"SessionStart"},
	"plan-guard":      {"PreToolUse"},
	"host-guard":      {"PreToolUse"},
	"brief-lint":      {"PreToolUse"},
	"grid-home-allow": {"PreToolUse", "PermissionRequest"},
}

func cliDriver(t *testing.T) golden.Driver {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	return golden.Driver{
		Resolve: func(c golden.Command) ([]string, bool) {
			if len(c.Argv) > 0 {
				// Explicit argv recordings run when the command table builds that command.
				if !cli.Implemented(c.Argv[0]) {
					return nil, false
				}
				return c.Argv, true
			}
			if c.Call != "" || !cli.Implemented("hook") {
				return nil, false
			}
			events := legacyHooks[c.Hook]
			if c.Event == "" && len(events) == 1 {
				c.Event = events[0] // unreadable input.json: the script's only event
			}
			for _, event := range events {
				if event == c.Event && hook.Handles(event) {
					return []string{"hook", strings.ToLower(event)}, true
				}
			}
			return nil, false
		},
		Run: func(in golden.Invocation) golden.Output {
			var stdout, stderr bytes.Buffer
			getenv := func(key string) string { return in.Env[key] }
			home := in.Env["HOME"]
			if home != "" {
				home = gridhome.Real(home)
			}
			gh := gridhome.Dir(getenv, home)
			if gh != "" {
				gh = gridhome.Real(gh)
			}
			code := cli.Run(in.Argv, cli.Env{
				Stdin: strings.NewReader(in.Stdin), Stdout: &stdout, Stderr: &stderr,
				Cwd: gridhome.Real(in.Cwd), Home: home, GridHome: gh, Getenv: getenv,
				Ceiling: in.Root, TempRoots: []string{in.Root}, Machine: gridhome.Machine(gh),
			})
			return golden.Output{Stdout: stdout.String(), Stderr: stderr.String(), Exit: code}
		},
	}
}

func TestGoldenCorpus(t *testing.T) {
	root := filepath.Join("..", "..")
	cases, err := golden.Cases(filepath.Join(root, "golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no recorded cases found")
	}
	known, err := knownDifferences(filepath.Join(root, "golden", "known-differences.txt"), root)
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ name, command, status string }
	var rows []row
	totals := map[string]int{}
	for _, dir := range cases {
		name, err := filepath.Rel(filepath.Join(root, "golden"), dir)
		if err != nil {
			t.Fatal(err)
		}
		r := row{name: filepath.ToSlash(name), status: "fail"}
		t.Run(r.name, func(t *testing.T) {
			result, err := golden.Replay(filepath.Join(root, "fixtures"), dir, t.TempDir(), cliDriver(t))
			r.command = result.Command
			if err != nil {
				t.Fatal(err)
			}
			if result.Skip != "" {
				r.status = "skip"
				t.Skip(result.Skip)
			}
			_, listed := known[r.name]
			switch {
			case len(result.Differences) > 0 && listed:
				r.status = "known"
				t.Skip("known difference: " + known[r.name])
			case len(result.Differences) > 0:
				t.Fatal(strings.Join(result.Differences, "\n"))
			case listed:
				t.Fatal("listed in known-differences.txt but matches the recording; remove the line")
			}
			r.status = "pass"
		})
		rows = append(rows, r)
		totals[r.status]++
		delete(known, r.name)
	}
	for name := range known {
		t.Errorf("known-differences.txt lists %s, which is not a recorded case", name)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "CASE\tCOMMAND\tRESULT")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.name, r.command, r.status)
	}
	fmt.Fprintf(w, "TOTAL %d\tpass %d / known difference %d / fail %d / skip %d\t\n", len(rows), totals["pass"], totals["known"], totals["fail"], totals["skip"])
	if err = w.Flush(); err != nil {
		t.Fatal(err)
	}
}

var laneCite = regexp.MustCompile(`docs/lanes/L[0-9]+\.md:([1-9][0-9]*)`)

// knownDifferences reads "case<TAB>reason" lines. Each reason must cite a line
// that exists in a docs/lanes file, so a difference is never accepted unexplained.
func knownDifferences(path, root string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	known := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, reason, ok := strings.Cut(line, "\t")
		m := laneCite.FindStringSubmatch(reason)
		if !ok || m == nil {
			return nil, fmt.Errorf("known-differences.txt: %q needs a reason citing docs/lanes/L<n>.md:<line>", line)
		}
		doc, err := os.ReadFile(filepath.Join(root, strings.SplitN(m[0], ":", 2)[0]))
		if err != nil {
			return nil, err
		}
		if n, _ := strconv.Atoi(m[1]); n > strings.Count(string(doc), "\n") {
			return nil, fmt.Errorf("known-differences.txt: %s is past the end of its file", m[0])
		}
		known[name] = reason
	}
	return known, nil
}
