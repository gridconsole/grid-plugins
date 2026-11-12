package contract_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/doctor"
)

var gjBacktick = regexp.MustCompile("`([^`]+)`")

// gjReadmeCheckNames takes the names from the "Checks, in order:" sentence of
// the `grid doctor --json` section of the README text.
func gjReadmeCheckNames(t *testing.T, readme string) []string {
	t.Helper()
	_, rest, ok := strings.Cut(readme, "\n### `grid doctor --json`\n")
	if !ok {
		t.Fatal("contract/README.md has no \"### `grid doctor --json`\" section")
	}
	section, _, _ := strings.Cut(rest, "\n### ")
	_, after, ok := strings.Cut(section, "Checks, in order:")
	if !ok {
		t.Fatal("the doctor section has no \"Checks, in order:\" line")
	}
	sentence, _, _ := strings.Cut(after, ". ")
	var names []string
	for _, m := range gjBacktick.FindAllStringSubmatch(sentence, -1) {
		names = append(names, m[1])
	}
	return names
}

// gjDoctorNames runs the doctor on a temp home and returns its check names.
func gjDoctorNames(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	gridHome := filepath.Join(home, ".grid")
	claude := filepath.Join(home, ".claude")
	codex := filepath.Join(home, ".codex")
	for _, d := range []string{gridHome, claude, codex} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", claude)
	t.Setenv("CODEX_HOME", codex)
	t.Setenv("GRID_HOME", gridHome)
	if h := os.Getenv("HOME"); !strings.HasPrefix(h, root+string(filepath.Separator)) {
		t.Fatalf("HOME is not under the temp root %s", root)
	}
	rep := doctor.Run(doctor.Options{
		Home:         home,
		SettingsPath: filepath.Join(claude, "settings.json"),
		GridHome:     gridHome,
		Label:        "~/.grid",
		GitVersion:   func() (string, error) { return "git version test", nil },
		LookPath:     func(string) (string, error) { return "", errors.New("test") },
	})
	var names []string
	for _, c := range rep.Checks {
		names = append(names, c.Name)
	}
	return names
}

func TestGjReadmeDoctorChecksMatch(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	want := gjDoctorNames(t)
	if len(want) != 10 {
		t.Fatalf("doctor reports %d checks %q, want 10", len(want), want)
	}
	if got := gjReadmeCheckNames(t, readme); !reflect.DeepEqual(got, want) {
		t.Fatalf("README names %q, doctor reports %q", got, want)
	}

	t.Run("a removed name is caught", func(t *testing.T) {
		edited := strings.Replace(readme, ", `deny block`", "", 1)
		if edited == readme {
			t.Fatal("the README line has no `deny block` to remove")
		}
		if got := gjReadmeCheckNames(t, edited); reflect.DeepEqual(got, want) {
			t.Fatalf("an edited line still matches: %q", got)
		}
	})
}
