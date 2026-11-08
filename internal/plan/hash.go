// Package plan reads plan items and installs numbered plan revisions.
package plan

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

// jsSpace matches ECMAScript's \s, deliberately not Go's ASCII-only \s or
// unicode.IsSpace (which also includes U+0085 and excludes U+FEFF).
const jsSpace = "[\\t\\n\\v\\f\\r \\x{00a0}\\x{1680}\\x{2000}-\\x{200a}\\x{2028}\\x{2029}\\x{202f}\\x{205f}\\x{3000}\\x{feff}]"

var (
	hashStep = regexp.MustCompile("^(" + jsSpace + "*- )\\[[ xX]\\]")
	stands   = regexp.MustCompile("^## Where it stands" + jsSpace + "*$")
	indented = regexp.MustCompile("^" + jsSpace + "+[^\\t\\n\\v\\f\\r \\x{00a0}\\x{1680}\\x{2000}-\\x{200a}\\x{2028}\\x{2029}\\x{202f}\\x{205f}\\x{3000}\\x{feff}]")
	volatile = regexp.MustCompile("^" + jsSpace + "+(?:evidence|needs):")
	trailing = regexp.MustCompile(jsSpace + "+$")
)

func lines(text string) []string { return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") }

// Hash is byte-compatible with plan-hash.mjs: six hex digits of SHA-256.
// The first line is always omitted, even on a plan without a revision heading.
// Internal blank lines, leading indentation and spaces within text still count.
func Hash(text string) string {
	sum := sha256.Sum256([]byte(canonical(text)))
	return fmt.Sprintf("%x", sum[:3])
}

func canonical(text string) string {
	inStep, inStands := false, false
	var kept []string
	for _, line := range lines(text)[1:] {
		if stands.MatchString(line) {
			inStands, inStep = true, false
			continue
		}
		step := hashStep.MatchString(line)
		if inStands && !strings.HasPrefix(line, "## ") {
			if step && strings.HasPrefix(line, "- ") {
				inStep = true
			} else if !(inStep && indented.MatchString(line)) {
				inStep = false
				continue
			} else if volatile.MatchString(line) {
				continue
			}
		} else {
			inStands = false
			if step {
				inStep = true
			} else if !indented.MatchString(line) {
				inStep = false
			} else if inStep && volatile.MatchString(line) {
				continue
			}
		}
		kept = append(kept, trailing.ReplaceAllString(hashStep.ReplaceAllString(line, "${1}[ ]"), ""))
	}
	return strings.Trim(strings.Join(kept, "\n"), "\n") + "\n"
}
