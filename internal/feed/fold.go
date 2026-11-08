package feed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// State is a pure function of the complete lines and takeover log. Liveness,
// elapsed time, handover announcements and chatter cannot change Job status.
type State struct {
	Task      string                    `json:"task"`
	Jobs      map[string]*JobState      `json:"jobs"`
	Questions map[string]*QuestionState `json:"questions"`
	ByRef     map[string][]string       `json:"by_ref"`
	Head      *Record                   `json:"head"`
	Hosts     map[string]Record         `json:"hosts"`
	Records   []Entry                   `json:"records"`
	Bad       map[string]int            `json:"bad"`
	NeedsYou  []string                  `json:"needs_you"`
	Wakes     []string                  `json:"wakes"`
}
type JobState struct {
	ID          string         `json:"id"`
	Post        string         `json:"post"`
	Body        map[string]any `json:"body"`
	Status      string         `json:"status"`
	Attempt     int            `json:"attempt"`
	Assign      *Assignment    `json:"assign"`
	History     []string       `json:"history"`
	Results     []string       `json:"results"`
	Submits     []string       `json:"submits"`
	Checkpoint  *Record        `json:"checkpoint"`
	Exited      *string        `json:"exited"`
	ExitReason  *string        `json:"exit_reason"`
	CloseReason *string        `json:"close_reason"`
	LevelRaised *string        `json:"level_raised"`
}
type Assignment struct {
	Job     string `json:"job"`
	Attempt int    `json:"attempt"`
	To      string `json:"to"`
	Fence   int64  `json:"fence"`
	Gen     int64  `json:"gen"`
	Until   string `json:"until"`
	Record  string `json:"record"`
}
type QuestionState struct {
	ID      string         `json:"id"`
	Body    map[string]any `json:"body"`
	Answers []string       `json:"answers"`
	Applied *string        `json:"applied"`
	Open    bool           `json:"open"`
}

func (s Store) State() (State, error) {
	read, err := s.Read(Filter{})
	if err != nil {
		return State{}, err
	}
	return Fold(s.TaskID, read), nil
}

