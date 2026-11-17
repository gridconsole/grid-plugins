package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var jbUpdate = flag.Bool("jb-update", false, "rewrite the golden Job records of the brief tests")

// The three briefs are real lane briefs, one each of a P- build, a C- change and an R-
// research lane, with paths, hosts and names scrubbed.
var jbBriefs = []string{"P-land-kind.md", "C-ak-open-home-without-task.md", "R-land-job-type.md"}

var jbAnchor = regexp.MustCompile(`#L([0-9]+)(?:-L([0-9]+))?$`)

// jbLines answers the lines a pointer such as inputs/b.md#L3-L5 names in text.
func jbLines(t *testing.T, text []byte, pointer string) []string {
	t.Helper()
	m := jbAnchor.FindStringSubmatch(pointer)
	if m == nil {
		t.Fatalf("pointer %q names no lines", pointer)
	}
	from, _ := strconv.Atoi(m[1])
	to := from
	if m[2] != "" {
		to, _ = strconv.Atoi(m[2])
	}
	lines := strings.Split(string(text), "\n")
	if from < 1 || to > len(lines) || from > to {
		t.Fatalf("pointer %q is outside the %d lines of the brief", pointer, len(lines))
	}
	return lines[from-1 : to]
}

// jbBody answers a stored record body as plain JSON values.
func jbBody(t *testing.T, b any) map[string]any {
	t.Helper()
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

// Rows 1, 2 and 4: grid job post --from-brief posts a record that lints with no error,
// copies the brief into the task's inputs, points intent at its lines, and posts the golden record.
func TestJbJobPostFromBrief(t *testing.T) {
	for _, name := range jbBriefs {
		t.Run(name, func(t *testing.T) {
			h, path, store := orchestratorHarness(t)
			src, err := filepath.Abs(filepath.Join("testdata", "jb_from_brief", name))
			if err != nil {
				t.Fatal(err)
			}
			if code := h.run(path, "job", "post", "--task", path, "--from-brief", src, "--json"); code != 0 {
				t.Fatalf("job post --from-brief exit %d\nstdout %s\nstderr %s", code, h.out, h.errb)
			}
			out := decodePost(t, h)
			if out.Findings == nil {
				t.Fatalf("the post printed no findings: %s", h.out)
			}
			for _, f := range *out.Findings {
				if f.Severity == "error" {
					t.Errorf("lint error %s: %s", f.Rule, f.Message)
				}
			}
			// Row 2: the brief is in the task's inputs, byte for byte.
			want, _ := os.ReadFile(src)
			copied, err := os.ReadFile(filepath.Join(path, "inputs", name))
			if err != nil || !bytes.Equal(copied, want) {
				t.Fatalf("inputs/%s is not the brief: %v", name, err)
			}
			rec := storedJob(t, store, "J-2")
			body := jbBody(t, rec.Body)
			// Row 2: intent points at the brief's lines: the Why line, the title line, the rows.
			intent, _ := body["intent"].(map[string]any)
			for key, prefix := range map[string]string{"owner": "Why:", "item": "# "} {
				p, _ := intent[key].(string)
				if !strings.HasPrefix(p, "inputs/"+name+"#") {
					t.Errorf("intent.%s = %q, want a pointer into inputs/%s", key, p, name)
				} else if l := jbLines(t, copied, p); len(l) != 1 || !strings.HasPrefix(l[0], prefix) {
					t.Errorf("intent.%s %s names %q, want the %q line", key, p, l, prefix)
				}
			}
			goal, _ := intent["goal"].(string)
			rows := jbLines(t, copied, goal)
			acc, _ := body["acceptance"].([]any)
			if len(rows) != len(acc) || len(acc) == 0 {
				t.Errorf("intent.goal %s names %d lines, the record has %d rows", goal, len(rows), len(acc))
			}
			for i, l := range rows {
				if !strings.HasPrefix(l, strconv.Itoa(i+1)+". ") {
					t.Errorf("intent.goal line %d is %q, want row %d", i+1, l, i+1)
				}
			}
			// Row 4: the record is the golden one.
			got, err := json.MarshalIndent(body, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			golden := filepath.Join("testdata", "jb_from_brief", strings.TrimSuffix(name, ".md")+".golden.json")
			if *jbUpdate {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			wantBody, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, wantBody) {
				t.Errorf("record differs from %s:\n%s", golden, got)
			}
		})
	}
}

// Row 3 input side: the record a brief posts carries model, effort and base for the spawn line.
func TestJbFromBriefCarriesModelEffortBase(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	src, _ := filepath.Abs(filepath.Join("testdata", "jb_from_brief", "P-land-kind.md"))
	if code := h.run(path, "job", "post", "--task", path, "--from-brief", src); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	body := jbBody(t, storedJob(t, store, "J-2").Body)
	if body["model"] != "sonnet" || body["effort"] != "high" || body["base"] != "main" {
		t.Fatalf("model %v effort %v base %v, want sonnet high main", body["model"], body["effort"], body["base"])
	}
}

// --from-brief builds the body itself: with --body or on another verb it is a usage error.
func TestJbFromBriefRefusesBody(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	if code := h.run(path, "job", "post", "--task", path, "--from-brief", "b.md", "--body", `{"id":"J-2"}`); code != 2 {
		t.Fatalf("exit %d, want 2: %s", code, h.errb)
	}
}
