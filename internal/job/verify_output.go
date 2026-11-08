package job

import (
	"bytes"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const verifyOutputLimit = 64 * 1024
const verifyLineLimit = 1024 * 1024

var (
	goTestLine    = regexp.MustCompile(`^--- (?:PASS|FAIL|SKIP): [^ /]+(?: |$)`)
	goPackageLine = regexp.MustCompile(`^(?:ok\s+|\?\s+|FAIL\s+)\S+`)
	nodeTests     = regexp.MustCompile(`^(?:#|ℹ) tests ([0-9]+)\s*$`)
	cargoTests    = regexp.MustCompile(`^test result: (?:ok|FAILED)\. ([0-9]+) passed; ([0-9]+) failed; ([0-9]+) ignored;`)
	pytestSummary = regexp.MustCompile(`^(?:=+\s*)?(?:[0-9]+ (?:passed|failed|skipped|error|errors|xfailed|xpassed|deselected)(?:, )?)+ in [0-9.]+s(?: .*)?$`)
	pytestCount   = regexp.MustCompile(`([0-9]+) (passed|failed|skipped|errors?|xfailed|xpassed|deselected)`)
	ansiEscape    = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")
)

// Parse all lines while retaining only bounded diagnostic output. An oversized
// line makes totals unknown rather than accepting a partial count.
type verifyOutput struct {
	output                       bytes.Buffer
	pending                      []byte
	truncated, overflow, discard bool
	known                        bool
	total                        int
	goCount                      int
	goSeen                       bool
	goPackageCount               int
}

func (w *verifyOutput) Write(p []byte) (int, error) {
	n := len(p)
	keep := min(n, verifyOutputLimit-w.output.Len())
	w.output.Write(p[:keep])
	w.truncated = w.truncated || keep < n
	for _, b := range p {
		if b == '\n' {
			if !w.discard {
				w.line(string(w.pending))
			}
			w.pending, w.discard = w.pending[:0], false
		} else if !w.discard {
			if len(w.pending) == verifyLineLimit {
				w.overflow, w.discard = true, true
				w.pending = w.pending[:0]
			} else {
				w.pending = append(w.pending, b)
			}
		}
	}
	return n, nil
}
func (w *verifyOutput) finish() {
	if len(w.pending) > 0 && !w.discard {
		w.line(string(w.pending))
	}
	w.pending = nil
}
func (w *verifyOutput) tests() *int {
	if w.overflow || (!w.known && !w.goSeen) || w.goCount > math.MaxInt-w.total {
		return nil
	}
	n := w.total + w.goCount
	return &n
}
func (w *verifyOutput) addCount(s string) {
	n, err := strconv.Atoi(s)
	if err != nil || n > math.MaxInt-w.total {
		w.overflow = true
		return
	}
	w.total += n
}
func (w *verifyOutput) line(line string) {
	line = strings.TrimRight(ansiEscape.ReplaceAllString(line, ""), "\r ")
	if strings.HasPrefix(line, "{") {
		var event struct{ Action, Test, Package string }
		if json.Unmarshal([]byte(line), &event) == nil && event.Action != "" {
			if event.Test != "" && !strings.Contains(event.Test, "/") && (event.Action == "pass" || event.Action == "fail" || event.Action == "skip") {
				w.goSeen = true
				w.goCount++
			} else if event.Test == "" && event.Package != "" && (event.Action == "pass" || event.Action == "fail" || event.Action == "skip") {
				w.goSeen = true
			}
			return // Output events repeat the same test; never count them twice.
		}
	}
	if goTestLine.MatchString(line) {
		w.goSeen = true
		w.goCount++
		w.goPackageCount++
		return
	}
	if goPackageLine.MatchString(line) {
		if strings.Contains(line, "[no test files]") || strings.Contains(line, "[no tests to run]") {
			w.goSeen = true
		} else if w.goPackageCount == 0 {
			w.overflow = true
		} // plain go test has no totals
		w.goPackageCount = 0
		return
	}
	if m := nodeTests.FindStringSubmatch(line); m != nil {
		w.known = true
		w.addCount(m[1])
		return
	}
	if m := cargoTests.FindStringSubmatch(line); m != nil {
		w.known = true
		w.addCount(m[1])
		w.addCount(m[2])
		w.addCount(m[3])
		return
	}
	if pytestSummary.MatchString(line) {
		w.known = true
		for _, m := range pytestCount.FindAllStringSubmatch(line, -1) {
			if m[2] != "deselected" {
				w.addCount(m[1])
			}
		}
	} else if regexpNoTests.MatchString(line) {
		w.known = true
	}
}

var regexpNoTests = regexp.MustCompile(`^(?:=+\s*)?no tests ran in [0-9.]+s(?: .*)?$`)
