package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// The read lists of the 11 Nov run: J-1's four pointers had no range and render
// refused them; J-2 named a section and line ranges and rendered.
var (
	findingsBadRead  = []string{"task:intent.md", "task:inputs/H-website-docs-report.md", "task:inputs/draft", "task:inputs/old-docs"}
	findingsGoodRead = []string{"task:intent.md#request", "task:inputs/H-website-docs-report.md:1-31", "task:inputs/draft/README.md:1-44"}
)

func findingsLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// findingsTask is the task folder of the run: intent.md with a request section and the three inputs.
func findingsTask(t *testing.T) (*harness, string, feed.Store) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	h.write(filepath.Join(path, "intent.md"), "# Intent\n\n## Request {#request}\n\nRewrite the website docs.\n")
	h.write(filepath.Join(path, "inputs", "H-website-docs-report.md"), findingsLines(34))
	h.write(filepath.Join(path, "inputs", "draft", "README.md"), findingsLines(46))
	h.write(filepath.Join(path, "inputs", "old-docs", "README.md"), findingsLines(5))
	return h, path, store
}

// findingsJob is the first Job's record as main posts it, with the given id and read list.
func findingsJob(id string, read []string) map[string]any {
	body := firstJob()
	body["id"], body["read"] = id, read
	return body
}

func postFindingsJob(t *testing.T, h *harness, path string, body map[string]any, extra ...string) int {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{"job", "post", "--type", "job", "--body", string(data)}, extra...)
	return h.run(path, args...)
}

type postOut struct {
	feed.Record
	Findings *[]lint.Finding `json:"findings"`
}

func decodePost(t *testing.T, h *harness) postOut {
	t.Helper()
	var out postOut
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, h.out)
	}
	return out
}

func hasRule(fs []lint.Finding, rule, text string) bool {
	for _, f := range fs {
		if f.Rule == rule && strings.Contains(f.Message, text) {
			return true
		}
	}
	return false
}

// storedJob reads the Job record back from the Feed.
func storedJob(t *testing.T, store feed.Store, id string) feed.Entry {
	t.Helper()
	res, err := store.ReadOnly(feed.Filter{Type: "job", Job: id})
	if err != nil || len(res.Records) != 1 {
		t.Fatalf("stored %s: %v %+v", id, err, res.Records)
	}
	return res.Records[0]
}

func TestJobPostReportsTheFindingsRenderRefusesOn(t *testing.T) {
	h, path, store := findingsTask(t)
	body := findingsJob("J-2", findingsBadRead)
	if code := postFindingsJob(t, h, path, body, "--json"); code != 0 {
		t.Fatalf("post exit %d: %s", code, h.errb)
	}
	out := decodePost(t, h)
	if out.Findings == nil || !hasRule(*out.Findings, "job.read-range", "Read pointer has no range: task:intent.md") {
		t.Fatalf("no job.read-range finding for task:intent.md: %s", h.out)
	}
	for _, f := range *out.Findings {
		if f.Rule == "job.read-range" && f.Severity != "error" {
			t.Fatalf("job.read-range is not an error: %+v", f)
		}
	}
	// The record is the one posted, under the same fields as without findings.
	if out.Type != "job" || out.Job == nil || *out.Job != "J-2" || out.ID == "" {
		t.Fatalf("record fields: %+v", out.Record)
	}
	st := storedJob(t, store, "J-2")
	if st.Type != "job" || !reflect.DeepEqual(fmt.Sprint(st.Body["read"]), fmt.Sprint([]any{"task:intent.md", "task:inputs/H-website-docs-report.md", "task:inputs/draft", "task:inputs/old-docs"})) {
		t.Fatalf("stored record: %+v", st)
	}
	if st.ID != out.ID {
		t.Fatalf("stored id %s, printed %s", st.ID, out.ID)
	}
}

func TestJobPostPrintsFindingsOnStderr(t *testing.T) {
	h, path, _ := findingsTask(t)
	if code := postFindingsJob(t, h, path, findingsJob("J-2", findingsBadRead)); code != 0 {
		t.Fatalf("post exit %d: %s", code, h.errb)
	}
	want := "grid job post: J-2: job.read-range: Read pointer has no range: task:intent.md (render will refuse this Job)\n"
	if !strings.Contains(h.errb.String(), want) {
		t.Fatalf("stderr lacks %q:\n%s", want, h.errb)
	}
	if got := strings.TrimSpace(h.out.String()); strings.Contains(got, "\n") || strings.Contains(got, "findings") || got == "" {
		t.Fatalf("stdout is the record id alone: %q", got)
	}
	for _, line := range strings.Split(strings.TrimSpace(h.errb.String()), "\n") {
		if !strings.HasPrefix(line, "grid job post: J-2: ") {
			t.Fatalf("stderr line without the prefix: %q", line)
		}
		if strings.Contains(line, "render will refuse") && !strings.Contains(line, "job.") {
			t.Fatalf("refusal without a rule: %q", line)
		}
	}
}

