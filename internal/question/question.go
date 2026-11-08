// Package question stores task questions and applies their deadline decisions.
package question

import (
	"errors"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"io"
	"slices"
	"strings"
	"time"
)

const DefaultAgeAfter = 8 * time.Hour

// FeedTime is the Feed's timestamp layout: UTC with exactly three fraction digits.
const FeedTime = "2006-01-02T15:04:05.000Z"

// Stamp is an instant written in FeedTime, so a record's times post unchanged.
type Stamp struct{ time.Time }

func (t Stamp) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.UTC().Format(FeedTime) + `"`), nil
}

// Record is the question-file contract, validated by internal/contract at the
// storage boundary. History and current state are committed in the same rename.
type Record struct {
	ID              string   `json:"id"`
	Post            string   `json:"post"`
	Job             string   `json:"job,omitempty"`
	Item            int      `json:"item,omitempty"`
	Question        string   `json:"question"`
	Who             string   `json:"who"`
	Why             string   `json:"why,omitempty"`
	Blocking        bool     `json:"blocking"`
	Options         []string `json:"options,omitempty"`
	Recommended     string   `json:"recommended,omitempty"`
	Gate            Gate     `json:"gate"`
	Asked           Stamp    `json:"asked"`
	State           string   `json:"state"`
	Answer          string   `json:"answer"`
	AgeAfterSeconds int64    `json:"age_after_seconds"`
	Aging           int      `json:"aging"`
	History         []Event  `json:"history"`
}

// Gate holds the default and its time as the Feed question post's gate does.
type Gate struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Default string `json:"default"`
	At      Stamp  `json:"at"`
}

type Event struct {
	Post string `json:"post"`
	Role string `json:"role"`
	Kind string `json:"kind"`
	At   Stamp  `json:"at"`
	Who  string `json:"who"`
	Text string `json:"text"`
}

type Input struct {
	Job                  string
	Item                 int
	Question, Who, Why   string
	Options              []string
	Recommended, Default string
	Applies              time.Time
	Blocking             bool
	AgeAfter             time.Duration
}

// ErrWithdrawRefused marks a withdrawal the caller may not apply.
var ErrWithdrawRefused = errors.New("question withdrawal refused")

// DefaultWithdrawAllowed allows owners and members to withdraw without a lease.
func DefaultWithdrawAllowed(role string) bool { return feed.DefaultWithdrawAllowed(role) }

type Store struct {
	// WithdrawAllowed may be replaced by a host; nil reads the workflow policy
	// or uses the owner/member default when no policy is specified.
	WithdrawAllowed func(role string) bool
	AgentShell      bool
	TaskDir         string
	TaskID          string
	Writer          feed.Writer
	ControlWriter   feed.Writer
	Now             func() time.Time
	Warn            io.Writer // receives warnings about skipped files; nil discards them
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC().Truncate(time.Millisecond)
	}
	return time.Now().UTC().Truncate(time.Millisecond)
}

func validateInput(in Input) error {
	if strings.TrimSpace(in.Question) == "" || strings.TrimSpace(in.Who) == "" {
		return errors.New("question and who are required")
	}
	if in.Item < 0 || in.Job != "" && !jobID.MatchString(in.Job) {
		return errors.New("invalid job or plan item")
	}
	seen := map[string]bool{}
	for _, option := range in.Options {
		if strings.TrimSpace(option) == "" || seen[option] {
			return errors.New("options must be nonempty and distinct")
		}
		seen[option] = true
	}
	if len(in.Options) > 0 && !slices.Contains(in.Options, in.Recommended) {
		return errors.New("recommended must name an option")
	}
	if in.Why != "" && strings.TrimSpace(in.Why) == "" {
		return errors.New("why must not be blank")
	}
	if strings.TrimSpace(in.Default) == "" || in.Applies.IsZero() {
		return errors.New("default and applies time are required")
	}
	if in.AgeAfter < 0 || in.AgeAfter > time.Duration(1<<63-1)/2 || in.AgeAfter > 0 && in.AgeAfter%time.Second != 0 {
		return errors.New("age-after must be a positive whole number of seconds within the supported range")
	}
	return nil
}

func (s Store) Add(in Input) (rec Record, err error) {
	if err = validateInput(in); err != nil {
		return
	}
	err = s.locked(true, func(dir string) error {
		if _, e := s.project(dir, true); e != nil {
			return e
		}
		next, e := nextID(dir, 1)
		if e != nil {
			return e
		}
		age := in.AgeAfter
		if age == 0 {
			age = DefaultAgeAfter
		}
		rec = Record{Job: in.Job, Item: in.Item, ID: fmt.Sprintf("Q-%d", next), Question: in.Question, Who: in.Who, Why: in.Why, Blocking: in.Blocking, Options: append([]string{}, in.Options...), Recommended: in.Recommended, Gate: Gate{ID: fmt.Sprintf("Q-%d", next), Kind: "person", Default: in.Default, At: Stamp{in.Applies.UTC().Truncate(time.Millisecond)}}, Asked: Stamp{s.now()}, State: "open", AgeAfterSeconds: int64(age / time.Second), History: []Event{}}
		return s.publishQuestion(dir, &rec)
	})
	return
}

// List projects the Feed. A task whose task.md has no ULID has no Feed, so its
// question files are listed as written.
func (s Store) List() ([]Record, error) {
	dir, err := s.directory(false)
	if err != nil {
		return nil, err
	}
	if s.TaskID == "" {
		if _, err := feed.TaskID(s.TaskDir); errors.Is(err, feed.ErrNoTaskID) {
			return s.readFiles(dir)
		}
	}
	return s.project(dir, false)
}

