package contract

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type TrustTier string

const (
	PersonTeam  TrustTier = "person-team"
	PersonGuest TrustTier = "person-guest"
	Agent       TrustTier = "agent"
	External    TrustTier = "external"
)

// NormalizeRole accepts caller roles with an optional plugin prefix.
func NormalizeRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "grid-" {
		return role
	} // A bare prefix is unknown, not a local CLI.
	return strings.TrimPrefix(role, "grid-")
}

// TierFor maps a stamped author role to the tier the envelope schema requires.
// Unknown roles map to agent; Validate refuses them by the role enum.
func TierFor(role string) TrustTier {
	switch NormalizeRole(role) {
	case "owner", "member", "lead":
		return PersonTeam
	case "guest":
		return PersonGuest
	case "external":
		return External
	}
	return Agent
}

type Author struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

// workerPattern is the envelope's worker pattern; a test keeps them equal.
var workerPattern = regexp.MustCompile(`^[^/\s]+@[0-7][0-9A-HJKMNP-TV-Z]{25}$`)

// ValidWorker reports whether id is a writer id, <member>@<machine ULID>. The
// member may hold any character but / and whitespace, including another @.
func ValidWorker(id string) bool { return workerPattern.MatchString(id) }

// WorkerMember returns the member part of a writer id, split on the last @.
func WorkerMember(id string) string {
	if i := strings.LastIndexByte(id, '@'); i > 0 {
		return id[:i]
	}
	return ""
}

// Post is the wire envelope. Stale is reader metadata, never a JSON field:
// the Feed reader sets it from takeover end offsets before calling Fold.
// Body references must be resolved and validated before folding.
type Post struct {
	ID      string          `json:"id"`
	Task    string          `json:"task"`
	Type    string          `json:"type"`
	TypeV   int             `json:"type_v"`
	Author  Author          `json:"author"`
	Tier    TrustTier       `json:"tier"`
	Worker  string          `json:"worker"`
	Lease   *string         `json:"lease"`
	Gen     int64           `json:"gen"`
	Fence   int64           `json:"fence"`
	Seq     int64           `json:"seq"`
	At      string          `json:"at"`
	Re      *string         `json:"re"`
	After   []string        `json:"after"`
	Job     *string         `json:"job"`
	Attempt *int            `json:"attempt"`
	Wake    *string         `json:"wake"`
	Body    json.RawMessage `json:"body"`
	Stale   bool            `json:"-"`
}

type Status string

const (
	Posted    Status = "posted"
	Ready     Status = "ready"
	Assigned  Status = "assigned"
	Running   Status = "running"
	Submitted Status = "submitted"
	Accepted  Status = "accepted"
	Stale     Status = "stale"
	Parked    Status = "parked"
	Rejected  Status = "rejected"
	Cancelled Status = "cancelled"
	// Held is a Job the reviewer confirmed waits on something outside it (Job,
	// landing or question): open, with no person point and no rework. A control
	// from the Judge sets it with the waits it copies from the waiting rows; a
	// wait on a permission stays Blocked (docs/decisions/D-job-hold.md).
	Held Status = "held"
)

type Assignment struct {
	Job     string `json:"job"`
	Attempt int    `json:"attempt"`
	To      string `json:"to"`
	Fence   int64  `json:"fence"`
	Until   string `json:"until"`
}

type Checkpoint struct {
	Job    string `json:"job"`
	Step   string `json:"step"`
	Commit string `json:"commit"`
}

// Job is the materialized run state. Retries are explicit assignments; the fold
// never spawns a worker, increments an attempt on its own, or consults a clock.
// ExitCode is the worker-exit's own code (an integer or a non-empty string, as
// the writer sent it); ExitReasonMissing says the worker-exit gave no reason, so
// ExitReason reads crash only by default.
type Job struct {
	ID    string `json:"id"`
	Task  string `json:"task"`
	Title string `json:"title"`
	Role  string `json:"role"`
	// Kind is the Job's free-form kind from its job post. Kind land marks a Job
	// that lands one accepted change; its record also carries target, branch and
	// sha (the job-record body), which feed read holds and the state does not fold.
	Kind            string          `json:"kind,omitempty"`
	Repo            string          `json:"repo,omitempty"`   // the repo path the Job names; empty is the project's first repo
	Base            string          `json:"base,omitempty"`   // the branch the Job's clone starts from; empty names none
	Model           string          `json:"model,omitempty"`  // the Job's own model; empty keeps the role's
	Effort          string          `json:"effort,omitempty"` // the Job's own effort; empty keeps the role's
	Status          Status          `json:"status"`
	Attempt         int             `json:"attempt"`
	Assign          *Assignment     `json:"assign"`
	Generation      int64           `json:"gen"`
	AssignPost      string          `json:"assign_post"`
	LeaseExpires    string          `json:"lease_expires"`
	Reason          string          `json:"reason"`
	CloseReason     *string         `json:"close_reason"`
	Results         []string        `json:"results"`
	Submits         []string        `json:"submits"`
	History         []string        `json:"history"`
	OpenQuestions   []string        `json:"open_questions"`
	Checkpoint      *Checkpoint     `json:"checkpoint"`
	Exited          string          `json:"exited"`
	ExitReason      string          `json:"exit_reason"`
	ReviewRounds    int             `json:"review_rounds"`
	Continues       *string         `json:"continues"`
	DiscoveredFrom  *string         `json:"discovered_from"`
	ProposedJobs    []ProposedJob   `json:"proposed_jobs"`
	DecisionsNeeded []string        `json:"decisions_needed"`
	Verified        json.RawMessage `json:"verified"`
	NeedsLead       bool            `json:"needs_lead"`
	Order           *int            `json:"order,omitempty"`
	// Acceptance holds the Job record's acceptance rows in record order, each with
	// the latest state a checkpoint or result of the Job set; a row nothing has
	// marked reads pending.
	Acceptance []AcceptanceRow `json:"acceptance"`
	// WaitsOn is what a Held Job waits on, copied from the control that held it, or
	// what a Blocked Job's waiting rows name. A later assign or a control that moves
	// the Job to another status clears it; a control with no status leaves it.
	WaitsOn     []HeldWait `json:"waits_on,omitempty"`
	State       string     `json:"state,omitempty"`
	StateReason string     `json:"state_reason,omitempty"`

	ExitCode          json.RawMessage `json:"exit_code,omitempty"`
	ExitReasonMissing bool            `json:"exit_reason_missing,omitempty"`
}

