package jobgraph

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// Scope patterns use slash-separated paths on every OS, with *, ?, character
// classes and ** as an entire segment. Repo prefixes are part of the pattern;
// an unprefixed policy applies only to the primary repo's unprefixed paths.
func validatePattern(pattern string) error {
	repo, local := splitScope(pattern)
	if strings.Contains(pattern, ":") && (repo == "" || strings.ContainsAny(repo, "/*?[]\\") || strings.Contains(local, ":")) {
		return fmt.Errorf("invalid scope repo prefix %q", pattern)
	}
	if local == "" || strings.HasPrefix(local, "/") || strings.Contains(local, `\`) {
		return fmt.Errorf("invalid scope pattern %q: use relative slash-separated paths", pattern)
	}
	for _, part := range strings.Split(local, "/") {
		if part == "" || part == "." || part == ".." || strings.Contains(part, "**") && part != "**" {
			return fmt.Errorf("invalid scope pattern %q", pattern)
		}
		if _, err := path.Match(part, ""); err != nil {
			return fmt.Errorf("invalid scope pattern %q: %w", pattern, err)
		}
	}
	return nil
}

func splitScope(pattern string) (string, string) {
	if repo, local, ok := strings.Cut(pattern, ":"); ok {
		return repo, local
	}
	return "", pattern
}

type span struct{ lo, hi rune }
type globToken struct {
	star  bool
	spans []span
}

// Segment automata need only the character boundaries present in the patterns.
// Intersecting their transitions avoids guessing a sample filename for a glob.
func segmentTokens(pattern string) []globToken {
	runes := []rune(pattern)
	tokens := []globToken{}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '*', '?':
			tokens = append(tokens, globToken{star: r == '*', spans: []span{{0, utf8.MaxRune}}})
		case '[':
			i++
			negate := runes[i] == '^'
			if negate {
				i++
			}
			ranges := []span{}
			for runes[i] != ']' {
				lo, hi := runes[i], runes[i]
				if i+1 < len(runes) && runes[i+1] == '-' {
					hi = runes[i+2]
					i += 2
				}
				ranges = append(ranges, span{lo, hi})
				i++
			}
			if negate {
				// Complement by splitting a small list of disjoint intervals.
				allowed := []span{{0, utf8.MaxRune}}
				for _, deny := range ranges {
					next := []span{}
					for _, a := range allowed {
						if deny.hi < a.lo || deny.lo > a.hi {
							next = append(next, a)
							continue
						}
						if a.lo < deny.lo {
							next = append(next, span{a.lo, deny.lo - 1})
						}
						if a.hi > deny.hi {
							next = append(next, span{deny.hi + 1, a.hi})
						}
					}
					allowed = next
				}
				ranges = allowed
			}
			tokens = append(tokens, globToken{spans: ranges})
		default:
			tokens = append(tokens, globToken{spans: []span{{r, r}}})
		}
	}
	return tokens
}

func segmentOverlap(a, b string) bool {
	x, y := segmentTokens(a), segmentTokens(b)
	type state struct {
		i, j int
		used bool
	}
	queue := []state{{}}
	seen := map[state]bool{}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if seen[s] {
			continue
		}
		seen[s] = true
		if s.i == len(x) && s.j == len(y) {
			if s.used {
				return true
			}
			continue
		}
		if s.i < len(x) && x[s.i].star {
			queue = append(queue, state{s.i + 1, s.j, s.used})
		}
		if s.j < len(y) && y[s.j].star {
			queue = append(queue, state{s.i, s.j + 1, s.used})
		}
		if s.i == len(x) || s.j == len(y) {
			continue
		}
		common := false
		for _, p := range x[s.i].spans {
			for _, q := range y[s.j].spans {
				if max(p.lo, q.lo) <= min(p.hi, q.hi) {
					common = true
				}
			}
		}
		if common {
			i, j := s.i, s.j
			if !x[i].star {
				i++
			}
			if !y[j].star {
				j++
			}
			queue = append(queue, state{i, j, true})
		}
	}
	return false
}

func scopeOverlap(a, b string) bool {
	repoA, localA := splitScope(a)
	repoB, localB := splitScope(b)
	if repoA != repoB {
		return false
	}
	a, b = localA, localB
	x, y := strings.Split(a, "/"), strings.Split(b, "/")
	type state struct{ i, j int }
	queue := []state{{}}
	seen := map[state]bool{}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if seen[s] {
			continue
		}
		seen[s] = true
		if s.i == len(x) && s.j == len(y) {
			return true
		}
		if s.i < len(x) && x[s.i] == "**" {
			queue = append(queue, state{s.i + 1, s.j})
		}
		if s.j < len(y) && y[s.j] == "**" {
			queue = append(queue, state{s.i, s.j + 1})
		}
		if s.i == len(x) || s.j == len(y) {
			continue
		}
		if x[s.i] == "**" || y[s.j] == "**" || segmentOverlap(x[s.i], y[s.j]) {
			i, j := s.i, s.j
			if x[i] != "**" {
				i++
			}
			if y[j] != "**" {
				j++
			}
			queue = append(queue, state{i, j})
		}
	}
	return false
}

func anyScope(write, tags []string) bool {
	for _, w := range write {
		for _, tag := range tags {
			if scopeOverlap(w, tag) {
				return true
			}
		}
	}
	return false
}

// covered is deliberately conservative for lowering the floor: equality,
// literal paths, or a literal directory subtree prove containment. Other glob
// intersections can raise a floor, but cannot establish low-risk coverage.
func covered(write string, tags []string) bool {
	for _, tag := range tags {
		if write == tag {
			return true
		}
		if !strings.ContainsAny(write, "*?[") && scopeOverlap(write, tag) {
			return true
		}
		if strings.HasSuffix(tag, "/**") {
			prefix := strings.TrimSuffix(tag, "**")
			if !strings.ContainsAny(prefix, "*?[") && strings.HasPrefix(write, prefix) {
				return true
			}
		}
	}
	return false
}

func lowRiskPath(write string) bool {
	// A fixed directory or filename suffix must constrain the entire write glob.
	local := write
	if i := strings.IndexByte(local, ':'); i >= 0 {
		local = local[i+1:]
	}
	parts := strings.Split(local, "/")
	for _, part := range parts[:len(parts)-1] {
		if part == "docs" || part == "tests" || part == "test" || part == "__tests__" {
			return true
		}
		if strings.ContainsAny(part, "*?[") {
			break
		}
	}
	name := parts[len(parts)-1]
	for _, suffix := range []string{".md", ".rst", "_test.go", ".test.js", ".test.mjs", ".test.ts", ".test.tsx", ".spec.js", ".spec.ts", ".spec.tsx"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py") || strings.HasSuffix(name, "_test.py")
}
