package plan

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// Item is a plan projection, not a Feed Job or task record. Optional fields
// retain their YAML value as JSON, including empty lists and explicit nulls.
type Item struct {
	ID     string          `json:"id"`
	Title  string          `json:"title"`
	State  string          `json:"state"`
	Write  json.RawMessage `json:"write,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
	Review json.RawMessage `json:"review,omitempty"`
	Risk   json.RawMessage `json:"risk,omitempty"`
	Verify json.RawMessage `json:"verify,omitempty"`
	text   string
}

var (
	itemLine     = regexp.MustCompile(`^- \[([ xX])\] (\d+) (\S.*)$`)
	blockHeading = regexp.MustCompile(`^## (i-\d+) · ([^·]+?) · (.+)$`)
	fieldLine    = regexp.MustCompile(`^( +)(write|after|review|risk|verify):(?:[ \t]*(.*))$`)
	itemNumber   = regexp.MustCompile(`^[0-9]+$`)
)

// Items reads top-level numbered checklist steps in document order. A step's
// block ends at a blank or unindented line, as in the old plan skill.
func Items(text string) ([]Item, error) {
	ls := lines(text)
	items := []Item{}
	seen := map[string]bool{}
	for i := 0; i < len(ls); i++ {
		m := itemLine.FindStringSubmatch(ls[i])
		if m == nil {
			continue
		}
		n, err := strconv.ParseUint(m[2], 10, 64)
		if err != nil || n == 0 {
			return nil, fmt.Errorf("plan line %d: item number must be a positive integer", i+1)
		}
		id := fmt.Sprintf("i-%d", n)
		if seen[id] {
			return nil, fmt.Errorf("plan line %d: duplicate item %s", i+1, id)
		}
		seen[id] = true
		item := Item{ID: id, Title: strings.TrimSpace(strings.NewReplacer("**", "", "`", "").Replace(m[3])), State: "open"}
		if m[1] != " " {
			item.State = "built"
		}
		end := i + 1
		for end < len(ls) && indented.MatchString(ls[end]) {
			end++
		}
		item.text = strings.Join(ls[i:end], "\n")
		if err := item.fields(ls[i+1 : end]); err != nil {
			return nil, fmt.Errorf("item %s: %w", id, err)
		}
		items = append(items, item)
		i = end - 1
	}
	return items, nil
}

func (item *Item) fields(block []string) error {
	fields := map[string]*json.RawMessage{"write": &item.Write, "after": &item.After, "review": &item.Review, "risk": &item.Risk, "verify": &item.Verify}
	for i := 0; i < len(block); i++ {
		m := fieldLine.FindStringSubmatch(block[i])
		if m == nil {
			continue
		}
		name, indent := m[2], len(m[1])
		if *fields[name] != nil {
			return fmt.Errorf("duplicate %s field", name)
		}
		end := i + 1
		for end < len(block) && len(block[end])-len(strings.TrimLeft(block[end], " ")) > indent {
			end++
		}
		var data json.RawMessage
		// Legacy verify is Markdown prose, often starting with a backtick or
		// containing a colon. Structured verify uses an inline map or block.
		value := strings.TrimSpace(m[3])
		if name == "verify" && value != "" && !strings.ContainsAny(value[:1], "{[|>'\"") {
			data, _ = json.Marshal(value)
		} else {
			var node yaml.Node
			field := strings.Join(block[i:end], "\n")
			if err := yaml.Unmarshal([]byte(field), &node); err != nil {
				return fmt.Errorf("invalid %s: %w", name, err)
			}
			var values map[string]any
			if err := node.Decode(&values); err != nil {
				return fmt.Errorf("invalid %s: %w", name, err)
			}
			var err error
			data, err = json.Marshal(values[name])
			if err != nil {
				return fmt.Errorf("%s must be JSON-compatible YAML: %w", name, err)
			}
		}
		*fields[name] = data
		i = end - 1
	}
	return nil
}

// WithLegacyState overlays items.md and running subagent briefs without writing
// either file. Titles and membership always come from the plan.
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
			if n, err := strconv.ParseUint(number, 10, 64); err == nil && itemNumber.MatchString(number) {
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
		if out[i].State == "built" && (state == "open" || state == "working") {
			state = "built"
		} else if state == "open" && running[out[i].ID] {
			state = "working"
		}
		out[i].State = state
	}
	return out
}
