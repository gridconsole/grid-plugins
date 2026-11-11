package jobgraph

import (
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/job"
	"path"
	"regexp"
	"strings"
)

// CheckPlanJob is the static check every planned Job passes in a plan: its
// write patterns, the plain values of risk, work kind and visible, and its
// verify command. It reports the first refusal. The retired review field is
// not read.
func CheckPlanJob(item job.Job) error {
	for _, pattern := range item.Write {
		if err := validatePattern(pattern); err != nil {
			return err
		}
	}
	for _, risk := range item.Risk {
		switch risk {
		case "concurrency", "security", "data-format", "cross-platform":
		default:
			return fmt.Errorf("%s: unknown risk %q", item.ID, risk)
		}
	}
	switch item.Kind {
	case "", "build", "code", "non-code", "research", "scout", "rename", "generated":
	default:
		return fmt.Errorf("%s: unknown work kind %q", item.ID, item.Kind)
	}
	if item.Visible != "" && item.Visible != "big" && item.Visible != "small" {
		return fmt.Errorf("%s: unknown visible value %q", item.ID, item.Visible)
	}
	return ValidateVerify(item)
}

// ValidateVerify is a static plan check, not evidence of an executed test count.
// Non-code Jobs may use the contract's file-exists or reviewer-check kinds.
func ValidateVerify(item job.Job) error {
	v := item.Verify
	nonCode := item.Kind == "non-code" || item.Kind == "research" || item.Kind == "scout"
	if v.Kind != "" {
		if !nonCode {
			return fmt.Errorf("code Job verify must name a test command")
		}
		if v.Kind != "file-exists" && v.Kind != "reviewer-check" {
			return fmt.Errorf("unknown verify kind %q", v.Kind)
		}
		if strings.TrimSpace(v.Command) != "" || v.MinTests != 0 {
			return fmt.Errorf("verify must use either a kind or a command with min_tests")
		}
		return nil
	}
	if v.MinTests <= 0 {
		return fmt.Errorf("verify.min_tests must be greater than 0")
	}
	if !namesTest(v.Command) {
		return fmt.Errorf("verify.command must name a test")
	}
	return nil
}

var testWord = regexp.MustCompile(`(?i)(^|[./_:-])(tests?|pytest|unittest|jest|vitest|mocha|rspec|ctest|phpunit|runtests)([./_:-]|$)`)

// Recognize a named test runner, test target or test script. Shell punctuation
// separates invocations; comments and quoted descriptions do not count. This
// deliberately cannot infer what an arbitrary wrapper such as "make check"
// runs, nor prove that a named test will run successfully.
//
// The line is read as POSIX sh first. A command written for cmd.exe keeps its
// backslashes as plain characters, so a second reading without backslash
// escapes is made when the first found no test and no backslash in the line is
// a POSIX escape that changes how the line splits. The answer is the same on
// every host.
func namesTest(command string) bool {
	commands, ok := commandWords(command, true)
	if !ok {
		return false
	}
	if commandsNameTest(commands) {
		return true
	}
	if !backslashesArePlain(command) {
		return false
	}
	commands, ok = commandWords(command, false)
	return ok && commandsNameTest(commands)
}

// backslashesArePlain reports whether the line has a backslash and none of
// them stands where POSIX sh would take it as an escape that changes the split.
func backslashesArePlain(command string) bool {
	found := false
	runes := []rune(command)
	for i, r := range runes {
		if r != '\\' {
			continue
		}
		found = true
		if i+1 == len(runes) || strings.ContainsRune(" \t\n;&|'\"#$`(){}\\", runes[i+1]) {
			return false
		}
	}
	return found
}

// programName is the base name of a command word, split at the last "/" or
// "\". One trailing .exe, .cmd, .bat or .com is dropped and, when one was, the
// name is lower-cased: Windows program names are case-blind.
func programName(word string) string {
	exe := path.Base(strings.ReplaceAll(word, `\`, "/"))
	lower := strings.ToLower(exe)
	for _, ext := range []string{".exe", ".cmd", ".bat", ".com"} {
		if len(lower) > len(ext) && strings.HasSuffix(lower, ext) {
			return strings.TrimSuffix(lower, ext)
		}
	}
	return exe
}

func commandsNameTest(commands [][]string) bool {
	for _, words := range commands {
		if len(words) > 0 && words[0] == "env" {
			words = words[1:]
		}
		for len(words) > 0 && strings.Contains(words[0], "=") {
			words = words[1:]
		}
		if len(words) == 0 {
			continue
		}
		exe := programName(words[0])
		switch exe {
		case "echo", "printf", "true", "false", "test", "[":
			continue
		}
		if testWord.MatchString(exe) {
			return true
		}
		switch exe {
		case "go", "cargo", "dotnet":
			if len(words) > 1 && words[1] == "test" {
				return true
			}
		case "npm", "pnpm", "yarn", "bun":
			if len(words) > 1 && testWord.MatchString(words[1]) {
				return true
			}
			if len(words) > 2 && (words[1] == "run" || words[1] == "exec") && testWord.MatchString(words[2]) {
				return true
			}
		case "make", "gmake", "node", "python", "python3", "ruby", "npx":
			inline := false
			for _, word := range words[1:] {
				if word == "-e" || word == "-c" || word == "--eval" {
					inline = true
				}
			}
			if inline {
				continue
			}
			for _, word := range words[1:] {
				if strings.ContainsAny(word, "= \t") {
					continue
				}
				if testWord.MatchString(word) {
					return true
				}
			}
		}
	}
	return false
}

// commandWords preserves quoted arguments and ignores shell comments. Dynamic
// expansions and compound shell syntax are outside this static check: callers
// should name a test script instead. No command is executed here. With
// escapes false a backslash is an ordinary character, as cmd.exe reads it.
func commandWords(command string, escapes bool) ([][]string, bool) {
	commands := [][]string{}
	words := []string{}
	var word strings.Builder
	var quote rune
	escape, comment, started := false, false, false
	flushWord := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	flushCommand := func() {
		flushWord()
		if len(words) > 0 {
			commands = append(commands, words)
			words = nil
		}
	}
	for _, r := range command {
		if comment {
			if r == '\n' {
				comment = false
				flushCommand()
			}
			continue
		}
		if escape {
			word.WriteRune(r)
			started = true
			escape = false
			continue
		}
		if escapes && r == '\\' && quote != '\'' {
			escape = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				if quote == '"' && (r == '$' || r == '`') {
					return nil, false
				}
				word.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			started = true
		case '$', '`', '(', ')', '{', '}':
			return nil, false
		case '#':
			if !started {
				comment = true
			} else {
				word.WriteRune(r)
			}
		case ' ', '\t', '\r':
			flushWord()
		case ';', '&', '|', '\n':
			flushCommand()
		default:
			word.WriteRune(r)
			started = true
		}
	}
	if quote != 0 || escape {
		return nil, false
	}
	flushCommand()
	return commands, true
}
