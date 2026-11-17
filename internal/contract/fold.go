package contract

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
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
// lines beyond an ended tenure. No timestamp affects a decision here except
// through PostedAs, which compares a post's own at with a grant's until; the
// clock is never read.
//
// grants are the people's grant notes (run-jobs-launcher.md Q8). A counted answer
// counts as the person's only with for, by and a covering grant (PostedAs), and
// FoldLoop's Loop.PostedAs records it; without one it folds as its stamped
// author's own.
// The grants never refuse or drop a post.
//
// Controls apply by (gen, fence, seq). Each grant consumes its matching Job
// tenure in seq order before the next control. Cross-file causal dependencies
// must be named by re/after. Unknown types and versions remain visible without
// affecting Jobs. This is an in-memory contract fold, not a filesystem reader
// or the scheduling/liveness loop.
func Fold(posts []Post, grants ...Grant) State {
	out, _ := FoldLoop(posts, grants...)
	return out
}

// Blocked is a Job whose attempt posted blocked and then exited: it waits for a
// person's approval and a new assign, not for judgement (V26).
const Blocked Status = "blocked"

// Loop is what one fold reads from the loop posts that State has no field for.
// Allow holds, by Job, the allow rules of the Job's current assign, and only when
// its grant names a counted approval by a person that approves every one of them
// for that Job (V22); any other allow folds absent. Landed holds, by Job, its
// counted landed post: what a landed after edge waits for (V27). PostedAs holds,
// by post id, the person a counted answer counts as when an agent posted it for
// that person under a covering grant (V22); it is nil when every answer counts as
// its stamped author.
type Loop struct {
	Allow    map[string][]string `json:"allow"`
	Landed   map[string]string   `json:"landed"`
	PostedAs map[string]Author   `json:"posted_as,omitempty"`
}

