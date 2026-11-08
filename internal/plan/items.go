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
	N     int              `json:"n"`
	ID    string           `json:"id"`
	Title string           `json:"title"`
	Prose string           `json:"prose"`
	Kind  string           `json:"kind,omitempty"`
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
)

// Items reads top-level numbered checklist blocks. Prose precedes a YAML part
// beginning with exactly two spaces and kind: or jobs:. Blank lines end blocks.
func Items(text string) ([]Item, error) {
	ls := lines(text)
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
			if start == end && (strings.HasPrefix(ls[k], "  kind:") || strings.HasPrefix(ls[k], "  jobs:")) {
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
		if start == end {
			return nil, fmt.Errorf("plan line %d: item %d missing jobs list", i+1, n)
		}
		var data map[string]any
		if err := yaml.Unmarshal([]byte(strings.Join(ls[start:end], "\n")), &data); err != nil {
			return nil, fmt.Errorf("plan line %d: %w", start+1, err)
		}
		for key := range data {
			if key != "kind" && key != "jobs" {
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
