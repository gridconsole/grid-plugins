package golden

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type entry struct {
	kind, data string
	mode       fs.FileMode
}
type snapshot map[string]entry

func takeSnapshot(root string) (snapshot, error) {
	result := snapshot{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var data []byte
		kind := "file"
		if info.Mode()&os.ModeSymlink != 0 {
			kind = "link"
			target, e := os.Readlink(path)
			err = e
			data = []byte(target)
		} else if info.Mode().IsRegular() {
			data, err = os.ReadFile(path)
		} else {
			return fmt.Errorf("unsupported fixture file: %s", path)
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = entry{kind, string(data), info.Mode().Perm()}
		return nil
	})
	return result, err
}

func folderDiff(before, after snapshot, n *normalizer) string {
	names := map[string]bool{}
	for name := range before {
		names[name] = true
	}
	for name := range after {
		names[name] = true
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	var out strings.Builder
	for _, name := range ordered {
		old, had := before[name]
		now, has := after[name]
		if had == has && old == now {
			continue
		}
		fmt.Fprintf(&out, "diff --grid a/%s b/%s\n", name, name)
		from, to := "a/"+name, "b/"+name
		switch {
		case !had:
			fmt.Fprintf(&out, "new %s mode %04o\n", now.kind, now.mode)
			from = "/dev/null"
		case !has:
			fmt.Fprintf(&out, "deleted %s mode %04o\n", old.kind, old.mode)
			to = "/dev/null"
		case old.kind != now.kind || old.mode != now.mode:
			fmt.Fprintf(&out, "mode/type %s %04o -> %s %04o\n", old.kind, old.mode, now.kind, now.mode)
		}
		a, b := n.text(old.data), n.text(now.data)
		out.WriteString(unifiedDiff(a, b, from, to))
		if had && has && old.data != now.data && a == b {
			out.WriteString("Only normalized values changed.\n")
		}
	}
	return out.String()
}

type block struct{ a, b, n int }
type opcode struct {
	tag        byte
	a, z, b, y int
}

// matchingBlocks implements difflib.SequenceMatcher, including its popular-line
// heuristic for inputs of 200 lines or more and earliest-match tie breaking.
func matchingBlocks(a, b []string) []block {
	positions := map[string][]int{}
	for j, line := range b {
		positions[line] = append(positions[line], j)
	}
	if len(b) >= 200 {
		for line, js := range positions {
			if len(js) > len(b)/100+1 {
				delete(positions, line)
			}
		}
	}
	var blocks []block
	var visit func(int, int, int, int)
	visit = func(lo, hi, lb, hb int) {
		best := block{a: lo, b: lb}
		prev := map[int]int{}
		for i := lo; i < hi; i++ {
			next := map[int]int{}
			for _, j := range positions[a[i]] {
				if j < lb {
					continue
				}
				if j >= hb {
					break
				}
				k := prev[j-1] + 1
				next[j] = k
				if k > best.n {
					best = block{i - k + 1, j - k + 1, k}
				}
			}
			prev = next
		}
		for best.a > lo && best.b > lb && a[best.a-1] == b[best.b-1] {
			best.a--
			best.b--
			best.n++
		}
		for best.a+best.n < hi && best.b+best.n < hb && a[best.a+best.n] == b[best.b+best.n] {
			best.n++
		}
		if best.n == 0 {
			return
		}
		if lo < best.a && lb < best.b {
			visit(lo, best.a, lb, best.b)
		}
		blocks = append(blocks, best)
		if best.a+best.n < hi && best.b+best.n < hb {
			visit(best.a+best.n, hi, best.b+best.n, hb)
		}
	}
	visit(0, len(a), 0, len(b))
	merged := []block{}
	for _, m := range blocks {
		if len(merged) > 0 {
			p := &merged[len(merged)-1]
			if p.a+p.n == m.a && p.b+p.n == m.b {
				p.n += m.n
				continue
			}
		}
		merged = append(merged, m)
	}
	return append(merged, block{len(a), len(b), 0})
}

// Python str.splitlines(keepends=True), including CRLF and Unicode separators.
func splitLines(s string) []string {
	var lines []string
	start, skip := 0, -1
	for i, r := range s {
		if i == skip {
			continue
		}
		size := 0
		switch r {
		case '\r':
			size = 1
			if i+1 < len(s) && s[i+1] == '\n' {
				size = 2
				skip = i + 1
			}
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e':
			size = 1
		case '\u0085':
			size = 2
		case '\u2028', '\u2029':
			size = 3
		}
		if size > 0 {
			lines = append(lines, s[start:i+size])
			start = i + size
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func unifiedDiff(old, now, from, to string) string {
	a, b := splitLines(old), splitLines(now)
	var codes []opcode
	i, j := 0, 0
	for _, m := range matchingBlocks(a, b) {
		tag := byte(0)
		switch {
		case i < m.a && j < m.b:
			tag = 'r'
		case i < m.a:
			tag = 'd'
		case j < m.b:
			tag = 'i'
		}
		if tag != 0 {
			codes = append(codes, opcode{tag, i, m.a, j, m.b})
		}
		if m.n > 0 {
			codes = append(codes, opcode{'e', m.a, m.a + m.n, m.b, m.b + m.n})
		}
		i, j = m.a+m.n, m.b+m.n
	}
	if len(codes) == 0 {
		return ""
	}
	if codes[0].tag == 'e' {
		c := &codes[0]
		c.a = max(c.a, c.z-3)
		c.b = max(c.b, c.y-3)
	}
	if codes[len(codes)-1].tag == 'e' {
		c := &codes[len(codes)-1]
		c.z = min(c.z, c.a+3)
		c.y = min(c.y, c.b+3)
	}
	var groups [][]opcode
	var group []opcode
	for _, c := range codes {
		if c.tag == 'e' && c.z-c.a > 6 {
			group = append(group, opcode{'e', c.a, c.a + 3, c.b, c.b + 3})
			groups = append(groups, group)
			group = nil
			c.a, c.b = c.z-3, c.y-3
		}
		group = append(group, c)
	}
	if len(group) > 0 && !(len(group) == 1 && group[0].tag == 'e') {
		groups = append(groups, group)
	}
	if len(groups) == 0 {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", from, to)
	span := func(start, end int) string {
		if end-start == 1 {
			return fmt.Sprint(start + 1)
		}
		if end == start {
			return fmt.Sprintf("%d,0", start)
		}
		return fmt.Sprintf("%d,%d", start+1, end-start)
	}
	line := func(prefix byte, s string) {
		out.WriteByte(prefix)
		out.WriteString(s)
		if !strings.HasSuffix(s, "\n") {
			out.WriteString("\n\\ No newline at end of file\n")
		}
	}
	for _, g := range groups {
		first, last := g[0], g[len(g)-1]
		fmt.Fprintf(&out, "@@ -%s +%s @@\n", span(first.a, last.z), span(first.b, last.y))
		for _, c := range g {
			if c.tag == 'e' {
				for _, s := range a[c.a:c.z] {
					line(' ', s)
				}
			}
			if c.tag == 'd' || c.tag == 'r' {
				for _, s := range a[c.a:c.z] {
					line('-', s)
				}
			}
			if c.tag == 'i' || c.tag == 'r' {
				for _, s := range b[c.b:c.y] {
					line('+', s)
				}
			}
		}
	}
	return out.String()
}