// The four states a Job reads in. The stored Status stays the record; these are
// how it reads to a person and the orchestrator.
const (
	StateNotStarted  = "Not started"
	StateStarted     = "Started"
	StateNeedsReview = "Needs review"
	StateDone        = "Done"
)

// StateOf maps a stored status to one of the four states. Cancelled reads as
// Done with the reason "cancelled". integrated (in the control schema, not the
// Status constants) reads as Needs review; Blocked reads as Started with the
// reason "blocked" and Held as Started with the reason "held"; any other status
// reads as Not started.
func StateOf(s Status) (state, reason string) {
	switch s {
	case Assigned, Running:
		return StateStarted, ""
	case Blocked:
		return StateStarted, "blocked"
	case Held:
		return StateStarted, "held"
	case Submitted, "integrated":
		return StateNeedsReview, ""
	case Accepted:
		return StateDone, ""
	case Cancelled:
		return StateDone, "cancelled"
	}
	return StateNotStarted, ""
}

// The states an acceptance row reads in. A worker may write not-met, the word job
// lint knows; the fold reads it as not met, the word of the result schema.
// RowWaiting is a row that cannot hold until something outside the Job lands
// (docs/decisions/D-job-hold.md); it is never not met.
const (
	RowPending = "pending"
	RowMet     = "met"
	RowNotMet  = "not met"
	RowDropped = "dropped"
	RowWaiting = "waiting"
)

// The kinds of thing a waiting row waits on, with the ref each names: another
// Job accepted (J-n), another Job landed (J-n), a question answered (Q-n), or a
// permission granted (the id of the blocked post that names the rule).
const (
	WaitJob        = "job"
	WaitLanded     = "landed"
	WaitQuestion   = "question"
	WaitPermission = "permission"
)

// Wait is what a waiting row waits on.
type Wait struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// HeldWait is one thing a held Job waits on: the acceptance row that cannot hold
// yet, and what it waits on. Met is the answer of whoever evaluates the wait (the
// orchestrator's tree and Ready); the fold never sets it, so it is nil there.
type HeldWait struct {
	Row  string `json:"row"`
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
	Met  *bool  `json:"met,omitempty"`
}

// AcceptanceRow is one acceptance row of a Job as the fold reads it. Note is the
// note that came with the latest mark; WaitsOn is set only on a waiting row.
type AcceptanceRow struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Note    string `json:"note,omitempty"`
	WaitsOn *Wait  `json:"waits_on,omitempty"`
}

// rowState reads a written row state in the fold's words: not-met is not met and
// an empty state is pending.
func rowState(s string) string {
	switch s {
	case "", RowPending:
		return RowPending
	case "not-met":
		return RowNotMet
	}
	return s
}

type ProposedJob struct {
	Title          string `json:"title"`
	Role           string `json:"role"`
	Why            string `json:"why"`
	Gap            string `json:"gap"`
	DiscoveredFrom string `json:"discovered_from"`
}

type Verdict struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"` // counted, ignored, stale, bad, held, unknown
	Reason string `json:"reason"`
}

// Question is a counted question with its answer candidates. Only a task
// control that selects one of them closes it; arrival order never does.
type Question struct {
	ID      string   `json:"id"`
	Job     *string  `json:"job"`
	Who     string   `json:"who"`
	Answers []string `json:"answers"`
	Applied *string  `json:"applied"`
	Open    bool     `json:"open"`
}

// State is the folded task. NeedsYou lists open questions, task-wide; Wakes
// lists lead wake candidates (security findings on a known Job, loop-detected),
// kept apart from human questions. ByRef indexes counted posts by their ref,
// Head is the newest task-lease head and Hosts the newest host post per worker.
type State struct {
	Jobs      map[string]*Job      `json:"jobs"`
	Verdicts  []Verdict            `json:"verdicts"`
	Questions map[string]*Question `json:"questions"`
	NeedsYou  []string             `json:"needs_you"`
	Wakes     []string             `json:"wakes"`
	ByRef     map[string][]string  `json:"by_ref"`
	Head      *string              `json:"head"`
	Hosts     map[string]string    `json:"hosts"`
}

func isJobSide(typ string) bool {
	switch typ {
	case "ack", "submit", "result", "checkpoint", "worker-exit":
		return true
	}
	return false
}

// TimeLayout is the fixed-width UTC timestamp the schemas require; Go's
// RFC3339Nano trims trailing zeros and fails their pattern.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// FormatTime renders t as UTC with exactly three fractional digits.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }
