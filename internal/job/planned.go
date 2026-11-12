package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// DecodePlanned checks presence after plan derivation and rejects run fields.
// Kind belongs to the step; the remaining floor inputs belong to each Job.
func DecodePlanned(raw map[string]any, kind string) (Job, []string) {
	problems := []string{}
	allowed := map[string]bool{"visible": true, "irreversible": true, "review_reason": true, "review": true, "repo": true}
	for _, key := range RetiredFields {
		allowed[key] = true
	}
	for _, key := range PlannedFields {
		allowed[key] = true
		if v, ok := raw[key]; !ok || v == nil {
			problems = append(problems, "missing required field "+key)
		}
	}
	keys := []string{}
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !allowed[key] {
			problems = append(problems, fmt.Sprintf("unknown planned field %q", key))
		}
	}
	retired := map[string]any{}
	for key, v := range raw {
		retired[key] = v
	}
	for _, key := range RetiredFields {
		delete(retired, key)
	}
	data, err := json.Marshal(retired)
	var j Job
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&j)
	}
	if err != nil {
		problems = append(problems, err.Error())
	}
	j.Kind = kind
	if intent, ok := raw["intent"].(map[string]any); ok {
		for _, key := range []string{"owner", "item", "goal"} {
			if v, ok := intent[key].(string); !ok || v == "" {
				problems = append(problems, "missing required field intent."+key)
			}
		}
	}
	return j, problems
}
