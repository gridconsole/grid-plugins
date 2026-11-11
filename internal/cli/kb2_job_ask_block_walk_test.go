package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// A Job agent runs under GRID_LEASE=<its Job>, where the feed_post tool refuses
// every write (TestMCPFeedPostRefusesJobSideWrites). The texts it reads must tell
// it to speak to the task with `grid job post`, which takes the Job lease, and
// must not tell it to use feed_post. These tests walk every agent text and every
// skill an agent lists, and run each `grid job post` example the texts show.

// kbJaTouched are the texts the lane rewrote; each must hold at least one span of
// each kind, so a walk over an unreadable or emptied set cannot pass.
var kbJaTouched = []string{"skills/run-job/SKILL.md", "agents/builder.md", "agents/reviewer.md"}

var (
	kbJaSpanRe        = regexp.MustCompile("`([^`\n]+)`")
	kbJaPlaceholderRe = regexp.MustCompile(`<[a-z][a-z-]*>`)
	kbJaItemRe        = regexp.MustCompile(`^\s+-\s+(\S+)\s*$`)
	kbJaSentenceRe    = regexp.MustCompile(`[.!?]\s+`)
	kbJaTopicRe       = regexp.MustCompile(`(?i)\b(question|finding|blocker)s?\b`)
	kbJaDirectiveRe   = regexp.MustCompile(`(?i)\b(post|send|tell|ask|write|use)\b[^;]*\bfeed_post\b`)
)

// kbJaRead reads a plugin file by its path from the plugin root, with LF line ends.
func kbJaRead(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// kbJaFrontSkills returns the skill names an agent's front matter lists under
// `skills:`, as a block list or as an inline list.
func kbJaFrontSkills(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return nil
	}
	var out []string
	in := false
	for _, l := range lines[1:] {
		if l == "---" {
			break
		}
		if strings.HasPrefix(l, "skills:") {
			in = true
			rest := strings.Trim(strings.TrimSpace(strings.TrimPrefix(l, "skills:")), "[]")
			for _, s := range strings.Split(rest, ",") {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
			continue
		}
		if in {
			if m := kbJaItemRe.FindStringSubmatch(l); m != nil {
				out = append(out, m[1])
				continue
			}
			in = false
		}
	}
	return out
}

// kbJaWalked is every file in agents/ plus the skills/<name>/SKILL.md of every
// skill an agent's front matter lists. A new agent joins by itself.
func kbJaWalked(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "..", "agents"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var files []string
	add := func(rel string) {
		if !seen[rel] {
			seen[rel] = true
			files = append(files, rel)
		}
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		rel := "agents/" + e.Name()
		add(rel)
		for _, name := range kbJaFrontSkills(kbJaRead(t, rel)) {
			add("skills/" + name + "/SKILL.md")
		}
	}
	sort.Strings(files)
	for _, rel := range kbJaTouched {
		if !seen[rel] {
			t.Fatalf("the walked set %q does not hold %s", files, rel)
		}
	}
	return files
}

type kbJaSpan struct{ file, text string }

// kbJaSpans returns, in walk order, every code span of the walked set that ok accepts.
func kbJaSpans(t *testing.T, ok func(span string) bool) []kbJaSpan {
	t.Helper()
	var out []kbJaSpan
	for _, rel := range kbJaWalked(t) {
		for _, m := range kbJaSpanRe.FindAllStringSubmatch(kbJaRead(t, rel), -1) {
			if ok(m[1]) {
				out = append(out, kbJaSpan{rel, m[1]})
			}
		}
	}
	return out
}

// kbJaTouchedHold fails for each Touches text with no span in spans.
func kbJaTouchedHold(t *testing.T, spans []kbJaSpan, what string) {
	t.Helper()
	have := map[string]int{}
	for _, s := range spans {
		have[s.file]++
	}
	for _, rel := range kbJaTouched {
		if have[rel] == 0 {
			t.Errorf("%s: no code span %s, so the walk would pass without running an example from it", rel, what)
		}
	}
}

// kbJaArgs turns a `grid job post ...` span into the arguments after `grid`, each
// placeholder filled from fill; a placeholder fill lacks fails the test.
func kbJaArgs(t *testing.T, span string, fill map[string]string) []string {
	t.Helper()
	fields := strings.Fields(span)
	if len(fields) < 3 || strings.Join(fields[:3], " ") != "grid job post" {
		t.Fatalf("not a grid job post span: %q", span)
	}
	args := fields[1:]
	for i, f := range args {
		for _, ph := range kbJaPlaceholderRe.FindAllString(f, -1) {
			v, ok := fill[ph]
			if !ok {
				t.Fatalf("span %q holds the placeholder %s, which the walk cannot fill", span, ph)
			}
			args[i] = strings.ReplaceAll(args[i], ph, v)
		}
	}
	return args
}

// kbJaJobPosts counts the post records the Feed holds for a Job.
func kbJaJobPosts(t *testing.T, store feed.Store, job string) int {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: "post"})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, rec := range r.Records {
		if rec.Job != nil && *rec.Job == job {
			n++
		}
	}
	return n
}

