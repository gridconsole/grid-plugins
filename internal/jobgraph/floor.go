package jobgraph

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Policy is the relevant projection of project brain path tags. No package
// global or filesystem state supplies these lists. Built-in UI and low-risk
// conventions supplement the lists; a project can tag additional paths.
type Policy struct {
	EnvironmentPaths []string `json:"environment_paths,omitempty"`
	RiskPaths        []string `json:"risk_paths,omitempty"`
	UIPaths          []string `json:"ui_paths,omitempty"`
	GeneratedPaths   []string `json:"generated_paths,omitempty"`
	TaskPaths        []string `json:"task_paths,omitempty"`
}

type Floor struct {
	Job         string   `json:"job"`
	Review      string   `json:"review"`
	Floor       string   `json:"floor"`
	Reason      string   `json:"reason"`
	Classify    bool     `json:"classify"`
	CaptureAtL3 bool     `json:"capture_at_l3"`
	Override    bool     `json:"override"`
	Errors      []string `json:"errors"`
}

func uiScope(write []string) bool {
	for _, w := range write {
		if i := strings.IndexByte(w, ':'); i >= 0 {
			w = w[i+1:]
		}
		parts := strings.Split(w, "/")
		for _, part := range parts[:len(parts)-1] {
			if part == "pages" || part == "ui" || part == "components" {
				return true
			}
		}
		for _, suffix := range []string{".tsx", ".jsx", ".vue", ".svelte"} {
			if strings.HasSuffix(w, suffix) {
				return true
			}
		}
	}
	return false
}

// ReviewFloor computes first-match precedence from job-contract.md section 5.
// It does not lower or mutate the requested review level. An exception reason
// permits a lower request but does not remove classify or capture requirements.
func ReviewFloor(item Item, policy Policy) (Floor, error) {
	out := Floor{Job: item.ID, Review: item.Review, Errors: []string{}}
	for _, patterns := range [][]string{item.Write, policy.EnvironmentPaths, policy.RiskPaths, policy.UIPaths, policy.GeneratedPaths, policy.TaskPaths} {
		for _, pattern := range patterns {
			if err := validatePattern(pattern); err != nil {
				return out, err
			}
		}
	}
	for _, risk := range item.Risk {
		switch risk {
		case "concurrency", "security", "data-format", "cross-platform":
		default:
			return out, fmt.Errorf("%s: unknown risk %q", item.ID, risk)
		}
	}
	switch item.Kind {
	case "", "code", "non-code", "research", "scout", "rename", "generated":
	default:
		return out, fmt.Errorf("%s: unknown work kind %q", item.ID, item.Kind)
	}
	if item.Visible != "" && item.Visible != "big" {
		return out, fmt.Errorf("%s: unknown visible value %q", item.ID, item.Visible)
	}
	lowRisk := len(item.Write) > 0
	taskOnly := len(item.Write) > 0
	for _, w := range item.Write {
		lowRisk = lowRisk && (lowRiskPath(w) || covered(w, policy.GeneratedPaths))
		taskOnly = taskOnly && covered(w, policy.TaskPaths)
	}
	switch {
	case item.Irreversible || anyScope(item.Write, policy.EnvironmentPaths):
		out.Floor, out.Reason, out.Classify = "L2", "environment scope or irreversible step", true
	case len(item.Risk) > 0 || anyScope(item.Write, policy.RiskPaths):
		out.Floor, out.Reason = "L2", "declared risk or risk path"
	case item.Visible == "big" || uiScope(item.Write) || anyScope(item.Write, policy.UIPaths):
		out.Floor, out.Reason, out.CaptureAtL3 = "L1", "visible change or UI scope", true
	case lowRisk || item.Kind == "rename" || item.Kind == "generated" || (item.Kind == "research" || item.Kind == "scout") && taskOnly:
		out.Floor, out.Reason = "L0", "tests, docs, renames, generated files, or task-only research"
	default:
		out.Floor, out.Reason = "L1", "ordinary work"
	}
	level := reviewRank(item.Review)
	if level < 0 {
		out.Errors = append(out.Errors, "review must be L0, L1, L2 or L3")
	} else if level < reviewRank(out.Floor) {
		if strings.TrimSpace(item.ReviewReason) == "" {
			out.Errors = append(out.Errors, "review "+item.Review+" is below floor "+out.Floor)
		} else {
			out.Override = true
		}
	}
	if err := ValidateVerify(item); err != nil {
		out.Errors = append(out.Errors, err.Error())
	}
	return out, nil
}

func reviewRank(level string) int {
	switch level {
	case "L0":
		return 0
	case "L1":
		return 1
	case "L2":
		return 2
	case "L3":
		return 3
	default:
		return -1
	}
}

// Floors returns a stable report, including per-Job review and verify refusals.
// Malformed ids, kinds, risks or patterns are errors instead of partial reports.
func Floors(items []Item, policy Policy) ([]Floor, error) {
	if _, err := New(items); err != nil {
		return nil, err
	}
	out := make([]Floor, 0, len(items))
	for _, item := range items {
		floor, err := ReviewFloor(item, policy)
		if err != nil {
			return nil, err
		}
		out = append(out, floor)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Job < out[j].Job })
	return out, nil
}

// ValidateVerify is a static plan check, not evidence of an executed test count.
// Non-code Jobs may use the contract's file-exists or reviewer-check kinds.
func ValidateVerify(item Item) error {
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
func namesTest(command string) bool {
	commands, ok := commandWords(command)
	if !ok {
		return false
	}
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
		exe := path.Base(words[0])
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
// should name a test script instead. No command is executed here.
func commandWords(command string) ([][]string, bool) {
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
		if r == '\\' && quote != '\'' {
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
