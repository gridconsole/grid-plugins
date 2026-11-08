package plan

import (
	"fmt"
	"path/filepath"
	"strconv"
)

type Change struct {
	Before         Item   `json:"before"`
	After          Item   `json:"after"`
	BeforePosition int    `json:"before_position"`
	AfterPosition  int    `json:"after_position"`
	BeforeText     string `json:"before_text"`
	AfterText      string `json:"after_text"`
}

type Diff struct {
	From    uint64   `json:"from"`
	To      uint64   `json:"to"`
	Added   []Item   `json:"added"`
	Removed []Item   `json:"removed"`
	Changed []Change `json:"changed"`
}

// Compare matches stable item ids; content and position changes count. Content
// uses the full canonical text, not its six-digit hash, and includes step prose.
// Tick, needs and evidence changes alone do not invalidate an item approval.
// Run state is never inferred from historical checkboxes.
func Compare(before, after string) (Diff, error) {
	a, err := Items(before)
	if err != nil {
		return Diff{}, err
	}
	b, err := Items(after)
	if err != nil {
		return Diff{}, err
	}
	d := Diff{Added: []Item{}, Removed: []Item{}, Changed: []Change{}}
	old, next := map[string]int{}, map[string]int{}
	for i, item := range a {
		old[item.ID] = i
	}
	for i, item := range b {
		next[item.ID] = i
	}
	for _, item := range a {
		if _, ok := next[item.ID]; !ok {
			d.Removed = append(d.Removed, item)
		}
	}
	for j, item := range b {
		i, ok := old[item.ID]
		if !ok {
			d.Added = append(d.Added, item)
			continue
		}
		oldText, newText := canonical("x\n"+a[i].text), canonical("x\n"+item.text)
		if i != j || oldText != newText {
			d.Changed = append(d.Changed, Change{Before: a[i], After: item, BeforePosition: i + 1, AfterPosition: j + 1, BeforeText: oldText, AfterText: newText})
		}
	}
	return d, nil
}

// Diff accepts positive integer revision numbers only, never paths. The current
// plan is read once; older revisions are immutable files under plan-revisions.
func (s Store) Diff(from, to string) (Diff, error) {
	parse := func(arg string) (uint64, error) {
		n, err := strconv.ParseUint(arg, 10, 64)
		if err != nil || n == 0 || !itemNumber.MatchString(arg) {
			return 0, fmt.Errorf("invalid revision %q; use a positive integer", arg)
		}
		return n, nil
	}
	a, err := parse(from)
	if err != nil {
		return Diff{}, err
	}
	b, err := parse(to)
	if err != nil {
		return Diff{}, err
	}
	current, err := s.Read()
	if err != nil {
		return Diff{}, err
	}
	rev, err := Revision(current)
	if err != nil {
		return Diff{}, err
	}
	dir, err := s.directory()
	if err != nil {
		return Diff{}, err
	}
	read := func(n uint64) (string, error) {
		if n == rev {
			return current, nil
		}
		archiveDir, err := childDirectory(dir, "plan-revisions", false)
		if err != nil {
			return "", fmt.Errorf("revision %d: %w", n, err)
		}
		data, err := readRegular(filepath.Join(archiveDir, fmt.Sprintf("%d.md", n)))
		if err != nil {
			return "", fmt.Errorf("revision %d: %w", n, err)
		}
		found, err := Revision(string(data))
		if err != nil && n == 1 {
			// Explicit form C migration archives the unnumbered source as 1.
			if _, form, upgradeErr := Upgrade(string(data)); upgradeErr == nil && form == "C" {
				return string(data), nil
			}
		}
		if err != nil || found != n {
			return "", fmt.Errorf("revision %d archive has an invalid or mismatched heading", n)
		}
		return string(data), nil
	}
	before, err := read(a)
	if err != nil {
		return Diff{}, err
	}
	after, err := read(b)
	if err != nil {
		return Diff{}, err
	}
	if _, err := Items(before); err != nil {
		before, _, err = Upgrade(before)
		if err != nil {
			return Diff{}, fmt.Errorf("revision %d: %w", a, err)
		}
	}
	if _, err := Items(after); err != nil {
		after, _, err = Upgrade(after)
		if err != nil {
			return Diff{}, fmt.Errorf("revision %d: %w", b, err)
		}
	}
	d, err := Compare(before, after)
	d.From, d.To = a, b
	return d, err
}
