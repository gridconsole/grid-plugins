package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
)

// object rejects unknown and repeated keys before any write, including keys in
// nested objects. JSON null and multiple documents are never settings objects.
func object(data []byte, keys []string) (map[string]json.RawMessage, error) {
	out, _, err := scanObject(data, keys, true)
	return out, err
}

// scanObject reads one JSON object. With strict set, a key outside keys is an
// error. Without it, such a key is returned in unknown (value kept verbatim) and
// the rest of the object is still read; repeated keys are refused either way.
func scanObject(data []byte, keys []string, strict bool) (known, unknown map[string]json.RawMessage, err error) {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil {
		return nil, nil, err
	}
	if tok != json.Delim('{') {
		return nil, nil, fmt.Errorf("expected a JSON object")
	}
	known, unknown = map[string]json.RawMessage{}, map[string]json.RawMessage{}
	for d.More() {
		tok, err = d.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, nil, fmt.Errorf("expected a key")
		}
		dst := known
		if err := allowed("key", key, keys); err != nil {
			if strict {
				return nil, nil, err
			}
			dst = unknown
		}
		_, inKnown := known[key]
		_, inUnknown := unknown[key]
		if inKnown || inUnknown {
			return nil, nil, fmt.Errorf("duplicate key %q", key)
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, nil, err
		}
		dst[key] = raw
	}
	if _, err = d.Token(); err != nil {
		return nil, nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, nil, fmt.Errorf("expected exactly one JSON object")
	}
	return known, unknown, nil
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
	// A top-level key this plugin does not know is not an error: another program
	// (the app, a newer plugin) may own it. It is warned about once, kept verbatim
	// through every rewrite, and the rest of the file is still read.
	raw, unknown, err := scanObject(data, append([]string{"agents", "skills", "hooks", "budget", "accounts"}, ezKeysAppNames...), false)
	if err != nil {
		return doc, err
	}
	for _, key := range keysOf(unknown) {
		doc.unknown = append(doc.unknown, unknownKey{key, unknown[key]})
		doc.Warnings = append(doc.Warnings, fmt.Sprintf("unknown key %q ignored; kept as it is", key))
	}
	for _, group := range keysOf(raw) {
		if slices.Contains(ezKeysAppNames, group) {
			// The app's keys: kept verbatim so a rewrite does not drop them.
			doc.ezKeysSet(group, raw[group])
			continue
		}
		if group == "accounts" {
			// The app's account records: kept verbatim so a rewrite does not drop them.
			if v := bytes.TrimSpace(raw[group]); len(v) == 0 || v[0] != '[' {
				return doc, fmt.Errorf("accounts: expected a JSON array")
			}
			doc.Accounts = raw[group]
			continue
		}
		if group == "budget" {
			s, err := textValue(raw[group])
			if err == nil {
				err = allowed("budget preset", s, budgetPresetNames)
			}
			if err != nil {
				return doc, fmt.Errorf("budget: %w", err)
			}
			doc.Budget = s
			continue
		}
		var names []string
		switch group {
		case "agents":
			names = AgentNames()
		case "skills":
			names = skills
		case "hooks":
			names = append(slices.Clone(hooks), retiredHooks...)
		}
		entries, err := object(raw[group], names)
		if err != nil {
			return doc, fmt.Errorf("%s: %w", group, err)
		}
		for _, name := range keysOf(entries) {
			value := entries[name]
			if group == "hooks" && slices.Contains(retiredHooks, name) {
				continue
			}
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
				if err := checkLevel(name, s); err != nil {
					return doc, err
				}
				doc.Hooks[name] = s
			}
		}
	}
	return doc, nil
}
