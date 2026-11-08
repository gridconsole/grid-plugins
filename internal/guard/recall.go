package guard

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

var planHash = regexp.MustCompile(`plan-hash\.mjs['"]?\s+(?:'([^']+)'|"([^"]+)"|(\S+))`)
var shellReady = regexp.MustCompile(`(?:'([^']*history\.md)'|"([^"]*history\.md)"|((?:[^\s'"<>|;&]*/)?history\.md))`)
var printFlag = regexp.MustCompile(`(?:^|\s)--print(?:\s|$)`)
var redirectToFile = regexp.MustCompile(`>[>|]?\s*(?:'[^']*history\.md'|"[^"]*history\.md"|[^\s'"<>|;&]*history\.md)`)
var otherWrite = regexp.MustCompile(`\b(?:sed|perl)\b[^|;&]*\s-[a-zA-Z]*i|\bpython3?\b[\s\S]*?\s-c\b[\s\S]*?(?:\bopen\([\s\S]*['"][wax]\+?b?['"]|\.write_text\()`)
var teeWord = regexp.MustCompile(`\btee\b`)
var teeArgs = regexp.MustCompile(`\btee\b([^|;&\n<>]*)`)
var indirectWrite = regexp.MustCompile(`\bxargs\b|\b(?:cp|mv|dd|install|ln)\b|\bof=`)
var singleQuoted = regexp.MustCompile(`'[^']*'`)
var literalPath = regexp.MustCompile(`^[A-Za-z0-9_./~+@%,:=-]+$`)

const recallMissing = "plan.md has no brain recall line. Add it under ## How we approach it: one `Used: <path>` line for each note of the recall you use, or `Recall: nothing found` when no note bears on the task (the recall hits are in your context, or run `grid-brain recall --task <id>`)."

func capture(m []string) string {
	for _, s := range m[1:] {
		if s != "" {
			return s
		}
	}
	return ""
}
func (c context) recallHeld(file string) bool {
	t := taskAt(c.path(file), realNative(c.projects, 0))
	if t == nil {
		return false
	}
	text := read(filepath.Join(t.dir, "task.md"))
	return !inactive(text) && heldStage(planStage(text, read(filepath.Join(t.dir, "history.md")))) && !recallLine.MatchString(read(filepath.Join(t.dir, "plan.md")))
}
func (c context) recallGate() hookio.Decision {
	command := str(c.in.ToolInput["command"])
	if !printFlag.MatchString(command) {
		for _, m := range planHash.FindAllStringSubmatch(command, -1) {
			file := capture(m)
			if filepath.Base(file) == "plan.md" && c.recallHeld(file) {
				return hookio.Deny("plan-hash.mjs is not run on " + file + ": " + recallMissing + " Then run it again." + off)
			}
		}
	}
	if !strings.Contains(command, "plan ready") {
		return hookio.Decision{}
	}
	files := []string{}
	for _, text := range []string{command, strings.NewReplacer("\"", "", "'", "").Replace(command)} {
		for _, m := range shellReady.FindAllStringSubmatch(text, -1) {
			files = append(files, capture(m))
		}
	}
	for _, m := range teeArgs.FindAllStringSubmatch(command, -1) {
		for _, w := range strings.Fields(m[1]) {
			if strings.ContainsAny(w, "*?[") {
				// Match the same basename glob the old script uses; no filesystem expansion.
				pattern := filepath.Base(w)
				var b strings.Builder
				b.WriteByte('^')
				for _, r := range pattern {
					switch r {
					case '*':
						b.WriteString(".*")
					case '?':
						b.WriteByte('.')
					case '[', ']':
						b.WriteRune(r)
					default:
						b.WriteString(regexp.QuoteMeta(string(r)))
					}
				}
				b.WriteByte('$')
				re, err := regexp.Compile(b.String())
				if err == nil && re.MatchString("history.md") {
					files = append(files, filepath.Join(filepath.Dir(w), "history.md"))
				}
			}
		}
	}
	for _, file := range files {
		if c.writesTo(command, c.path(file)) && c.recallHeld(file) {
			return hookio.Deny("This command is not run: it adds a plan ready line to " + file + ", and " + recallMissing + off)
		}
	}
	return hookio.Decision{}
}
func (c context) writesTo(command, file string) bool {
	if redirectToFile.MatchString(command) || otherWrite.MatchString(command) {
		return true
	}
	if !teeWord.MatchString(command) {
		return false
	}
	if indirectWrite.MatchString(command) || strings.Contains(singleQuoted.ReplaceAllString(command, ""), "$") {
		return true
	}
	for _, m := range teeArgs.FindAllStringSubmatch(command, -1) {
		for _, w := range strings.Fields(m[1]) {
			if strings.HasPrefix(w, "-") {
				continue
			}
			if !literalPath.MatchString(w) || c.path(w) == file {
				return true
			}
		}
	}
	return false
}
