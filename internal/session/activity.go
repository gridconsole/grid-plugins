package session

import (
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
	SessionID  string                       `json:"session_id"`
	Last       time.Time                    `json:"last"`
	Transcript string                       `json:"transcript,omitempty"`
	Usage      map[string]Usage             `json:"usage,omitempty"`
	Seen       map[string]bool              `json:"seen,omitempty"`
	Snapshots  map[string]map[string]string `json:"snapshots,omitempty"`
	ExitReason string                       `json:"exit_reason,omitempty"`
}

// Activity is hook evidence, separate from the supervisor's process record.
// No hook can invent a pid, machine identity, fence or Running state.
func (s Store) Activity(id string) (Activity, error) {
	if !component.MatchString(id) {
		return Activity{}, errors.New("invalid session id")
	}
	dir, err := s.storageDir(false, "sessions", "hooks")
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
		if err = change(&a); err != nil {
			return err
		}
		a.Last = s.now()
		dir, err := s.storageDir(true, "sessions", "hooks")
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
