package mcp

import (
	"bytes"
	"encoding/json"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// render keeps untrusted bodies out of the JSON metadata text. Each is replaced
// by a data_ref, then emitted once in the Feed's fixed warning/fence. This also
// covers repeated bodies in state.questions, state.jobs, head and checkpoints.
// The Feed record and persisted projection are never modified.
func render(value any, records []feed.Record) (Result, error) {
	data := map[string]feed.Record{}
	for _, r := range records {
		if r.Data() {
			data[r.ID] = r
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return Result{}, err
	}
	var copy any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&copy); err != nil {
		return Result{}, err
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			id, _ := x["id"].(string)
			if post, ok := x["post"].(string); ok {
				id = post
			}
			if _, ok := data[id]; ok {
				if _, hasBody := x["body"]; hasBody {
					x["body"] = map[string]string{"data_ref": id}
				}
			}
			for key, child := range x {
				// A body is opaque data, never a nested Feed envelope.
				if key != "body" {
					walk(child)
				}
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(copy)
	raw, err = json.Marshal(copy)
	if err != nil {
		return Result{}, err
	}
	result := Result{Content: []Content{{"text", string(raw)}}}
	seen := map[string]bool{}
	for _, r := range records {
		if r.Data() && !seen[r.ID] {
			result.Content = append(result.Content, Content{"text", feed.MarshalData(r)})
			seen[r.ID] = true
		}
	}
	return result, nil
}
