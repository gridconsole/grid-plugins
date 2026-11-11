package plan

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
