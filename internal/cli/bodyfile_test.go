package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/signal"
)

// --body-file is --body - for an agent whose shell allows no redirect, pipe or
// here-document: the body is a file the agent wrote, named by a flag.

// A relative path is read from the current folder, and the body is the whole file.
func TestBodyFileJobResultReadsTheFileFromTheCurrentFolder(t *testing.T) {
	h, path, store := bdPersonJob(t)
	h.write(filepath.Join(path, "answer.json"), "{\"answer\":\"Yes\"}\n")
	if code := h.run(path, "job", "result", "J-2", "--body-file", "answer.json"); code != 0 {
		t.Fatalf("job result --body-file: %d %s", code, h.errb)
	}
	bdRequireRefolded(t, h, store)
}

func TestBodyFileJobPostTakesAnAbsolutePath(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = "J-2"
	body["acceptance"] = []any{map[string]any{"id": "A1", "text": "`grid deliver --task T-01 --json` exit 0, the owner's $HOME text", "traces": "item"}}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "job.json")
	h.write(file, string(data))
	if code := h.run(path, "job", "post", "J-2", "--type", "job", "--body-file", file); code != 0 {
		t.Fatalf("job post --body-file: %d %s", code, h.errb)
	}
	read, err := store.Read(feed.Filter{Type: "job", Job: "J-2"})
	if err != nil || len(read.Records) == 0 {
		t.Fatalf("J-2 not posted: %v", err)
	}
	out, _ := json.Marshal(read.Records)
	if !strings.Contains(string(out), "`grid deliver --task T-01 --json` exit 0, the owner's $HOME text") {
		t.Errorf("the body lost backticks, the quote or the dollar sign: %s", out)
	}
}

