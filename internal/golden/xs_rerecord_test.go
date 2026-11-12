package golden_test

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
)

// xsRrFlag names corpus cases (for example stop-check/mark-done, comma separated)
// whose recorded stdout, stderr, exit and files.diff are rewritten from the Go
// hook's own normalised output when TestGoldenCorpus replays them:
//
//	go test ./internal/golden -run TestGoldenCorpus -args -xs-rerecord=stop-check/mark-done
//
// It is a test flag on purpose, never an environment variable. A case with a
// go-diff.md keeps its legacy recording and is rewritten in go-expected, the
// folder Replay then reads. Review the resulting git diff like any code change.
var xsRrFlag = flag.String("xs-rerecord", "", "corpus cases to re-record from the Go hook, comma separated")

func xsRrWanted(name string) bool {
	for _, want := range strings.Split(*xsRrFlag, ",") {
		if want != "" && want == name {
			return true
		}
	}
	return false
}

// xsRrApply is called by TestGoldenCorpus after Replay. For a wanted case it
// writes the replayed artifacts into the folder Replay compares against and
// clears the differences, so the same run reports the case as matching.
func xsRrApply(t *testing.T, name, dir string, result *golden.Result) {
	t.Helper()
	if !xsRrWanted(name) || result.Skip != "" || result.Artifacts == nil {
		return
	}
	if err := xsRrWrite(dir, result.Artifacts); err != nil {
		t.Fatal(err)
	}
	result.Differences = nil
	t.Logf("re-recorded %s from the Go hook", name)
}

func xsRrWrite(dir string, artifacts map[string]string) error {
	into := dir
	if _, err := os.Stat(filepath.Join(dir, "go-diff.md")); err == nil {
		into = filepath.Join(dir, "go-expected")
	}
	if err := os.MkdirAll(into, 0o755); err != nil {
		return err
	}
	for name, content := range artifacts {
		if err := os.WriteFile(filepath.Join(into, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("re-record %s: %w", name, err)
		}
	}
	return nil
}

func xsRrCopy(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestXsRerecord stales one case in a temp copy of the corpus, re-records it
// with the flag, and shows the copy then matches the Go hook without a skip.
// One case records in the case folder, one (go-diff.md) in go-expected.
func TestXsRerecord(t *testing.T) {
	fixtures := filepath.Join("..", "..", "fixtures")
	for _, name := range []string{"stop-check/mark-done", "agent-routing/session"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "golden", filepath.FromSlash(name))
			xsRrCopy(t, filepath.Join("..", "..", "golden", filepath.FromSlash(name)), dir)
			into := dir
			if _, err := os.Stat(filepath.Join(dir, "go-diff.md")); err == nil {
				into = filepath.Join(dir, "go-expected")
			}
			if err := os.WriteFile(filepath.Join(into, "stdout"), []byte("stale recording\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			replay := func() golden.Result {
				result, err := golden.Replay(fixtures, dir, t.TempDir(), cliDriver(t))
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			stale := replay()
			if stale.Skip != "" || len(stale.Differences) == 0 {
				t.Fatalf("a stale recording must differ and run: skip %q, differences %d", stale.Skip, len(stale.Differences))
			}

			saved := *xsRrFlag
			t.Cleanup(func() { *xsRrFlag = saved })
			*xsRrFlag = "some/other-case"
			other := replay()
			xsRrApply(t, name, dir, &other)
			if len(other.Differences) == 0 {
				t.Fatal("a case the flag does not name must not be re-recorded")
			}
			*xsRrFlag = "x/y," + name
			fresh := replay()
			xsRrApply(t, name, dir, &fresh)
			if len(fresh.Differences) != 0 {
				t.Fatalf("re-recorded run still differs: %v", fresh.Differences)
			}

			again := replay()
			if again.Skip != "" || len(again.Differences) != 0 {
				t.Fatalf("corpus copy does not match its re-recording: skip %q, %v", again.Skip, again.Differences)
			}
			got, err := os.ReadFile(filepath.Join(into, "stdout"))
			if err != nil || string(got) == "stale recording\n" || string(got) != again.Artifacts["stdout"] {
				t.Fatalf("stdout not rewritten from the Go hook: %q %v", got, err)
			}
		})
	}
}
