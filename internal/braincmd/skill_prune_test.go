package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// The Prune section of skills/brain-keeper/SKILL.md is text the keeper follows.
// These tests read it and run its example commands on a seeded brain.

// pruneSection returns the text from the "## Prune" heading to the next "## ".
func pruneSection(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "skills", "brain-keeper", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	start := strings.Index("\n"+text, "\n## Prune\n")
	if start < 0 {
		t.Fatal("skills/brain-keeper/SKILL.md has no `## Prune` heading")
	}
	rest := text[start:]
	if end := strings.Index(rest[len("## Prune\n"):], "\n## "); end >= 0 {
		rest = rest[:len("## Prune\n")+end]
	}
	return rest
}

// pruneExample returns the lines of the one fenced text block in the section.
func pruneExample(t *testing.T, section string) []string {
	t.Helper()
	var lines []string
	fences, in := 0, false
	for _, l := range strings.Split(section, "\n") {
		switch {
		case strings.HasPrefix(l, "```text"):
			fences++
			in = true
		case strings.HasPrefix(l, "```"):
			in = false
		case in && strings.TrimSpace(l) != "":
			lines = append(lines, l)
		}
	}
	if fences != 1 || len(lines) == 0 {
		t.Fatalf("want one fenced text block with commands, got %d fences, %d lines", fences, len(lines))
	}
	return lines
}

// words splits a command line the way a shell would for plain and double quotes.
func words(line string) []string {
	var out []string
	var cur strings.Builder
	quoted, has := false, false
	for _, r := range line {
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

func TestPruneSectionHeadingAndShape(t *testing.T) {
	section := pruneSection(t)
	example := pruneExample(t, section)
	inFence := false
	for _, l := range strings.Split(section, "\n") {
		if strings.HasPrefix(l, "```") {
			inFence = !inFence
			continue
		}
		if !inFence && (strings.HasPrefix(l, "grid ") || strings.HasPrefix(l, "$ ")) {
			t.Fatalf("a line outside the example starts a shell command: %q", l)
		}
	}
	for _, l := range example {
		if !strings.HasPrefix(l, "grid brain ") {
			t.Fatalf("example line is not a grid brain command: %q", l)
		}
	}
	// Retiring a note never removes or hand-edits it.
	for _, bad := range []string{"rm ", "grid brain delete", "sed ", "> knowledge"} {
		if strings.Contains(section, bad) {
			t.Fatalf("section holds %q", bad)
		}
	}
	for _, l := range example {
		if strings.Contains(l, "delete") || strings.Contains(l, " rm ") {
			t.Fatalf("example holds a delete: %q", l)
		}
	}
}

func TestPruneVerbsAndFlagsExistInTheContract(t *testing.T) {
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name  string `json:"name"`
				Flags []struct {
					Name string `json:"name"`
				} `json:"flags"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	flags := map[string]map[string]bool{}
	for _, c := range doc.Commands {
		if c.Name != "brain" {
			continue
		}
		for _, s := range c.Subcommands {
			flags[s.Name] = map[string]bool{}
			for _, f := range s.Flags {
				flags[s.Name][f.Name] = true
			}
		}
	}
	if len(flags) == 0 {
		t.Fatal("no brain command in contractdata.Commands")
	}
	section := pruneSection(t)
	verbRe := regexp.MustCompile("grid brain ([a-z]+)([^`\n]*)")
	flagRe := regexp.MustCompile(`(^| )(--[a-z-]+)`)
	seen := map[string]bool{}
	for _, m := range verbRe.FindAllStringSubmatch(section, -1) {
		verb := m[1]
		known, ok := flags[verb]
		if !ok {
			t.Fatalf("grid brain %s is not in the contract", verb)
		}
		seen[verb] = true
		// Drop quoted text so a "--" inside a reason is not read as a flag.
		rest := regexp.MustCompile(`"[^"]*"`).ReplaceAllString(m[2], "")
		for _, f := range flagRe.FindAllStringSubmatch(rest, -1) {
			if !known[f[2]] {
				t.Fatalf("grid brain %s has no flag %s in the contract", verb, f[2])
			}
		}
	}
	for _, v := range []string{"search", "supersede", "revert", "changes", "accept"} {
		if !seen[v] {
			t.Errorf("section never names grid brain %s", v)
		}
	}
}