// Fold never sorts decisions by timestamps. Controls are ordered by generation,
// fence and file position; Job records must match a counted assignment.
func Fold(task string, read ReadResult) State {
	state := State{Task: task, Jobs: map[string]*JobState{}, Questions: map[string]*QuestionState{}, ByRef: map[string][]string{}, Hosts: map[string]Record{}, Records: append([]Entry{}, read.Records...), Bad: read.Bad, NeedsYou: []string{}, Wakes: []string{}}
	sort.Slice(state.Records, func(i, j int) bool {
		a, b := state.Records[i], state.Records[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.End < b.End
	})
	for i := range state.Records {
		state.Records[i].Arrived = ""
	}
	byID := map[string]*Entry{}
	for i := range state.Records {
		byID[state.Records[i].ID] = &state.Records[i]
	}
	duplicates := map[string]bool{}
	// Questions and Jobs are immutable definitions. Multiple conflicting Job
	// definitions are ignored instead of being chosen by a cross-writer clock.
	for i := range state.Records {
		e := &state.Records[i]
		if e.Verdict != "counted" {
			continue
		}
		switch e.Type {
		case "job":
			id := value(e.Job)
			if duplicates[id] {
				ignore(e, "duplicate Job definition")
				continue
			}
			if old := state.Jobs[id]; old != nil {
				ignore(e, "duplicate Job definition")
				ignore(byID[old.Post], "duplicate Job definition")
				delete(state.Jobs, id)
				duplicates[id] = true
				continue
			}
			state.Jobs[id] = &JobState{ID: id, Post: e.ID, Body: e.Body, Status: "posted", History: []string{}, Results: []string{}, Submits: []string{}}
		case "question":
			state.Questions[e.ID] = &QuestionState{ID: e.ID, Body: e.Body, Answers: []string{}, Open: true}
		}
	}
	// Answers are candidates only. A task control explicitly applies one.
	for i := range state.Records {
		e := &state.Records[i]
		if e.Verdict != "counted" || e.Type != "answer" {
			continue
		}
		q := state.Questions[value(e.Re)]
		if q == nil {
			ignore(e, "answer target is not a question")
			continue
		}
		if e.Author.Role == "guest" && str(q.Body["who"]) != e.Author.Member {
			ignore(e, "question is not addressed to this guest")
			continue
		}
		q.Answers = append(q.Answers, e.ID)
	}
	controls := []*Entry{}
	for i := range state.Records {
		e := &state.Records[i]
		if e.Verdict == "counted" && e.Type == "control" {
			controls = append(controls, e)
		}
	}
	sort.Slice(controls, func(i, j int) bool {
		a, b := controls[i], controls[j]
		if a.Gen != b.Gen {
			return a.Gen < b.Gen
		}
		if a.Fence != b.Fence {
			return a.Fence < b.Fence
		}
		return a.End < b.End
	})
	grants := map[string]*Assignment{}
	for _, e := range controls {
		b := e.Body
		if a := object(b["assign"]); a != nil {
			id := str(a["job"])
			job := state.Jobs[id]
			if job == nil {
				ignore(e, "assign target is not a Job")
				continue
			}
			attempt := int(number(a["attempt"]))
			key := fmt.Sprintf("%s/%d", id, attempt)
			if grants[key] != nil || attempt <= job.Attempt {
				ignore(e, "attempt already assigned or superseded")
				continue
			}
			grant := &Assignment{Job: id, Attempt: attempt, To: str(a["to"]), Fence: number(a["fence"]), Gen: e.Gen, Until: str(a["until"]), Record: e.ID}
			grants[key] = grant
			job.Assign = grant
			job.Attempt = attempt
			job.Status = "assigned"
			job.History = append(job.History, e.ID)
			continue
		}
		target := str(b["target"])
		if q := state.Questions[target]; q != nil {
			if answer := str(b["answer"]); answer != "" {
				found := false
				for _, id := range q.Answers {
					if id == answer {
						found = true
					}
				}
				if !found {
					ignore(e, "applied answer is not a candidate")
					continue
				}
				q.Applied = ptr(answer)
				q.Open = false
			} else if status := str(b["status"]); status == "closed" || status == "cancelled" {
				q.Open = false
			} else {
				ignore(e, "question control must select an answer or close it")
			}
			continue
		}
		job := state.Jobs[target]
		if job == nil {
			if post := byID[target]; post != nil && post.Verdict == "counted" {
				typ, _ := lookup(post.Type)
				authorized := true
				if typ.Lease != nil && *typ.Lease == "job" {
					attempt := 0
					if post.Attempt != nil {
						attempt = *post.Attempt
					}
					grant := grants[fmt.Sprintf("%s/%d", value(post.Job), attempt)]
					authorized = grant != nil && grant.To == post.Worker && grant.Fence == post.Fence && grant.Gen == post.Gen
				}
				if authorized {
					job = state.Jobs[value(post.Job)]
				}
			}
		}
		if job == nil {
			ignore(e, "control target is not a Job or question")
			continue
		}
		if status := str(b["status"]); status != "" {
			job.Status = status
			if status == "done" {
				job.Status = "accepted"
			}
		}
		if str(b["verdict"]) == "accept" {
			job.Status = "accepted"
		}
		if reason := str(b["close_reason"]); reason != "" {
			job.CloseReason = ptr(reason)
		}
		job.History = append(job.History, e.ID)
	}
	// The current assignment determines which attempt may update current state.
	for i := range state.Records {
		e := &state.Records[i]
		if e.Verdict != "counted" {
			continue
		}
		r := e.Record
		t, _ := lookup(r.Type)
		if t.Lease != nil && *t.Lease == "job" {
			attempt := 0
			if r.Attempt != nil {
				attempt = *r.Attempt
			}
			grant := grants[fmt.Sprintf("%s/%d", value(r.Job), attempt)]
			if grant == nil || grant.To != r.Worker || grant.Fence != r.Fence || grant.Gen != r.Gen {
				ignore(e, "no matching counted assignment")
				continue
			}
			if r.Type == "ack" && !contains(r.After, grant.Record) {
				ignore(e, "ack does not depend on its assignment")
				continue
			}
			job := state.Jobs[value(r.Job)]
			if job.Assign != grant {
				continue
			}
			switch r.Type {
			case "ack":
				if job.Status == "assigned" {
					job.Status = "running"
				}
			case "submit":
				job.Submits = append(job.Submits, r.ID)
			case "result":
				job.Results = append(job.Results, r.ID)
			case "checkpoint":
				copy := r
				job.Checkpoint = &copy
			case "worker-exit":
				job.Exited = ptr(r.ID)
				reason := str(r.Body["reason"])
				if reason == "" {
					reason = "crash"
				}
				job.ExitReason = ptr(reason)
			}
		}
		switch r.Type {
		case "post":
			ref := str(r.Body["ref"])
			state.ByRef[ref] = append(state.ByRef[ref], r.ID)
		case "head":
			// A task-lease head can be compared by fence; unleased heads remain visible.
			if value(r.Lease) == "task" && (state.Head == nil || newer(r, *state.Head)) {
				copy := r
				state.Head = &copy
			}
		case "host":
			old, exists := state.Hosts[r.Worker]
			if !exists || newer(r, old) {
				state.Hosts[r.Worker] = r
			}
		case "found":
			if str(r.Body["kind"]) == "security" {
				if job := state.Jobs[str(r.Body["job"])]; job != nil {
					job.LevelRaised = ptr("security")
					state.Wakes = append(state.Wakes, r.ID)
				}
			}
		case "loop-detected":
			state.Wakes = append(state.Wakes, r.ID)
		}
	}
	for id, q := range state.Questions {
		sort.Strings(q.Answers)
		if q.Open {
			state.NeedsYou = append(state.NeedsYou, id)
		}
	}
	sort.Strings(state.NeedsYou)
	sort.Strings(state.Wakes)
	// State must not depend on the input's display ordering or a reader's arrival clock.
	sort.Slice(state.Records, func(i, j int) bool {
		a, b := state.Records[i], state.Records[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.End < b.End
	})
	for key := range state.ByRef {
		sort.Strings(state.ByRef[key])
	}
	return state
}
func ignore(e *Entry, reason string) { e.Verdict = "ignored"; e.Reason = reason }
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
func newer(a, b Record) bool {
	return a.Gen > b.Gen || a.Gen == b.Gen && (a.Fence > b.Fence || a.Fence == b.Fence && a.Seq > b.Seq)
}

// SaveState serializes concurrent folds and atomically publishes each projection.
// Readers needing an indivisible snapshot should use State or state.json, which
// includes needs_you as well as the separately published needs-you.json.
func (s Store) SaveState() (State, error) {
	if err := s.prepare(); err != nil {
		return State{}, err
	}
	if err := os.MkdirAll(filepath.Join(s.Dir(), ".cache"), 0o700); err != nil {
		return State{}, err
	}
	f, err := os.OpenFile(filepath.Join(s.Dir(), ".cache", "fold.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return State{}, err
	}
	defer f.Close()
	if err := lock(f, true); err != nil {
		return State{}, err
	}
	defer unlock(f)
	state, err := s.State()
	if err != nil {
		return State{}, err
	}
	if err := writeProjection(s.Dir(), "state.json", state); err != nil {
		return State{}, err
	}
	if err := writeProjection(s.Dir(), "needs-you.json", state.NeedsYou); err != nil {
		return State{}, err
	}
	return state, nil
}
func writeProjection(dir, name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(dir, ".projection-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}
