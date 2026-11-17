package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// cpOfferOut is the SessionStart output as the tests read it.
type cpOfferOut struct {
	SystemMessage string `json:"systemMessage"`
	Specific      struct {
		Event   string `json:"hookEventName"`
		Context string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// cpOfferRun runs SessionStart on a machine whose user home is home and whose Grid
// home is <home>/grid-data (an empty folder), and returns the decoded output.
func cpOfferRun(t *testing.T, home string) (cpOfferOut, int) {
	t.Helper()
	gh := filepath.Join(home, "grid-data")
	if err := os.MkdirAll(gh, 0o700); err != nil {
		t.Fatal(err)
	}
	r := where.Resolver{GridHome: gh, Home: home, Ceiling: home, Machine: "machine-demo"}
	in, _ := json.Marshal(map[string]string{"cwd": home, "session_id": "session-offer", "hook_event_name": "SessionStart"})
	var out, log bytes.Buffer
	code := Run(Options{Event: "SessionStart", Stdin: bytes.NewReader(in), Stdout: &out, Stderr: &log, Resolver: r, Cwd: home, Getenv: func(string) string { return "" }})
	var got cpOfferOut
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("SessionStart output is not JSON: %v\n%s", err, out.String())
		}
	}
	return got, code
}

func cpOfferWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCpOfferNoConfig(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	got, code := cpOfferRun(t, home)
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(got.SystemMessage, "/grid:first-run") {
		t.Errorf("systemMessage %q does not offer the first run", got.SystemMessage)
	}
	if strings.Contains(got.SystemMessage, "\n") {
		t.Errorf("systemMessage is more than one line: %q", got.SystemMessage)
	}
	if n := strings.Count(got.Specific.Context, "grid:first-run"); n != 1 {
		t.Errorf("additionalContext names grid:first-run %d times, want 1:\n%s", n, got.Specific.Context)
	}
	if !strings.Contains(got.Specific.Context, "Grid tracks tasks") {
		t.Errorf("additionalContext lost its usual context:\n%s", got.Specific.Context)
	}
	// Reading the offer's own check leaves nothing behind that would silence a later session.
	if again, _ := cpOfferRun(t, home); again.SystemMessage == "" {
		t.Errorf("a second SessionStart on the same machine dropped the offer")
	}
}

func TestCpOfferQuietWithConfig(t *testing.T) {
	cases := map[string]func(home string) string{
		"config at gridconfig.Path": func(home string) string {
			return gridconfig.Path(func(string) string { return "" }, home)
		},
		"only the old ~/.grid/config.json": func(home string) string { return gridconfig.OldPath(home) },
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			cpOfferWrite(t, path(home), `{"v":3}`)
			got, code := cpOfferRun(t, home)
			if code != 0 || got.Specific.Event != "SessionStart" {
				t.Fatalf("exit %d, output %+v", code, got)
			}
			if got.SystemMessage != "" || strings.Contains(got.Specific.Context, "first-run") {
				t.Errorf("offer shown with a config present: %+v", got)
			}
		})
	}
}

func TestCpOfferQuietWithData(t *testing.T) {
	cases := map[string]string{
		"VERSION under ~/.grid":          filepath.Join(".grid", "VERSION"),
		"projects under ~/.grid":         filepath.Join(".grid", "projects", "demo", "project.json"),
		"data under ~/.grid/<workspace>": filepath.Join(".grid", "acme", "projects", "demo", "project.json"),
	}
	for name, rel := range cases {
		t.Run(name, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			cpOfferWrite(t, filepath.Join(home, rel), "1")
			got, code := cpOfferRun(t, home)
			if code != 0 || got.Specific.Event != "SessionStart" {
				t.Fatalf("exit %d, output %+v", code, got)
			}
			if got.SystemMessage != "" || strings.Contains(got.Specific.Context, "first-run") {
				t.Errorf("offer shown with data present: %+v", got)
			}
		})
	}
}
