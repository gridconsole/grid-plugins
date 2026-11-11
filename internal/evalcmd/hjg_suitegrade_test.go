package evalcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hjgGraders are grader files of every graded type; the keys are the file names.
var hjgGraders = map[string]string{
	"last-message": "---\ntype: regex\npattern: 'Verdict:\\s+ACCEPTED'\ntarget: last_message\n---\n",
	"body-pattern": "---\ntype: regex\ntarget: last_message\nflags: i\n---\nverdict:\\s+accepted\n",
	"not-contains": "---\ntype: regex\npattern: 'rm -rf'\nmatch: not_contains\ntarget: trace\n---\n",
	"count-one":    "---\ntype: regex\npattern: '\"name\":\"Bash\"'\nmatch: 'count:1'\ntarget: trace\n---\n",
	"file-target":  "---\ntype: regex\npattern: '^## Outcome'\nflags: m\ntarget:\n  source: file\n  path: out/report.md\n---\n",
	"files-target": "---\ntype: regex\npattern: 'sam'\ntarget: files\n---\n",
	"used":         "---\ntype: tool_used\ntool: Bash\ninput_match: 'go test \\./\\.\\.\\. && echo'\n---\n",
	"max-zero":     "---\ntype: tool_used\ntool: Bash\ninput_match: 'rm -rf'\nmax: 0\n---\n",
	"min-two":      "---\ntype: tool_used\ntool: Bash\nmin: 2\n---\n",
	"order":        "---\ntype: tool_order\nbefore: Read\nafter:\n  tool: Write\n  input_match: 'out/report'\n---\n",
	"exists":       "---\ntype: file_exists\npath: 'out/**/*.md'\n---\n",
	"exists-not":   "---\ntype: file_exists\npath: 'tmp/*.lock'\nexists: false\n---\n",
}

