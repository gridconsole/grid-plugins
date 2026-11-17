package contract

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
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
	out, _ := FoldLoop(posts)
	return out
}

// Blocked is a Job whose attempt posted blocked and then exited: it waits for a
// person's approval and a new assign, not for judgement (V26).
const Blocked Status = "blocked"

// Loop is what one fold reads from the loop posts that State has no field for.
// Allow holds, by Job, the allow rules of the Job's current assign, and only when
// its grant names a counted approval by a person that approves every one of them
// for that Job (V22); any other allow folds absent. Landed holds, by Job, its
// counted landed post: what a landed after edge waits for (V27).
type Loop struct {
	Allow  map[string][]string `json:"allow"`
	Landed map[string]string   `json:"landed"`
	// Home is the machine that holds a per-post board's task now: the home of the
	// highest gen the posts name. A JSONL feed has none.
	Home string `json:"home,omitempty"`
}

// FoldLoop is Fold, also returning the Loop that the same pass read.
func FoldLoop(posts []Post) (State, Loop) { return foldLoop(posts, nil) }

// BoardPost is one post of a per-post board (internal/board/postfile): the post with
// its by, <machine>:<login>, in place of a worker, lease and fence, and the verdict
// a migrated post carried, in Legacy.
type BoardPost struct {
	Post
	By     string
	Legacy string
}

// FoldBoard folds a per-post board. It is FoldLoop with the board's rules:
//
//   - Posts sort by (gen, seq, id).
//   - The home of gen 0 is the home in the body of the first Job record; a counted
//     task-move of gen n names the home of gen n in its to. A control moves a Job
//     only when its author has an orchestrator, cli or launcher role and the machine
//     of its by is the home of its gen. Any other control, a person's always, is a
//     request: it counts and moves nothing, as an unleased control does on a JSONL
//     feed. Loop.Home is the current home.
//   - An assign must raise the Job's attempt. A Job-side post belongs to an assign
//     whose to is its author at the machine of its by.
//   - A second claim, a started or ack with the same Job, attempt, type and body
//     firing as an earlier one, folds ignored, "duplicate claim".
//   - Legacy "counted" is authority as it was counted on the JSONL feed it came
//     from, and legacy "stale" stays stale; any other legacy verdict is folded again.
//
// The body is validated against its type's schema as on a JSONL feed.
func FoldBoard(posts []BoardPost) (State, Loop) {
	plain := make([]Post, len(posts))
	info := make(map[string]boardInfo, len(posts))
	for i, p := range posts {
		plain[i] = p.Post
		if _, dup := info[p.ID]; !dup {
			info[p.ID] = boardInfo{by: p.By, legacy: p.Legacy}
		}
	}
	return foldLoop(plain, info)
}

type boardInfo struct{ by, legacy string }

// boardWorker stands in for the worker a board post has none of, so the envelope
// schema checks every other field as on a JSONL feed.
const boardWorker = "board@00000000000000000000000000"

// boardData is the post as the schemas read it: the stand-in worker, the given lease
// and fence, the tier of its author's role, and its by and legacy.
func boardData(p Post, b boardInfo, lease *string, fence int64) ([]byte, error) {
	p.Worker, p.Lease, p.Fence, p.Wake = boardWorker, lease, fence, nil
	p.Tier = TierFor(p.Author.Role)
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	m["by"], _ = json.Marshal(b.by)
	if b.legacy != "" {
		m["legacy"], _ = json.Marshal(b.legacy)
	}
	return json.Marshal(m)
}

// boardValid validates a board post against its type's schema with no lease, with the
// task lease and with its Job's lease, as a writer of each would have sent it. A body
// that one of them accepts is valid; the error is that of the first.
func boardValid(p Post, b boardInfo) error {
	// The likely lease first: a Job-side post's is its Job's, any other the task's.
	task := "task"
	leases := []*string{&task, nil}
	if p.Job != nil && p.Type != "control" && p.Type != "job" {
		leases = []*string{p.Job, &task, nil}
	} else if p.Job != nil {
		leases = append(leases, p.Job)
	}
	var first error
	for _, lease := range leases {
		fence := int64(1)
		if lease == nil {
			fence = 0
		}
		data, err := boardData(p, b, lease, fence)
		if err == nil {
			err = Validate(p.Type, data)
		}
		if err == nil {
			return nil
		}
		if first == nil {
			first = err
		}
	}
	return first
}

// boardMachine is the machine half of a by.
func boardMachine(by string) string {
	m, _, _ := strings.Cut(by, ":")
	return m
}

// boardController reports the roles whose controls the home counts: an orchestrator,
// cli or launcher role (docs/research/brain-board.md, the fold.go:192-198 row). Of
// these the envelope's role enum holds cli, the role the orchestrator pass writes
// with today. A person's control is always a request.
func boardController(role string) bool {
	switch NormalizeRole(role) {
	case "orchestrator", "cli", "launcher":
		return true
	}
	return false
}

func foldLoop(posts []Post, info map[string]boardInfo) (State, Loop) {
	board := info != nil
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
		var data []byte
		var err error
		if board {
			data, err = boardData(p, info[p.ID], nil, 0)
		} else {
			data, err = json.Marshal(p)
		}
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
	// A board sorts by (gen, seq, id); two posts with one seq tie by id (F5).
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if board {
			if a.Gen != b.Gen {
				return a.Gen < b.Gen
			}
			if a.Seq != b.Seq {
				return a.Seq < b.Seq
			}
			return a.ID < b.ID
		}
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
		if board && info[p.ID].legacy == "stale" {
			set(p, "stale", "legacy verdict stale")
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
		var err error
		if board {
			err = boardValid(p, info[p.ID])
		} else {
			data, _ := json.Marshal(p)
			err = Validate(p.Type, data)
		}
		if err != nil {
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
	var homeOf func(gen int64) string
	if board {
		active = boardClaims(active, set)
		homeOf = boardHomes(active, info)
		loop.Home = homeOf(math.MaxInt64)
	}
	// authority reports a board control the home of its gen wrote, or one a JSONL
	// feed counted before it was migrated.
	authority := func(p Post) bool {
		b := info[p.ID]
		if b.legacy == "counted" {
			return true
		}
		home := homeOf(p.Gen)
		return home != "" && boardController(p.Author.Role) && boardMachine(b.by) == home
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
		// On a board, a control the home of its gen did not write is the same request.
		if board && !authority(p) || !board && p.Lease == nil {
			continue
		}
		if b.Assign != nil {
			a := b.Assign
			var refused bool
			switch {
			case board && info[p.ID].legacy == "counted":
				refused = a.Attempt < j.Attempt
			case board:
				refused = a.Attempt <= j.Attempt
			default:
				sameRetry := a.Attempt == j.Attempt && j.ExitReason == "usage_limit" && j.Assign != nil && a.Fence > j.Assign.Fence
				refused = a.Attempt < j.Attempt || a.Attempt == j.Attempt && !sameRetry
			}
			if refused || j.Status == Accepted || j.Status == Cancelled || j.Status == Rejected {
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
				if !isJobSide(side.Type) || side.Job == nil || *side.Job != id || side.Attempt == nil || *side.Attempt != a.Attempt || consumed[side.ID] {
					continue
				}
				if board {
					if s := info[side.ID]; s.legacy != "counted" && side.Author.ID+"@"+boardMachine(s.by) != a.To {
						continue
					}
				} else if side.Worker != a.To || side.Fence != a.Fence || side.Gen != p.Gen {
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
			// On a board the home's head is ordered, by (gen, seq).
			leased := deref(p.Lease) == "task"
			if board {
				leased = authority(p)
			}
			if leased && (head == nil || newer(p, *head)) {
				h := p
				head = &h
			}
		case "host":
			key := p.Worker
			if board {
				key = info[p.ID].by
			}
			if old, ok := hosts[key]; !ok || newer(p, old) {
				hosts[key] = p
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

// boardClaims folds ignored each started or ack whose Job, attempt, type and body
// firing an earlier one in (gen, seq, id) order holds: two machines' copies of one
// claim met after sync, and the first counts. It answers the rest.
func boardClaims(active []Post, set func(Post, string, string)) []Post {
	claimed := map[string]bool{}
	out := make([]Post, 0, len(active))
	for _, p := range active {
		if (p.Type == "started" || p.Type == "ack") && p.Job != nil && p.Attempt != nil {
			var b struct {
				Firing json.RawMessage `json:"firing"`
			}
			_ = json.Unmarshal(p.Body, &b)
			key := *p.Job + "\x00" + strconv.Itoa(*p.Attempt) + "\x00" + p.Type + "\x00" + string(b.Firing)
			if claimed[key] {
				set(p, "ignored", "duplicate claim")
				continue
			}
			claimed[key] = true
		}
		out = append(out, p)
	}
	return out
}

// boardHomes reads a board's homes from its counted posts: gen 0's is the home in the
// first Job record that names one, and gen n's is the to of the first task-move of
// gen n by a person of the team. It answers the home of a gen: that of the highest gen
// at or below it that has one. task.md's home: line is a display copy and is not read.
func boardHomes(active []Post, info map[string]boardInfo) func(int64) string {
	homes := map[int64]string{}
	for _, p := range active {
		switch p.Type {
		case "job":
			if _, ok := homes[0]; ok {
				continue
			}
			var b struct {
				Home string `json:"home"`
			}
			if json.Unmarshal(p.Body, &b) == nil && b.Home != "" {
				homes[0] = b.Home
			}
		case "task-move":
			if _, ok := homes[p.Gen]; ok || p.Gen < 1 {
				continue
			}
			if TierFor(p.Author.Role) != PersonTeam && info[p.ID].legacy != "counted" {
				continue
			}
			var b struct {
				To string `json:"to"`
			}
			if json.Unmarshal(p.Body, &b) == nil && b.To != "" {
				homes[p.Gen] = b.To
			}
		}
	}
	gens := make([]int64, 0, len(homes))
	for g := range homes {
		gens = append(gens, g)
	}
	sort.Slice(gens, func(i, j int) bool { return gens[i] < gens[j] })
	return func(gen int64) string {
		home := ""
		for _, g := range gens {
			if g > gen {
				break
			}
			home = homes[g]
		}
		return home
	}
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