// Row 1: a text that names feed_post says it refuses a write under a Job lease,
// and none tells the agent to post a question, a finding or a blocker with it.
func TestKbJaNoFeedPostWrite(t *testing.T) {
	named := map[string]bool{}
	for _, rel := range kbJaWalked(t) {
		for _, p := range strings.Split(kbJaRead(t, rel), "\n\n") {
			if !strings.Contains(p, "feed_post") {
				continue
			}
			named[rel] = true
			if !strings.Contains(p, "refuses") {
				t.Errorf("%s: a paragraph names feed_post and does not say it refuses a Job-lease write: %s", rel, p)
			}
			for _, s := range kbJaSentenceRe.Split(p, -1) {
				if !strings.Contains(s, "feed_post") || strings.Contains(s, "refuses") && strings.Contains(s, "Job lease") {
					continue
				}
				if kbJaTopicRe.MatchString(s) && kbJaDirectiveRe.MatchString(s) {
					t.Errorf("%s: a sentence tells the agent to post a question, a finding or a blocker with feed_post: %s", rel, s)
				} else {
					t.Errorf("%s: a sentence names feed_post without saying it refuses a Job-lease write: %s", rel, s)
				}
			}
		}
	}
	for _, rel := range kbJaTouched {
		if !named[rel] {
			t.Errorf("%s: no paragraph names feed_post; keep the note that it refuses a Job-lease write", rel)
		}
	}
}

// Row 2: each `grid job post ... --type post` example runs under the Job lease an
// app-started agent has, and leaves a post record for its Job.
func TestKbJaPostSpansRunUnderJobLease(t *testing.T) {
	spans := kbJaSpans(t, func(s string) bool {
		return strings.HasPrefix(s, "grid job post") && strings.Contains(s, "--type post")
	})
	kbJaTouchedHold(t, spans, "starting `grid job post` with `--type post`")
	for i, sp := range spans {
		t.Run(fmt.Sprintf("%s#%d", sp.file, i), func(t *testing.T) {
			h, path, store, _, _ := assignedExitHarness(t, true)
			t.Setenv("GRID_ROLE", "worker")
			t.Setenv("GRID_LEASE", "J-1")
			t.Setenv("GRID_JOB", "J-1")
			t.Setenv("GRID_ATTEMPT", "1")
			before := kbJaJobPosts(t, store, "J-1")
			args := kbJaArgs(t, sp.text, map[string]string{
				"<task>": path, "<job>": "J-1", "<review-job>": "J-1", "<json>": exitMessage, "<owner>": "dana",
			})
			if code := h.run(path, args...); code != 0 {
				t.Fatalf("%s: `%s` exit %d: %s", sp.file, sp.text, code, h.errb)
			}
			if got := kbJaJobPosts(t, store, "J-1"); got != before+1 {
				t.Fatalf("%s: `%s` left %d post records for J-1, want %d", sp.file, sp.text, got, before+1)
			}
		})
	}
}

// Row 3: each `grid job post --person ... --body <json>` example runs as the agent
// of J-1, a task write on GRID_TASK_EPOCH, and adds a Job with role Person.
func TestKbJaPersonSpansRunOnTaskEpoch(t *testing.T) {
	spans := kbJaSpans(t, func(s string) bool {
		return strings.HasPrefix(s, "grid job post") && strings.Contains(s, "--person") && strings.Contains(s, "--body <json>")
	})
	kbJaTouchedHold(t, spans, "starting `grid job post` with `--person` and `--body <json>`")
	for i, sp := range spans {
		t.Run(fmt.Sprintf("%s#%d", sp.file, i), func(t *testing.T) {
			a := newAgentTask(t)
			a.agentEnv(t, a.epoch())
			t.Setenv("GRID_JOB", "J-1")
			t.Setenv("GRID_ATTEMPT", "1")
			args := kbJaArgs(t, sp.text, map[string]string{
				"<task>": a.path, "<job>": "J-1", "<json>": personBody(t, "J-2", nil), "<owner>": "dana",
			})
			if code := a.h.run(a.path, args...); code != 0 {
				t.Fatalf("%s: `%s` exit %d: %s", sp.file, sp.text, code, a.h.errb)
			}
			a.h.out.Reset()
			if code := a.h.run(a.path, "job", "tree", "--json"); code != 0 {
				t.Fatalf("job tree: exit %d %s", code, a.h.errb)
			}
			var rows []struct{ ID, Role string }
			if err := json.Unmarshal(a.h.out.Bytes(), &rows); err != nil {
				t.Fatal(err, a.h.out.String())
			}
			for _, r := range rows {
				if r.ID == "J-2" {
					if r.Role != "Person" {
						t.Fatalf("%s: J-2 has role %q, want Person", sp.file, r.Role)
					}
					return
				}
			}
			t.Fatalf("%s: job tree does not show J-2 after `%s`: %s", sp.file, sp.text, a.h.out.String())
		})
	}
}
