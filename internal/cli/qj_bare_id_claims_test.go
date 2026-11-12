package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// qjSkills are the three skills that carry the bare-id sentence.
var qjSkills = []string{"deliver", "questions", "research"}

// qjRun is a run of backticked verbs joined by ", " or " and ".
const qjRun = "((?:`[^`]+`(?:, | and )?)+)"

var (
	qjRefuse = regexp.MustCompile(qjRun + ` answers? "needs --project"`)
	qjFind   = regexp.MustCompile(`(?:works only for ` + qjRun + `|` + qjRun + ` still find it)`)
	qjTick   = regexp.MustCompile("`([^`]+)`")
)

// qjSentence cuts the bare-id sentence out of one skill: from "A bare id such as"
// to the end of its line.
func qjSentence(t *testing.T, skill string) string {
	t.Helper()
	text, ok := gridplugins.SkillFor(skill)
	if !ok {
		t.Fatalf("no skill %s", skill)
	}
	i := strings.Index(text, "A bare id such as")
	if i < 0 {
		t.Fatalf("skills/%s/SKILL.md has no bare-id sentence", skill)
	}
	s := text[i:]
	if j := strings.IndexByte(s, '\n'); j >= 0 {
		s = s[:j]
	}
	return s
}

// qjVerbs are the verbs the sentence names in the run a pattern matches.
func qjVerbs(re *regexp.Regexp, sentence string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(sentence, -1) {
		for _, run := range m[1:] {
			for _, v := range qjTick.FindAllStringSubmatch(run, -1) {
				out = append(out, v[1])
			}
		}
	}
	return out
}

// qjArgs is the read-only run of each verb the three sentences name, given the
// task operand. `answer` and `task history` take the task as an operand.
func qjArgs(verb, task string) []string {
	switch verb {
	case "plan items", "plan":
		return []string{"plan", "items", "--task", task}
	case "question list", "question":
		return []string{"question", "list", "--task", task}
	case "answer":
		return []string{"answer", task, "Q-1", "x"}
	case "person":
		return []string{"person", "approvers", "--area", "x", "--task", task}
	case "where":
		return []string{"where", "--task", task}
	case "job show":
		return []string{"job", "show", "--task", task, "J-1"}
	case "job result":
		return []string{"job", "result", "--task", task, "--job", "J-1", "--body", "{}"}
	case "proof":
		return []string{"proof", "index", "--task", task}
	case "brain":
		return []string{"brain", "search", "x", "--task", task}
	case "task history":
		return []string{"task", "history", task}
	case "deliver":
		return []string{"deliver", "--dry-run", "--outcome", "x", "--task", task}
	}
	return nil
}

// qjWorld is a registered project with a work folder and one task T-01, a folder in
// no project, and the caller's role.
func qjWorld(t *testing.T, role string) (h *harness, projDir, noProject, taskDir string) {
	t.Helper()
	h = newHarness(t)
	h.env.Machine = feedMachine
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_ROLE", role)
	for _, k := range []string{"GRID_JOB", "GRID_TASK", "GRID_FEED_TASK", "GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_PROJECT"} {
		t.Setenv(k, "")
	}
	projDir = filepath.Join(h.root, "work", "example")
	h.mkdir(projDir)
	h.record(project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: h.env.Machine, Path: projDir}}})
	if code := h.run(projDir, "task", "new", "Add search", "--json"); code != 0 {
		t.Fatalf("task new: %d %s", code, h.errb)
	}
	var made struct{ ID, Path string }
	if err := json.Unmarshal(h.out.Bytes(), &made); err != nil || made.ID != "T-01" {
		t.Fatalf("task new gave %q, %v; want T-01", h.out, err)
	}
	if _, err := os.Stat(made.Path); err != nil {
		t.Fatal(err)
	}
	return h, projDir, h.tmp, made.Path
}

// qjRefused reports which resolver message the run printed.
func qjRefused(h *harness) (needsProject, notFound bool) {
	e := h.errb.String()
	return strings.Contains(e, "needs --project"), strings.Contains(e, "task not found")
}

