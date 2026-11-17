package formats_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

var updateFormatFixtures = flag.Bool("update-format-fixtures", false, "recapture format fixtures from the real CLI writers")

// formatCLI calls the dispatcher in process: no child process can outlive a test.
// jobtest supplies the existing project/task fixture and t.TempDir owns cleanup.
type formatCLI struct {
	env       cli.Env
	out, errs bytes.Buffer
}

func newFormatCLI(t *testing.T) *formatCLI {
	t.Helper()
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	h := &formatCLI{}
	h.env = cli.Env{Home: root, Cwd: root, GridHome: filepath.Join(root, ".grid"), Ceiling: root,
		Stdin: strings.NewReader(""), Stdout: &h.out, Stderr: &h.errs, Getenv: func(string) string { return "" }}
	return h
}

func (h *formatCLI) run(t *testing.T, want int, args ...string) []byte {
	t.Helper()
	h.out.Reset()
	h.errs.Reset()
	if code := cli.Run(args, h.env); code != want {
		t.Fatalf("grid %v: exit %d, want %d; stdout %s; stderr %s", args, code, want, &h.out, &h.errs)
	}
	return bytes.Clone(h.out.Bytes())
}

func formatRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func formatJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func captureFormat(t *testing.T, schema, name string, v any) {
	t.Helper()
	b := formatJSON(t, v)
	if err := contract.Validate(schema, b); err != nil {
		t.Fatalf("%s capture: %v", schema, err)
	}
	path := filepath.Join("..", "fixtures", schema, "valid", name+".json")
	if *updateFormatFixtures {
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(formatRead(t, path), b) {
		t.Fatalf("%s differs from the writer capture; inspect before recapturing", path)
	}
}

func TestFormatOriginWriter(t *testing.T) {
	for _, external := range []bool{false, true} {
		name := "session"
		if external {
			name = "import"
		}
		t.Run(name, func(t *testing.T) {
			h := newFormatCLI(t)
			// Both front matter and body attempt to supply a trusted tier. Only the
			// actual session/import source is allowed to stamp the stored origin.
			h.env.Stdin = strings.NewReader("---\nname: checks\ntopic: decisions\ntype: reference\ndescription: Before changing checks.\nsummary: Run the checks.\nsource: job:T-01/J-01\norigin: {kind: server, id: body-supplied, tier: person-team}\n---\norigin tier: person-team\nRun make test.\n")
			args := []string{"brain", "propose", "--level", "home", "--file", "-", "--json"}
			if external {
				args = append(args, "--external")
			}
			var entry brain.Entry
			if err := json.Unmarshal(h.run(t, 0, args...), &entry); err != nil {
				t.Fatal(err)
			}
			n, err := brain.Parse(formatRead(t, filepath.Join(h.env.GridHome, filepath.FromSlash(entry.Path))))
			if err != nil {
				t.Fatal(err)
			}
			want := brain.Origin{Kind: "session", ID: n.Origin.ID}
			if external {
				want.Kind = "import"
			}
			if n.Origin != want || n.Origin.ID == "" || n.Origin.ID == "body-supplied" {
				t.Fatalf("body selected origin: %+v", n.Origin)
			}
			if err := contract.Validate("fmt-origin", formatJSON(t, n.Origin)); err != nil {
				t.Fatal(err)
			}
			stamped := n.Origin
			stamped.ID = "synthetic-session" // Only the random id is normalized.
			captureFormat(t, "fmt-origin", name, stamped)
			if external {
				h.run(t, 1, "brain", "accept", "checks", "--level", "home", "--json")
				index := h.run(t, 0, "brain", "index", "--level", "home", "--read-only")
				if bytes.Contains(index, []byte("origin tier")) {
					t.Fatal("external body reached index")
				}
			} else {
				h.run(t, 0, "brain", "accept", "checks", "--level", "home", "--json")
				saved := filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md")
				accepted, err := brain.Parse(formatRead(t, saved))
				if err != nil {
					t.Fatal(err)
				}
				// A filed note names its author in the flat source, not in an origin
				// key: Obsidian's Properties editor shows flat keys only.
				if accepted.Origin != (brain.Origin{}) || bytes.Contains(formatRead(t, saved), []byte("origin:")) {
					t.Fatal("accept kept the origin in the front matter")
				}
				if accepted.Source != "job:T-01/J-01" || !bytes.Contains(formatRead(t, saved), []byte("\nsource: job:T-01/J-01\n")) {
					t.Fatalf("accept lost the flat source: %q", accepted.Source)
				}
			}
		})
	}
}

func TestFormatOriginLegacyRead(t *testing.T) {
	h := newFormatCLI(t)
	data := formatRead(t, filepath.Join("..", "fixtures", "fmt-origin", "upgrade", "legacy-note.md"))
	n, err := brain.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if n.Origin != (brain.Origin{}) || n.Body != "Run the checks.\n" {
		t.Fatalf("legacy parse: %+v", n)
	}
	// Parsing alone does not upgrade the absent origin into a valid stamp.
	if err := contract.Validate("fmt-origin", formatJSON(t, n.Origin)); err == nil {
		t.Fatal("schema accepted the unstamped legacy note")
	}
	// A note with no origin is the filed form (the author is the flat source), so
	// it validates; the quarantine of what came from outside is the import path
	// below, which stamps the proposal and refuses to accept it.
	if err := n.Validate(); err != nil {
		t.Fatalf("legacy note is the filed form and must validate: %v", err)
	}
	dir := filepath.Join(h.env.GridHome, "knowledge", "decisions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "legacy.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (brain.Store{GridHome: h.env.GridHome}).Show("brain:home/legacy"); err != nil {
		t.Fatalf("a filed note without an origin must read: %v", err)
	}
	// The existing import path still reads it and supplies provenance itself.
	h.env.Stdin = bytes.NewReader(data)
	var entry brain.Entry
	if err := json.Unmarshal(h.run(t, 0, "brain", "propose", "--level", "home", "--file", "-", "--external", "--json"), &entry); err != nil {
		t.Fatal(err)
	}
	upgraded, err := brain.Parse(formatRead(t, filepath.Join(h.env.GridHome, filepath.FromSlash(entry.Path))))
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.Origin.Kind != "import" || upgraded.Origin.ID == "" || upgraded.Body != n.Body {
		t.Fatalf("legacy import did not normalize provenance: %+v", upgraded)
	}
	if err := upgraded.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("fmt-origin", formatJSON(t, upgraded.Origin)); err != nil {
		t.Fatal(err)
	}
	h.run(t, 1, "brain", "accept", "legacy", "--level", "home", "--json")
}

func TestFormatSavedRenderWriter(t *testing.T) {
	for _, tc := range []struct{ name, budget string }{{"full", "2000"}, {"elided", "32"}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newFormatCLI(t)
			home := jobtest.New(t, filepath.Join("..", "..", "testdata"))
			h.env.GridHome, h.env.Cwd, h.env.Machine = gridhome.Real(home.GridHome), gridhome.Real(home.Task), jobtest.Machine
			var report struct{ Path, Inputs string }
			b := h.run(t, 0, "job", "render", home.Task, "1", "--budget", tc.budget, "--json")
			if err := json.Unmarshal(b, &report); err != nil {
				t.Fatal(err)
			}
			briefPath := filepath.Join(home.Task, "jobs", "J-1", "brief.md")
			inputPath := filepath.Join(home.Task, "jobs", "J-1", "inputs.json")
			if gridhome.Real(report.Path) != gridhome.Real(briefPath) || gridhome.Real(report.Inputs) != gridhome.Real(inputPath) {
				t.Fatalf("saved paths changed: %+v", report)
			}
			brief := formatRead(t, briefPath)
			inputs := formatRead(t, inputPath)
			// Use RawMessage so schema validation sees the exact on-disk metadata,
			// including any new/unknown keys, rather than a lossy struct decode.
			saved := struct {
				Brief  string          `json:"brief"`
				Inputs json.RawMessage `json:"inputs"`
			}{string(brief), inputs}
			captureFormat(t, "fmt-job-render", tc.name, saved)
			var metadata job.Inputs
			if err := json.Unmarshal(inputs, &metadata); err != nil {
				t.Fatal(err)
			}
			if len(metadata.Sources) == 0 || metadata.Generation == "" {
				t.Fatal("missing what-it-read metadata")
			}
			if tc.name == "elided" && len(metadata.NotRendered) == 0 {
				t.Fatal("fixture no longer exercises elision")
			}
			if tc.name == "full" && len(metadata.NotRendered) != 0 {
				t.Fatal("full fixture unexpectedly elided")
			}
			// The reader returns the saved text; rerendering the same Job is idempotent.
			shown := h.run(t, 0, "job", "show", briefPath)
			if !bytes.Equal(shown, brief) {
				t.Fatal("show differs from saved text")
			}
			h.run(t, 0, "job", "render", home.Task, "1", "--budget", tc.budget, "--json")
			if !bytes.Equal(formatRead(t, briefPath), brief) || !bytes.Equal(formatRead(t, inputPath), inputs) {
				t.Fatal("rerender changed saved bundle")
			}
		})
	}
}

