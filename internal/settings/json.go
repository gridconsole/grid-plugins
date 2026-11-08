package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// object rejects unknown and repeated keys before any write, including keys in
// nested objects. JSON null and multiple documents are never settings objects.
func object(data []byte, keys []string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, fmt.Errorf("expected a JSON object")
	}
	out := map[string]json.RawMessage{}
	for d.More() {
		tok, err = d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("expected a key")
		}
		if err := allowed("key", key, keys); err != nil {
			return nil, err
		}
		if _, ok := out[key]; ok {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		out[key] = raw
	}
	if _, err = d.Token(); err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fmt.Errorf("expected exactly one JSON object")
	}
	return out, nil
}
func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func textValue(raw json.RawMessage) (string, error) {
	var s string
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("expected a string")
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}
func decode(data []byte) (Document, error) {
	doc := empty()
	raw, err := object(data, []string{"agents", "skills", "hooks"})
	if err != nil {
		return doc, err
	}
	for _, group := range keysOf(raw) {
		var names []string
		switch group {
		case "agents":
			names = AgentNames()
		case "skills":
			names = skills
		case "hooks":
			names = hooks
		}
		entries, err := object(raw[group], names)
		if err != nil {
			return doc, fmt.Errorf("%s: %w", group, err)
		}
		for _, name := range keysOf(entries) {
			value := entries[name]
			switch group {
			case "agents":
				fields, err := object(value, []string{"model", "effort"})
				if err != nil {
					return doc, err
				}
				a := Agent{}
				for _, key := range keysOf(fields) {
					s, err := textValue(fields[key])
					if err != nil {
						return doc, err
					}
					if s == "" {
						return doc, fmt.Errorf("%s must not be empty", key)
					}
					if key == "model" {
						a.Model = s
					} else {
						a.Effort = s
					}
				}
				if err := validateAgent(a); err != nil {
					return doc, err
				}
				doc.Agents[name] = a
			case "skills":
				if !bytes.Equal(value, []byte("true")) && !bytes.Equal(value, []byte("false")) {
					return doc, fmt.Errorf("skill %s must be true or false", name)
				}
				doc.Skills[name] = bytes.Equal(value, []byte("true"))
			case "hooks":
				s, err := textValue(value)
				if err != nil {
					return doc, err
				}
				if err := allowed("hook level", s, levels); err != nil {
					return doc, err
				}
				doc.Hooks[name] = s
			}
		}
	}
	return doc, nil
}
