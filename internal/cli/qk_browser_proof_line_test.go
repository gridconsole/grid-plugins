package cli

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/proof"
)

var (
	qkInline  = regexp.MustCompile("`(grid proof add [^`]*)`")
	qkAnyHole = regexp.MustCompile(`<[^<>]*>`)
)

// qkProofLines cuts every `grid proof add` command from the skill text: inline in
// backticks, or a line of a code block.
func qkProofLines(text string) []string {
	var out []string
	inBlock := false
	for _, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(ln, "```") {
			inBlock = !inBlock
			continue
		}
		if inBlock && strings.HasPrefix(ln, "grid proof add ") {
			out = append(out, ln)
			continue
		}
		for _, m := range qkInline.FindAllStringSubmatch(ln, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// qkSplit splits a command on spaces and keeps a double-quoted run as one word.
func qkSplit(s string) []string {
	var words []string
	var cur strings.Builder
	quoted, has := false, false
	for _, r := range s {
		switch {
		case r == '"':
			quoted, has = !quoted, true
		case r == ' ' && !quoted:
			if has {
				words = append(words, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		words = append(words, cur.String())
	}
	return words
}

func TestQkBrowserProofLinesRun(t *testing.T) {
	text, ok := gridplugins.SkillFor("browser")
	if !ok {
		t.Fatal("no browser skill")
	}
	lines := qkProofLines(text)
	if len(lines) != 2 {
		t.Fatalf("want 2 proof add commands in the browser skill, got %d: %q", len(lines), lines)
	}
	h, id, _ := proofTask(t)
	file := filepath.Join(h.tmp, "shot.png")
	h.write(file, "png-bytes")
	fill := strings.NewReplacer("<item>", "1", "<file>", file, "<task>", id, "<agent>", "alex")
	for _, ln := range lines {
		cmd := qkAnyHole.ReplaceAllString(fill.Replace(ln), "words")
		args := qkSplit(cmd)
		if len(args) < 4 || strings.Join(args[:3], " ") != "grid proof add" {
			t.Fatalf("not a proof add command: %q", cmd)
		}
		if code := h.run(h.home, args[1:]...); code != 0 {
			t.Errorf("%s\nexit %d: %s", ln, code, strings.TrimSpace(h.errb.String()))
		}
	}
	if code := h.run(h.home, "proof", "index", "--task", id, "--json"); code != 0 {
		t.Fatalf("index exit %d: %s", code, h.errb)
	}
	var idx proof.Index
	if err := json.Unmarshal(h.out.Bytes(), &idx); err != nil || len(idx.Items) == 0 {
		t.Fatalf("index %q: %v", h.out.String(), err)
	}
	if !idx.Items[0].Proven || len(idx.Items[0].Evidence) != 2 {
		t.Errorf("item 1 %+v, want 2 proofs", idx.Items[0])
	}
}
