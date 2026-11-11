package ljall

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/evalcmd"
)

const ljAllSuiteDir = "../../suite"

// ljAllTranscript is the one-line transcript of internal/evalcmd/icx_suiteexport_test.go:208.
const ljAllTranscript = `{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}` + "\n"

// ljAllKnownOpen lists the cases GradeCase refuses today, each with the substring
// its error must contain.
var ljAllKnownOpen = map[string]string{
	// found-before-result.md uses before_tool/before_match/after_tool/after_match, not the before/after maps (graderKeys, suitegrade.go:59-64).
	"review-posts-found": "unknown key",
}

// ljAllWalk grades every case folder of suiteDir; nil means the case grades clean.
func ljAllWalk(suiteDir string) map[string]error {
	out := map[string]error{}
	entries, err := os.ReadDir(suiteDir)
	if err != nil {
		out[""] = err
		return out
	}
	files, err := os.MkdirTemp("", "ljall-files")
	if err != nil {
		out[""] = err
		return out
	}
	defer os.RemoveAll(files)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "results" {
			continue
		}
		out[e.Name()] = ljAllCase(filepath.Join(suiteDir, e.Name()), files)
	}
	return out
}

func ljAllCase(dir, files string) error {
	if !ljAllExists(filepath.Join(dir, "prompt.md")) && !ljAllExists(filepath.Join(dir, "case.yaml")) {
		return fmt.Errorf("no prompt.md and no case.yaml")
	}
	gdir := filepath.Join(dir, "graders")
	st, err := os.Stat(gdir)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("no graders/ folder")
	}
	md, _ := filepath.Glob(filepath.Join(gdir, "*.md"))
	if len(md) == 0 {
		return fmt.Errorf("no *.md in graders/")
	}
	_, err = evalcmd.GradeCase(dir, []byte(ljAllTranscript), files)
	return err
}

func ljAllExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ljAllCheck compares a walk with the known-open list both ways and returns the failures.
func ljAllCheck(res map[string]error, open map[string]string) []string {
	var fails []string
	for name, err := range res {
		want, listed := open[name]
		switch {
		case !listed && err != nil:
			fails = append(fails, fmt.Sprintf("case %s: %v", name, err))
		case listed && err == nil:
			fails = append(fails, fmt.Sprintf("remove %s from ljAllKnownOpen", name))
		case listed && !strings.Contains(err.Error(), want):
			fails = append(fails, fmt.Sprintf("case %s: error %q lacks %q; remove %s from ljAllKnownOpen or fix the entry", name, err, want, name))
		}
	}
	for name := range open {
		if _, ok := res[name]; !ok {
			fails = append(fails, fmt.Sprintf("ljAllKnownOpen lists %s but its folder is gone", name))
		}
	}
	sort.Strings(fails)
	return fails
}

func TestLjAllSuite(t *testing.T) {
	res := ljAllWalk(ljAllSuiteDir)
	names := make([]string, 0, len(res))
	for n := range res {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if res[n] == nil {
			t.Logf("%s: graded", n)
		} else if _, ok := ljAllKnownOpen[n]; ok {
			t.Logf("%s: known-open: %v", n, res[n])
		} else {
			t.Logf("%s: PROBLEM: %v", n, res[n])
		}
	}
	for _, f := range ljAllCheck(res, ljAllKnownOpen) {
		t.Error(f)
	}
}

func ljAllWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ljAllCopy(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, info.Mode())
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLjAllCatches(t *testing.T) {
	root := t.TempDir()
	ljAllWrite(t, filepath.Join(root, "a-nograders", "prompt.md"), "do it\n")
	ljAllWrite(t, filepath.Join(root, "b-nomd", "prompt.md"), "do it\n")
	ljAllWrite(t, filepath.Join(root, "b-nomd", "graders", "notes.txt"), "x\n")
	ljAllWrite(t, filepath.Join(root, "c-badtype", "prompt.md"), "do it\n")
	ljAllWrite(t, filepath.Join(root, "c-badtype", "graders", "g.md"), "---\ntype: nosuch\n---\n")
	ljAllWrite(t, filepath.Join(root, "d-name", "prompt.md"), "do it\n")
	ljAllWrite(t, filepath.Join(root, "d-name", "graders", "g.md"), "---\nname: g\ntype: tool_used\ntool: Bash\nmin: 1\n---\n")
	ljAllCopy(t, filepath.Join(ljAllSuiteDir, "fix-total"), filepath.Join(root, "e-clean"))
	ljAllWrite(t, filepath.Join(root, "results", "x.md"), "ignored\n")
	ljAllWrite(t, filepath.Join(root, "README.md"), "ignored\n")

	res := ljAllWalk(root)
	for _, n := range []string{"a-nograders", "b-nomd", "c-badtype", "d-name"} {
		if res[n] == nil {
			t.Errorf("%s: want a problem, got none", n)
		} else {
			t.Logf("%s: %v", n, res[n])
		}
	}
	if err := res["e-clean"]; err != nil {
		t.Errorf("e-clean: %v", err)
	}
	if _, ok := res["results"]; ok {
		t.Error("results/ must be skipped")
	}
	if len(res) != 5 {
		t.Errorf("want 5 cases, got %d: %v", len(res), res)
	}
}

func TestLjAllKnownOpenBothWays(t *testing.T) {
	root := t.TempDir()
	ljAllCopy(t, filepath.Join(ljAllSuiteDir, "fix-total"), filepath.Join(root, "clean-one"))
	fails := ljAllCheck(ljAllWalk(root), map[string]string{"clean-one": "unknown key"})
	if len(fails) != 1 || !strings.Contains(fails[0], "remove clean-one from ljAllKnownOpen") {
		t.Errorf("listed clean case: got %v", fails)
	}

	root = t.TempDir()
	ljAllWrite(t, filepath.Join(root, "bad-one", "prompt.md"), "do it\n")
	ljAllWrite(t, filepath.Join(root, "bad-one", "graders", "g.md"), "---\nname: g\ntype: tool_used\ntool: Bash\nmin: 1\n---\n")
	fails = ljAllCheck(ljAllWalk(root), nil)
	if len(fails) != 1 || !strings.Contains(fails[0], "bad-one") || !strings.Contains(fails[0], "unknown key") {
		t.Errorf("unlisted bad case: got %v", fails)
	}

	fails = ljAllCheck(map[string]error{}, map[string]string{"gone": "unknown key"})
	if len(fails) != 1 || !strings.Contains(fails[0], "gone") {
		t.Errorf("gone folder: got %v", fails)
	}
	fails = ljAllCheck(map[string]error{"x": fmt.Errorf("other")}, map[string]string{"x": "unknown key"})
	if len(fails) != 1 || !strings.Contains(fails[0], "remove x from ljAllKnownOpen") {
		t.Errorf("wrong substring: got %v", fails)
	}
}
