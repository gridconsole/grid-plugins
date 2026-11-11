package routine

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// Run `go test ./internal/routine -run Golden -update-golden` to rewrite the
// files in testdata/golden, then read the diff before committing it.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/golden from the current renderers")

// The three schedules a person is likely to write. The OS timer is a one-minute
// tick for all of them; `routine run --due` decides in-process whether the
// schedule is due, so the schedule text never reaches the plist or the XML.
var goldenSchedules = []struct{ name, every string }{
	{"daily", "0 9 * * *"},
	{"hourly", "1h"},
	{"weekday", "0 9 * * 1-5"},
}

type goldenCase struct {
	scheduler Scheduler
	plan      TimerPlan
	home      string
	gridHome  string
	binary    string
}

func newGoldenCase(t *testing.T, backend Backend, name, every string) goldenCase {
	t.Helper()
	home := gridhome.Real(t.TempDir())
	// Not a prefix of .grid-routine-timers, so normalize cannot split that folder name.
	gh := filepath.Join(home, "grid data")
	binary := filepath.Join(home, "bin & space", "grid")
	store := Store{GridHome: gh}
	if _, err := store.Add(name, every, "grid version"); err != nil {
		t.Fatal(err)
	}
	s := Scheduler{Home: home, GridHome: gh, Binary: binary, Path: "/usr/local/bin:/usr/bin:/bin", Backend: backend, Runner: &fakeOS{}}
	p, err := s.plan("", name)
	if err != nil {
		t.Fatal(err)
	}
	return goldenCase{scheduler: s, plan: p, home: home, gridHome: gh, binary: binary}
}

