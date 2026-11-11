package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"
)

// Usage contains observed totals only, never prompts, results or guessed prices.
// A tool id replaces its previous observation, making retries idempotent.
type Usage struct {
	Input        float64 `json:"input_tokens,omitempty"`
	Output       float64 `json:"output_tokens,omitempty"`
	CacheRead    float64 `json:"cache_read_input_tokens,omitempty"`
	CacheWrite   float64 `json:"cache_creation_input_tokens,omitempty"`
	Total        float64 `json:"total_tokens,omitempty"`
	Milliseconds float64 `json:"duration_ms,omitempty"`
}

type Activity struct {
	VerifyBlocks map[string]int               `json:"verify_blocks,omitempty"`
	VerifyFailed map[string]bool              `json:"verify_failed,omitempty"`
	Transcripts  map[string]string            `json:"transcripts,omitempty"`
	Exits        map[string]string            `json:"exits,omitempty"`
	PlannerTask  string                       `json:"planner_task,omitempty"`
	SessionID    string                       `json:"session_id"`
	Last         time.Time                    `json:"last"`
	Transcript   string                       `json:"transcript,omitempty"`
	Usage        map[string]Usage             `json:"usage,omitempty"`
	Seen         map[string]bool              `json:"seen,omitempty"`
	Snapshots    map[string]map[string]string `json:"snapshots,omitempty"`
	ExitReason   string                       `json:"exit_reason,omitempty"`
	// ExitCodes and ExitCode keep the raw end reason next to the mapped one,
	// under the same key as Exits, so an unknown end is never read as a clean one.
	ExitCodes map[string]string `json:"exit_codes,omitempty"`
	ExitCode  string            `json:"exit_code,omitempty"`
}

// Activity is hook evidence, separate from the supervisor's process record.
// No hook can invent a pid, machine identity, fence or Running state.
func (s Store) Activity(id string) (Activity, error) {
	if !component.MatchString(id) {
		return Activity{}, errors.New("invalid session id")
	}
	dir, err := s.storageDir(false, "sessions", ".hooks")
	if errors.Is(err, os.ErrNotExist) {
		return Activity{SessionID: id}, nil
	}
	if err != nil {
		return Activity{}, err
	}
	b, err := readFile(filepath.Join(dir, id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return Activity{SessionID: id}, nil
	}
	if err != nil {
		return Activity{}, err
	}
	var a Activity
	err = json.Unmarshal(b, &a)
	if err == nil && a.SessionID != id {
		err = errors.New("activity identity mismatch")
	}
	return a, err
}

func (s Store) UpdateActivity(id string, change func(*Activity) error) error {
	if !component.MatchString(id) {
		return errors.New("invalid session id")
	}
	return s.locked(func() error {
		a, err := s.Activity(id)
		if err != nil {
			return err
		}
		if a.Usage == nil {
			a.Usage = map[string]Usage{}
		}
		if a.Seen == nil {
			a.Seen = map[string]bool{}
		}
		if a.Snapshots == nil {
			a.Snapshots = map[string]map[string]string{}
		}
		before, _ := json.Marshal(a)
		if err = change(&a); err != nil {
			return err
		}
		after, _ := json.Marshal(a)
		if !a.Last.IsZero() && bytes.Equal(before, after) {
			return nil
		}
		a.Last = s.now()
		dir, err := s.storageDir(true, "sessions", ".hooks")
		if err != nil {
			return err
		}
		return atomicJSON(filepath.Join(dir, id+".json"), a)
	})
}

func (s Store) RecordUsage(id, tool string, u Usage) error {
	for _, n := range []float64{u.Input, u.Output, u.CacheRead, u.CacheWrite, u.Total, u.Milliseconds} {
		if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			return errors.New("invalid usage number")
		}
	}
	if tool == "" {
		return errors.New("usage needs a tool use id")
	}
	return s.UpdateActivity(id, func(a *Activity) error { a.Usage[tool] = u; return nil })
}

// FoldUsage sums recorded tool observations, never transcript text or Feed posts.
func FoldUsage(records []Activity) Usage {
	var total Usage
	for _, a := range records {
		for _, u := range a.Usage {
			total.Input += u.Input
			total.Output += u.Output
			total.CacheRead += u.CacheRead
			total.CacheWrite += u.CacheWrite
			total.Total += u.Total
			total.Milliseconds += u.Milliseconds
		}
	}
	return total
}

// Activities reads the observations used by usage totals, without creating files.
func (s Store) Activities() ([]Activity, error) {
	dir, err := s.storageDir(false, "sessions", ".hooks")
	if errors.Is(err, os.ErrNotExist) {
		return []Activity{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := []Activity{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		a, err := s.Activity(e.Name()[:len(e.Name())-5])
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}
