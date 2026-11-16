package contract

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
)

type controlBody struct {
	Assign      *Assignment `json:"assign"`
	Target      string      `json:"target"`
	Status      Status      `json:"status"`
	Reason      string      `json:"reason"`
	CloseReason *string     `json:"close_reason"`
	Attempt     int         `json:"attempt"`
	Answer      string      `json:"answer"`
	Order       *int        `json:"order"`
}

type question struct {
	post    Post
	who     string
	closed  bool
	applied *string
}

// Fold materializes one task's admitted posts. The caller checks writer-file
// identity, complete lines and takeover byte offsets, setting Post.Stale for
// lines beyond an ended tenure. No timestamp affects a decision here.
//
// Controls apply by (gen, fence, seq). Each grant consumes its matching Job
// tenure in seq order before the next control. Cross-file causal dependencies
// must be named by re/after. Unknown types and versions remain visible without
// affecting Jobs. This is an in-memory contract fold, not a filesystem reader
// or the scheduling/liveness loop.
func Fold(posts []Post) State {
	out := State{Jobs: map[string]*Job{}, Verdicts: []Verdict{}, Questions: map[string]*Question{}, NeedsYou: []string{}, Wakes: []string{}, ByRef: map[string][]string{}, Hosts: map[string]string{}}
	verdicts := map[string]Verdict{}
	set := func(p Post, kind, reason string) { verdicts[p.ID] = Verdict{p.ID, kind, reason} }
	seen := map[string]Post{}
	var ordered []Post
	tasks := map[string]bool{}
	for _, p := range posts {
		if _, duplicate := seen[p.ID]; duplicate {
			continue
		}
		data, err := json.Marshal(p)
		if err == nil {
			err = Validate("envelope", data)
		}
		if err != nil {
			set(p, "bad", fmt.Sprint(err))
			continue
		}
		seen[p.ID] = p
		tasks[p.Task] = true
		ordered = append(ordered, p)
	}
	// Sorting is only an implementation traversal. Cross-writer authority comes
	// from the task controls and their granted Job fences, never these tie-breaks.
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.Gen != b.Gen {
			return a.Gen < b.Gen
		}
		if a.Fence != b.Fence {
			return a.Fence < b.Fence
		}
		if a.Worker != b.Worker {
			return a.Worker < b.Worker
		}
		return a.Seq < b.Seq
	})
	var active []Post
	for _, p := range ordered {
		if len(tasks) != 1 {
			set(p, "bad", "Fold requires exactly one task")
			continue
		}
		if p.Stale {
			set(p, "stale", "past tenure end offset")
			continue
		}
		missing := ""
		refs := append([]string{}, p.After...)
		if p.Re != nil {
			refs = append(refs, *p.Re)
		}
		for _, id := range refs {
			if _, ok := seen[id]; !ok {
				missing = id
				break
			}
		}
		if missing != "" {
			set(p, "held", "waiting for "+missing)
			continue
		}
		schema, known := schemas.byName[p.Type+".json"]
		if !known || schema["x-class"] == nil {
			set(p, "unknown", "unknown or retired Feed type")
			continue
		}
		if version := typeVersion(schema); version == 0 || p.TypeV != version {
			set(p, "unknown", "unknown type version")
			continue
		}
		data, _ := json.Marshal(p)
		if err := Validate(p.Type, data); err != nil {
			set(p, "bad", err.Error())
			continue
		}
		var body map[string]json.RawMessage
		_ = json.Unmarshal(p.Body, &body)
		if len(body) == 1 && body["ref"] != nil {
			set(p, "held", "resolve and validate external body before folding")
			continue
		}
		set(p, "counted", "")
		active = append(active, p)
	}

	questions := map[string]*question{}
	// personJobs holds, by the id of its first job record, each Job's role, owner
	// and kind: what GuestAddressee reads for a guest's answer to a person Job.
	type personJob struct{ role, owner, kind string }
	personJobs := map[string]personJob{}
	for _, p := range active {
		switch p.Type {
		case "job":
			var b struct {
				ID, Title, Role, Kind string
				Repo                  string
				Continues             *string `json:"continues"`
				DiscoveredFrom        *string `json:"discovered_from"`
				Order                 *int    `json:"order"`
				Intent                struct{ Owner string }
				Acceptance            []struct{ ID, State string }
			}
			// An old record may still carry review and max_rounds; they are not
			// decoded, so they are ignored.
			_ = json.Unmarshal(p.Body, &b)
			if _, exists := out.Jobs[b.ID]; exists {
				set(p, "ignored", "Job already exists; revisions need a new Job")
				continue
			}
			// A record row has no state or dropped; one without reads pending.
			rows := []AcceptanceRow{}
			for _, a := range b.Acceptance {
				rows = append(rows, AcceptanceRow{ID: a.ID, State: rowState(a.State)})
			}
			out.Jobs[b.ID] = &Job{ID: b.ID, Task: p.Task, Title: b.Title, Role: b.Role, Kind: b.Kind, Repo: b.Repo, Status: Posted, Continues: b.Continues, DiscoveredFrom: b.DiscoveredFrom, Order: b.Order, Results: []string{}, Submits: []string{}, History: []string{p.ID}, OpenQuestions: []string{}, ProposedJobs: []ProposedJob{}, DecisionsNeeded: []string{}, Acceptance: rows}
			personJobs[p.ID] = personJob{b.Role, b.Intent.Owner, b.Kind}
		case "question":
			var b struct{ Who string }
			_ = json.Unmarshal(p.Body, &b)
			questions[p.ID] = &question{post: p, who: b.Who}
		case "handover":
			// Announcement only: neither this nor transfer changes authority.
		}
	}
	consumed := map[string]bool{}
	for _, p := range active {
		if p.Type != "control" {
			continue
		}
		var b controlBody
		_ = json.Unmarshal(p.Body, &b)
		if q := questions[b.Target]; q != nil {
			answer, exists := seen[b.Answer]
			if !exists || answer.Type != "answer" || verdicts[answer.ID].Kind != "counted" || answer.Re == nil || *answer.Re != q.post.ID || NormalizeRole(answer.Author.Role) == "guest" && answer.Author.ID != q.who {
				set(p, "ignored", "control must select an eligible answer to this question")
				continue
			}
			q.closed, q.applied = true, &answer.ID
			continue
		}
		id := b.Target
		if b.Assign != nil {
			id = b.Assign.Job
		}
		if target, ok := seen[id]; ok && target.Job != nil {
			id = *target.Job
		}
		j := out.Jobs[id]
		if j == nil {
			set(p, "ignored", "unknown Job target")
			continue
		}
		// An unleased control is a person's cancel request (the leaseless cancel
		// branch of control.json). It stays counted, so the task holder's pass reads
		// it, but it never moves the Job: the holder's leased control does
		// (orchestrator personCancels, D-r19 rule point 2).
		if p.Lease == nil {
			continue
		}
		if b.Assign != nil {
			a := b.Assign
			sameRetry := a.Attempt == j.Attempt && j.ExitReason == "usage_limit" && j.Assign != nil && a.Fence > j.Assign.Fence
			if a.Attempt < j.Attempt || a.Attempt == j.Attempt && !sameRetry || j.Status == Accepted || j.Status == Cancelled || j.Status == Rejected {
				set(p, "ignored", "attempt already granted, regressed, or Job closed")
				continue
			}
			if j.Assign != nil && a.Fence <= j.Assign.Fence && p.Gen <= j.Generation {
				set(p, "ignored", "grant must advance the tenure")
				continue
			}
			j.Assign, j.AssignPost, j.Generation = a, p.ID, p.Gen
			j.Attempt, j.Status, j.LeaseExpires = a.Attempt, Assigned, a.Until
			j.Exited, j.ExitReason, j.Reason = "", "", ""
			j.ExitCode, j.ExitReasonMissing = nil, false
			j.History = append(j.History, p.ID)
			acknowledged := false
			for _, side := range active {
				if !isJobSide(side.Type) || side.Job == nil || *side.Job != id || side.Attempt == nil || *side.Attempt != a.Attempt || side.Worker != a.To || side.Fence != a.Fence || side.Gen != p.Gen || consumed[side.ID] {
					continue
				}
				consumed[side.ID] = true
				if j.Exited != "" {
					set(side, "ignored", "attempt already exited")
					continue
				}
				if side.Type == "ack" {
					if acknowledged || !contains(side.After, p.ID) {
						set(side, "ignored", "ack must name its assign exactly once")
						continue
					}
					acknowledged, j.Status = true, Running
				} else if !acknowledged {
					set(side, "ignored", "Job tenure must start with ack")
					continue
				}
				if reason := applySide(j, side); reason != "" {
					set(side, "ignored", reason)
				}
			}
			continue
		}
		// An order record changes only the queue position, and only before the Job starts.
		if b.Order != nil && b.Status == "" && b.Reason == "" {
			if state, _ := StateOf(j.Status); state != StateNotStarted {
				set(p, "ignored", "order changes only a Job that has not started")
				continue
			}
			j.Order = b.Order
			j.History = append(j.History, p.ID)
			continue
		}
		if b.Attempt != 0 && b.Attempt != j.Attempt {
			set(p, "ignored", "control names another attempt")
			continue
		}
		if (b.Status == Cancelled || b.Status == Rejected) && (b.CloseReason == nil || *b.CloseReason == "") {
			set(p, "ignored", "close_reason required")
			continue
		}
		if b.Status != "" {
			j.Status = b.Status
			j.CloseReason = b.CloseReason
		}
		j.Reason = b.Reason
		j.History = append(j.History, p.ID)
	}
	for _, p := range active {
		if isJobSide(p.Type) && !consumed[p.ID] {
			set(p, "ignored", "no matching counted assign for this tenure")
		}
		if p.Type == "answer" && NormalizeRole(p.Author.Role) == "guest" {
			q := questions[*p.Re]
			pj := personJobs[*p.Re]
			if (q == nil || q.who != p.Author.ID) && !GuestAddressee(p.Author.Role, p.Author.ID, p.Lease != nil || p.Fence != 0, pj.role, pj.owner, pj.kind) {
				set(p, "ignored", "question is not addressed to this guest")
			}
		}
		if p.Type == "found" {
			var b struct{ Job, Kind string }
			_ = json.Unmarshal(p.Body, &b)
			if j := out.Jobs[b.Job]; j != nil && b.Kind == "security" {
				j.NeedsLead = true
			}
		}
	}
	for _, j := range out.Jobs {
		j.State, j.StateReason = StateOf(j.Status)
	}
	for id, q := range questions {
		open := !q.closed
		if q.post.Job != nil {
			if j := out.Jobs[*q.post.Job]; j != nil && j.Status == Cancelled {
				open = false
			} else if j != nil && open {
				j.OpenQuestions = append(j.OpenQuestions, id)
			}
		}
		out.Questions[id] = &Question{ID: id, Job: q.post.Job, Who: q.who, Answers: []string{}, Applied: q.applied, Open: open}
		if open {
			out.NeedsYou = append(out.NeedsYou, id)
		}
	}
	var head *Post
	hosts := map[string]Post{}
	for _, p := range active {
		if verdicts[p.ID].Kind != "counted" {
			continue
		}
		switch p.Type {
		case "answer":
			if q := out.Questions[deref(p.Re)]; q != nil {
				q.Answers = append(q.Answers, p.ID)
			}
		case "post":
			var b struct{ Ref string }
			_ = json.Unmarshal(p.Body, &b)
			out.ByRef[b.Ref] = append(out.ByRef[b.Ref], p.ID)
		case "head":
			// Only a task-lease head is ordered by tenure; unleased heads stay visible.
			if deref(p.Lease) == "task" && (head == nil || newer(p, *head)) {
				h := p
				head = &h
			}
		case "host":
			if old, ok := hosts[p.Worker]; !ok || newer(p, old) {
				hosts[p.Worker] = p
			}
		case "found":
			var b struct{ Job, Kind string }
			_ = json.Unmarshal(p.Body, &b)
			if b.Kind == "security" && out.Jobs[b.Job] != nil {
				out.Wakes = append(out.Wakes, p.ID)
			}
		case "loop-detected":
			out.Wakes = append(out.Wakes, p.ID)
		}
	}
	if head != nil {
		out.Head = &head.ID
	}
	for worker, p := range hosts {
		out.Hosts[worker] = p.ID
	}
	for _, j := range out.Jobs {
		sort.Strings(j.OpenQuestions)
	}
	for _, q := range out.Questions {
		sort.Strings(q.Answers)
	}
	for _, ids := range out.ByRef {
		sort.Strings(ids)
	}
	sort.Strings(out.NeedsYou)
	sort.Strings(out.Wakes)
	for _, v := range verdicts {
		out.Verdicts = append(out.Verdicts, v)
	}
	sort.Slice(out.Verdicts, func(i, j int) bool { return out.Verdicts[i].ID < out.Verdicts[j].ID })
	return out
}

