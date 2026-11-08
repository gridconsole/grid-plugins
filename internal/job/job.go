// Package job defines planned Jobs and their deterministic start briefs.
package job

import (
	"encoding/json"
	"fmt"
)

// Job contains the planned fields and the run fields of job-contract.md section 1.
// Status and FollowUp retain compatibility with the old brief reader.
type Job struct {
	ID             string       `json:"id"`
	Item           int          `json:"item"`
	Title          string       `json:"title"`
	Role           string       `json:"role"`
	Skills         []string     `json:"skills"`
	Review         string       `json:"review"`
	Risk           []string     `json:"risk"`
	MaxRounds      int          `json:"max_rounds"`
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
	Status         string       `json:"status"`
	FollowUp       string       `json:"follow_up"`
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
	Command  string `json:"command,omitempty"`
	MinTests int    `json:"min_tests"`
	Kind     string `json:"kind,omitempty"`
	Path     string `json:"path,omitempty"`
	Job      string `json:"job,omitempty"`
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
	Kind string `json:"kind"`
	Why  string `json:"why,omitempty"`
}

func (d *Dependency) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		d.Job, d.Kind = s, "done"
		return nil
	}
	type plain Dependency
	if err := json.Unmarshal(b, (*plain)(d)); err != nil {
		return fmt.Errorf("after: %w", err)
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
