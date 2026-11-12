package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	if v, ok := s.progressMemo[j.ID]; ok {
		return v
	}
	h := sha256.New()
	if s.WorktreesDir != "" {
		paths := []string{}
		switch v := snap.Bodies[j.ID]["worktree"].(type) {
		case string:
			paths = append(paths, v)
		case []any:
			for _, path := range v {
				if text, ok := path.(string); ok {
					paths = append(paths, text)
				}
			}
		case map[string]any:
			for _, path := range v {
				if text, ok := path.(string); ok {
					paths = append(paths, text)
				}
			}
		}
		sort.Strings(paths)
		for _, name := range paths {
			path := filepath.Join(s.WorktreesDir, name)
			if rel, e := filepath.Rel(s.WorktreesDir, path); e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			if _, e := os.Stat(path); os.IsNotExist(e) {
				base, _, _ := strings.Cut(name, "@")
				path = filepath.Join(s.WorktreesDir, base)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			out, e := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--verify", "HEAD").Output()
			cancel()
			if e == nil {
				fmt.Fprintf(h, "worktree %s %s", name, out)
			}
		}
	}
	fmt.Fprint(h, s.taskProgress)
	evidence := []string{}
	for id, r := range snap.Records {
		if r.Type == "post" || r.Type == "checkpoint" && r.Job != nil && *r.Job == j.ID || r.Type == "answer" && r.Author.ID == s.Owner {
			evidence = append(evidence, id)
		}
	}
	sort.Strings(evidence)
	fmt.Fprint(h, evidence)
	value := hex.EncodeToString(h.Sum(nil))
	if s.progressMemo != nil {
		s.progressMemo[j.ID] = value
	}
	return value
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
	// A retry after a crash between the question and park reuses the question.
	// The crash may also have come before the question file was written, so a
	// task with an identity projects the Feed to files first, as ReserveGate does.
	for _, r := range snap.Records {
		if r.Type == "question" && r.Wake != nil && *r.Wake == m.Failure+"/1" && r.Job != nil && *r.Job == j.ID {
			if s.Feed.TaskID != "" {
				if _, err := (question.Store{TaskDir: s.Feed.TaskDir, TaskID: s.Feed.TaskID}).Sync(); err != nil {
					return err
				}
			}
			return s.control(j, contract.Parked, "needs_person", m.Failure)
		}
	}
	err := (question.Store{TaskDir: s.Feed.TaskDir, TaskID: s.Feed.TaskID}).ReserveGate(gate, func(id string) error {
		_, err := s.Feed.Post(feed.Input{Type: "question", Job: j.ID, Wake: m.Failure + "/1", Body: map[string]any{
			"question": "Three attempts made no progress. How should " + j.ID + " continue?",
			"who":      owner, "blocking": true, "options": []string{"Keep parked", "Retry"}, "recommended": "Keep parked", "why": qtPkWhy(snap, m.Failures),
			"gate": map[string]any{"id": id, "kind": "person", "default": "Keep parked", "at": contract.FormatTime(s.now().Add(24 * time.Hour))},
		}}, s.Writer)
		return err
	})
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
			body["worktree"] = nil
			event := j.Results[len(j.Results)-1]
			if _, err := s.Feed.Post(feed.Input{Type: "job", Job: child, Re: event, Body: body}, s.Writer); err != nil {
				return err
			}
		}
	}
	return nil
}
func fits(parent map[string]any, j *contract.Job, p contract.ProposedJob) bool {
	if p.DiscoveredFrom != j.ID || contract.NormalizeRole(p.Role) != contract.NormalizeRole(j.Role) {
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
	for id, q := range snap.State.Questions {
		if q.Open && len(q.Answers) > 1 {
			job := ""
			if q.Job != nil {
				job = *q.Job
			}
			candidates[id] = job
		}
	}
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
	if m.WakeCounts == nil {
		m.WakeCounts = map[string]int{}
	}
	acknowledged := map[string]bool{}
	for _, r := range snap.Records {
		if r.Wake != nil {
			acknowledged[*r.Wake] = true
		}
	}
	// L34 used n=1 for every event and marked delivery before any write. On
	// upgrade, honor existing acknowledgements by their original key and reserve
	// all legacy delivery slots so replay cannot overwrite an earlier render.
	if m.WakeVersion == 0 {
		counts := map[string]int{}
		for event, seen := range m.Delivered {
			if seen {
				counts[candidates[event]]++
			}
		}
		m.Delivered = map[string]bool{}
		for wake := range acknowledged {
			event, number, ok := strings.Cut(wake, "/")
			n, err := strconv.Atoi(number)
			job, candidate := candidates[event]
			if ok && err == nil && n > 0 && candidate {
				m.Delivered[event] = true
				counts[job] = max(counts[job], n)
			}
		}
		for job, n := range counts {
			m.WakeCounts[job] = max(m.WakeCounts[job], n)
		}
		m.Pending = nil
		m.Active = nil
		m.WakeVersion = 1
	}
	key := func(w Wake) string { return fmt.Sprintf("%s/%d", w.Event, w.N) }
	// A result wake asks the Orchestrator to judge the result. Once the Job is
	// accepted there is nothing left to judge, whether or not the accepting control
	// carried --wake, so the wake is consumed rather than left standing.
	settled := map[string]bool{}
	for _, id := range ids(snap) {
		if j := snap.State.Jobs[id]; j.Status == contract.Accepted {
			for _, r := range j.Results {
				settled[r] = true
			}
		}
	}
	pending := m.Pending[:0]
	for _, w := range m.Pending {
		if settled[w.Event] {
			m.Delivered[w.Event] = true
		} else {
			pending = append(pending, w)
		}
	}
	m.Pending = pending
	if m.Active != nil {
		all := settled[m.Active.Event]
		for _, event := range m.Active.Events {
			all = all && settled[event]
		}
		if all {
			m.Active = nil
		}
	}
	if m.Active != nil {
		if !acknowledged[key(*m.Active)] {
			return m.Active
		}
		done := map[string]bool{m.Active.Event: true}
		for _, event := range m.Active.Events {
			done[event] = true
		}
		pending := m.Pending[:0]
		for _, w := range m.Pending {
			if done[w.Event] {
				m.Delivered[w.Event] = true
			} else {
				pending = append(pending, w)
			}
		}
		m.Pending = pending
		m.Active = nil
	}
	queued := map[string]bool{}
	for _, w := range m.Pending {
		queued[w.Event] = true
	}
	for _, event := range keys {
		if settled[event] {
			m.Delivered[event] = true
			continue
		}
		if !m.Delivered[event] && !queued[event] {
			job := candidates[event]
			m.WakeCounts[job]++
			w := Wake{Job: job, Event: event, N: m.WakeCounts[job]}
			if acknowledged[key(w)] {
				m.Delivered[event] = true
			} else {
				m.Pending = append(m.Pending, w)
			}
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
		m.Overflow = false
	}
	m.Wakes = append(m.Wakes, s.now())
	m.Active = &out
	return m.Active
}

// A single eligible answer needs no arbitration. Multiple answers are a
// judgement event; neither arrival time nor a record id chooses their winner.
// On a person gate only an answer feed.PersonAnswer admits is eligible; the other
// gate kinds keep the contract's rule, as the feed's person layer does.
func (s Store) answers(snap Snapshot) error {
	ids := []string{}
	for id := range snap.State.Questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		q := snap.State.Questions[id]
		if !q.Open {
			continue
		}
		gate, _ := snap.Records[id].Body["gate"].(map[string]any)
		person := stringValue(gate["kind"]) == "person"
		admitted := []string{}
		for _, aid := range q.Answers {
			if !person || feed.PersonAnswer(snap.Records[aid], q.Who) {
				admitted = append(admitted, aid)
			}
		}
		if len(admitted) != 1 {
			continue
		}
		answer := admitted[0]
		if _, err := s.Feed.Post(feed.Input{Type: "control", Re: answer, After: []string{id}, Body: map[string]any{"target": id, "answer": answer, "status": "answered"}}, s.Writer); err != nil {
			return err
		}
	}
	return nil
}

// Rebuild failure evidence if a pass died before publishing its local metadata.
// The counted history is authoritative; the metadata only saves repeat work.
func (s Store) recoverMemory(snap Snapshot, j *contract.Job) *jobMemory {
	m := &jobMemory{Failures: []string{}, Progress: s.progress(snap, j)}
	seen := map[string]bool{}
	for _, id := range j.History {
		r := snap.Records[id]
		failure := ""
		switch r.Type {
		case "checkpoint":
			m.Failures = nil
		case "worker-exit":
			switch stringValue(r.Body["reason"]) {
			case "usage_limit", "cancelled":
				m.Failures = nil
			case "done":
			default:
				failure = id
			}
		case "control":
			if r.Body["status"] == "ready" && r.Re != nil {
				for qid, q := range snap.State.Questions {
					question := snap.Records[qid]
					if question.Job != nil && *question.Job == j.ID && question.Wake != nil && *question.Wake == m.Failure+"/1" && q.Applied != nil && *q.Applied == *r.Re && snap.Records[*q.Applied].Body["answer"] == "Retry" {
						m.Failures = nil
					}
				}
			}
			if r.Body["status"] == "stale" && r.Re != nil {
				failure = *r.Re
			}
		}
		if failure != "" && !seen[failure] {
			seen[failure] = true
			m.Failures = append(m.Failures, failure)
			m.Failure = failure
		}
	}
	if j.Status == contract.Parked && j.Reason == "needs_person" {
		m.ParkProgress = m.Progress
	}
	return m
}