// pruneBrain seeds a project brain with deploy-old and deploy-new, each with a
// first version, and returns the harness.
func pruneBrain(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.put(t, "projects/example/project.md", "---\nid: 01EXAMPLE\n---\n")
	for _, name := range []string{"deploy-old", "deploy-new"} {
		if code := h.run("propose", "--project", "01EXAMPLE", "--name", name, "--topic", "decisions",
			"--description", "Before changing the deploy pipeline.", "--summary", "How the deploy pipeline runs.",
			"--source", "job:T-01/J-01", "--body", "# "+name+"\nThe deploy pipeline runs on make deploy.\n"); code != 0 {
			t.Fatalf("propose %s: %d %s", name, code, &h.err)
		}
		if code := h.run("accept", name, "--project", "example"); code != 0 {
			t.Fatalf("accept %s: %d %s", name, code, &h.err)
		}
	}
	return h
}

func TestPruneExamplesRunOnASeededBrain(t *testing.T) {
	h := pruneBrain(t)
	example := pruneExample(t, pruneSection(t))
	sub := strings.NewReplacer("<project>", "example", "<old>", "deploy-old", "<new>", "deploy-new", "<topic>", "decisions", "<n>", "1")

	hits := func() []string {
		t.Helper()
		if code := h.run("search", "deploy pipeline", "--project", "example", "--json"); code != 0 {
			t.Fatalf("search: %d %s", code, &h.err)
		}
		var got struct{ Hits []struct{ ID string } }
		if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, x := range got.Hits {
			ids = append(ids, x.ID)
		}
		return ids
	}
	if ids := hits(); len(ids) != 2 {
		t.Fatalf("before the examples search lists %v, want both notes", ids)
	}

	ran := map[string]string{}
	revertReason := ""
	for _, line := range example {
		args := words(sub.Replace(line))
		if len(args) < 3 || args[0] != "grid" || args[1] != "brain" {
			t.Fatalf("example line: %q", line)
		}
		if code := h.run(args[2:]...); code != 0 {
			t.Fatalf("%q: exit %d, stderr %q", line, code, &h.err)
		}
		ran[args[2]] = h.out.String()
		if args[2] == "revert" {
			for i, a := range args {
				if a == "--reason" && i+1 < len(args) {
					revertReason = args[i+1]
				}
			}
		}
		if args[2] == "supersede" {
			oldFile, err := os.ReadFile(filepath.Join(h.env.GridHome, "projects", "example", "knowledge", "decisions", "deploy-old.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(oldFile), "superseded_by: brain:project/deploy-new") {
				t.Fatalf("deploy-old after supersede:\n%s", oldFile)
			}
			if ids := hits(); len(ids) != 1 || ids[0] != "brain:project/deploy-new" {
				t.Fatalf("after supersede search lists %v, want only deploy-new", ids)
			}
			// The new note holds a supersedes link with the reason as its why.
			if code := h.run("show", "brain:project/deploy-new", "--project", "example", "--json"); code != 0 {
				t.Fatalf("show: %d %s", code, &h.err)
			}
			var e struct {
				Note struct {
					Links []struct{ To, Rel, Why string }
				}
			}
			if err := json.Unmarshal(h.out.Bytes(), &e); err != nil {
				t.Fatal(err)
			}
			if len(e.Note.Links) != 1 || e.Note.Links[0].Rel != "supersedes" || e.Note.Links[0].To != "brain:project/deploy-old" || len(e.Note.Links[0].Why) < 8 {
				t.Fatalf("deploy-new links: %+v", e.Note.Links)
			}
		}
	}
	for _, v := range []string{"search", "supersede", "revert", "changes"} {
		if _, ok := ran[v]; !ok {
			t.Fatalf("the example never ran grid brain %s", v)
		}
	}
	// The example's own changes call lists the revert with the example's reason.
	if revertReason == "" || !strings.Contains(ran["changes"], "revert to v1: "+revertReason) {
		t.Fatalf("changes after the revert example:\n%s", ran["changes"])
	}
}
