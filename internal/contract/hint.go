package contract

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// refusal finds the failing rule in a Validate message: the path, then one of
// the three rules Hint can explain. A oneOf/anyOf message nests the branch
// that got furthest, so the last match is the one that names the field.
var refusal = regexp.MustCompile(`(\$[^\s:]*): (value outside enum|missing required (\S+)|unknown property (\S+))`)

// Hint says what the schema allows where a Validate error failed: the allowed
// values for an enum, the known property names for an unknown property, the
// missing field for a missing required. It returns "" when it cannot tell: an
// unknown schema, an error from another rule, or a path the schema does not
// reach. The error messages themselves are unchanged.
func Hint(name string, err error) string {
	if err == nil {
		return ""
	}
	schemas.Do(loadSchemas)
	if schemas.err != nil {
		return ""
	}
	schema, ok := schemas.byName[strings.TrimSuffix(name, ".json")+".json"]
	if !ok {
		return ""
	}
	matches := refusal.FindAllStringSubmatch(err.Error(), -1)
	if len(matches) == 0 {
		return ""
	}
	m := matches[len(matches)-1]
	path, rule := m[1], m[2]
	nodes := reach(schema, path)
	if len(nodes) == 0 {
		return ""
	}
	where := "the record"
	if path != "$" {
		where = path
	}
	switch {
	case rule == "value outside enum":
		var values []string
		seen := map[string]bool{}
		for _, n := range nodes {
			list, _ := n["enum"].([]any)
			for _, v := range list {
				text := enumText(v)
				if !seen[text] {
					seen[text] = true
					values = append(values, text)
				}
			}
		}
		if len(values) == 0 {
			return ""
		}
		return fmt.Sprintf("Set %s to one of: %s.", where, strings.Join(values, ", "))
	case m[3] != "":
		return fmt.Sprintf("Add the required field %q to %s.", m[3], where)
	case m[4] != "":
		known := knownProperties(nodes)
		if len(known) == 0 {
			return ""
		}
		return fmt.Sprintf("Remove %q from %s; the known properties are: %s.", m[4], where, strings.Join(known, ", "))
	}
	return ""
}

func enumText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// knownProperties lists the property names an object at the failing path may
// hold. Nodes that close the object (additionalProperties false) are the ones
// that refuse an unknown property, so they decide when there are any.
func knownProperties(nodes []map[string]any) []string {
	pick := func(closedOnly bool) []string {
		set := map[string]bool{}
		for _, n := range nodes {
			if closedOnly && n["additionalProperties"] != false {
				continue
			}
			props, _ := n["properties"].(map[string]any)
			for k := range props {
				set[k] = true
			}
		}
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	if out := pick(true); len(out) > 0 {
		return out
	}
	return pick(false)
}

// reach returns the schema nodes that apply at a "$.a[0].b" path.
func reach(root map[string]any, path string) []map[string]any {
	nodes := expand([]map[string]any{root})
	rest := strings.TrimPrefix(path, "$")
	for rest != "" && len(nodes) > 0 {
		var next []map[string]any
		switch rest[0] {
		case '.':
			rest = rest[1:]
			end := strings.IndexAny(rest, ".[")
			if end < 0 {
				end = len(rest)
			}
			key := rest[:end]
			rest = rest[end:]
			for _, n := range nodes {
				props, _ := n["properties"].(map[string]any)
				if child, ok := props[key].(map[string]any); ok {
					next = append(next, child)
				} else if child, ok := n["additionalProperties"].(map[string]any); ok {
					next = append(next, child)
				}
			}
		case '[':
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				return nil
			}
			if _, err := strconv.Atoi(rest[1:end]); err != nil {
				return nil
			}
			rest = rest[end+1:]
			for _, n := range nodes {
				if child, ok := n["items"].(map[string]any); ok {
					next = append(next, child)
				}
			}
		default:
			return nil
		}
		nodes = expand(next)
	}
	return nodes
}

// expand adds every schema a node applies through: $ref targets, allOf, anyOf,
// oneOf and then branches.
func expand(start []map[string]any) []map[string]any {
	var out []map[string]any
	queue := append([]map[string]any(nil), start...)
	for len(queue) > 0 && len(out) < 1000 {
		n := queue[0]
		queue = queue[1:]
		out = append(out, n)
		if ref, ok := n["$ref"].(string); ok {
			if target, ok := schemas.byName[ref]; ok {
				queue = append(queue, target)
			}
		}
		for _, key := range []string{"allOf", "anyOf", "oneOf"} {
			children, _ := n[key].([]any)
			for _, c := range children {
				if child, ok := c.(map[string]any); ok {
					queue = append(queue, child)
				}
			}
		}
		if child, ok := n["then"].(map[string]any); ok {
			queue = append(queue, child)
		}
	}
	return out
}
