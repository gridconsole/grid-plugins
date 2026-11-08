package mcp

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Items       *Property `json:"items,omitempty"`
	Minimum     *int      `json:"minimum,omitempty"`
}
type Schema struct {
	Type                 string              `json:"type"`
	Properties           map[string]Property `json:"properties"`
	Required             []string            `json:"required"`
	AdditionalProperties bool                `json:"additionalProperties"`
}
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema Schema `json:"inputSchema"`
}

// Tools returns independent schemas, excluding writer and lease authority.
func Tools() []Tool {
	var out []Tool
	for _, spec := range []struct {
		name, description string
		fields, required  []string
	}{
		{"feed_post", "Append a typed Feed post. feed_types describes body fields. Identity and lease come from the session.", []string{"type", "body", "id", "re", "job", "attempt", "after"}, []string{"type", "body"}},
		{"feed_read", "Read all writer streams with optional display filters; guest and external bodies are fenced data.", []string{"since", "type", "job"}, nil},
		{"feed_state", "Fold the Feed and save state.json and needs-you.json; return the same state with untrusted bodies fenced.", nil, nil},
		{"feed_types", "List the Feed post types with their contract schemas, versions, classes and authoring roles; no task is needed.", nil, nil},
		{"feed_handover", "Announce an already granted task lease. This does not transfer or take a lease.", []string{"body", "id", "re", "job", "attempt", "after", "reason"}, nil},
	} {
		props := map[string]Property{
			"task":    {Type: "string", Description: "Task folder or display id. The session's GRID_FEED_TASK wins and a different value is refused; without it, the current folder."},
			"project": {Type: "string", Description: "Project name or id for task resolution."},
		}
		for _, field := range spec.fields {
			props[field] = Property{Type: "string"}
		}
		if _, ok := props["job"]; ok && spec.name != "feed_read" {
			props["job"] = Property{Type: "string", Description: "Job id; the session's GRID_JOB wins and a different value is refused."}
		}
		if _, ok := props["body"]; ok {
			props["body"] = Property{Type: "object", Description: "Fields of the post type; see feed_types."}
		}
		if _, ok := props["attempt"]; ok {
			min := 1
			props["attempt"] = Property{Type: "integer", Description: "Attempt number; the session's GRID_ATTEMPT wins and a different value is refused.", Minimum: &min}
			props["after"] = Property{Type: "array", Items: &Property{Type: "string"}}
		}
		required := append([]string{}, spec.required...)
		out = append(out, Tool{spec.name, spec.description, Schema{"object", props, required, false}})
	}
	return out
}

func (s Schema) validate(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return fmt.Errorf("arguments must be an object")
	}
	for _, key := range s.Required {
		if _, ok := fields[key]; !ok {
			return fmt.Errorf("%s is required", key)
		}
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		p, ok := s.Properties[key]
		if !ok {
			return fmt.Errorf("unknown argument %q", key)
		}
		raw := fields[key]
		valid := string(raw) != "null"
		switch p.Type {
		case "string":
			var value string
			valid = valid && json.Unmarshal(raw, &value) == nil
		case "object":
			var value map[string]any
			valid = valid && json.Unmarshal(raw, &value) == nil
		case "integer":
			var value int
			valid = valid && json.Unmarshal(raw, &value) == nil && (p.Minimum == nil || value >= *p.Minimum)
		case "array":
			var values []json.RawMessage
			valid = valid && json.Unmarshal(raw, &values) == nil
			for _, item := range values {
				var value string
				valid = valid && string(item) != "null" && json.Unmarshal(item, &value) == nil
			}
		}
		if !valid {
			return fmt.Errorf("%s must be %s with the advertised constraints", key, p.Type)
		}
	}
	return nil
}

type arguments struct {
	Task    string         `json:"task"`
	Project string         `json:"project"`
	Type    string         `json:"type"`
	Body    map[string]any `json:"body"`
	ID      string         `json:"id"`
	Re      string         `json:"re"`
	Job     string         `json:"job"`
	Attempt int            `json:"attempt"`
	After   []string       `json:"after"`
	Since   string         `json:"since"`
	Reason  string         `json:"reason"`
}
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type Result struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

func toolError(err error) Result {
	// Quote errors too: validation may echo attacker-controlled field names.
	raw, _ := json.Marshal(map[string]string{"error": err.Error()})
	return Result{Content: []Content{{"text", string(raw)}}, IsError: true}
}

func (s Server) call(tool Tool, raw json.RawMessage) (Result, error) {
	if err := tool.InputSchema.validate(raw); err != nil {
		return Result{}, err
	}
	var a arguments
	if err := json.Unmarshal(raw, &a); err != nil {
		return Result{}, err
	}
	if tool.Name == "feed_types" {
		types, err := postTypes()
		if err != nil {
			return Result{}, err
		}
		return render(map[string]any{"types": types}, nil)
	}
	if s.Resolve == nil {
		return Result{}, fmt.Errorf("Feed resolver is unavailable")
	}
	store, err := s.Resolve(a.Task, a.Project)
	if err != nil {
		return Result{}, err
	}
	switch tool.Name {
	case "feed_read":
		read, err := store.Read(feed.Filter{Since: a.Since, Type: a.Type, Job: a.Job})
		if err != nil {
			return Result{}, err
		}
		return render(read, records(read.Records))
	case "feed_state":
		state, err := store.SaveState()
		if err != nil {
			return Result{}, err
		}
		return render(state, nil)
	default:
		if s.Session == nil {
			return Result{}, fmt.Errorf("Feed session is unavailable")
		}
		if a.Body == nil {
			a.Body = map[string]any{}
		}
		if a.Reason != "" {
			a.Body["reason"] = a.Reason
		}
		in, writer, err := s.Session(feed.Input{ID: a.ID, Type: a.Type, Body: a.Body, Re: a.Re, Job: a.Job, Attempt: a.Attempt, After: a.After})
		if err != nil {
			return Result{}, err
		}
		var post feed.Record
		if tool.Name == "feed_handover" {
			post, err = store.Handover(in, writer)
		} else {
			post, err = store.Post(in, writer)
		}
		if err != nil {
			return Result{}, err
		}
		return render(map[string]any{"post": post, "tier": post.Tier, "data": post.Data()}, []feed.Record{post})
	}
}

// postType adds the canonical contract schema, which holds the body fields.
type postType struct {
	contract.PostType
	Schema json.RawMessage `json:"schema"`
}

func postTypes() ([]postType, error) {
	var out []postType
	for _, t := range contract.PostTypes() {
		schema, err := contract.Schema(t.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, postType{t, schema})
	}
	return out, nil
}

func records(entries []feed.Entry) []feed.Record {
	out := make([]feed.Record, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Record)
	}
	return out
}
