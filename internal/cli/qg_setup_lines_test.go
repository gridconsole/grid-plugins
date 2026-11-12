package cli

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

var (
	qgSpan  = regexp.MustCompile("`([^`]+)`")
	qgTopic = regexp.MustCompile("^[a-z][a-z0-9-]*$")
)

// qgSplit splits a command line on spaces, keeping a double-quoted run as one word.
func qgSplit(s string) []string {
	var out []string
	var cur strings.Builder
	quoted, has := false, false
	for _, r := range s {
		switch {
		case r == '"':
			quoted, has = !quoted, true
		case r == ' ' && !quoted:
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		out = append(out, cur.String())
	}
	return out
}

// qgSetupLine returns the first line of the setup skill that holds all of the words.
func qgSetupLine(t *testing.T, text string, words ...string) string {
	t.Helper()
	for _, l := range strings.Split(text, "\n") {
		ok := true
		for _, w := range words {
			ok = ok && strings.Contains(l, w)
		}
		if ok {
			return l
		}
	}
	t.Fatalf("setup skill has no line with %q", words)
	return ""
}

// TestQgSetupLinesWalk runs what the setup skill tells Claude to run: each
// `grid project scan` command of the skill against a folder holding one git repo,
// and the propose line once for each topic it names, then brain check.
func TestQgSetupLinesWalk(t *testing.T) {
	text, ok := gridplugins.SkillFor("setup")
	if !ok {
		t.Fatal("no setup skill")
	}
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "machine"), "01ARZ3NDEKTSV4RRFFQ69G5FAV\n")

	discovery := filepath.Join(h.root, "work")
	repo := filepath.Join(discovery, "suite")
	h.mkdir(repo)
	testgit.Init(t, repo, "git@example.com:org/app.git", false)

	scans := 0
	for _, span := range qgSpan.FindAllStringSubmatch(text, -1) {
		if !strings.HasPrefix(span[1], "grid project scan") {
			continue
		}
		scans++
		args := qgSplit(strings.TrimPrefix(span[1], "grid "))
		for i, a := range args {
			if a == "<folder>" {
				args[i] = discovery
			}
		}
		if code := h.run(h.root, args...); code != 0 {
			t.Errorf("`%s` exit %d: %s", span[1], code, h.errb)
		}
	}
	if scans == 0 {
		t.Fatal("the setup skill has no `grid project scan` command")
	}

	h.record(project.Record{Name: "example"})
	line := qgSetupLine(t, text, "grid brain propose", "Topics:")
	var template []string
	for _, span := range qgSpan.FindAllStringSubmatch(line, -1) {
		if strings.HasPrefix(span[1], "grid brain propose") {
			template = qgSplit(strings.TrimPrefix(span[1], "grid "))
			break
		}
	}
	if len(template) == 0 {
		t.Fatalf("no propose command in %q", line)
	}
	// The topics are the words in code spans between "Topics:" and the next "(" or ".".
	seg := line[strings.Index(line, "Topics:")+len("Topics:"):]
	if i := strings.IndexAny(seg, "(."); i >= 0 {
		seg = seg[:i]
	}
	var topics []string
	for _, span := range qgSpan.FindAllStringSubmatch(seg, -1) {
		if qgTopic.MatchString(span[1]) {
			topics = append(topics, span[1])
		}
	}
	if len(topics) == 0 {
		t.Fatalf("no topic named in %q", line)
	}
	for _, topic := range topics {
		args := append([]string{}, template...)
		for i := 1; i < len(args); i++ {
			if !strings.HasPrefix(args[i-1], "--") {
				continue
			}
			switch flag := args[i-1]; {
			case flag == "--name":
				args[i] = "qg-" + topic
			case flag == "--topic":
				args[i] = topic
			case flag == "--project":
				args[i] = "example"
			case strings.HasPrefix(args[i], "<"):
				args[i] = "README.md text for " + topic
			}
		}
		if code := h.run(h.root, args...); code != 0 {
			t.Errorf("topic %s: `grid %s` exit %d: %s", topic, strings.Join(args, " "), code, h.errb)
		}
	}
	// A proposal that check flags as config_topic exits 3 and prints its code.
	code := h.run(h.root, "brain", "check", "--project", "example", "--json")
	if code != 0 || strings.Contains(h.out.String(), "config_topic") {
		t.Errorf("brain check exit %d: %s %s", code, h.out, h.errb)
	}
}
