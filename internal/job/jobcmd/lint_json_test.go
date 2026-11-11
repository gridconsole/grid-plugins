package jobcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// A job record lints through the CLI like a brief: clean exits 0, a record
// missing a required field exits 1 with the field path, a brief is unchanged.
func TestLintJobRecordJSON(t *testing.T) {
	e, out, errs := fixture(t)
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	brief, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	j, _, err := job.DecodeBrief(string(brief))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := j.Record()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "job.json")
	if err := os.WriteFile(good, rec, 0o644); err != nil {
		t.Fatal(err)
	}
	got := call(t, e, out, errs, 0, "lint", good, "--task", "T-01", "--json")
	if !got.OK {
		t.Fatalf("%+v", got)
	}

	// Content sniffing: a record in a file with no .json suffix.
	sniffed := filepath.Join(dir, "record.txt")
	if err := os.WriteFile(sniffed, append([]byte("\n  "), rec...), 0o644); err != nil {
		t.Fatal(err)
	}
	call(t, e, out, errs, 0, "lint", sniffed, "--task", "T-01", "--json")

	var m map[string]any
	if err := json.Unmarshal(rec, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "title")
	bad, _ := json.Marshal(m)
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	got = call(t, e, out, errs, 1, "lint", badPath, "--task", "T-01", "--json")
	if got.OK || len(got.Findings) == 0 || got.Findings[0].Rule != "job.record" || !strings.Contains(got.Findings[0].Message, "title") {
		t.Fatalf("%+v", got)
	}

	// The saved markdown brief lints exactly as before.
	md := call(t, e, out, errs, 0, "lint", r.Path, "--json")
	if !md.OK || len(md.Findings) != 0 {
		t.Fatalf("%+v", md)
	}
}
