package evalcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// icxSuite writes a one-task Go suite and returns its folder.
func icxSuite(t *testing.T, id, setup, checks string) string {
	t.Helper()
	src := t.TempDir()
	dir := filepath.Join(src, id)
	icxWrite(t, filepath.Join(dir, "task.json"), `{"schema_version":1,"id":"`+id+`","title":"Export probe","tier":"b","size":"tiny"}`)
	icxWrite(t, filepath.Join(dir, "prompt.md"), "Write the note for sam.\n")
	icxWrite(t, filepath.Join(dir, "setup.sh"), setup)
	icxWrite(t, filepath.Join(dir, "checks.json"), checks)
	return src
}

func icxWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func icxRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func icxGraderNames(t *testing.T, caseDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(caseDir, "graders"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

const icxWantInputMatch = `"\"file_path\":\"(?:[^\"\\\\]|\\\\.)*notes/today\\.md"`

func TestIcxExportMaps(t *testing.T) {
	setup := "#!/bin/sh\nset -eu\nmkdir -p notes\n"
	src := icxSuite(t, "maps", setup, `[
  {"id":"note-exists","type":"file_exists","path":"notes/today.md"},
  {"id":"note-text","type":"text_contains","path":"notes/today.md","text":"Done (alex)."},
  {"id":"judge","type":"llm_judge","rubric":"The note thanks sam."},
  {"id":"edits","type":"tool_used","tool":"Edit","count_min":2},
  {"id":"no-bash","type":"tool_used","tool":"Bash","absent":true},
  {"id":"wrote","type":"tool_used","tool":"Write","input_contains":{"file_path":"notes/today.md"}},
  {"id":"read-first","type":"tool_order","before":{"tool":"Read"},"after":{"tool":"Edit","input_contains":{"file_path":"notes/today.md"}}}
]`)
	dst := t.TempDir()
	report, err := ExportSuite(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Dropped) != 0 || len(report.Cases) != 1 || report.Cases[0] != (ExportedCase{ID: "maps", Graders: 7}) {
		t.Fatalf("report = %+v", report)
	}
	caseDir := filepath.Join(dst, "maps")
	want := map[string]string{
		"prompt.md":  "---\nruns: 1\ntimeout_seconds: 300\n---\n\nWrite the note for sam.\n",
		"case.yaml":  "schema_version: \"1.1\"\nname: maps\ncontext:\n  scaffold_script: fixture.sh\n",
		"fixture.sh": setup,
		"graders/note-exists.md": "---\ntype: file_exists\npath: \"notes/today.md\"\n---\n",
		"graders/note-text.md":   "---\ntype: regex\ntarget: {source: file, path: \"notes/today.md\"}\npattern: \"Done \\\\(alex\\\\)\\\\.\"\n---\n",
		"graders/judge.md":       "---\ntype: llm\n---\n\nThe note thanks sam.\n",
		"graders/edits.md":       "---\ntype: tool_used\ntool: \"Edit\"\nmin: 2\n---\n",
		"graders/no-bash.md":     "---\ntype: tool_used\ntool: \"Bash\"\nmax: 0\n---\n",
		"graders/wrote.md":       "---\ntype: tool_used\ntool: \"Write\"\ninput_match: " + icxWantInputMatch + "\nmin: 1\n---\n",
		"graders/read-first.md":  "---\ntype: tool_order\nbefore: \"Read\"\nafter: {tool: \"Edit\", input_match: " + icxWantInputMatch + "}\n---\n",
	}
	for rel, w := range want {
		if got := icxRead(t, filepath.Join(caseDir, filepath.FromSlash(rel))); got != w {
			t.Errorf("%s:\n got %q\nwant %q", rel, got, w)
		}
	}
	if got := icxGraderNames(t, caseDir); !slices.Equal(got, []string{"edits.md", "judge.md", "no-bash.md", "note-exists.md", "note-text.md", "read-first.md", "wrote.md"}) {
		t.Errorf("graders = %v", got)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(filepath.Join(caseDir, "fixture.sh")); err != nil || info.Mode()&0o100 == 0 {
			t.Errorf("fixture.sh is not executable: %v %v", info, err)
		}
	}

	// The project's own reader of the built-in layout grades the export as the Go
	// checks meant it: every file grader passes on a run that did the work.
	files := t.TempDir()
	icxWrite(t, filepath.Join(files, "notes", "today.md"), "Done (alex).\n")
	call := func(name, path string) string {
		return `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"` + name + `","input":{"file_path":"` + path + `"}}]}}`
	}
	transcript := strings.Join([]string{call("Read", "notes/today.md"), call("Edit", "notes/today.md"), call("Edit", "notes/today.md"), call("Write", "notes/today.md")}, "\n") + "\n"
	grade, err := GradeCase(caseDir, []byte(transcript), files)
	if err != nil {
		t.Fatal(err)
	}
	if !grade.Passed || len(grade.Graders) != 6 || !slices.Equal(grade.NotGraded, []string{"judge"}) {
		t.Errorf("grade = %+v", grade)
	}
}

func TestIcxExportNamesWhatItDrops(t *testing.T) {
	src := icxSuite(t, "drops", "#!/bin/sh\ntrue\n", `[
  {"id":"kept","type":"file_exists","path":"out.txt"},
  {"id":"runs-tests","type":"command_passes","command":["sh","-c","test -f out.txt"]}
]`)
	dst := t.TempDir()
	report, err := ExportSuite(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Dropped) != 1 {
		t.Fatalf("dropped = %+v", report.Dropped)
	}
	if d := report.Dropped[0]; d.Case != "drops" || d.Check != "runs-tests" || d.Type != "command_passes" || d.Reason == "" {
		t.Errorf("dropped = %+v", d)
	}
	if report.Cases[0] != (ExportedCase{ID: "drops", Graders: 1, Dropped: 1}) {
		t.Errorf("case = %+v", report.Cases[0])
	}
	if got := icxGraderNames(t, filepath.Join(dst, "drops")); !slices.Equal(got, []string{"kept.md"}) {
		t.Errorf("graders = %v, want only kept.md", got)
	}
	if got := report.Totals(); got != "1 cases: 1 checks carried as graders, 1 dropped; 0 cases have no grader left" {
		t.Errorf("totals = %q", got)
	}

	// The verb prints the same report as JSON.
	var out, errb bytes.Buffer
	cliOut := filepath.Join(t.TempDir(), "cases")
	if code := Run([]string{"export-suite", "--suite", src, "--out", cliOut, "--json"}, Env{Cwd: t.TempDir(), Stdout: &out, Stderr: &errb}); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var printed Report
	if err := json.Unmarshal(out.Bytes(), &printed); err != nil {
		t.Fatal(err)
	}
	if len(printed.Dropped) != 1 || printed.Dropped[0] != report.Dropped[0] {
		t.Errorf("printed dropped = %+v", printed.Dropped)
	}
}

// export-suite has its own flags: an unknown one, or one of the run filters,
// exits 2 and writes nothing.
func TestIcxExportSuiteOwnFlags(t *testing.T) {
	src := icxSuite(t, "flags", "#!/bin/sh\ntrue\n", `[{"id":"kept","type":"file_exists","path":"out.txt"}]`)
	for _, extra := range [][]string{{"--no-such-flag"}, {"--results", "r"}, {"--tier", "b"}, {"--size", "tiny"}, {"stray"}} {
		dst := filepath.Join(t.TempDir(), "cases")
		var out, errb bytes.Buffer
		args := append([]string{"export-suite", "--suite", src, "--out", dst, "--json"}, extra...)
		if code := Run(args, Env{Cwd: t.TempDir(), Stdout: &out, Stderr: &errb}); code != 2 {
			t.Errorf("%v: exit %d, want 2", extra, code)
		}
		if _, err := os.Stat(dst); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%v: %s was written (%v)", extra, dst, err)
		}
	}
	var out, errb bytes.Buffer
	if code := Run([]string{"export-suite", "--suite", src}, Env{Cwd: t.TempDir(), Stdout: &out, Stderr: &errb}); code != 2 || !strings.Contains(errb.String(), "--out is required") {
		t.Errorf("no --out: exit %d, stderr %q", code, errb.String())
	}
}

func TestIcxExportShippedSuite(t *testing.T) {
	src := filepath.Join("..", "..", "evals", "tasks")
	dst := t.TempDir()
	report, err := ExportSuite(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	folders := 0
	for _, e := range entries {
		if e.IsDir() {
			folders++
		}
	}
	if len(report.Cases) != folders {
		t.Fatalf("exported %d cases, suite has %d folders", len(report.Cases), folders)
	}
	empty := t.TempDir()
	transcript := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}` + "\n")
	for _, c := range report.Cases {
		caseDir := filepath.Join(dst, c.ID)
		if _, err := os.Stat(filepath.Join(caseDir, "prompt.md")); err != nil {
			t.Errorf("%s: %v", c.ID, err)
		}
		if got := len(icxGraderNames(t, caseDir)); got != c.Graders {
			t.Errorf("%s: %d grader files, report says %d", c.ID, got, c.Graders)
		}
		if c.Graders > 0 {
			// Every written grader parses with the project's reader of the layout.
			if _, err := GradeCase(caseDir, transcript, empty); err != nil {
				t.Errorf("%s: %v", c.ID, err)
			}
		}
		t.Logf("case %s: %d graders, %d dropped", c.ID, c.Graders, c.Dropped)
	}
	byType := map[string]int{}
	for _, d := range report.Dropped {
		byType[d.Type]++
	}
	t.Logf("dropped by type: %v", byType)
	t.Logf("totals: %s", report.Totals())
}