func TestQjBareIdClaims(t *testing.T) {
	owner, ownerProj, ownerNone, ownerTask := qjWorld(t, "owner")
	seen := map[string]bool{}
	for _, skill := range qjSkills {
		sentence := qjSentence(t, skill)
		refuse, find := qjVerbs(qjRefuse, sentence), qjVerbs(qjFind, sentence)
		if len(refuse) == 0 || len(find) == 0 {
			t.Fatalf("%s: could not read which verbs refuse and which find a bare id in: %s", skill, sentence)
		}
		// Without the project-folder qualifier the sentence says a bare id is
		// refused wherever it is run.
		insideOK := strings.Contains(sentence, "inside the task's project folder")
		pathOK := strings.Contains(sentence, "the folder path works from anywhere")
		for _, group := range []struct {
			verbs   []string
			refuses bool
		}{{refuse, true}, {find, false}} {
			for _, verb := range group.verbs {
				seen[verb] = true
				args := qjArgs(verb, "T-01")
				if args == nil {
					t.Fatalf("%s: no read-only run for verb %q", skill, verb)
				}
				owner.run(ownerProj, args...)
				if needs, _ := qjRefused(owner); needs != (group.refuses && !insideOK) {
					t.Errorf("%s: `%s` bare id from the project folder printed needs --project=%v; the sentence says it refuses=%v\n  %v\n  stderr: %s\n  %s",
						skill, verb, needs, group.refuses && !insideOK, args, owner.errb, sentence)
				}
				owner.run(ownerNone, args...)
				if needs, notFound := qjRefused(owner); needs != group.refuses || notFound {
					t.Errorf("%s: `%s` bare id from a folder in no project printed needs --project=%v task-not-found=%v; the sentence says it refuses=%v\n  %v\n  stderr: %s\n  %s",
						skill, verb, needs, notFound, group.refuses, args, owner.errb, sentence)
				}
				if pathOK {
					path := qjArgs(verb, ownerTask)
					owner.run(ownerNone, path...)
					if needs, notFound := qjRefused(owner); needs || notFound {
						t.Errorf("%s: `%s` with the folder path from a folder in no project printed %s", skill, verb, owner.errb)
					}
				}
			}
		}
		if len(refuse) > 0 && !strings.Contains(sentence, `"task not found"`) {
			t.Errorf("%s: a role with no Personal scope gets \"task not found\" from a folder in no project, and the sentence does not say so: %s", skill, sentence)
		}
	}
	var names []string
	for v := range seen {
		names = append(names, v)
	}
	sort.Strings(names)
	want := "answer,brain,deliver,job result,job show,person,plan,plan items,proof,question,question list,task history,where"
	if got := strings.Join(names, ","); got != want {
		t.Errorf("the three sentences name %s; want %s", got, want)
	}

	// A role with no Personal scope gets "task not found" in place of "needs --project".
	member, _, memberNone, _ := qjWorld(t, "member")
	for _, verb := range []string{"plan items", "question list", "where", "job show"} {
		member.run(memberNone, qjArgs(verb, "T-01")...)
		if needs, notFound := qjRefused(member); needs || !notFound {
			t.Errorf("member, `%s` from a folder in no project: needs --project=%v task-not-found=%v: %s", verb, needs, notFound, member.errb)
		}
	}
}

// qjQuoted are the double-quoted errors in the sentence of research step 6 that
// names `grid job result`.
func qjQuoted(t *testing.T) []string {
	t.Helper()
	text, _ := gridplugins.SkillFor("research")
	i := strings.Index(text, "`grid job result` needs")
	if i < 0 {
		t.Fatal("skills/research/SKILL.md has no sentence on what `grid job result` needs")
	}
	s := text[i:]
	if j := strings.Index(s, ". "); j >= 0 {
		s = s[:j]
	}
	var out []string
	for _, m := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestQjJobResultErrors(t *testing.T) {
	quoted := qjQuoted(t)
	h, path, _ := orchestratorHarness(t)
	h.run(path, "job", "result", "--task", path, "--body", "{}")
	noJob := h.errb.String()
	t.Setenv("GRID_MEMBER", "")
	h.run(path, "job", "result", "--task", path, "--job", "J-1", "--body", "{}")
	noMember := h.errb.String()
	printed := map[string]bool{
		"Job id is required":      strings.Contains(noJob, "Job id is required"),
		"GRID_MEMBER is required": strings.Contains(noMember, "GRID_MEMBER is required"),
	}
	if len(quoted) != 2 {
		t.Fatalf("research names %q as the errors of `grid job result`; want two", quoted)
	}
	for _, q := range quoted {
		ok, known := printed[q]
		if !known {
			t.Errorf("research quotes %q, which `job result` was not seen to print (no Job id gave %q; no GRID_MEMBER gave %q)", q, noJob, noMember)
		} else if !ok {
			t.Errorf("research quotes %q, but the run printed %q / %q", q, noJob, noMember)
		}
	}
}
