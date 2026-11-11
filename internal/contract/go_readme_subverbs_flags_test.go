package contract

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// cmdsDoc is the part of contract/commands.json this test reads.
type cmdsDoc struct {
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

// srfKnown is the known list: README spans that start with `grid ` but are not
// a command line the walk can check. Each entry is matched by the span's text.
var srfKnown = []struct{ text, reason string }{
	{"grid approve: <job> cannot be answered yet: <reason>", "error line, `approve:` is the message prefix"},
	{"grid workflow show: invalid <key> in <workspace|project> workflow", "error line, `show:` is the message prefix"},
	{"grid workflow write: project \"<selector>\" is missing or ambiguous", "error line, `write:` is the message prefix"},
	{"grid workflow write: project is missing or ambiguous", "error line, `write:` is the message prefix"},
	{"grid brain <changes|revert>: caller role is not allowed to <read brain changes|revert brain entries>", "error line, `<changes|revert>:` is the message prefix"},
	{"grid usage threshold ...", "removed usage verb, stated gone on purpose"},
	{"grid usage budget ...", "removed usage verb, stated gone on purpose"},
	{"grid <version>", "placeholder in the version output line, not a command"},
	{"grid home", "doctor check label, not a verb"},
	{"grid home writable", "doctor check label, not a verb"},
	{"grid on PATH", "doctor check label, not a verb"},
	{"grid feed", "bare name in the exit-code sentence; named with the alternation below"},
	{"grid job post|ack|result|cancel", "alternation; each part is checked as a sub-verb"},
}

var (
	srfSpanRe = regexp.MustCompile("`([^`\n]+(?:\n[^`\n]+)*)`")
	srfWordRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	srfFlagRe = regexp.MustCompile(`--[a-z][a-z0-9-]*`)
)

type srfSpan struct {
	line int
	text string
}

// srfSpans returns the backticked spans that start with "grid ", outside
// fenced blocks, with the line each begins on and whitespace folded.
func srfSpans(readme string) []srfSpan {
	lines := strings.Split(readme, "\n")
	var kept []string
	fence := false
	for _, l := range lines {
		if strings.HasPrefix(l, "```") {
			fence = !fence
			kept = append(kept, "")
			continue
		}
		if fence {
			kept = append(kept, "")
			continue
		}
		kept = append(kept, l)
	}
	text := strings.Join(kept, "\n")
	var out []srfSpan
	for _, m := range srfSpanRe.FindAllStringSubmatchIndex(text, -1) {
		s := text[m[2]:m[3]]
		if !strings.HasPrefix(s, "grid ") {
			continue
		}
		out = append(out, srfSpan{
			line: strings.Count(text[:m[2]], "\n") + 1,
			text: strings.Join(strings.Fields(s), " "),
		})
	}
	return out
}

// srfCheck returns the problems with one span against the command table.
func srfCheck(cmds cmdsDoc, text string) []string {
	toks := strings.Fields(text)
	if len(toks) < 2 || !srfWordRe.MatchString(toks[1]) {
		return []string{"not a command line: `" + text + "`"}
	}
	verb := toks[1]
	ci := -1
	for i, c := range cmds.Commands {
		if c.Name == verb {
			ci = i
		}
	}
	if ci < 0 {
		return []string{"`grid " + verb + "` is not in commands.json"}
	}
	c := cmds.Commands[ci]
	named := map[string]int{}
	for i, s := range c.Subcommands {
		if s.Name != "" {
			named[s.Name] = i
		}
	}
	// Sub-verb candidates: an alternation `a|b|c` gives each part.
	var subs []string
	if len(toks) > 2 && srfWordRe.MatchString(strings.SplitN(toks[2], "|", 2)[0]) {
		subs = strings.Split(toks[2], "|")
	} else if len(toks) > 2 && strings.Contains(toks[2], "|") && !strings.ContainsAny(toks[2][:1], "<[-\"") {
		subs = strings.Split(toks[2], "|")
	}
	var probs []string
	flagSet := map[string]bool{}
	addFlags := func(i int) {
		for _, f := range c.Subcommands[i].Flags {
			flagSet[f.Name] = true
		}
	}
	if len(named) > 0 && len(subs) > 0 {
		for _, s := range subs {
			i, ok := named[s]
			if !ok {
				probs = append(probs, "`"+s+"` is not a sub-verb of `grid "+verb+"` in commands.json")
				continue
			}
			addFlags(i)
		}
	} else {
		for i, s := range c.Subcommands {
			if s.Name == "" || len(named) == 0 {
				addFlags(i)
			}
		}
	}
	if len(probs) > 0 {
		return probs
	}
	for _, f := range srfFlagRe.FindAllString(text, -1) {
		if !flagSet[f] {
			probs = append(probs, "flag "+f+" is not a flag of `grid "+verb+"` in commands.json")
		}
	}
	return probs
}

func srfLoad(t *testing.T) (cmdsDoc, string) {
	t.Helper()
	var cmds cmdsDoc
	b, err := os.ReadFile("../../contract/commands.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &cmds); err != nil {
		t.Fatal(err)
	}
	r, err := os.ReadFile("../../contract/README.md")
	if err != nil {
		t.Fatal(err)
	}
	return cmds, string(r)
}

func srfIsKnown(text string) bool {
	for _, k := range srfKnown {
		if text == k.text {
			return true
		}
	}
	return false
}

func TestGoReadmeSubverbsInCommandsJSON(t *testing.T) {
	cmds, readme := srfLoad(t)
	spans := srfSpans(readme)
	if len(spans) < 50 {
		t.Fatalf("found only %d `grid ...` spans; the scan is broken", len(spans))
	}

	t.Run("known list", func(t *testing.T) {
		if len(srfKnown) != 13 {
			t.Fatalf("known list has %d entries, want 13", len(srfKnown))
		}
		for _, k := range srfKnown {
			if k.reason == "" {
				t.Errorf("known entry %q has no reason", k.text)
			}
			found := false
			for _, s := range spans {
				if s.text == k.text {
					found = true
				}
			}
			if !found {
				t.Errorf("known entry %q matches no README span; remove or fix it", k.text)
			}
		}
	})

	t.Run("README", func(t *testing.T) {
		for _, s := range spans {
			if srfIsKnown(s.text) {
				// Alternations on the list are still checked part by part.
				if s.text == "grid job post|ack|result|cancel" {
					for _, p := range srfCheck(cmds, s.text) {
						t.Errorf("contract/README.md:%d: %s", s.line, p)
					}
				}
				continue
			}
			for _, p := range srfCheck(cmds, s.text) {
				t.Errorf("contract/README.md:%d: %s (in `%s`)", s.line, p, s.text)
			}
		}
	})

	// Edited copies of README lines: both defects must be reported with the line.
	t.Run("edited copy reports unknown sub-verb and flag", func(t *testing.T) {
		edited := "ok line\n" +
			"Run `grid workflow frobnicate --json` here.\n" +
			"Run `grid workflow show --no-such-flag` here.\n" +
			"Exit codes of `grid feed frobnicate` here.\n"
		var got []string
		for _, s := range srfSpans(edited) {
			if srfIsKnown(s.text) {
				continue
			}
			for _, p := range srfCheck(cmds, s.text) {
				got = append(got, s.text+" => "+p)
			}
		}
		sort.Strings(got)
		if len(got) != 3 ||
			!strings.Contains(got[0], "`frobnicate` is not a sub-verb of `grid feed`") ||
			!strings.Contains(got[1], "`frobnicate` is not a sub-verb of `grid workflow`") ||
			!strings.Contains(got[2], "flag --no-such-flag") {
			t.Fatalf("edited copy: got %q", got)
		}
		if l := srfSpans(edited)[1].line; l != 3 {
			t.Fatalf("flag line reported as %d, want 3", l)
		}
	})
}