// normalize swaps the temporary paths and the hashed label for fixed tokens and
// uses forward slashes, so one golden file matches on every host.
func (c goldenCase) normalize(text string) string {
	text = strings.ReplaceAll(text, c.plan.Label, "{{LABEL}}")
	text = strings.ReplaceAll(text, c.gridHome, "{{GRID_HOME}}")
	text = strings.ReplaceAll(text, c.home, "{{HOME}}")
	return strings.ReplaceAll(text, `\`, "/")
}

func (c goldenCase) commands(plan []OSCommand) string {
	var lines []string
	for _, command := range plan {
		parts := []string{command.Program}
		for _, arg := range command.Args {
			if strings.ContainsAny(arg, " \t") {
				arg = fmt.Sprintf("%q", arg)
			}
			parts = append(parts, arg)
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return c.normalize(strings.Join(lines, "\n") + "\n")
}

func compareGolden(t *testing.T, file, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", file)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file missing: %v", err)
	}
	if string(want) != got {
		t.Fatalf("%s differs from the renderer output\nwant: %s\n got: %s", file, want, got)
	}
}

// plistTopLevel returns the keys of the top-level dict in order, its string and
// integer values, and the ProgramArguments strings.
func plistTopLevel(t *testing.T, content string) (keys []string, values map[string]string, args []string) {
	t.Helper()
	values = map[string]string{}
	decoder := xml.NewDecoder(strings.NewReader(content))
	depth, current := 0, ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("plist is not well-formed XML: %v", err)
		}
		switch el := token.(type) {
		case xml.StartElement:
			var text string
			switch {
			case el.Name.Local == "key" && depth == 2:
				if err := decoder.DecodeElement(&text, &el); err != nil {
					t.Fatal(err)
				}
				current = text
				keys = append(keys, text)
				continue
			case el.Name.Local == "string" && depth == 3 && current == "ProgramArguments":
				if err := decoder.DecodeElement(&text, &el); err != nil {
					t.Fatal(err)
				}
				args = append(args, text)
				continue
			case (el.Name.Local == "string" || el.Name.Local == "integer") && depth == 2:
				if err := decoder.DecodeElement(&text, &el); err != nil {
					t.Fatal(err)
				}
				values[current] = text
				continue
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
}

func TestLaunchdPlistGolden(t *testing.T) {
	for _, tc := range goldenSchedules {
		t.Run(tc.name, func(t *testing.T) {
			c := newGoldenCase(t, Launchd{UID: 501}, tc.name, tc.every)
			if len(c.plan.Files) != 1 {
				t.Fatalf("one plist expected, got %d files", len(c.plan.Files))
			}
			file := c.plan.Files[0]
			wantPath := filepath.Join(c.home, "Library", "LaunchAgents", c.plan.Label+".plist")
			if file.Path != wantPath {
				t.Fatalf("plist path: %s, want %s", file.Path, wantPath)
			}
			compareGolden(t, "launchd-"+tc.name+".plist", c.normalize(file.Content))
			compareGolden(t, "launchd-"+tc.name+".cmd", c.commands(append(append([]OSCommand{}, c.plan.Install...), c.plan.Remove...)))

			// The checks below read the plist itself, not the golden text.
			keys, values, args := plistTopLevel(t, file.Content)
			wantKeys := []string{"Label", "ProgramArguments", "WorkingDirectory", "EnvironmentVariables", "StandardOutPath", "StandardErrorPath", "StartInterval"}
			if !reflect.DeepEqual(keys, wantKeys) {
				t.Fatalf("plist keys: %v, want %v (no StartCalendarInterval: the schedule is read by routine run --due)", keys, wantKeys)
			}
			if values["StartInterval"] != "60" {
				t.Fatalf("StartInterval: %q", values["StartInterval"])
			}
			if wantArgs := []string{c.binary, "routine", "run", tc.name, "--due"}; !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("ProgramArguments: %q, want %q", args, wantArgs)
			}
			if !strings.Contains(file.Content, "<string>"+xmlText(c.binary)+"</string>") {
				t.Fatal("grid path with a space and an ampersand is not one escaped string")
			}
			logDir := filepath.Join(c.gridHome, "logs", "routines", c.plan.Label)
			if values["StandardOutPath"] != filepath.Join(logDir, "stdout.log") || values["StandardErrorPath"] != filepath.Join(logDir, "stderr.log") {
				t.Fatalf("log paths: %q %q, want under %s", values["StandardOutPath"], values["StandardErrorPath"], logDir)
			}
			if values["Label"] != c.plan.Label || !strings.HasPrefix(c.plan.Label, LabelPrefix) || values["WorkingDirectory"] != c.gridHome {
				t.Fatalf("label or working directory: %q %q", values["Label"], values["WorkingDirectory"])
			}
		})
	}
}

type schtasksXML struct {
	Command   string `xml:"Actions>Exec>Command"`
	Arguments string `xml:"Actions>Exec>Arguments"`
	Interval  string `xml:"Triggers>TimeTrigger>Repetition>Interval"`
}

func TestSchtasksGolden(t *testing.T) {
	for _, tc := range goldenSchedules {
		t.Run(tc.name, func(t *testing.T) {
			c := newGoldenCase(t, Schtasks{}, tc.name, tc.every)
			if len(c.plan.Files) != 1 {
				t.Fatalf("one XML file expected, got %d files", len(c.plan.Files))
			}
			file := c.plan.Files[0]
			wantPath := filepath.Join(c.home, ".grid-routine-timers", c.plan.Label+".xml")
			if file.Path != wantPath || file.Encoding != "UTF-16LE" {
				t.Fatalf("XML path or encoding: %s %s", file.Path, file.Encoding)
			}
			compareGolden(t, "schtasks-"+tc.name+".xml", c.normalize(file.Content))
			compareGolden(t, "schtasks-"+tc.name+".cmd", c.commands(append(append([]OSCommand{}, c.plan.Install...), c.plan.Remove...)))

			// The checks below read the task XML itself, not the golden text.
			var task schtasksXML
			decoder := xml.NewDecoder(strings.NewReader(file.Content))
			decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
			if err := decoder.Decode(&task); err != nil {
				t.Fatal(err)
			}
			if task.Command != c.binary || task.Interval != "PT1M" {
				t.Fatalf("command or repetition: %q %q", task.Command, task.Interval)
			}
			wantArgs := strings.Join([]string{windowsArg("routine"), windowsArg("run"), windowsArg(tc.name), windowsArg("--due"), "--grid-home", windowsArg(c.gridHome)}, " ")
			if task.Arguments != wantArgs {
				t.Fatalf("arguments: %q, want %q", task.Arguments, wantArgs)
			}
			wantInstall := []OSCommand{{"schtasks", []string{"/Create", "/TN", c.plan.Label, "/XML", wantPath}}}
			wantRemove := []OSCommand{{"schtasks", []string{"/Delete", "/TN", c.plan.Label, "/F"}}}
			if !reflect.DeepEqual(c.plan.Install, wantInstall) || !reflect.DeepEqual(c.plan.Remove, wantRemove) {
				t.Fatalf("schtasks commands: %v %v", c.plan.Install, c.plan.Remove)
			}
		})
	}
}

// Schedule and unschedule on native Windows only preview; this pins that the
// preview of the same three schedules reports the XML and both command lines
// without writing a file or calling the OS.
func TestSchtasksPreviewCoversAllSchedules(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	gh := filepath.Join(home, ".grid")
	store := Store{GridHome: gh}
	for _, tc := range goldenSchedules {
		if _, err := store.Add(tc.name, tc.every, "grid version"); err != nil {
			t.Fatal(err)
		}
	}
	calls := &fakeOS{}
	s := Scheduler{Home: home, GridHome: gh, Binary: filepath.Join(home, "grid.exe"), Backend: Schtasks{}, Runner: calls}
	for _, remove := range []bool{false, true} {
		plans, err := s.Preview(store, remove)
		if err != nil || len(plans) != len(goldenSchedules) {
			t.Fatalf("preview remove=%v: %d plans, %v", remove, len(plans), err)
		}
		for _, p := range plans {
			if remove && (len(p.Install) != 0 || len(p.Remove) != 1) || !remove && (len(p.Install) != 1 || len(p.Remove) != 0) {
				t.Fatalf("preview remove=%v carries the wrong commands: %v %v", remove, p.Install, p.Remove)
			}
			if _, err := os.Stat(p.Files[0].Path); !os.IsNotExist(err) {
				t.Fatalf("preview wrote %s: %v", p.Files[0].Path, err)
			}
		}
	}
	if len(calls.calls) != 0 {
		t.Fatalf("preview called the OS: %v", calls.calls)
	}
}
