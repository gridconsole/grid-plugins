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

// TierFor maps a stamped author role to the tier the envelope schema requires.
// Unknown roles map to agent; Validate refuses them by the role enum.
func TierFor(role string) TrustTier {
	switch role {
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

type LevelRaised struct {
	From string `json:"from"`
	To   string `json:"to"`
	Why  string `json:"why"`
}

// Job is the materialized run state. Retries are explicit assignments; the fold
// never spawns a worker, increments an attempt on its own, or consults a clock.
type Job struct {
	ID              string          `json:"id"`
	Task            string          `json:"task"`
	Title           string          `json:"title"`
	Role            string          `json:"role"`
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
	Review          string          `json:"review"`
	LevelRaised     *LevelRaised    `json:"level_raised"`
	ReviewRounds    int             `json:"review_rounds"`
	MaxRounds       int             `json:"max_rounds"`
	Continues       *string         `json:"continues"`
	DiscoveredFrom  *string         `json:"discovered_from"`
	ProposedJobs    []ProposedJob   `json:"proposed_jobs"`
	DecisionsNeeded []string        `json:"decisions_needed"`
	Verified        json.RawMessage `json:"verified"`
	NeedsLead       bool            `json:"needs_lead"`
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

// MergeLease belongs to a project, not a task. A release clears Holder. Its
// fifteen-minute window starts at RenewedAt, or Since when never renewed.
type MergeLease struct {
	Lease     string  `json:"lease"`
	Project   string  `json:"project"`
	Holder    *string `json:"holder"`
	Fence     int64   `json:"fence"`
	Gen       int64   `json:"gen"`
	Since     string  `json:"since"`
	RenewedAt *string `json:"renewed_at,omitempty"`
	Until     string  `json:"until"`
}

// Breakable reports release or expiry; a malformed held lease is not breakable.
func (l MergeLease) Breakable(now time.Time) bool {
	data, err := json.Marshal(l)
	if err != nil || Validate("merge-lease", data) != nil {
		return false
	}
	end, _ := time.Parse(time.RFC3339Nano, l.Until)
	return l.Holder == nil || !now.Before(end)
}

func isJobSide(typ string) bool {
	switch typ {
	case "ack", "submit", "result", "checkpoint", "worker-exit":
		return true
	}
	return false
}
