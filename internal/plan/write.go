package plan

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	revisionLine = regexp.MustCompile(`^# Plan, rev ([0-9]+) · [0-9a-f]{6,}$`)
	planHeading  = regexp.MustCompile(`^# Plan\b`)
	recallLine   = regexp.MustCompile(`(?i)^[ \t]*(?:[-*+][ \t]+)?(?:\*\*|__)?(Recall|Used)(?:\*\*|__)?[ \t]*:(?:\*\*|__)?[ \t]*(.*)$`)
	comments     = regexp.MustCompile(`(?s)<!--.*?(?:-->|$)`)
)

// Revision reads the positive integer in the original revision heading format.
func Revision(text string) (uint64, error) {
	m := revisionLine.FindStringSubmatch(lines(text)[0])
	if m == nil {
		return 0, errors.New("plan must start with # Plan, rev <number> · <hash>")
	}
	n, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil || n == 0 {
		return 0, errors.New("plan revision must be a positive integer")
	}
	return n, nil
}

// HasRecall requires a nonempty Used citation or Recall: nothing found in the
// visible plan, allowing the old bullet/bold spelling. Examples do not count.
func HasRecall(text string) bool {
	fence, width := byte(0), 0
	for _, line := range lines(comments.ReplaceAllString(text, "")) {
		trim := strings.TrimSpace(line)
		if len(trim) >= 3 && (trim[0] == '`' || trim[0] == '~') {
			n := 0
			for n < len(trim) && trim[n] == trim[0] {
				n++
			}
			if fence == 0 && n >= 3 {
				fence, width = trim[0], n
				continue
			}
			if fence == trim[0] && n >= width && strings.TrimSpace(trim[n:]) == "" {
				fence = 0
				continue
			}
		}
		if fence != 0 || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			continue
		}
		m := recallLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[2])
		if strings.EqualFold(m[1], "Recall") {
			if strings.EqualFold(strings.TrimSpace(strings.TrimRight(value, "*_")), "nothing found") {
				return true
			}
		} else if value != "" && !strings.ContainsAny(value[:1], "*_") {
			return true
		}
	}
	return false
}

// Written describes the newly installed plan, not a task record or Feed post.
type Written struct {
	Revision uint64 `json:"revision"`
	Hash     string `json:"hash"`
}

// Write installs a new revision, including when only ticks or whitespace changed;
// bytes identical to the current plan leave it and its revision unchanged.
// The installed heading determines the next number; the input's number is ignored.
// A failed replacement can leave a complete archive of the still-current revision;
// retrying accepts that archive only if its bytes agree exactly.
func (s Store) Write(text string) (Written, error) {
	if !HasRecall(text) {
		return Written{}, errors.New("plan write requires Used: <note> or Recall: nothing found")
	}
	if _, err := Items(text); err != nil {
		return Written{}, err
	}
	dir, err := s.directory()
	if err != nil {
		return Written{}, err
	}
	var result Written
	err = locked(dir, func() error {
		current, err := readRegular(filepath.Join(dir, "plan.md"))
		next := uint64(1)
		if err == nil {
			rev, err := Revision(string(current))
			if err != nil {
				return fmt.Errorf("current plan: %w", err)
			}
			if rev == math.MaxUint64 {
				return errors.New("plan revision overflow")
			}
			next = rev + 1
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		ls := lines(text)
		if planHeading.MatchString(ls[0]) {
			ls = ls[1:]
		}
		body := strings.TrimLeft(strings.Join(ls, "\n"), "\n")
		hash := Hash("x\n" + body)
		if current != nil && string(current) == fmt.Sprintf("# Plan, rev %d · %s\n\n%s", next-1, hash, body) {
			result = Written{Revision: next - 1, Hash: hash}
			return nil
		}
		out := fmt.Sprintf("# Plan, rev %d · %s\n\n%s", next, hash, body)
		if current != nil {
			archiveDir, err := childDirectory(dir, "plan-revisions", true)
			if err != nil {
				return err
			}
			archive := filepath.Join(archiveDir, fmt.Sprintf("%d.md", next-1))
			old, err := readRegular(archive)
			if err == nil {
				if string(old) != string(current) {
					return errors.New("previous plan revision archive conflicts with current plan; refusing to overwrite it")
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			} else if err := atomicWrite(archive, current); err != nil {
				return err
			}
		}
		if err := atomicWrite(filepath.Join(dir, "plan.md"), []byte(out)); err != nil {
			return err
		}
		result = Written{Revision: next, Hash: hash}
		return nil
	})
	return result, err
}