// applySide applies one Job-side post of the current tenure. It returns why the
// post is ignored, or "" when it counts.
func applySide(j *Job, p Post) string {
	// A checkpoint that marks one acceptance row sets that row's state and note
	// and leaves the step checkpoint as it was; a row the Job record lacks is ignored.
	var mark struct {
		Acceptance *struct{ ID, State, Note string }
	}
	if p.Type == "checkpoint" && json.Unmarshal(p.Body, &mark) == nil && mark.Acceptance != nil {
		row := rowOf(j, mark.Acceptance.ID)
		if row == nil {
			return "Job has no acceptance row " + mark.Acceptance.ID
		}
		j.History = append(j.History, p.ID)
		row.State, row.Note = rowState(mark.Acceptance.State), mark.Acceptance.Note
		return ""
	}
	j.History = append(j.History, p.ID)
	switch p.Type {
	case "submit":
		j.Submits = append(j.Submits, p.ID)
	case "result":
		j.Results = append(j.Results, p.ID)
		var b struct {
			ProposedJobs    []ProposedJob   `json:"proposed_jobs"`
			DecisionsNeeded []string        `json:"decisions_needed"`
			Verified        json.RawMessage `json:"verified"`
			Acceptance      []struct{ ID, State, Decision string }
		}
		_ = json.Unmarshal(p.Body, &b)
		// The result's row states are the latest marks; a row the record lacks
		// is not added.
		for _, a := range b.Acceptance {
			if row := rowOf(j, a.ID); row != nil {
				row.State, row.Note = rowState(a.State), a.Decision
			}
		}
		j.ProposedJobs = append(j.ProposedJobs, b.ProposedJobs...)
		j.DecisionsNeeded = append(j.DecisionsNeeded, b.DecisionsNeeded...)
		j.Verified = b.Verified
		j.NeedsLead = j.NeedsLead || len(b.DecisionsNeeded) > 0
	case "checkpoint":
		var b Checkpoint
		_ = json.Unmarshal(p.Body, &b)
		j.Checkpoint = &b
	case "worker-exit":
		var b struct {
			Reason string
			Code   json.RawMessage
		}
		_ = json.Unmarshal(p.Body, &b)
		// No reason still reads as crash; ExitReasonMissing keeps a writer that
		// gave none apart from one that said crash.
		j.ExitReasonMissing = b.Reason == ""
		if j.ExitReasonMissing {
			b.Reason = "crash"
		}
		j.Exited, j.ExitReason, j.ExitCode = p.ID, b.Reason, exitCode(b.Code)
		if b.Reason == "usage_limit" {
			j.Reason = "usage_limit"
		}
		if b.Reason == "loop" {
			j.ReviewRounds++
			j.NeedsLead = true
		}
	}
	return ""
}

