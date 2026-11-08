// Package contract validates Grid records and folds admitted Feed posts.
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

var schemas struct {
	sync.Once
	byName map[string]map[string]any
	err    error
}

func loadSchemas() {
	schemas.byName = make(map[string]map[string]any)
	entries, err := contractdata.Schemas.ReadDir("schemas")
	if err != nil {
		schemas.err = err
		return
	}
	for _, entry := range entries {
		data, err := contractdata.Schemas.ReadFile("schemas/" + entry.Name())
		if err != nil {
			schemas.err = err
			return
		}
		var schema map[string]any
		if err = decode(data, &schema); err == nil {
			err = checkSubset(schema)
		}
		if err != nil {
			schemas.err = fmt.Errorf("schema %s: %w", entry.Name(), err)
			return
		}
		schemas.byName[entry.Name()] = schema
	}
}

// SchemaNames returns the names accepted by Validate, in sorted order.
func SchemaNames() []string {
	schemas.Do(loadSchemas)
	names := make([]string, 0, len(schemas.byName))
	for name := range schemas.byName {
		names = append(names, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(names)
	return names
}

// Schema returns a fresh copy of a canonical, embedded schema.
func Schema(name string) ([]byte, error) {
	return contractdata.Schemas.ReadFile("schemas/" + strings.TrimSuffix(name, ".json") + ".json")
}

// Validate checks the documented JSON Schema subset and cross-field rules that
// JSON Schema cannot express (matching identifiers and fixed lease durations).
// It does not fetch references or read clocks, files, processes or the network.
func Validate(name string, data []byte) error {
	schemas.Do(loadSchemas)
	if schemas.err != nil {
		return schemas.err
	}
	name = strings.TrimSuffix(name, ".json")
	schema, ok := schemas.byName[name+".json"]
	if !ok {
		return fmt.Errorf("unknown schema %q", name)
	}
	var value any
	if err := decode(data, &value); err != nil {
		return err
	}
	if err := validate(schema, value, "$", 0); err != nil {
		return err
	}
	return semantic(name, value)
}

func decode(data []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return nil
}

// checkSubset fails closed when a schema introduces a keyword we do not support.
func checkSubset(s map[string]any) error {
	for k, v := range s {
		switch k {
		case "$schema", "$id", "title", "description", "x-class", "x-roles", "$ref", "type", "enum", "const", "required", "minimum", "maximum", "minLength", "maxLength", "minItems", "maxItems", "uniqueItems":
		case "format":
			if v != "date-time" {
				return fmt.Errorf("unsupported format %v", v)
			}
		case "pattern":
			if _, err := regexp.Compile(v.(string)); err != nil {
				return err
			}
		case "properties":
			for _, child := range v.(map[string]any) {
				if err := checkSubset(child.(map[string]any)); err != nil {
					return err
				}
			}
		case "items", "not", "if", "then", "additionalProperties":
			if child, ok := v.(map[string]any); ok {
				if err := checkSubset(child); err != nil {
					return err
				}
			}
		case "allOf", "anyOf", "oneOf":
			for _, child := range v.([]any) {
				if err := checkSubset(child.(map[string]any)); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported keyword %q", k)
		}
	}
	return nil
}

func validate(s map[string]any, v any, path string, depth int) error {
	if depth > 100 {
		return fmt.Errorf("%s: schema nesting limit", path)
	}
	fail := func(rule string) error { return fmt.Errorf("%s: %s", path, rule) }
	check := func(child any) error { return validate(child.(map[string]any), v, path, depth+1) }
	if name, ok := s["$ref"].(string); ok {
		target, ok := schemas.byName[name]
		if !ok {
			return fail("unresolved reference " + name)
		}
		if err := check(target); err != nil {
			return err
		}
	}
	if typ, ok := s["type"].(string); ok && !matchesType(typ, v) {
		return fail("expected " + typ)
	}
	if c, ok := s["const"]; ok && !equal(c, v) {
		return fail("const mismatch")
	}
	if values, ok := s["enum"].([]any); ok {
		found := false
		for _, value := range values {
			found = found || equal(value, v)
		}
		if !found {
			return fail("value outside enum")
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if children, ok := s[key].([]any); ok {
			count := 0
			var last error
			for _, child := range children {
				if err := check(child); err == nil {
					count++
				} else {
					last = err
				}
			}
			if key == "allOf" && count != len(children) {
				return last
			}
			if key == "anyOf" && count == 0 || key == "oneOf" && count != 1 {
				return fail(fmt.Sprintf("%s matched %d branches: %v", key, count, last))
			}
		}
	}
	if child, ok := s["not"]; ok && check(child) == nil {
		return fail("not constraint")
	}
	if test, ok := s["if"]; ok && check(test) == nil {
		if child, ok := s["then"]; ok {
			if err := check(child); err != nil {
				return err
			}
		}
	}
	if object, ok := v.(map[string]any); ok {
		if required, ok := s["required"].([]any); ok {
			for _, key := range required {
				if _, exists := object[key.(string)]; !exists {
					return fail("missing required " + key.(string))
				}
			}
		}
		props, _ := s["properties"].(map[string]any)
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := object[key]
			child, exists := props[key]
			if !exists {
				switch extra := s["additionalProperties"].(type) {
				case bool:
					if !extra {
						return fail("unknown property " + key)
					}
				case map[string]any:
					child = extra
				}
			}
			if child != nil {
				if err := validate(child.(map[string]any), value, path+"."+key, depth+1); err != nil {
					return err
				}
			}
		}
	}
	if list, ok := v.([]any); ok {
		if below(s, "minItems", len(list)) || above(s, "maxItems", len(list)) {
			return fail("array length")
		}
		for i, value := range list {
			if s["uniqueItems"] == true {
				for _, earlier := range list[:i] {
					if equal(value, earlier) {
						return fail("duplicate array item")
					}
				}
			}
			if child, ok := s["items"].(map[string]any); ok {
				if err := validate(child, value, fmt.Sprintf("%s[%d]", path, i), depth+1); err != nil {
					return err
				}
			}
		}
	}
	if str, ok := v.(string); ok {
		if below(s, "minLength", utf8.RuneCountInString(str)) || above(s, "maxLength", utf8.RuneCountInString(str)) {
			return fail("string length")
		}
		if pattern, ok := s["pattern"].(string); ok {
			re, err := regexp.Compile(pattern)
			if err != nil || !re.MatchString(str) {
				return fail("pattern " + pattern)
			}
		}
		if s["format"] == "date-time" {
			if _, err := time.Parse(time.RFC3339Nano, str); err != nil {
				return fail("invalid date-time")
			}
		}
	}
	if number, ok := v.(json.Number); ok {
		n := rational(number)
		if min, ok := s["minimum"].(json.Number); ok && n.Cmp(rational(min)) < 0 {
			return fail("below minimum")
		}
		if max, ok := s["maximum"].(json.Number); ok && n.Cmp(rational(max)) > 0 {
			return fail("above maximum")
		}
	}
	return nil
}

func matchesType(typ string, v any) bool {
	switch typ {
	case "null":
		return v == nil
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "number", "integer":
		n, ok := v.(json.Number)
		return ok && (typ == "number" || rational(n).IsInt())
	}
	return false
}

func rational(n json.Number) *big.Rat {
	r, ok := new(big.Rat).SetString(string(n))
	if !ok {
		return new(big.Rat)
	}
	return r
}

func equal(a, b any) bool {
	if x, ok := a.(json.Number); ok {
		y, ok := b.(json.Number)
		return ok && rational(x).Cmp(rational(y)) == 0
	}
	return reflect.DeepEqual(a, b)
}

func below(s map[string]any, key string, n int) bool {
	limit, ok := s[key].(json.Number)
	return ok && new(big.Rat).SetInt64(int64(n)).Cmp(rational(limit)) < 0
}

func above(s map[string]any, key string, n int) bool {
	limit, ok := s[key].(json.Number)
	return ok && new(big.Rat).SetInt64(int64(n)).Cmp(rational(limit)) > 0
}

func semantic(name string, value any) error {
	v, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	if name == "merge-lease" || name == "lease" && v["lease"] == "merge" {
		return duration(v, "since", "until", 15*time.Minute)
	}
	if name == "heartbeat" {
		return duration(v, "beat_at", "until", 10*time.Minute)
	}
	if name == "takeover" && v["file"] != fmt.Sprintf("%s.%s.jsonl", v["writer"], v["fence"]) {
		return fmt.Errorf("$.file: must match writer and fence")
	}
	if body, ok := v["body"].(map[string]any); ok {
		if isJobSide(name) && v["lease"] != v["job"] {
			return fmt.Errorf("$.lease: must match job")
		}
		if _, external := body["ref"]; external && len(body) == 1 {
			return nil
		}
		if name == "job" && v["job"] != body["id"] || name == "checkpoint" && v["job"] != body["job"] || name == "usage-notice" && v["task"] != body["task"] {
			return fmt.Errorf("$.body: identifier must match envelope")
		}
	}
	return nil
}

func duration(v map[string]any, start, end string, want time.Duration) error {
	a, _ := time.Parse(time.RFC3339Nano, v[start].(string))
	b, _ := time.Parse(time.RFC3339Nano, v[end].(string))
	if b.Sub(a) != want {
		return fmt.Errorf("$.%s: must equal %s plus %s", end, start, want)
	}
	return nil
}