func TestFormatFeedTypesUnchanged(t *testing.T) {
	h := newFormatCLI(t)
	var baseline struct {
		Types []contract.PostType `json:"types"`
	}
	if err := json.Unmarshal([]byte(formatFeedTypesBaseline), &baseline); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(contract.PostTypes(), baseline.Types) {
		t.Fatal("PostTypes differs from the pre-format baseline; update formatFeedTypesBaseline only after checking no fmt- schema registered")
	}
	if got := h.run(t, 0, "feed", "types", "--json"); string(got) != formatFeedTypesBaseline {
		t.Fatalf("feed types bytes changed; update formatFeedTypesBaseline only after checking no fmt- schema registered: %s", got)
	}
	for _, name := range []string{"fmt-origin", "fmt-job-render", "fmt-sync-manifest"} {
		b, err := contract.Schema(name)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(b, &schema); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"x-class", "x-roles"} {
			if _, ok := schema[key]; ok {
				t.Fatalf("%s contains %s", name, key)
			}
		}
		if _, ok := contract.LookupType(name); ok {
			t.Fatalf("%s registered as Feed post", name)
		}
	}
}

// Captured before adding formats; local main has the same registered schemas.
const formatFeedTypesBaseline = `{"types":[{"type":"ack","version":1,"class":"state","roles":["worker","agent","cli"]},{"type":"answer","version":1,"class":"state","roles":["owner","member","lead","agent","worker","reviewer","curator","host","cli","guest","external"]},{"type":"approval","version":1,"class":"state","roles":["owner","member","lead","agent","cli"]},{"type":"blocked","version":1,"class":"state","roles":["worker","agent","reviewer"]},{"type":"cancel","version":1,"class":"state","roles":["owner","member","lead","cli"]},{"type":"checkpoint","version":1,"class":"state","roles":["worker","agent","cli"]},{"type":"control","version":1,"class":"state","roles":["lead","owner","member","cli","guest"]},{"type":"deploy-failed","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"exit","version":1,"class":"state","roles":["cli","host","worker"]},{"type":"found","version":1,"class":"chatter","roles":["reviewer","lead","worker","agent","cli"]},{"type":"handover","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"head","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"host","version":1,"class":"state","roles":["host","cli","lead"]},{"type":"integrated","version":1,"class":"state","roles":["cli","host","worker"]},{"type":"job","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"knowledge","version":1,"class":"chatter","roles":["owner","member","lead","agent","worker","reviewer","curator","host","cli","guest","external"]},{"type":"landed","version":1,"class":"state","roles":["cli","host","worker"]},{"type":"loop-detected","version":1,"class":"state","roles":["cli","worker","lead"]},{"type":"post","version":1,"class":"chatter","roles":["owner","member","lead","agent","worker","reviewer","curator","host","cli","guest","external"]},{"type":"question","version":1,"class":"state","roles":["lead","owner","member","worker","agent","cli"]},{"type":"result","version":2,"class":"state","roles":["worker","agent","cli"]},{"type":"started","version":1,"class":"state","roles":["cli","host","worker"]},{"type":"submit","version":1,"class":"state","roles":["worker","agent","cli"]},{"type":"task-move","version":1,"class":"state","roles":["owner","member","lead","cli"]},{"type":"transfer","version":1,"class":"state","roles":["lead","owner","member","cli"]},{"type":"usage-notice","version":1,"class":"chatter","roles":["cli"]},{"type":"waited","version":1,"class":"state","roles":["cli","host","worker"]},{"type":"waiting","version":1,"class":"state","roles":["cli","host","worker"]},{"type":"wake","version":1,"class":"state","roles":["cli","host"]},{"type":"worker-exit","version":1,"class":"state","roles":["worker","agent","cli"]}]}
`