// Answer accepts option labels or free text. Repeating an identical answer is a
// no-op; a different human answer cannot silently overwrite an earlier answer.
// A default is provisional and always permits a subsequent human answer.
func (s Store) Answer(id, text string) (Record, error) {
	switch s.Writer.Author.Role {
	case "owner", "member", "lead", "guest":
	default:
		return Record{}, errors.New("only a person may answer a person gate")
	}
	if strings.TrimSpace(text) == "" {
		return Record{}, errors.New("answer must not be empty")
	}
	return s.change(id, func(r *Record) error {
		if s.Writer.Author.Role == "guest" && s.Writer.Author.ID != r.Who {
			return errors.New("guest answer must name a question addressed to that guest")
		}
		if r.State == "answered" && r.Answer == text {
			return nil
		}
		if r.State != "open" && r.State != "defaulted" {
			return fmt.Errorf("%s is %s", id, r.State)
		}
		r.State = "answered"
		r.Answer = text
		r.History = append(r.History, Event{Kind: "answered", At: Stamp{s.now()}, Text: text})
		return nil
	})
}

// Withdraw closes an open question for an allowed person or a task lease holder.
func (s Store) Withdraw(id, reason string) (Record, error) {
	if s.ControlWriter.Lease != "task" {
		allowed, err := s.withdrawPolicy()
		if err != nil {
			return Record{}, err
		}
		if !allowed(s.Writer.Author.Role) {
			return Record{}, &withdrawRefusal{fmt.Sprintf("%s role %q may not withdraw", id, s.Writer.Author.Role)}
		}
		if s.AgentShell || s.Writer.Author.Role == "agent" || s.Writer.Author.Role == "cli" {
			return Record{}, &withdrawRefusal{fmt.Sprintf("%s agent shell requires a task lease to withdraw", id)}
		}
	}
	return s.change(id, func(r *Record) error {
		if r.State == "withdrawn" {
			return nil
		}
		if r.State != "open" {
			return &withdrawRefusal{fmt.Sprintf("%s is %s", id, r.State)}
		}
		r.State = "withdrawn"
		r.History = append(r.History, Event{Kind: "withdrawn", At: Stamp{s.now()}, Text: reason})
		return nil
	})
}

type withdrawRefusal struct{ message string }

func (e *withdrawRefusal) Error() string { return e.message }
func (e *withdrawRefusal) Unwrap() error { return ErrWithdrawRefused }

func (s Store) change(id string, fn func(*Record) error) (rec Record, err error) {
	if !validID.MatchString(id) {
		return rec, errors.New("question id must be Q- followed by a positive number")
	}
	err = s.locked(true, func(dir string) error {
		if dir == "" {
			return fmt.Errorf("question %s does not exist", id)
		}
		records, e := s.project(dir, false)
		if e != nil {
			return e
		}
		var r Record
		for _, candidate := range records {
			if candidate.ID == id {
				r = candidate
				break
			}
		}
		if r.ID == "" {
			return fmt.Errorf("question %s does not exist", id)
		}
		if r.Post == "" {
			return fmt.Errorf("question %s is not in the Feed; migrate it before changing it", id)
		}
		before := len(r.History)
		if e = fn(&r); e != nil {
			return e
		}
		if _, e = s.project(dir, true); e != nil {
			return e
		}
		rec = r
		if len(r.History) == before {
			return nil
		}
		if e = s.publishEvents(dir, &r, before); e != nil {
			return e
		}
		rec = r
		return nil
	})
	return
}

// Tick records due default and aging audit events under the task lock.
// Deadlines already release gates on reads; the default control is optional
// audit history. The returned slice contains only newly recorded events.
func (s Store) Tick() (changed []Record, err error) {
	changed = []Record{}
	err = s.locked(true, func(dir string) error {
		records, e := s.project(dir, true)
		if e != nil {
			return e
		}
		now := s.now()
		for _, r := range records {
			if r.Post == "" || r.State != "open" && r.State != "defaulted" {
				continue
			}
			if slices.ContainsFunc(r.History, func(e Event) bool { return e.Kind == "defaulted" }) {
				continue
			}
			before := len(r.History)
			if !now.Before(r.Gate.At.Time) {
				r.State = "defaulted"
				r.Answer = r.Gate.Default
				r.History = append(r.History, Event{Kind: "defaulted", At: Stamp{now}, Who: "cli", Text: r.Gate.Default})
			} else if r.Blocking {
				age := time.Duration(r.AgeAfterSeconds) * time.Second
				for r.Aging < 2 && !now.Before(r.Asked.Add(time.Duration(r.Aging+1)*age)) {
					r.Aging++
					r.History = append(r.History, Event{Kind: "aged", At: Stamp{now}, Who: "cli", Text: fmt.Sprintf("Priority raised to %d", r.Aging)})
				}
			}
			if len(r.History) != before {
				if e = s.publishEvents(dir, &r, before); e != nil {
					return e
				}
				changed = append(changed, r)
			}
		}
		return nil
	})
	return
}

// withdrawPolicy shares workflow policy with Feed state and permits host overrides.
func (s Store) withdrawPolicy() (func(string) bool, error) {
	if s.WithdrawAllowed != nil {
		return s.WithdrawAllowed, nil
	}
	return feed.QuestionWithdrawPolicy(s.TaskDir)
}
