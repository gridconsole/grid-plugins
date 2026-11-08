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
	return fmt.Sprintf("%x", sum[:])[:6]
}

// Digest binds canonical plan content with full SHA-256, including line one
// and step metadata. Ticks, trailing whitespace, CRLF and status prose remain
// normalized so live workflow edits preserve approval.
func Digest(text string) string {
	sum := sha256.Sum256([]byte(canonicalPlan(text, true)))
	return fmt.Sprintf("%x", sum[:])
}

func canonical(text string) string { return canonicalPlan(text, false) }

func canonicalPlan(text string, approval bool) string {
	var kept []string
	for _, line := range canonicalLines(text, approval) {
		kept = append(kept, line.text)
	}
	return strings.Join(kept, "\n") + "\n"
}

type canonicalLine struct {
	number int
	text   string
}

// DigestCoversLines reports whether every line in a one-based inclusive range
// survives approval canonicalization. Normalized ticks and whitespace are safe;
// omitted status prose and trimmed boundary blank lines are not anchor sources.
func DigestCoversLines(text string, first, last int) bool {
	if first < 1 || last < first {
		return false
	}
	count := 0
	for _, line := range canonicalLines(text, true) {
		if line.number >= first && line.number <= last {
			count++
		}
	}
	return count == last-first+1
}

func canonicalLines(text string, approval bool) []canonicalLine {
	inStep, inStands := false, false
	var kept []canonicalLine
	for i, line := range lines(text) {
		if i == 0 && !approval {
			continue
		}
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
			} else if !approval && volatile.MatchString(line) {
				continue
			}
		} else {
			inStands = false
			if step {
				inStep = true
			} else if !indented.MatchString(line) {
				inStep = false
			} else if !approval && inStep && volatile.MatchString(line) {
				continue
			}
		}
		kept = append(kept, canonicalLine{i + 1, trailing.ReplaceAllString(hashStep.ReplaceAllString(line, "${1}[ ]"), "")})
	}
	for len(kept) > 0 && kept[0].text == "" {
		kept = kept[1:]
	}
	for len(kept) > 0 && kept[len(kept)-1].text == "" {
		kept = kept[:len(kept)-1]
	}
	return kept
}
