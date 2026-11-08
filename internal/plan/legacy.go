package plan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"go.yaml.in/yaml/v4"
)

var legacyField = regexp.MustCompile(`^  ([A-Za-z_][A-Za-z0-9_-]*):`)
var legacyStep = regexp.MustCompile(`^## Step ([1-9][0-9]*) \{#step-([1-9][0-9]*)\}$`)

// Upgrade converts a pre-checklist-with-jobs plan without changing current plans.
// The returned form is A (prose verification), B (step Job fields), C (front
// matter Jobs), or current. Conversion is deliberately separate from storage.
func Upgrade(text string) (out string, form string, err error) {
	if _, err := Items(text); err == nil {
		return text, "current", nil
	}
	if lines(text)[0] == "---" {
		out, err = upgradeFrontmatter(text)
		form = "C"
	} else {
		out, form, err = upgradeSteps(text)
	}
	if err != nil {
		return "", form, err
	}
	if _, err := Items(out); err != nil {
		return "", form, fmt.Errorf("converted plan: %w", err)
	}
	return out, form, nil
}

func legacyError(line int, message string) error {
	return fmt.Errorf("plan line %d: %s", line, message)
}

func upgradeSteps(text string) (string, string, error) {
	ls := lines(text)
	out := []string{}
	form := ""
	fence, width := byte(0), 0
	for i := 0; i < len(ls); {
		trim := strings.TrimSpace(ls[i])
		if len(trim) >= 3 && (trim[0] == '`' || trim[0] == '~') {
			n := 0
			for n < len(trim) && trim[n] == trim[0] {
				n++
			}
			if fence == 0 && n >= 3 {
				fence, width = trim[0], n
			} else if fence == trim[0] && n >= width && strings.TrimSpace(trim[n:]) == "" {
				fence = 0
			}
			out = append(out, ls[i])
			i++
			continue
		}
		if fence != 0 {
			out = append(out, ls[i])
			i++
			continue
		}
		m := itemLine.FindStringSubmatch(ls[i])
		if m == nil {
			if forbiddenField.MatchString(ls[i]) {
				return "", form, legacyError(i+1, "Job field outside an item")
			}
			if strings.HasPrefix(ls[i], "Verify: ") {
				out = append(out, "## Verify", "", strings.TrimPrefix(ls[i], "Verify: "))
			} else {
				out = append(out, ls[i])
			}
			i++
			continue
		}
		start := i
		end := i + 1
		for end < len(ls) && indented.MatchString(ls[end]) {
			end++
		}
		// A blank ends a current item; do not silently detach a legacy paragraph.
		next := end
		for next < len(ls) && strings.TrimSpace(ls[next]) == "" {
			next++
		}
		if next > end && next < len(ls) && indented.MatchString(ls[next]) {
			return "", form, legacyError(end+1, "blank line inside a legacy step")
		}
		prose, fields := []string{}, []string{}
		for k := i + 1; k < end; {
			f := legacyField.FindStringSubmatch(ls[k])
			if f == nil {
				if !strings.HasPrefix(ls[k], "  ") || strings.HasPrefix(ls[k], "   ") {
					return "", form, legacyError(k+1, "prose must be indented two spaces")
				}
				prose = append(prose, ls[k])
				k++
				continue
			}
			key := f[1]
			if key == "kind" || key == "jobs" {
				return "", form, legacyError(k+1, "mixed current and legacy plan fields")
			}
			stop := k + 1
			for stop < end && strings.HasPrefix(ls[stop], "    ") {
				stop++
			}
			switch key {
			case "needs", "evidence", "host":
				if stop != k+1 {
					return "", form, legacyError(k+1, "legacy prose field must be a single line")
				}
				prose = append(prose, ls[k])
			case "write", "after", "review", "risk", "verify":
				value := strings.TrimSpace(strings.TrimPrefix(ls[k], "  verify:"))
				if key == "verify" && value != "" && !strings.HasPrefix(value, "{") {
					prose = append(prose, strings.Replace(ls[k], "  verify:", "  Verified by:", 1))
					for _, continuation := range ls[k+1 : stop] {
						prose = append(prose, "  "+strings.TrimSpace(continuation))
					}
				} else {
					fields = append(fields, ls[k:stop]...)
				}
			default:
				return "", form, legacyError(k+1, fmt.Sprintf("unknown step field %q", key))
			}
			k = stop
		}
		if len(fields) > 0 {
			form = "B"
		} else if form == "" {
			form = "A"
		}
		out = append(out, ls[start])
		out = append(out, prose...)
		out = append(out, "  kind: build")
		if len(fields) == 0 {
			out = append(out, "  jobs: []")
		} else {
			var data map[string]any
			if err := yaml.Unmarshal([]byte(strings.Join(fields, "\n")), &data); err != nil {
				return "", form, legacyError(start+2, err.Error())
			}
			out = append(out, "  jobs:", "    - id: J-"+m[2])
			for _, field := range fields {
				out = append(out, "    "+field)
			}
		}
		i = end
	}
	if form == "" {
		return "", "", legacyError(1, "unrecognized legacy plan")
	}
	return strings.Join(out, "\n"), form, nil
}