// rowOf returns the Job's acceptance row with this id, or nil.
func rowOf(j *Job, id string) *AcceptanceRow {
	for i := range j.Acceptance {
		if j.Acceptance[i].ID == id {
			return &j.Acceptance[i]
		}
	}
	return nil
}

// exitCode keeps a worker-exit code as the writer sent it when it is what the
// schemas allow (an integer or a non-empty string); anything else is dropped so
// state.json stays valid. A whole number written 2.0 reads as 2.
func exitCode(raw json.RawMessage) json.RawMessage {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if text == "" {
			return nil
		}
		return raw
	}
	var n json.Number
	if len(raw) == 0 || raw[0] == '"' || json.Unmarshal(raw, &n) != nil {
		return nil
	}
	if _, err := strconv.ParseInt(n.String(), 10, 64); err == nil {
		return raw
	}
	if f, err := strconv.ParseFloat(n.String(), 64); err == nil && f == math.Trunc(f) && math.Abs(f) < 1<<53 {
		return json.RawMessage(strconv.FormatInt(int64(f), 10))
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// newer orders posts of one stream or lease by tenure, never by timestamp.
func newer(a, b Post) bool {
	return a.Gen > b.Gen || a.Gen == b.Gen && (a.Fence > b.Fence || a.Fence == b.Fence && a.Seq > b.Seq)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// typeVersion returns the type_v const a post type's schema pins, or 0 when it pins none.
func typeVersion(schema map[string]any) int {
	parts, _ := schema["allOf"].([]any)
	for _, part := range parts {
		m, _ := part.(map[string]any)
		props, _ := m["properties"].(map[string]any)
		typeV, _ := props["type_v"].(map[string]any)
		if n, ok := typeV["const"].(json.Number); ok {
			if v, err := n.Int64(); err == nil {
				return int(v)
			}
		}
	}
	return 0
}