// hjgCase writes a case folder with the named graders, each content as given.
func hjgCase(t *testing.T, graders map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	prompt, err := os.ReadFile(filepath.Join("testdata", "hjg", "prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	hjgWrite(t, filepath.Join(dir, "prompt.md"), string(prompt))
	for name, content := range graders {
		hjgWrite(t, filepath.Join(dir, "graders", name+".md"), content)
	}
	return dir
}

func hjgWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hjgData(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "hjg", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func hjgWS(name string) string { return filepath.Join("testdata", "hjg", name) }

// hjgOne grades one grader file against a transcript and a workspace.
func hjgOne(t *testing.T, grader string, transcript []byte, ws string) GraderResult {
	t.Helper()
	got, err := GradeCase(hjgCase(t, map[string]string{"g": grader}), transcript, ws)
	if err != nil {
		t.Fatalf("GradeCase: %v", err)
	}
	if len(got.Graders) != 1 || len(got.NotGraded) != 0 {
		t.Fatalf("want one graded grader, got %+v", got)
	}
	if got.Passed != got.Graders[0].Passed {
		t.Fatalf("Passed %v disagrees with its only grader %+v", got.Passed, got.Graders[0])
	}
	return got.Graders[0]
}

func TestHjgGraders(t *testing.T) {
	good, bad := hjgData(t, "good.jsonl"), hjgData(t, "bad.jsonl")
	goodWS, badWS := hjgWS("ws-good"), hjgWS("ws-bad")
	// Each row grades one grader against a transcript and workspace that should
	// pass it, then against ones that should fail it.
	rows := []struct {
		name, grader   string
		passTr, failTr []byte
		passWS, failWS string
		wantType       string
	}{
		{"regex last_message", "last-message", good, bad, goodWS, badWS, "regex"},
		{"regex pattern in the body", "body-pattern", good, bad, goodWS, badWS, "regex"},
		{"regex not_contains", "not-contains", good, bad, goodWS, badWS, "regex"},
		{"regex count:N and N+1", "count-one", good, bad, goodWS, badWS, "regex"},
		{"regex file target", "file-target", good, good, goodWS, badWS, "regex"},
		{"regex files target", "files-target", good, good, goodWS, badWS, "regex"},
		{"tool_used input_match with &&", "used", good, bad, goodWS, badWS, "tool_used"},
		{"tool_used max 0", "max-zero", good, bad, goodWS, badWS, "tool_used"},
		{"tool_used min 2", "min-two", bad, good, goodWS, badWS, "tool_used"},
		{"tool_order", "order", good, bad, goodWS, badWS, "tool_order"},
		{"file_exists with **", "exists", good, good, goodWS, badWS, "file_exists"},
		{"file_exists exists false", "exists-not", good, good, goodWS, badWS, "file_exists"},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			grader := hjgGraders[r.grader]
			pass := hjgOne(t, grader, r.passTr, r.passWS)
			if !pass.Passed || pass.Type != r.wantType || pass.Name != "g" {
				t.Errorf("passing fixture: %+v", pass)
			}
			fail := hjgOne(t, grader, r.failTr, r.failWS)
			if fail.Passed || fail.Reason == "" {
				t.Errorf("failing fixture passed: %+v", fail)
			}
			t.Logf("pass: %s | fail: %s", pass.Reason, fail.Reason)
		})
	}

	t.Run("count:N fails on N+1 and N-1", func(t *testing.T) {
		for _, c := range []struct {
			match string
			want  bool
		}{{"count:2", true}, {"count:1", false}, {"count:3", false}} {
			g := "---\ntype: regex\npattern: '\"name\":\"Bash\"'\nmatch: '" + c.match + "'\ntarget: trace\n---\n"
			if got := hjgOne(t, g, bad, "").Passed; got != c.want {
				t.Errorf("%s over two Bash calls: passed %v, want %v", c.match, got, c.want)
			}
		}
	})

	t.Run("tool_used max 0 fails when the tool was called", func(t *testing.T) {
		g := "---\ntype: tool_used\ntool: Read\nmax: 0\n---\n"
		if r := hjgOne(t, g, good, ""); r.Passed {
			t.Errorf("Read was called, max 0 passed: %+v", r)
		}
		g = "---\ntype: tool_used\ntool: Edit\nmax: 0\n---\n"
		if r := hjgOne(t, g, good, ""); !r.Passed {
			t.Errorf("Edit was never called, max 0 failed: %+v", r)
		}
	})

	t.Run("tool_used default min is 1", func(t *testing.T) {
		g := "---\ntype: tool_used\ntool: Edit\n---\n"
		if r := hjgOne(t, g, good, ""); r.Passed {
			t.Errorf("Edit was never called, default min passed: %+v", r)
		}
	})

	t.Run("last_message is the last text of the run, not of a subagent", func(t *testing.T) {
		tr := string(good) +
			`{"type":"assistant","parent_tool_use_id":"toolu_1","message":{"content":[{"type":"text","text":"Verdict: REJECTED"}]}}` + "\n"
		if r := hjgOne(t, hjgGraders["last-message"], []byte(tr), ""); !r.Passed {
			t.Errorf("a subagent's text replaced the run's last message: %+v", r)
		}
	})

	t.Run("a missing file never passes not_contains", func(t *testing.T) {
		g := "---\ntype: regex\npattern: 'x'\nmatch: not_contains\ntarget:\n  source: file\n  path: out/none.md\n---\n"
		if r := hjgOne(t, g, good, goodWS); r.Passed {
			t.Errorf("no file, not_contains passed: %+v", r)
		}
	})

	t.Run("an llm and a baseline grader are not graded", func(t *testing.T) {
		dir := hjgCase(t, map[string]string{
			"a-regex": hjgGraders["last-message"],
			"b-llm":   "---\ntype: llm\nfocus: trace\n---\nPASS if the reply is polite.\n",
			"c-base":  "---\ntype: baseline\n---\n",
		})
		got, err := GradeCase(dir, good, "")
		if err != nil {
			t.Fatal(err)
		}
		if !got.Passed || len(got.Graders) != 1 || strings.Join(got.NotGraded, ",") != "b-llm,c-base" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("one failing grader fails the case", func(t *testing.T) {
		dir := hjgCase(t, map[string]string{"a": hjgGraders["last-message"], "b": hjgGraders["not-contains"]})
		got, err := GradeCase(dir, bad, "")
		if err != nil {
			t.Fatal(err)
		}
		if got.Passed || len(got.Graders) != 2 || got.Graders[0].Passed || got.Graders[1].Passed {
			t.Errorf("got %+v", got)
		}
	})
}

func TestHjgRefuses(t *testing.T) {
	good := hjgData(t, "good.jsonl")
	one := func(g string) string { return hjgCase(t, map[string]string{"g": g}) }

	errRows := []struct {
		name       string
		dir        string
		transcript []byte
		files      string
		want       string
	}{
		{"unknown type", one("---\ntype: sentiment\n---\n"), good, "", "unknown grader type"},
		{"no type", one("---\npattern: x\n---\n"), good, "", "no type"},
		{"bad regex", one("---\ntype: regex\npattern: '('\ntarget: trace\n---\n"), good, "", "bad regex"},
		{"lookahead is not RE2", one("---\ntype: regex\npattern: 'a(?=b)'\ntarget: trace\n---\n"), good, "", "bad regex"},
		{"bad tool_used input_match", one("---\ntype: tool_used\ntool: Bash\ninput_match: '['\n---\n"), good, "", "input_match"},
		{"bad flag", one("---\ntype: regex\npattern: x\nflags: z\ntarget: trace\n---\n"), good, "", "unknown flag"},
		{"empty pattern", one("---\ntype: regex\ntarget: trace\n---\n"), good, "", "empty pattern"},
		{"unknown match", one("---\ntype: regex\npattern: x\nmatch: equals\ntarget: trace\n---\n"), good, "", "unknown match"},
		{"bad count", one("---\ntype: regex\npattern: x\nmatch: 'count:x'\ntarget: trace\n---\n"), good, "", "bad match"},
		{"unknown target", one("---\ntype: regex\npattern: x\ntarget: stdout\n---\n"), good, "", "unknown target"},
		{"no target", one("---\ntype: regex\npattern: x\n---\n"), good, "", "no target"},
		{"misspelt key", one("---\ntype: tool_used\ntool: Bash\nmaxx: 0\n---\n"), good, "", "unknown key"},
		{"max below min", one("---\ntype: tool_used\ntool: Bash\nmin: 3\nmax: 1\n---\n"), good, "", "do not fit"},
		{"tool_order without after", one("---\ntype: tool_order\nbefore: Read\n---\n"), good, "", "after must be"},
		{"file_exists without a files folder", one(hjgGraders["exists"]), good, "", "no files folder"},
		{"file_exists with a missing files folder", one(hjgGraders["exists"]), good, filepath.Join(t.TempDir(), "gone"), "not a folder"},
		{"path leaves the files folder", one("---\ntype: file_exists\npath: '../x'\n---\n"), good, hjgWS("ws-good"), "leaves"},
		{"no front matter", one("type: regex\n"), good, "", "front matter"},
		{"empty transcript", one(hjgGraders["last-message"]), nil, "", "empty transcript"},
		{"blank transcript", one(hjgGraders["last-message"]), []byte(" \n\n"), "", "empty transcript"},
		{"transcript without a JSON line", one(hjgGraders["last-message"]), []byte("not json\n"), "", "no trace"},
		{"no graders dir", func() string {
			dir := hjgCase(t, nil)
			return dir
		}(), good, "", "graders"},
		{"empty graders dir", func() string {
			dir := hjgCase(t, nil)
			if err := os.MkdirAll(filepath.Join(dir, "graders"), 0o755); err != nil {
				t.Fatal(err)
			}
			return dir
		}(), good, "", "no *.md grader"},
		{"not a case folder", t.TempDir(), good, "", "no prompt.md"},
	}
	for _, r := range errRows {
		t.Run(r.name, func(t *testing.T) {
			got, err := GradeCase(r.dir, r.transcript, r.files)
			if err == nil {
				t.Fatalf("no error, grade %+v", got)
			}
			if !strings.Contains(err.Error(), r.want) {
				t.Errorf("error %q lacks %q", err, r.want)
			}
			if got.Passed || len(got.Graders) != 0 {
				t.Errorf("an error came with a grade: %+v", got)
			}
		})
	}

	t.Run("an llm grader alone is not graded and does not pass", func(t *testing.T) {
		dir := hjgCase(t, map[string]string{"judge": "---\ntype: llm\nfocus: trace\n---\nPASS if sam agrees.\n"})
		got, err := GradeCase(dir, good, "")
		if err != nil {
			t.Fatal(err)
		}
		if got.Passed || len(got.Graders) != 0 || len(got.NotGraded) != 1 || got.NotGraded[0] != "judge" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a pattern in the front matter and the body is refused", func(t *testing.T) {
		g := "---\ntype: regex\npattern: a\ntarget: trace\n---\nb\n"
		if _, err := GradeCase(one(g), good, ""); err == nil || !strings.Contains(err.Error(), "both") {
			t.Errorf("err = %v", err)
		}
	})
}
