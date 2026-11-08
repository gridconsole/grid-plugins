package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/question"
)

func (s Store) progress(snap Snapshot, j *contract.Job) string {
	h := sha256.New()
	// Only work products count. Mechanical projections, leases and spawn files
	// must not reset a crash loop merely because this pass wrote them.
	_ = filepath.WalkDir(s.Feed.TaskDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "feed", "spawned", "questions", ".git", "brain-proposals":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			b, err := os.ReadFile(path)
			if err == nil {
				rel, _ := filepath.Rel(s.Feed.TaskDir, path)
				fmt.Fprintf(h, "%s\x00%x\n", rel, sha256.Sum256(b))
			}
		}
		return nil
	})
	evidence := []string{}
	for id, r := range snap.Records {
		if r.Type == "post" || r.Type == "checkpoint" && r.Job != nil && *r.Job == j.ID || r.Type == "answer" && r.Author.ID == s.Owner {
			evidence = append(evidence, id)
		}
	}
	sort.Strings(evidence)
	fmt.Fprint(h, evidence)
	return hex.EncodeToString(h.Sum(nil))
}
func (s Store) park(snap Snapshot, j *contract.Job, m *jobMemory) error {
	owner := s.Owner
	if owner == "" {
		return fmt.Errorf("crash loop for %s: task owner is required (--owner or GRID_OWNER)", j.ID)
	}
	if s.Incident != nil {
		if err := s.Incident(j.ID, m.Failures); err != nil {
			return err
		}
	}
	gate := 1
	for _, r := range snap.Records {
		g, _ := r.Body["gate"].(map[string]any)
		if n, err := strconv.Atoi(strings.TrimPrefix(stringValue(g["id"]), "Q-")); err == nil {
			gate = max(gate, n+1)
		}
	}
	_, err := s.Feed.Post(feed.Input{Type: "question", Job: j.ID, Wake: m.Failure + "/1", Body: map[string]any{
		"question": "Three attempts made no progress. How should " + j.ID + " continue?",
		"who":      owner, "blocking": true, "options": []string{"Keep parked", "Retry"}, "why": strings.Join(m.Failures, ", "),
		"gate": map[string]any{"id": fmt.Sprintf("Q-%d", gate), "kind": "person", "default": "Keep parked", "at": s.now().Add(24 * time.Hour).Format(question.FeedTime)},
	}}, s.Writer)
	if err != nil {
		return err
	}
	return s.control(j, contract.Parked, "needs_person", m.Failure)
}

// Proposals cannot widen a plan: only an existing acceptance row on the same
// item and role is mechanical rework. Everything else remains a lead decision.
func (s Store) proposals(snap Snapshot) error {
	for _, id := range ids(snap) {
		j := snap.State.Jobs[id]
		if len(j.ProposedJobs) == 0 {
			continue
		}
		items, err := (plan.Store{TaskDir: s.Feed.TaskDir}).Items()
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		parent := snap.Bodies[id]
		raw, _ := json.Marshal(parent["item"])
		var item int
		_ = json.Unmarshal(raw, &item)
		inPlan := false
		for _, p := range items {
			if p.ID == fmt.Sprintf("i-%d", item) {
				inPlan = true
			}
		}
		if !inPlan || len(j.DecisionsNeeded) > 0 {
			continue
		}
		for index, p := range j.ProposedJobs {
			child := fmt.Sprintf("%s.%d", id, index+1)
			if snap.State.Jobs[child] != nil {
				continue
			}
			if !fits(parent, j, p) {
				continue
			}
			body := map[string]any{}
			b, _ := json.Marshal(parent)
			if err := json.Unmarshal(b, &body); err != nil {
				return err
			}
			body["id"], body["title"], body["role"] = child, p.Title, p.Role
			body["discovered_from"], body["continues"] = id, id
			body["attempt"], body["review_rounds"], body["checkpoint"], body["close_reason"] = 1, 0, nil, nil
			event := j.Results[len(j.Results)-1]
			if _, err := s.Feed.Post(feed.Input{Type: "job", Job: child, Re: event, Wake: event + "/1", Body: body}, s.Writer); err != nil {
				return err
			}
		}
	}
	return nil
}
func fits(parent map[string]any, j *contract.Job, p contract.ProposedJob) bool {
	if p.DiscoveredFrom != j.ID || p.Role != j.Role {
		return false
	}
	b, _ := json.Marshal(parent["acceptance"])
	var rows []struct {
		ID    string
		State string
	}
	_ = json.Unmarshal(b, &rows)
	for _, r := range rows {
		if p.Gap == r.ID && r.State != "dropped" {
			return true
		}
	}
	return false
}
func (s Store) wake(snap Snapshot, m *memory) *Wake {
	candidates := map[string]string{}
	for _, id := range ids(snap) {
		j := snap.State.Jobs[id]
		for _, r := range j.Results {
			candidates[r] = id
		}
		if j.Status == contract.Rejected {
			for _, r := range j.History {
				if snap.Records[r].Body["status"] == "rejected" {
					candidates[r] = id
				}
			}
		}
	}
	for _, event := range snap.State.Wakes {
		r := snap.Records[event]
		job := stringValue(r.Body["job"])
		if r.Job != nil {
			job = *r.Job
		}
		candidates[event] = job
	}
	keys := []string{}
	for event := range candidates {
		keys = append(keys, event)
	}
	sort.Strings(keys)
	for _, event := range keys {
		if !m.Delivered[event] {
			m.Pending = append(m.Pending, Wake{Job: candidates[event], Event: event, N: 1})
			m.Delivered[event] = true
		}
	}
	times := m.Wakes[:0]
	for _, at := range m.Wakes {
		if s.now().Sub(at) < time.Hour {
			times = append(times, at)
		}
	}
	m.Wakes = times
	if len(m.Pending) == 0 {
		return nil
	}
	if len(m.Wakes) >= 20 {
		m.Overflow = true
		return nil
	}
	out := m.Pending[0]
	// Slot 20 is reserved for the summary, and a queue crossing the hourly
	// boundary is summarized as well. The queue itself stays durable while full.
	if len(m.Pending) > 1 && (m.Overflow || len(m.Wakes) == 19 || len(m.Pending) > 20) {
		out.Events = []string{}
		for _, w := range m.Pending {
			out.Events = append(out.Events, w.Event)
		}
		m.Pending = nil
		m.Overflow = false
	} else {
		m.Pending = m.Pending[1:]
	}
	m.Wakes = append(m.Wakes, s.now())
	return &out
}
