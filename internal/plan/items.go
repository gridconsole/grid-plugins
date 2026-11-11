package plan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"go.yaml.in/yaml/v4"
)

// Item owns a checklist block. Jobs are planned YAML objects, decoded by job.
// State is an optional legacy run projection; the checkbox never supplies it.
type Item struct {
	N     int    `json:"n"`
	ID    string `json:"id"`
	Title string `json:"title"`
	Prose string `json:"prose"`
	Kind  string `json:"kind,omitempty"`
	// Proof is the evidence level a step needs (check, log, screenshot or video).
	// The key is provisional until the owner confirms it; see docs/plan.md.
	Proof string `json:"proof,omitempty"`
	// Needs is read from the step's prose `needs:` line: [] for none, the step
	// numbers in order, nil (JSON null) when the line is missing or malformed.
	// The line itself stays in Prose.
	Needs []int            `json:"needs"`
	Jobs  []map[string]any `json:"jobs"`
	State string           `json:"state"`
	text  string
	line  string
}

var (
	itemLine       = regexp.MustCompile(`^- \[([ xX])\] (\d+) (\S.*)$`)
	blockHeading   = regexp.MustCompile(`^## (i-\d+) · ([^·]+?) · (.+)$`)
	itemNumber     = regexp.MustCompile(`^[0-9]+$`)
	forbiddenField = regexp.MustCompile(`^  (write|after|review|risk|verify):`)
	itemRef        = regexp.MustCompile(`^(?:i-|item-)?([0-9]+)$`)
)

// ItemID turns the forms a person types for a step (1, i-1, item-1) into the
// id Items gives it (i-1). Plan is the only owner of item identity; callers that
// take an item argument use this instead of parsing it themselves. A zero or
// non-numeric reference is not an id.
func ItemID(ref string) (string, bool) {
	m := itemRef.FindStringSubmatch(ref)
	if m == nil {
		return "", false
	}
	n, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil || n == 0 {
		return "", false
	}
	return fmt.Sprintf("i-%d", n), true
}

// Items reads top-level numbered checklist blocks. Prose precedes a YAML part
// beginning with exactly two spaces and kind:, jobs: or proof:, in any order.
// Blank lines end blocks.
func Items(text string) ([]Item, error) {
	ls := lines(text)
	if ls[0] == "---" {
		return nil, fmt.Errorf("plan line 1: front matter is refused; use checklist items with jobs lists")
	}
	items := []Item{}
	seen := map[int]bool{}
	for i := 0; i < len(ls); i++ {
		if forbiddenField.MatchString(ls[i]) {
			return nil, fmt.Errorf("plan line %d: step-level %s is refused; put it under jobs", i+1, strings.TrimSpace(ls[i]))
		}
		m := itemLine.FindStringSubmatch(ls[i])
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[2])
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("plan line %d: item number must be a positive integer", i+1)
		}
		if seen[n] {
			return nil, fmt.Errorf("plan line %d: duplicate item %d", i+1, n)
		}
		seen[n] = true
		item := Item{N: n, ID: fmt.Sprintf("i-%d", n), Title: strings.TrimSpace(strings.NewReplacer("**", "", "`", "").Replace(m[3])), State: "open", line: ls[i]}
		end := i + 1
		for end < len(ls) && indented.MatchString(ls[end]) {
			end++
		}
		start := end
		for k := i + 1; k < end; k++ {
			if forbiddenField.MatchString(ls[k]) {
				return nil, fmt.Errorf("plan line %d: step-level %s is refused; put it under jobs", k+1, strings.TrimSpace(ls[k]))
			}
			if start == end && (strings.HasPrefix(ls[k], "  kind:") || strings.HasPrefix(ls[k], "  jobs:") || strings.HasPrefix(ls[k], "  proof:")) {
				start = k
			}
			if k < start && (!strings.HasPrefix(ls[k], "  ") || strings.HasPrefix(ls[k], "   ")) {
				return nil, fmt.Errorf("plan line %d: prose must be indented two spaces", k+1)
			}
		}
		prose := []string{}
		for _, l := range ls[i+1 : start] {
			prose = append(prose, strings.TrimPrefix(l, "  "))
		}
		item.Prose = strings.Join(prose, "\n")
		item.Needs, _, _ = NqNeedsRead(item.Prose)
		if start == end {
			return nil, fmt.Errorf("plan line %d: item %d missing jobs list", i+1, n)
		}
		var data map[string]any
		if err := yaml.Unmarshal([]byte(strings.Join(ls[start:end], "\n")), &data); err != nil {
			return nil, fmt.Errorf("plan line %d: %w", start+1, err)
		}
		for key := range data {
			if key != "kind" && key != "jobs" && key != "proof" {
				return nil, fmt.Errorf("plan line %d: unknown step field %q", start+1, key)
			}
		}
		if value, ok := data["kind"]; ok {
			var valid bool
			item.Kind, valid = value.(string)
			if !valid {
				return nil, fmt.Errorf("plan line %d: kind must be a string", start+1)
			}
		}
		if value, ok := data["proof"]; ok {
			var valid bool
			item.Proof, valid = value.(string)
			if !valid {
				return nil, fmt.Errorf("plan line %d: proof must be a string", start+1)
			}
		}
		jobs, ok := data["jobs"].([]any)
		if !ok {
			return nil, fmt.Errorf("plan line %d: jobs must be a list", start+1)
		}
		item.Jobs = []map[string]any{}
		for _, value := range jobs {
			raw, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("plan line %d: each jobs entry must be an object", start+1)
			}
			if _, ok := raw["item"]; !ok {
				raw["item"] = n
			}
			if _, ok := raw["title"]; !ok {
				raw["title"] = item.Title
			}
			intent, ok := raw["intent"].(map[string]any)
			if _, exists := raw["intent"]; !exists {
				intent = map[string]any{}
				raw["intent"] = intent
				ok = true
			}
			if ok {
				if _, exists := intent["item"]; !exists {
					intent["item"] = fmt.Sprintf("plan.md#step-%d", n)
				}
				if _, exists := intent["goal"]; !exists {
					intent["goal"] = "plan.md#verify"
				}
			}
			item.Jobs = append(item.Jobs, raw)
		}
		item.text = strings.Join(ls[i:end], "\n")
		items = append(items, item)
		i = end - 1
	}
	return items, nil
}

var nqNeedsValue = regexp.MustCompile(`^(none|[0-9]+(, ?[0-9]+)*)$`)

// NqNeedsRead reads the first `needs:` line of a step's prose. found is whether
// the line exists, value its text after `needs:`, needs the step numbers ([] for
// none, nil when the value is outside the grammar). It never fails.
func NqNeedsRead(prose string) (needs []int, value string, found bool) {
	for _, l := range strings.Split(prose, "\n") {
		rest, ok := strings.CutPrefix(l, "needs:")
		if !ok {
			continue
		}
		value = strings.TrimSpace(rest)
		if !nqNeedsValue.MatchString(value) {
			return nil, value, true
		}
		needs = []int{}
		if value != "none" {
			for _, part := range strings.Split(value, ",") {
				n, err := strconv.Atoi(strings.TrimSpace(part))
				if err != nil {
					return nil, value, true
				}
				needs = append(needs, n)
			}
		}
		return needs, value, true
	}
	return nil, "", false
}

// Anchor returns the checklist line and prose, or the first Verify paragraph.
func Anchor(text, anchor string) (string, error) {
	if strings.HasPrefix(anchor, "step-") {
		items, err := Items(text)
		if err != nil {
			return "", err
		}
		for _, item := range items {
			if anchor == fmt.Sprintf("step-%d", item.N) {
				out := item.line
				if item.Prose != "" {
					out += "\n  " + strings.ReplaceAll(item.Prose, "\n", "\n  ")
				}
				return out, nil
			}
		}
	} else if anchor == "verify" {
		ls := lines(text)
		for i, l := range ls {
			if l == "## Verify" {
				start := i + 1
				for start < len(ls) && strings.TrimSpace(ls[start]) == "" {
					start++
				}
				end := start
				for end < len(ls) && strings.TrimSpace(ls[end]) != "" && !strings.HasPrefix(ls[end], "#") {
					end++
				}
				if end > start {
					return strings.Join(ls[start:end], "\n"), nil
				}
				break
			}
		}
	}
	return "", fmt.Errorf("anchor %q not found", anchor)
}

// WithLegacyState overlays items.md and running briefs, never plan ticks.
func WithLegacyState(items []Item, blocks string, briefs []string) []Item {
	states := map[string]string{}
	for _, line := range lines(blocks) {
		if m := blockHeading.FindStringSubmatch(line); m != nil {
			states[m[1]] = strings.TrimSpace(m[2])
		}
	}
	running := map[string]bool{}
	for _, brief := range briefs {
		fm := taskfile.ScalarFrontmatter(brief)
		if fm == nil {
			continue
		}
		status, _ := fm.Data["status"].(string)
		numbers, _ := fm.Data["item"].(string)
		if status != "running" {
			continue
		}
		for _, number := range strings.Split(numbers, ",") {
			number = strings.TrimSpace(number)
			if n, err := strconv.Atoi(number); err == nil && itemNumber.MatchString(number) {
				running[fmt.Sprintf("i-%d", n)] = true
			}
		}
	}
	out := append([]Item{}, items...)
	for i := range out {
		state := states[out[i].ID]
		if state == "" {
			state = "open"
		}
		if state == "open" && running[out[i].ID] {
			state = "working"
		}
		out[i].State = state
	}
	return out
}