// FoldLoop is Fold, also returning the Loop that the same pass read.
func FoldLoop(posts []Post, grants ...Grant) (State, Loop) {
	loop := Loop{Allow: map[string][]string{}, Landed: map[string]string{}}
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
	var active, readerStale []Post
	for _, p := range ordered {
		if len(tasks) != 1 {
			set(p, "bad", "Fold requires exactly one task")
			continue
		}
		if p.Stale {
			// A Job-side one of an older attempt gets "older attempt" once the
			// Jobs are folded (olderAttempt below).
			set(p, "stale", "past tenure end offset")
			readerStale = append(readerStale, p)
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
				Base, Model, Effort   string
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
			out.Jobs[b.ID] = &Job{ID: b.ID, Task: p.Task, Title: b.Title, Role: b.Role, Kind: b.Kind, Repo: b.Repo, Base: b.Base, Model: b.Model, Effort: b.Effort, Status: Posted, Continues: b.Continues, DiscoveredFrom: b.DiscoveredFrom, Order: b.Order, Results: []string{}, Submits: []string{}, History: []string{p.ID}, OpenQuestions: []string{}, ProposedJobs: []ProposedJob{}, DecisionsNeeded: []string{}, Acceptance: rows}
			personJobs[p.ID] = personJob{b.Role, b.Intent.Owner, b.Kind}
		case "question":
			var b struct{ Who string }
			_ = json.Unmarshal(p.Body, &b)
			questions[p.ID] = &question{post: p, who: b.Who}
		case "handover":
			// Announcement only: neither this nor transfer changes authority.
		}
	}
	// An answer counts as a person's only under a covering grant, read for the
	// kind of the person Job it answers; the stored author is never changed.
	for _, p := range active {
		if p.Type != "answer" {
			continue
		}
		if who, _ := PostAs(p, personJobs[deref(p.Re)].kind, grants); who != p.Author {
			if loop.PostedAs == nil {
				loop.PostedAs = map[string]Author{}
			}
			loop.PostedAs[p.ID] = who
		}
	}
	countedAs := func(p Post) Author {
		if who, ok := loop.PostedAs[p.ID]; ok {
			return who
		}
		return p.Author
	}
	consumed := map[string]bool{}
	// moved holds the Jobs whose status a control set after their latest assign;
	// the attempt's loop posts below do not undo it.
	moved := map[string]bool{}
	for _, p := range active {
		if p.Type != "control" {
			continue
		}
		var b controlBody
		_ = json.Unmarshal(p.Body, &b)
		if q := questions[b.Target]; q != nil {
			answer, exists := seen[b.Answer]
			who := countedAs(answer)
			if !exists || answer.Type != "answer" || verdicts[answer.ID].Kind != "counted" || answer.Re == nil || *answer.Re != q.post.ID || NormalizeRole(who.Role) == "guest" && who.ID != q.who {
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
			j.Assign, j.AssignPost, j.Generation = a, p.ID, p.Gen
			j.Attempt, j.Status, j.LeaseExpires = a.Attempt, Assigned, a.Until
			j.Exited, j.ExitReason, j.Reason = "", "", ""
			j.ExitCode, j.ExitReasonMissing = nil, false
			j.History = append(j.History, p.ID)
			moved[id] = false
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
				} else if side.Type == "worker-exit" && j.ExitReason == "done" && resultOf(j, seen, a.Attempt) {
					// Done with a result of this attempt is awaiting judgement with no
					// control post. A done exit with none is left to the tick's control.
					j.Status, j.Reason = Submitted, "awaiting judgement"
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
			moved[id] = true
		}
		j.Reason = b.Reason
		j.History = append(j.History, p.ID)
	}
	foldAttempts(out.Jobs, active, seen, moved, set)
	foldGrants(out.Jobs, seen, verdicts, loop.Allow)
	for _, p := range active {
		if p.Type != "landed" {
			continue
		}
		// A landed post names its Job in the envelope; there is no attempt rule.
		j := out.Jobs[deref(p.Job)]
		if j == nil {
			set(p, "ignored", "unknown Job target")
			continue
		}
		if old, ok := loop.Landed[j.ID]; !ok || p.ID < old {
			loop.Landed[j.ID] = p.ID
		}
		j.History = append(j.History, p.ID)
	}
	// A Job-side post of an attempt lower than its Job's that no tenure counted
	// was superseded by a newer assign: it folds stale and changes nothing. One
	// its own tenure counted stays counted.
	olderAttempt := func(p Post) bool {
		j := out.Jobs[deref(p.Job)]
		return isJobSide(p.Type) && j != nil && p.Attempt != nil && *p.Attempt < j.Attempt
	}
	for _, p := range readerStale {
		if olderAttempt(p) {
			set(p, "stale", "older attempt")
		}
	}
	for _, p := range active {
		if isJobSide(p.Type) && !consumed[p.ID] {
			if olderAttempt(p) {
				set(p, "stale", "older attempt")
			} else {
				set(p, "ignored", "no matching counted assign for this tenure")
			}
		}
		if who := countedAs(p); p.Type == "answer" && NormalizeRole(who.Role) == "guest" {
			q := questions[*p.Re]
			pj := personJobs[*p.Re]
			if (q == nil || q.who != who.ID) && !GuestAddressee(who.Role, who.ID, p.Lease != nil || p.Fence != 0, pj.role, pj.owner, pj.kind) {
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
	return out, loop
}

// isAttemptSide reports the loop posts that belong to one attempt of a Job and
// carry no lease: the launcher's started and exit, the agent's blocked and
// waiting, and a cancel. A tenure's posts (isJobSide) are not among them.
func isAttemptSide(typ string) bool {
	switch typ {
	case "started", "exit", "blocked", "waiting", "cancel":
		return true
	}
	return false
}

// foldAttempts folds the attempt-side loop posts. They carry no lease and fence
// 0, so no tenure consumes them: each matches its Job and attempt only (RJ :165,
// :171). One of an attempt below its Job's folds stale, "older attempt"; one of
// an attempt the Job was not assigned is ignored. Each Job's posts are read
// together after the controls, so the outcome does not hang on the order the
// writers sort in: cli@ sorts before worker@, so the launcher's exit is walked
// before the agent's blocked.
//
// An attempt's first exit counts and sets Exited, ExitReason and ExitCode unless
// a worker-exit of the attempt already did; a second exit is ignored. Then, when
// no control set the Job's status after its assign: a blocked and an exit fold
// Blocked, not Submitted (V26); else an exit done with a result of the attempt
// folds Submitted, "awaiting judgement"; else a started moves Assigned to Running.
// A blocked with no exit, a waiting and a cancel count and move nothing.
func foldAttempts(jobs map[string]*Job, active []Post, seen map[string]Post, moved map[string]bool, set func(Post, string, string)) {
	type attempt struct {
		exit, blocked *Post
		started       bool
	}
	attempts := map[string]*attempt{}
	var ids []string
	for _, p := range active {
		if !isAttemptSide(p.Type) {
			continue
		}
		j := jobs[deref(p.Job)]
		switch {
		case j == nil:
			set(p, "ignored", "unknown Job target")
			continue
		case p.Attempt != nil && *p.Attempt < j.Attempt:
			set(p, "stale", "older attempt")
			continue
		case p.Attempt == nil || j.Assign == nil || *p.Attempt != j.Attempt:
			set(p, "ignored", "no counted assign for this attempt")
			continue
		}
		a := attempts[j.ID]
		if a == nil {
			a = &attempt{}
			attempts[j.ID] = a
			ids = append(ids, j.ID)
		}
		switch p.Type {
		case "exit":
			if a.exit != nil {
				set(p, "ignored", "attempt already exited")
				continue
			}
			a.exit = &p
		case "blocked":
			if a.blocked == nil {
				a.blocked = &p
			}
		case "started":
			a.started = true
		}
		j.History = append(j.History, p.ID)
	}
	for _, id := range ids {
		j, a := jobs[id], attempts[id]
		var exit struct {
			Reason string
			Code   json.RawMessage
		}
		if a.exit != nil {
			_ = json.Unmarshal(a.exit.Body, &exit)
			if j.Exited == "" {
				j.Exited, j.ExitReason, j.ExitCode = a.exit.ID, exit.Reason, exitCode(exit.Code)
			}
		}
		if moved[id] {
			continue
		}
		switch {
		case a.exit != nil && a.blocked != nil:
			var b struct{ Rule string }
			_ = json.Unmarshal(a.blocked.Body, &b)
			j.Status, j.Reason = Blocked, "blocked on "+b.Rule
		case a.exit != nil && exit.Reason == "done" && resultOf(j, seen, j.Attempt):
			j.Status, j.Reason = Submitted, "awaiting judgement"
		case a.started && j.Status == Assigned:
			j.Status = Running
		}
	}
}

// foldGrants fills allow with the allow rules of each Job's current assign when
// its grant names a counted approval by a person (owner, member or lead) that
// approves, names the Job in its envelope and lists every one of those rules.
// The approval's attempt is not compared: it names the blocked attempt and the
// assign is the next one. Any other allow folds absent and the assign still
// counts. A grant note (P-loop-grants) is not read here.
func foldGrants(jobs map[string]*Job, seen map[string]Post, verdicts map[string]Verdict, allow map[string][]string) {
	for id, j := range jobs {
		var b struct {
			Assign struct {
				Allow []string `json:"allow"`
				Grant string   `json:"grant"`
			} `json:"assign"`
		}
		if j.AssignPost == "" || json.Unmarshal(seen[j.AssignPost].Body, &b) != nil || len(b.Assign.Allow) == 0 {
			continue
		}
		if approves(seen[b.Assign.Grant], verdicts, id, b.Assign.Allow) {
			allow[id] = append([]string(nil), b.Assign.Allow...)
		}
	}
}

// approves reports whether g is a counted approval by a person that approves
// every rule for job.
func approves(g Post, verdicts map[string]Verdict, job string, rules []string) bool {
	var b struct {
		Rules    []string `json:"rules"`
		Decision string   `json:"decision"`
	}
	if g.Type != "approval" || verdicts[g.ID].Kind != "counted" || TierFor(g.Author.Role) != PersonTeam || deref(g.Job) != job || json.Unmarshal(g.Body, &b) != nil || b.Decision != "approve" {
		return false
	}
	for _, r := range rules {
		if !contains(b.Rules, r) {
			return false
		}
	}
	return true
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

// resultOf says whether the Job counted a result of this attempt. Results stay
// across a new assign, so an older attempt's result is not one.
func resultOf(j *Job, seen map[string]Post, attempt int) bool {
	for _, id := range j.Results {
		if r := seen[id]; r.Attempt != nil && *r.Attempt == attempt {
			return true
		}
	}
	return false
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

// Grant is a person's grant note, grid/people/<person>/grants/<name>.md in the
// brain (run-jobs-launcher.md Q8): it lets the agents it names post as that
// person. Link, Person and Role come from where the note lives and the person's
// own note, never from a post.
type Grant struct {
	Link   string   // the note's path from the brain root, without .md
	Person string   // the person whose folder holds the note
	Role   string   // that person's role (owner, member, guest)
	Agents []string // the agent ids or roles it names
	Posts  []string // the post types it covers, such as answer
	Kinds  []string // the person-Job kinds an answer may close; approve only when named
	Tasks  []string // the task ids it covers; empty or "all" covers every task
	Until  string   // RFC 3339; it covers a post whose own time is before this
}

// PostAs is PostedAs for a contract post.
func PostAs(p Post, kind string, grants []Grant) (Author, string) {
	var body map[string]any
	_ = json.Unmarshal(p.Body, &body)
	return PostedAs(p.Type, p.Author, p.Task, p.At, body, kind, grants)
}

// PostedAs is the author a post counts as (run-jobs-launcher.md Q8, ledger V22).
// A post by an agent counts as a person's only when its body carries `for` (the
// person), `by` (its own stamped author id) and `grant` (a link to the grant
// note), and that note is the person's, names the agent by id or role, covers
// the post type, the task and the person-Job kind, and had not expired at the
// post's own time. Otherwise it counts as its stamped author's.
//
// This is a rule on reading, not on writing: no post is refused or dropped, and
// the stored author is never changed. The reason is "" when the post counts as
// the person's or asks for no person; otherwise it says why the agent's own
// author stands, for a note a reader may show. kind is the person-Job kind the
// post answers, "" for none. The expiry compares the post's at with the grant's
// until; the clock is never read.
func PostedAs(typ string, author Author, task, at string, body map[string]any, kind string, grants []Grant) (Author, string) {
	person, _ := body["for"].(string)
	by, _ := body["by"].(string)
	link, _ := body["grant"].(string)
	if person == "" && by == "" && link == "" {
		return author, ""
	}
	if person == "" || by == "" || link == "" {
		return author, "for, by and grant are all needed to post as a person"
	}
	if TierFor(author.Role) != Agent {
		return author, "only an agent posts for a person; a person posts as themselves"
	}
	if by != author.ID {
		return author, fmt.Sprintf("by %q is not the post's author %q", by, author.ID)
	}
	target := GrantTarget(link)
	var g *Grant
	for i := range grants {
		if grants[i].Link == target {
			g = &grants[i]
			break
		}
	}
	switch {
	case g == nil:
		return author, fmt.Sprintf("no grant note %s", target)
	case g.Person != person:
		return author, fmt.Sprintf("the grant %s is %s's, not %s's", target, g.Person, person)
	case TierFor(g.Role) != PersonTeam:
		return author, fmt.Sprintf("the grant %s is not of a person of the team", target)
	case !contains(g.Agents, author.ID) && !contains(g.Agents, NormalizeRole(author.Role)):
		return author, fmt.Sprintf("the grant %s does not name %s", target, author.ID)
	case !contains(g.Posts, typ):
		return author, fmt.Sprintf("the grant %s does not cover %s posts", target, typ)
	case kind == "approve" && !contains(g.Kinds, kind) || len(g.Kinds) > 0 && !contains(g.Kinds, kind):
		return author, fmt.Sprintf("the grant %s does not cover %q person Jobs", target, kind)
	case len(g.Tasks) > 0 && !contains(g.Tasks, "all") && !contains(g.Tasks, task):
		return author, fmt.Sprintf("the grant %s does not cover task %s", target, task)
	}
	posted, err := time.Parse(time.RFC3339Nano, at)
	until, uerr := time.Parse(time.RFC3339Nano, g.Until)
	if err != nil || uerr != nil || !posted.Before(until) {
		return author, fmt.Sprintf("the grant %s had expired at %s (until %q)", target, at, g.Until)
	}
	return Author{ID: g.Person, Role: NormalizeRole(g.Role)}, ""
}

// GrantTarget is the note path a grant link names: [[grid/people/dana/grants/x|label]],
// grid/people/dana/grants/x.md and /grid/people/dana/grants/x all name the same note.
func GrantTarget(link string) string {
	link = strings.TrimSpace(link)
	link = strings.TrimSuffix(strings.TrimPrefix(link, "[["), "]]")
	link, _, _ = strings.Cut(link, "|")
	link, _, _ = strings.Cut(link, "#")
	return strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(link), ".md"), "/")
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