func TestBodyFileFeedPost(t *testing.T) {
	h, task := feedHarness(t)
	h.write(filepath.Join(task, "msg.json"), `{"ref":"task","kind":"message","text":"from a file"}`)
	if code := h.run(task, "feed", "post", "--type", "post", "--body-file", "msg.json"); code != 0 {
		t.Fatalf("feed post --body-file: %d %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), "from a file") {
		t.Errorf("the post does not show the file's text: %s", h.out)
	}
}

// Stdin is not read when a file is named, so a closed or empty stdin does not
// replace the body.
func TestBodyFileIgnoresStdin(t *testing.T) {
	h, task := feedHarness(t)
	h.write(filepath.Join(task, "msg.json"), `{"ref":"task","kind":"message","text":"file wins"}`)
	h.env.Stdin = strings.NewReader(`{"ref":"task","kind":"message","text":"stdin loses"}`)
	if code := h.run(task, "feed", "post", "--type", "post", "--body-file", "msg.json"); code != 0 {
		t.Fatalf("feed post --body-file: %d %s", code, h.errb)
	}
	if out := h.out.String(); !strings.Contains(out, "file wins") || strings.Contains(out, "stdin loses") {
		t.Errorf("the post is not the file's body: %s", out)
	}
}

func TestBodyFileSignalAddKeepsTheBodyAsItIs(t *testing.T) {
	h := newHarness(t)
	folder := lvSigProject(t, h)
	text := "Call me `today` about $HOME and the owner's \"plan\".\n"
	h.write(filepath.Join(folder, "body.txt"), text)
	code := h.run(folder, "signal", "add", "--source", "mail", "--source-id", "m-1", "--title", "T", "--body-file", "body.txt", "--json")
	if code != 0 {
		t.Fatalf("signal add --body-file: %d %s", code, h.errb)
	}
	var r struct{ Result, Note string }
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil || r.Result != "written" {
		t.Fatalf("answer %q: %v", h.out, err)
	}
	if note := lvSigNote(t, h, r.Note); !strings.Contains(note, strings.TrimSuffix(text, "\n")) {
		t.Errorf("the note lost part of the body: %q", note)
	}
}

func TestBodyFileSignalAddRefusesAFileOverTheLimit(t *testing.T) {
	h := newHarness(t)
	folder := lvSigProject(t, h)
	h.write(filepath.Join(folder, "big.txt"), strings.Repeat("a", signal.MaxBytes+1))
	code := h.run(folder, "signal", "add", "--source", "mail", "--source-id", "m-2", "--title", "T", "--body-file", "big.txt")
	if code == 0 || !strings.Contains(h.errb.String(), "exceeds") {
		t.Fatalf("a body over the limit: exit %d, stderr %q", code, h.errb)
	}
	if names := lvSigNotes(h); len(names) != 0 {
		t.Errorf("a refused add wrote %v", names)
	}
}

// One body: --body and --body-file together are refused and nothing is written.
func TestBodyFileWithBodyIsRefused(t *testing.T) {
	h, task := feedHarness(t)
	h.write(filepath.Join(task, "msg.json"), `{"ref":"task","kind":"message","text":"x"}`)
	for _, args := range [][]string{
		{"feed", "post", "--type", "post", "--body", "{}", "--body-file", "msg.json"},
		{"feed", "post", "--type", "post", "--body-file", "msg.json", "--body", "-"},
		{"job", "post", "J-2", "--type", "job", "--body", "{}", "--body-file", "msg.json"},
	} {
		code := h.run(task, args...)
		if code == 0 || !strings.Contains(h.errb.String(), "--body and --body-file are both given") {
			t.Errorf("%v: exit %d, stderr %q", args, code, h.errb)
		}
	}
	hr, path, _ := bdPersonJob(t)
	hr.write(filepath.Join(path, "answer.json"), `{"answer":"Yes"}`)
	if code := hr.run(path, "job", "result", "J-2", "--body", "{}", "--body-file", "answer.json"); code == 0 || !strings.Contains(hr.errb.String(), "--body and --body-file are both given") {
		t.Errorf("job result: exit %d, stderr %q", code, hr.errb)
	}
	h2 := newHarness(t)
	folder := lvSigProject(t, h2)
	h2.write(filepath.Join(folder, "body.txt"), "text")
	code := h2.run(folder, "signal", "add", "--source", "mail", "--title", "T", "--body", "-", "--body-file", "body.txt")
	if code == 0 || !strings.Contains(h2.errb.String(), "--body and --body-file are both given") {
		t.Errorf("signal add: exit %d, stderr %q", code, h2.errb)
	}
	if names := lvSigNotes(h2); len(names) != 0 {
		t.Errorf("a refused add wrote %v", names)
	}
}

func TestBodyFileMissingFileIsRefused(t *testing.T) {
	h, task := feedHarness(t)
	code := h.run(task, "feed", "post", "--type", "post", "--body-file", "nope.json")
	if code == 0 || !strings.Contains(h.errb.String(), "--body-file") || !strings.Contains(h.errb.String(), "nope.json") {
		t.Fatalf("a missing file: exit %d, stderr %q", code, h.errb)
	}
}

// A file that is not a JSON object fails the way a bad --body does.
func TestBodyFileNotJSONIsRefused(t *testing.T) {
	h, task := feedHarness(t)
	h.write(filepath.Join(task, "bad.json"), "not json")
	code := h.run(task, "feed", "post", "--type", "post", "--body-file", "bad.json")
	if code != 3 || !strings.Contains(h.errb.String(), "--body must be a JSON object") {
		t.Fatalf("a body that is not JSON: exit %d, stderr %q", code, h.errb)
	}
}

// The role policy skips the value of --body-file, as it does the value of --body,
// so text after it cannot pose as the --act the policy matches on.
func TestBodyFileRoleFlagSkipsTheValue(t *testing.T) {
	args := []string{"signal", "act", "S-1", "--act", "dismiss", "--body-file", "--act=turn"}
	if got := roleFlag(args, "act", "note"); got != "dismiss" {
		t.Fatalf("roleFlag(%q) = %q, want dismiss", args, got)
	}
}

func TestBodyFileHelpNamesTheFlag(t *testing.T) {
	h, task := feedHarness(t)
	for _, args := range [][]string{{"job", "post", "--help"}, {"job", "result", "--help"}, {"feed", "post", "--help"}, {"signal", "add", "--help"}} {
		h.run(task, args...)
		help := h.errb.String() + h.out.String()
		if !strings.Contains(help, "-body-file") || !strings.Contains(help, "here-document") {
			t.Errorf("%v help lacks --body-file: %s", args, help)
		}
	}
}