// Use the shared front-matter reader for boundaries and validation, and YAML
// nodes for source positions. Jobs are copied as text to retain field order,
// comments, quoting, flow collections and continuation indentation.
func upgradeFrontmatter(text string) (string, error) {
	doc, err := frontmatter.Parse(text)
	if err != nil {
		return "", err
	}
	for key := range doc.Data {
		if key != "items" {
			return "", legacyError(doc.Lines[key], fmt.Sprintf("unknown front-matter field %q", key))
		}
	}
	ls := lines(text)
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(ls[1:doc.BodyLine-2], "\n")), &root); err != nil {
		return "", err
	}
	if len(root.Content) == 0 {
		return "", legacyError(2, "missing items list")
	}
	top := root.Content[0]
	if len(top.Content) != 2 || top.Content[0].Value != "items" || top.Content[1].Kind != yaml.SequenceNode {
		return "", legacyError(2, "expected an items Job list")
	}
	jobs := top.Content[1].Content
	if top.Content[1].Style&yaml.FlowStyle != 0 {
		return "", legacyError(top.Content[1].Line+1, "expected a block items list")
	}
	// Comments outside Jobs have no destination once the front matter is removed.
	prefixEnd := doc.BodyLine - 2
	if len(jobs) > 0 {
		prefixEnd = jobs[0].Line
	}
	for i := 1; i < prefixEnd; i++ {
		if ls[i] != "items:" && strings.TrimSpace(ls[i]) != "" {
			return "", legacyError(i+1, "cannot place front-matter text outside Jobs")
		}
	}
	type entry struct {
		title, role string
		line        int
		text        []string
	}
	byItem := map[int]entry{}
	for index, node := range jobs {
		if node.Kind != yaml.MappingNode || node.Style&yaml.FlowStyle != 0 {
			return "", legacyError(node.Line+1, "expected a block Job mapping")
		}
		var raw struct {
			Item  int
			Title string
			Role  string
		}
		if err := node.Decode(&raw); err != nil {
			return "", legacyError(node.Line+1, err.Error())
		}
		if raw.Item <= 0 || raw.Title == "" || strings.ContainsAny(raw.Title, "\r\n") {
			return "", legacyError(node.Line+1, "Job requires a positive item and a one-line title")
		}
		previous, exists := byItem[raw.Item]
		if exists && (previous.title != raw.Title || (previous.role == "writer") != (raw.Role == "writer")) {
			return "", legacyError(node.Line+1, "Jobs for one item disagree on its title or kind")
		}
		end := doc.BodyLine - 2
		if index+1 < len(jobs) {
			end = jobs[index+1].Line
		}
		source := append([]string{}, ls[node.Line:end]...)
		for len(source) > 0 && source[len(source)-1] == "" {
			source = source[:len(source)-1]
		}
		for j, line := range source {
			if !strings.HasPrefix(line, "  ") {
				return "", legacyError(node.Line+j+1, "cannot place Job text")
			}
			source[j] = "  " + line
		}
		if exists {
			previous.text = append(previous.text, source...)
			byItem[raw.Item] = previous
		} else {
			byItem[raw.Item] = entry{raw.Title, raw.Role, node.Line + 1, source}
		}
	}
	body := lines(doc.Body)
	out := []string{}
	for i := 0; i < len(body); {
		line := doc.BodyLine + i
		m := legacyStep.FindStringSubmatch(body[i])
		if m == nil {
			if itemLine.MatchString(body[i]) || strings.HasPrefix(body[i], "## Step ") {
				return "", legacyError(line, "mixed or unrecognized legacy step")
			}
			out = append(out, strings.ReplaceAll(body[i], "## Verify {#verify}", "## Verify"))
			i++
			continue
		}
		if m[1] != m[2] {
			return "", legacyError(line, "step anchor does not match its number")
		}
		n, _ := strconv.Atoi(m[1])
		job, ok := byItem[n]
		if !ok {
			return "", legacyError(line, "step has no matching Job (or is duplicated)")
		}
		delete(byItem, n)
		end := i + 1
		for end < len(body) && !strings.HasPrefix(body[end], "## ") {
			end++
		}
		first, last := i+1, end
		for first < last && body[first] == "" {
			first++
		}
		for last > first && body[last-1] == "" {
			last--
		}
		out = append(out, fmt.Sprintf("- [ ] %d %s", n, job.title))
		for k := first; k < last; k++ {
			if strings.TrimSpace(body[k]) == "" || strings.HasPrefix(body[k], " ") || strings.HasPrefix(body[k], "\t") || legacyField.MatchString("  "+body[k]) {
				return "", legacyError(doc.BodyLine+k, "cannot place step body as two-space prose")
			}
			out = append(out, "  "+body[k])
		}
		kind := "build"
		if job.role == "writer" {
			kind = "non-code"
		}
		out = append(out, "  kind: "+kind, "  jobs:")
		out = append(out, job.text...)
		out = append(out, body[last:end]...)
		i = end
	}
	for _, node := range jobs {
		var raw struct{ Item int }
		_ = node.Decode(&raw)
		if job, ok := byItem[raw.Item]; ok {
			return "", legacyError(job.line, "Job has no matching step item")
		}
	}
	return strings.Join(out, "\n"), nil
}
