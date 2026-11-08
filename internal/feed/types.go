package feed

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed types/*.json
var typeFiles embed.FS

// Type describes the version, durability, lease and author rules of a body.
type Type struct {
	Name    string         `json:"type"`
	Version int            `json:"version"`
	Class   string         `json:"class"`
	Lease   *string        `json:"lease"`
	Roles   []string       `json:"roles"`
	Schema  map[string]any `json:"schema"`
}

// Types returns independent copies of the built-in registry, sorted by name.
func loadTypes() []Type {
	files, err := typeFiles.ReadDir("types")
	if err != nil {
		panic(err)
	}
	result := make([]Type, 0, len(files))
	for _, f := range files {
		raw, err := typeFiles.ReadFile("types/" + f.Name())
		if err != nil {
			panic(err)
		}
		var t Type
		if err := json.Unmarshal(raw, &t); err != nil {
			panic(err)
		}
		result = append(result, t)
	}
	return result
}

var registry = loadTypes()

func Types() []Type {
	data, _ := json.Marshal(registry)
	var copy []Type
	_ = json.Unmarshal(data, &copy)
	return copy
}

func lookup(name string) (Type, bool) {
	for _, t := range registry {
		if t.Name == name {
			return t, true
		}
	}
	return Type{}, false
}

func validateSchema(body map[string]any, schema map[string]any) error {
	required, _ := schema["required"].([]any)
	for _, key := range required {
		if v, ok := body[str(key)]; !ok || v == nil {
			return fmt.Errorf("body.%s is required", key)
		}
	}
	for name, spec := range object(schema["properties"]) {
		v, exists := body[name]
		if !exists {
			continue
		}
		s := object(spec)
		kinds, ok := s["type"].([]any)
		if !ok {
			kinds = []any{s["type"]}
		}
		valid := false
		for _, kind := range kinds {
			switch kind {
			case "null":
				valid = valid || v == nil
			case "string":
				_, ok = v.(string)
				valid = valid || ok
			case "boolean":
				_, ok = v.(bool)
				valid = valid || ok
			case "object":
				_, ok = v.(map[string]any)
				valid = valid || ok
			case "array":
				_, ok = v.([]any)
				valid = valid || ok
			case "number":
				_, ok = v.(float64)
				valid = valid || ok
			}
		}
		if !valid {
			return fmt.Errorf("body.%s has the wrong type", name)
		}
		if number(s["minLength"]) > 0 && strings.TrimSpace(str(v)) == "" {
			return fmt.Errorf("body.%s must not be empty", name)
		}
		if choices, ok := s["enum"].([]any); ok && !slices.Contains(choices, v) {
			return fmt.Errorf("body.%s has an unknown value", name)
		}
	}
	return nil
}

var postRef = regexp.MustCompile(`^(task|item:i-[0-9]+|question:Q-[0-9]+)$`)
var acceptanceRow = regexp.MustCompile(`^(A[0-9]+|write|verify|classify)$`)

// Validate checks a normalized record. Caller identity is separate from body.
// The store additionally checks the lease and a guest answer's addressee.
func Validate(r Record) error {
	t, ok := lookup(r.Type)
	if !ok {
		return fmt.Errorf("unregistered post type %q", r.Type)
	}
	if r.TypeV != t.Version {
		return fmt.Errorf("unsupported %s type_v %d", r.Type, r.TypeV)
	}
	if !slices.Contains(t.Roles, r.Author.Role) {
		return fmt.Errorf("role %q may not author %s", r.Author.Role, r.Type)
	}
	if !IsID(r.ID) || !IsID(r.Task) {
		return fmt.Errorf("record and task ids must be ULIDs")
	}
	if !validWorker(r.Worker) || !memberPattern.MatchString(r.Author.Member) {
		return fmt.Errorf("invalid author or worker identity")
	}
	if !strings.HasPrefix(r.Worker, r.Author.Member+"@") {
		return fmt.Errorf("author and worker disagree")
	}
	if r.Gen < 0 || r.Fence < 0 {
		return fmt.Errorf("gen and fence must be nonnegative")
	}
	if (r.Fence == 0) != (r.Lease == nil) {
		return fmt.Errorf("lease and fence must both be present or both absent")
	}
	if r.Lease != nil && *r.Lease != "task" && !jobPattern.MatchString(*r.Lease) {
		return fmt.Errorf("invalid lease")
	}
	if r.Job != nil && !jobPattern.MatchString(*r.Job) {
		return fmt.Errorf("invalid job id")
	}
	if r.Attempt != nil && *r.Attempt < 1 {
		return fmt.Errorf("attempt must be positive")
	}
	if r.Re != nil && !IsID(*r.Re) {
		return fmt.Errorf("re must be a record ULID")
	}
	for _, id := range r.After {
		if !IsID(id) {
			return fmt.Errorf("after must contain record ULIDs")
		}
	}
	if r.Wake != nil {
		parts := strings.Split(*r.Wake, "/")
		if len(parts) != 2 || !IsID(parts[0]) || !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(parts[1]) {
			return fmt.Errorf("wake must be <event ULID>/<n>")
		}
	}
	if t.Lease != nil {
		if r.Lease == nil {
			return fmt.Errorf("%s requires a lease", r.Type)
		}
		if *t.Lease == "task" && *r.Lease != "task" {
			return fmt.Errorf("%s requires the task lease", r.Type)
		}
		if *t.Lease == "job" && (r.Job == nil || *r.Lease != *r.Job || r.Attempt == nil) {
			return fmt.Errorf("%s requires its Job lease and attempt", r.Type)
		}
	}
	if r.Body == nil {
		return fmt.Errorf("body must be an object")
	}
	if err := validateSchema(r.Body, t.Schema); err != nil {
		return err
	}
	for _, retired := range []string{"epoch"} {
		if _, ok := r.Body[retired]; ok {
			return fmt.Errorf("body.%s is retired", retired)
		}
	}
	if r.Data() {
		if _, ok := r.Body["proposed_jobs"]; ok {
			return fmt.Errorf("untrusted data cannot propose Jobs")
		}
		if r.Type == "knowledge" && r.Body["external"] != true {
			return fmt.Errorf("untrusted knowledge must be marked external")
		}
	}
	switch r.Type {
	case "job":
		if r.Job == nil {
			return fmt.Errorf("job requires an explicit Job id")
		}
		for _, key := range []string{"priority", "defer_until"} {
			if _, ok := r.Body[key]; ok {
				return fmt.Errorf("%s is not part of the v1 Job schema", key)
			}
		}
		if id := str(r.Body["id"]); id != "" && id != value(r.Job) {
			return fmt.Errorf("Job body id disagrees with envelope")
		}
		if edges, ok := r.Body["after"].([]any); ok {
			for _, edge := range edges {
				if id, ok := edge.(string); ok {
					if !jobPattern.MatchString(id) && !regexp.MustCompile(`^gate:Q-[0-9]+$`).MatchString(id) {
						return fmt.Errorf("invalid Job dependency")
					}
					continue
				}
				m := object(edge)
				why := str(m["why"])
				if !jobPattern.MatchString(str(m["job"])) || !slices.Contains([]string{"done", "failed", "children-done"}, str(m["kind"])) || utf8.RuneCountInString(why) < 8 || utf8.RuneCountInString(why) > 160 {
					return fmt.Errorf("Job dependency requires job, kind and why (8 to 160 characters)")
				}
			}
		}

	case "post":
		if !postRef.MatchString(str(r.Body["ref"])) {
			return fmt.Errorf("invalid post ref")
		}
		if str(r.Body["kind"]) == "message" && strings.TrimSpace(str(r.Body["text"])) == "" {
			return fmt.Errorf("message needs text")
		}
		if utf8.RuneCountInString(str(r.Body["text"])) > 4000 {
			return fmt.Errorf("post text exceeds 4000 characters")
		}
		if r.Data() && str(r.Body["kind"]) != "message" {
			return fmt.Errorf("untrusted posts may only add feedback messages")
		}
		if a, ok := r.Body["attachments"].([]any); ok {
			for _, v := range a {
				p := str(v)
				if p == "" || strings.Contains(p, "\\") || path.Clean(p) != p || !strings.HasPrefix(p, "attachments/posts/"+r.ID+"/") {
					return fmt.Errorf("invalid post attachment")
				}
			}
		}
	case "answer":
		if r.Re == nil {
			return fmt.Errorf("answer requires re")
		}
	case "ack":
		if len(r.After) == 0 {
			return fmt.Errorf("ack requires the assign id in after")
		}
	case "checkpoint":
		if str(r.Body["job"]) != value(r.Job) {
			return fmt.Errorf("checkpoint job disagrees with envelope")
		}
	case "control":
		a := object(r.Body["assign"])
		if a != nil {
			if !jobPattern.MatchString(str(a["job"])) || !validWorker(str(a["to"])) || !positiveInteger(a["attempt"]) || !positiveInteger(a["fence"]) {
				return fmt.Errorf("assign needs job, to, positive attempt and fence")
			}
			if _, err := time.Parse(time.RFC3339Nano, str(a["until"])); err != nil {
				return fmt.Errorf("assign needs an RFC3339 until")
			}
		} else if str(r.Body["target"]) == "" {
			return fmt.Errorf("control requires target or assign")
		}
		if status := str(r.Body["status"]); status != "" {
			if !slices.Contains([]string{"posted", "ready", "assigned", "running", "submitted", "integrated", "done", "accepted", "stale", "parked", "rejected", "conflict", "cancelled", "closed", "answered"}, status) {
				return fmt.Errorf("invalid control status")
			}
			if (status == "cancelled" || status == "rejected") && str(r.Body["close_reason"]) == "" {
				return fmt.Errorf("closing a Job requires close_reason")
			}
		}
	case "result":
		if _, ok := r.Body["next"]; ok {
			return fmt.Errorf("result.next is replaced by proposed_jobs")
		}
		if v := object(r.Body["verified"]); v != nil {
			if !slices.Contains([]string{"L1", "L2", "L3"}, str(v["level"])) || !slices.Contains([]string{"sonnet", "grid-reviewer"}, str(v["by"])) {
				return fmt.Errorf("invalid verified level or by")
			}
			if _, ok := v["pass"].(bool); !ok {
				return fmt.Errorf("verified.pass must be boolean")
			}
			if _, ok := v["evidence"].([]any); !ok {
				return fmt.Errorf("verified.evidence must be an array")
			}
			if str(v["level"]) == "L1" && !jobPattern.MatchString(str(v["job"])) {
				return fmt.Errorf("L1 verification requires its validating Job")
			}
		}
		if proposals, ok := r.Body["proposed_jobs"].([]any); ok {
			for _, proposal := range proposals {
				m := object(proposal)
				if strings.TrimSpace(str(m["title"])) == "" || strings.TrimSpace(str(m["role"])) == "" || strings.TrimSpace(str(m["why"])) == "" || !jobPattern.MatchString(str(m["discovered_from"])) {
					return fmt.Errorf("proposed Job requires title, role, why and discovered_from")
				}
			}
		}
		if found, ok := r.Body["found"].([]any); ok {
			for _, id := range found {
				if !IsID(str(id)) {
					return fmt.Errorf("result.found must contain record ULIDs")
				}
			}
		}
		if rows, ok := r.Body["must_fix"].([]any); ok {
			for _, row := range rows {
				m := object(row)
				if !acceptanceRow.MatchString(str(m["row"])) || str(m["at"]) == "" {
					return fmt.Errorf("must_fix needs an acceptance row and at; post other findings as found")
				}
			}
		}
	case "usage-notice":
		_, d := r.Body["dollars"]
		_, m := r.Body["minutes"]
		if d == m {
			return fmt.Errorf("usage-notice requires exactly one of dollars or minutes")
		}
		if str(r.Body["task"]) != r.Task {
			return fmt.Errorf("usage-notice task disagrees with envelope")
		}
	case "loop-detected":
		quoted := 0
		for _, key := range []string{"calls", "errors"} {
			if list, ok := r.Body[key].([]any); ok {
				quoted += len(list)
			}
		}
		if quoted > 5 {
			return fmt.Errorf("loop-detected may quote at most five calls or errors")
		}
	}
	return nil
}
func positiveInteger(v any) bool { n, ok := v.(float64); return ok && n >= 1 && n == math.Trunc(n) }