func TestJobPostCleanRecordMatchesJobLint(t *testing.T) {
	h, path, store := findingsTask(t)
	body := findingsJob("J-2", findingsGoodRead)
	if code := postFindingsJob(t, h, path, body, "--json"); code != 0 {
		t.Fatalf("post exit %d: %s", code, h.errb)
	}
	out := decodePost(t, h)
	if out.Findings == nil {
		t.Fatalf("no findings array: %s", h.out)
	}
	if lint.HasErrors(*out.Findings) {
		t.Fatalf("clean record has an error finding: %+v", *out.Findings)
	}
	if strings.Contains(h.errb.String(), "render will refuse") {
		t.Fatalf("stderr: %s", h.errb)
	}
	if st := storedJob(t, store, "J-2"); st.Type != "job" {
		t.Fatalf("stored: %+v", st)
	}
	// The same record through grid job lint gives the same findings.
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "J-2.json")
	h.write(file, string(data))
	posted := *out.Findings
	if code := h.run(path, "job", "lint", file, "--task", path, "--json"); code != 0 {
		t.Fatalf("job lint exit %d: %s %s", code, h.out, h.errb)
	}
	var linted struct {
		Findings []lint.Finding `json:"findings"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &linted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(posted, linted.Findings) {
		t.Fatalf("post findings %+v\njob lint findings %+v", posted, linted.Findings)
	}
}

func TestJobPostWarningHasNoRefusalText(t *testing.T) {
	var b strings.Builder
	printPostFindings(&b, "J-2", []lint.Finding{{Rule: "job.x", Severity: "warning", Message: "careful"}, {Rule: "job.y", Severity: "error", Message: "bad"}}, nil)
	want := "grid job post: J-2: job.x: careful\ngrid job post: J-2: job.y: bad (render will refuse this Job)\n"
	if b.String() != want {
		t.Fatalf("got %q want %q", b.String(), want)
	}
}

func TestJobPostPersonJobIsNotLinted(t *testing.T) {
	h, path, _ := findingsTask(t)
	body := findingsJob("J-2", findingsBadRead)
	if code := postFindingsJob(t, h, path, body, "--person", "member"); code != 0 {
		t.Fatalf("person post exit %d: %s", code, h.errb)
	}
	if strings.TrimSpace(h.errb.String()) != "" || strings.Contains(h.out.String(), "findings") {
		t.Fatalf("a person Job was linted: out %q err %q", h.out, h.errb)
	}
	if code := postFindingsJob(t, h, path, findingsJob("J-3", findingsBadRead), "--person", "member", "--json"); code != 0 {
		t.Fatalf("person post exit %d: %s", code, h.errb)
	}
	if strings.Contains(h.out.String(), "findings") {
		t.Fatalf("--json of a person Job holds findings: %s", h.out)
	}
}

func TestJobPostOtherTypesAreNotLinted(t *testing.T) {
	h, path, _ := findingsTask(t)
	if code := h.run(path, "job", "post", "J-1", "--type", "control", "--body", `{"target":"J-1","order":5}`, "--json"); code != 0 {
		t.Fatalf("post exit %d: %s", code, h.errb)
	}
	if strings.Contains(h.out.String(), "findings") || strings.Contains(h.errb.String(), "grid job post:") {
		t.Fatalf("a non-job post was linted: out %q err %q", h.out, h.errb)
	}
}

func TestJobPostFindingsUnavailableStillPosts(t *testing.T) {
	h, path, store := findingsTask(t)
	// A repo path that leaves the project folder makes the task's sources fail to resolve.
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: feedMachine, Path: h.root}}, Repos: []project.Repo{{Path: "../outside"}}})
	if code := postFindingsJob(t, h, path, findingsJob("J-2", findingsBadRead), "--json"); code != 0 {
		t.Fatalf("post exit %d: %s", code, h.errb)
	}
	out := decodePost(t, h)
	if out.Findings == nil || len(*out.Findings) != 0 {
		t.Fatalf("findings should be []: %s", h.out)
	}
	if !strings.Contains(h.out.String(), `"findings":[]`) {
		t.Fatalf("findings is not an empty array: %s", h.out)
	}
	if n := strings.Count(strings.TrimSpace(h.errb.String()), "\n") + 1; n != 1 || !strings.Contains(h.errb.String(), "the findings could not be checked") {
		t.Fatalf("stderr: %q", h.errb)
	}
	if st := storedJob(t, store, "J-2"); st.Type != "job" {
		t.Fatalf("stored: %+v", st)
	}
}
