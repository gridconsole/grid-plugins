// Package job defines planned Jobs and their deterministic start briefs.
package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Job contains the planned fields and the run fields of job-contract.md section 1.
// Status and FollowUp retain compatibility with the old brief reader.
type Job struct {
	Kind         string `json:"kind,omitempty"`
	Visible      string `json:"visible,omitempty"`
	Irreversible bool   `json:"irreversible,omitempty"`
	ReviewReason string `json:"review_reason,omitempty"`
	// Repo names the project repo a code Job works in, by its path in project.md;
	// empty is the project's first repo.
	Repo   string   `json:"repo,omitempty"`
	ID     string   `json:"id"`
	Item   int      `json:"item"`
	Title  string   `json:"title"`
	Role   string   `json:"role"`
	Skills []string `json:"skills"`
	// Review is a retired level; old plans and records may still carry it. It is
	// read and ignored: nothing validates it and new Jobs do not set it.
	Review         string       `json:"review,omitempty"`
	Risk           []string     `json:"risk"`
	Intent         Intent       `json:"intent"`
	Links          []Link       `json:"links"`
	Keep           []string     `json:"keep"`
	Decisions      []string     `json:"decisions"`
	Read           []string     `json:"read"`
	Write          []string     `json:"write"`
	OutOfScope     []string     `json:"out_of_scope"`
	Acceptance     []Acceptance `json:"acceptance"`
	Verify         Verify       `json:"verify"`
	Budget         Budget       `json:"budget"`
	OnBlocked      string       `json:"on_blocked"`
	After          []Dependency `json:"after"`
	Attempt        int          `json:"attempt"`
	Continues      *string      `json:"continues"`
	DiscoveredFrom *string      `json:"discovered_from"`
	CloseReason    *string      `json:"close_reason"`
	Checkpoint     *Checkpoint  `json:"checkpoint"`
	ReviewRounds   int          `json:"review_rounds"`
	LevelRaised    *LevelRaised `json:"level_raised"`
	Worktree       Worktree     `json:"worktree"`
	PostsBack      []string     `json:"posts_back"`
	Order          *int         `json:"order,omitempty"`
	Status         string       `json:"status,omitempty"`
	FollowUp       string       `json:"follow_up,omitempty"`
}

type Intent struct {
	Owner string `json:"owner"`
	Item  string `json:"item"`
	Goal  string `json:"goal"`
}
type Link struct {
	To  string `json:"to"`
	Rel string `json:"rel"`
	Why string `json:"why"`
}
type Acceptance struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Traces string `json:"traces"`
	State  string `json:"state,omitempty"`
}
type Verify struct {
	Command  string   `json:"command,omitempty"`
	MinTests int      `json:"min_tests"`
	Kind     string   `json:"kind,omitempty"`
	Path     string   `json:"path,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Job      string   `json:"job,omitempty"`
}
type Budget struct {
	TurnsEstimate        int `json:"turns_estimate"`
	ReadsBeforeFirstEdit int `json:"reads_before_first_edit"`
}
type Checkpoint struct {
	Step   string `json:"step"`
	Commit string `json:"commit"`
}
type LevelRaised struct {
	From string `json:"from"`
	To   string `json:"to"`
	Why  string `json:"why"`
}

// Worktree accepts a single worktree or a map keyed by repository.
type Worktree map[string]string

func (w *Worktree) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*w = nil
		return nil
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		*w = Worktree{"": s}
		return nil
	}
	type plain Worktree
	return json.Unmarshal(b, (*plain)(w))
}

type Dependency struct {
	Job  string `json:"job"`
	Kind string `json:"kind,omitempty"`
	Why  string `json:"why,omitempty"`
}

// MarshalJSON preserves the bare-id shorthand when there is no explanation.
func (e Dependency) MarshalJSON() ([]byte, error) {
	if e.Kind == "" && e.Why == "" {
		return json.Marshal(e.Job)
	}
	type plain Dependency
	return json.Marshal(plain(e))
}

func (e *Dependency) UnmarshalJSON(data []byte) error {
	var id string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &id); err != nil {
			return err
		}
		*e = Dependency{Job: id}
		return nil
	}
	type plain Dependency
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	// Object edges must explain the dependency; bare ids are the shorthand.
	if err := ValidDependencyWhy(value.Why, value.Job); err != nil {
		return err
	}
	*e = Dependency(value)
	return nil
}

func ValidDependencyWhy(why, target string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(why))
	s := strings.ToLower(strings.TrimSpace(why))
	if n < 8 || n > 160 || s == "see also" || s == "related" || s == strings.ToLower(target) {
		return fmt.Errorf("after %s: why must explain the link in 8 to 160 characters", target)
	}
	return nil
}

// Defaults supplies run metadata, never missing planned instructions.
func (j *Job) Defaults() {
	if j.Status == "" {
		j.Status = "briefed"
	}
	if j.FollowUp == "" {
		j.FollowUp = "none"
	}
	if j.Attempt == 0 {
		j.Attempt = 1
	}
}

// RolePerson is the role of a Job that a person answers; no worker is assigned.
const RolePerson = "Person"

// IsPerson reports whether the Job is a person's step.
func (j Job) IsPerson() bool { return strings.EqualFold(strings.TrimSpace(j.Role), RolePerson) }

// PersonName is the person the step waits for. It is kept in intent.owner.
func (j Job) PersonName() string { return j.Intent.Owner }

// PlannedFields are present even when their value is an empty list.
var PlannedFields = []string{"id", "item", "title", "role", "skills", "risk", "intent", "links", "keep", "decisions", "read", "write", "out_of_scope", "acceptance", "verify", "budget", "on_blocked", "after"}

// RetiredFields were planned fields once and the Job no longer has them. A plan
// or record that still carries one reads without error and the value is ignored.
// (review is a retired optional field: a Job may still carry it, and nothing
// reads it.)
var RetiredFields = []string{"max_rounds"}
