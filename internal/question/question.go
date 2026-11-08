// Package question stores task questions and applies their deadline decisions.
package question

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
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
	Question        string   `json:"question"`
	Who             string   `json:"who"`
	Why             string   `json:"why,omitempty"`
	Blocking        bool     `json:"blocking"`
	Options         []string `json:"options"`
	Recommended     string   `json:"recommended"`
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
	Default string `json:"default"`
	At      Stamp  `json:"at"`
}

type Event struct {
	Kind string `json:"kind"`
	At   Stamp  `json:"at"`
	Who  string `json:"who"`
	Text string `json:"text"`
}

type Input struct {
	Question, Who, Why   string
	Options              []string
	Recommended, Default string
	Applies              time.Time
	Blocking             bool
	AgeAfter             time.Duration
}

type Store struct {
	TaskDir string
	Now     func() time.Time
	Warn    io.Writer // receives warnings about skipped files; nil discards them
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
	if len(in.Options) == 0 {
		return errors.New("at least one option is required")
	}
	seen := map[string]bool{}
	for _, option := range in.Options {
		if strings.TrimSpace(option) == "" || seen[option] {
			return errors.New("options must be nonempty and distinct")
		}
		seen[option] = true
	}
	if !slices.Contains(in.Options, in.Recommended) {
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
		records, e := s.readAll(dir)
		if e != nil {
			return e
		}
		next := 1
		for _, r := range records {
			n, _ := strconv.Atoi(strings.TrimPrefix(r.ID, "Q-"))
			if n == int(^uint(0)>>1) {
				return errors.New("question id range exhausted")
			}
			if n >= next {
				next = n + 1
			}
		}
		age := in.AgeAfter
		if age == 0 {
			age = DefaultAgeAfter
		}
		rec = Record{ID: fmt.Sprintf("Q-%d", next), Question: in.Question, Who: in.Who, Why: in.Why, Blocking: in.Blocking, Options: append([]string{}, in.Options...), Recommended: in.Recommended, Gate: Gate{Default: in.Default, At: Stamp{in.Applies.UTC().Truncate(time.Millisecond)}}, Asked: Stamp{s.now()}, State: "open", AgeAfterSeconds: int64(age / time.Second), History: []Event{}}
		return save(dir, rec)
	})
	return
}

func (s Store) List() (records []Record, err error) {
	records = []Record{}
	err = s.locked(false, func(dir string) error { var e error; records, e = s.readAll(dir); return e })
	return
}

// Answer accepts option labels or free text. Repeating an identical answer is a
// no-op; a different human answer cannot silently overwrite an earlier answer.
// A default is provisional and always permits a subsequent human answer.
func (s Store) Answer(id, text, who string) (Record, error) {
	if strings.TrimSpace(text) == "" {
		return Record{}, errors.New("answer must not be empty")
	}
	return s.change(id, func(r *Record) error {
		if r.State == "answered" && r.Answer == text {
			return nil
		}
		if r.State != "open" && r.State != "defaulted" {
			return fmt.Errorf("%s is %s", id, r.State)
		}
		if who == "" {
			who = r.Who
		}
		r.State = "answered"
		r.Answer = text
		r.History = append(r.History, Event{Kind: "answered", At: Stamp{s.now()}, Who: who, Text: text})
		return nil
	})
}

func (s Store) Withdraw(id, reason string) (Record, error) {
	return s.change(id, func(r *Record) error {
		if r.State == "withdrawn" {
			return nil
		}
		if r.State != "open" {
			return fmt.Errorf("%s is %s", id, r.State)
		}
		r.State = "withdrawn"
		r.History = append(r.History, Event{Kind: "withdrawn", At: Stamp{s.now()}, Who: "cli", Text: reason})
		return nil
	})
}

func (s Store) change(id string, fn func(*Record) error) (rec Record, err error) {
	if !validID.MatchString(id) {
		return rec, errors.New("question id must be Q- followed by a positive number")
	}
	err = s.locked(false, func(dir string) error {
		if dir == "" {
			return fmt.Errorf("question %s does not exist", id)
		}
		r, e := read(dir, id)
		if e != nil {
			return e
		}
		before := len(r.History)
		if e = fn(&r); e != nil {
			return e
		}
		rec = r
		if len(r.History) == before {
			return nil
		}
		return save(dir, r)
	})
	return
}

// Tick commits a due default and its audit event together under the task lock.
// It has no external side effects to replay after a crash. The returned slice
// contains only records changed by this invocation.
func (s Store) Tick() (changed []Record, err error) {
	changed = []Record{}
	err = s.locked(false, func(dir string) error {
		records, e := s.readAll(dir)
		if e != nil {
			return e
		}
		now := s.now()
		for _, r := range records {
			if r.State != "open" {
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
				if e = save(dir, r); e != nil {
					return e
				}
				changed = append(changed, r)
			}
		}
		return nil
	})
	return
}
