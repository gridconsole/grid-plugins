package golden_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// These are semantic replacements, not guesses based on exit status. Unknown
// hook events silently return 0, so probing them would hide missing commands.
// Add a mapping when a legacy command's Go implementation becomes available.
var legacyArgv = map[string][]string{
	"grid-env":      {"hook", "sessionstart"},
	"agent-routing": {"hook", "sessionstart"},
}

func cliDriver() golden.Driver {
	return golden.Driver{
		Resolve: func(c golden.Command) ([]string, bool) {
			if len(c.Argv) > 0 {
				// Explicit argv recordings can use implemented top-level commands. Read
				// the dispatcher's help instead of maintaining a duplicate command table.
				var help bytes.Buffer
				cli.Run([]string{"help"}, cli.Env{Stdout: &help, Stderr: &help})
				for _, line := range strings.Split(help.String(), "\n") {
					fields := strings.Fields(line)
					if len(fields) > 0 && fields[0] == c.Argv[0] && !strings.Contains(line, "(not built yet)") {
						if c.Argv[0] == "hook" && (len(c.Argv) != 2 || !strings.EqualFold(c.Argv[1], "sessionstart")) {
							return nil, false
						}
						return c.Argv, true
					}
				}
				return nil, false
			}
			if c.Call != "" {
				return nil, false
			}
			args, ok := legacyArgv[c.Hook]
			return args, ok
		},
		Run: func(in golden.Invocation) golden.Output {
			var stdout, stderr bytes.Buffer
			getenv := func(key string) string { return in.Env[key] }
			home := gridhome.Real(in.Env["HOME"])
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
	if os.Getenv("GRID_GOLDEN") != "1" {
		t.Skip("run make golden to replay the legacy corpus")
	}
	root := filepath.Join("..", "..")
	cases, err := golden.Cases(filepath.Join(root, "golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no recorded cases found")
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
			result, err := golden.Replay(filepath.Join(root, "fixtures"), dir, t.TempDir(), cliDriver())
			r.command = result.Command
			if err != nil {
				t.Fatal(err)
			}
			if result.Skip != "" {
				r.status = "skip"
				t.Skip(result.Skip)
			}
			if len(result.Differences) > 0 {
				t.Fatal(strings.Join(result.Differences, "\n"))
			}
			r.status = "pass"
		})
		rows = append(rows, r)
		totals[r.status]++
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "CASE\tCOMMAND\tRESULT")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.name, r.command, r.status)
	}
	fmt.Fprintf(w, "TOTAL %d\tpass %d / fail %d / skip %d\t\n", len(rows), totals["pass"], totals["fail"], totals["skip"])
	if err = w.Flush(); err != nil {
		t.Fatal(err)
	}
}
