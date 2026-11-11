// Package usage records task usage deltas without collecting prompts or transcripts.
package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/localdata"
)

// Input carries one delta. Nil amounts distinguish omission from an explicit zero.
type Input struct {
	Job, Session, Model string
	// Key, when set, books the delta once: a post whose Key is already in the log appends nothing.
	Key              string
	Dollars, Minutes *float64
	Tokens           *uint64
}

// Record is one complete line of usage/usage.jsonl. Amounts are always deltas.
type Record struct {
	V       int     `json:"v"`
	At      string  `json:"at"`
	Job     string  `json:"job"`
	Session string  `json:"session"`
	Model   string  `json:"model"`
	Dollars float64 `json:"dollars"`
	Minutes float64 `json:"minutes"`
	Tokens  *uint64 `json:"tokens,omitempty"`
	// Key is the once-only key of a row booked from a run's result line; absent on typed posts.
	Key string `json:"key,omitempty"`
}

// Store is scoped to an existing task resolved by where. Now is optional.
type Store struct {
	TaskPath string
	GridHome string
	Now      func() time.Time
}

// PostResult is the usage log after one post. Usage is a number: a post never
// writes a notice and nothing is compared with a limit. Notices and Warnings stay
// in the answer, always empty, so readers of the post JSON keep their shape.
type PostResult struct {
	Summary
	Notices  []feed.Record `json:"notices"`
	Warnings []string      `json:"warnings"`
	// AlreadyBooked is true when the input's Key was already in the log and nothing was appended.
	AlreadyBooked bool `json:"alreadyBooked,omitempty"`
}

// ParseTokens accepts only ASCII decimal digits in the uint64 range.
func ParseTokens(value string) (uint64, error) {
	if value == "" || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, errors.New("tokens must be a non-negative integer in the uint64 range")
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.New("tokens must be a non-negative integer in the uint64 range")
	}
	return n, nil
}

var jobPattern = regexp.MustCompile(`^(lead|J-[1-9][0-9]*(\.[1-9][0-9]*)*)$`)
var sessionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@+-]{0,159}$`)
var modelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./:@+-]{0,159}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@+-]{0,199}$`)

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 }

func (in Input) validate() error {
	if !jobPattern.MatchString(in.Job) {
		return errors.New("job must be lead or J-N with positive numeric components")
	}
	if in.Dollars == nil && in.Minutes == nil && in.Tokens == nil {
		return errors.New("at least one of --dollars, --minutes or --tokens is required")
	}
	for name, n := range map[string]*float64{"dollars": in.Dollars, "minutes": in.Minutes} {
		if n != nil && !finite(*n) {
			return fmt.Errorf("%s must be a non-negative finite number", name)
		}
	}
	if in.Session != "" && !sessionPattern.MatchString(in.Session) {
		return errors.New("session must be an identifier, not a path or text")
	}
	if in.Model != "" && !modelPattern.MatchString(in.Model) {
		return errors.New("model must be a name of at most 160 characters, not text")
	}
	if in.Key != "" && !keyPattern.MatchString(in.Key) {
		return errors.New("key must be an identifier, not a path or text")
	}
	return nil
}

// taskDir validates the task path and returns its usage folder path.
func (s Store) taskDir() (string, error) {
	if s.TaskPath == "" {
		return "", errors.New("a task is required")
	}
	info, err := os.Stat(s.TaskPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("task is not a directory")
	}
	return localdata.Safe(s.TaskPath, "usage")
}

// readData is lock-free and creates nothing: appends end in a complete line or a
// torn tail that readLog ignores, and the configuration is replaced atomically.
func (s Store) readData(fn func(string) error) error {
	dir, err := s.taskDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fn("")
	} else if err != nil {
		return err
	}
	return fn(dir)
}

// withData holds the usage fslock for appends and configuration changes.
// The lock inode is never replaced when data files are replaced atomically.
func (s Store) withData(fn func(string) error) error {
	dir, err := s.taskDir()
	if err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := atomicfile.SyncDir(s.TaskPath); err != nil {
		return err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return err
	}
	defer fslock.Unlock(f)
	return fn(dir)
}

// Show folds complete log lines read-only, leaving any torn final line untouched.
func (s Store) Show() (Summary, error) {
	out := emptySummary()
	err := s.readData(func(dir string) error {
		var err error
		out, _, err = readLog(dir)
		return err
	})
	return out, err
}

// Post durably appends one delta, repairing an incomplete tail under the lock.
func (s Store) Post(in Input) (PostResult, error) {
	if err := in.validate(); err != nil {
		return PostResult{}, err
	}
	out := PostResult{Summary: emptySummary(), Notices: []feed.Record{}, Warnings: []string{}}
	err := s.withData(func(dir string) error {
		before, end, err := readLog(dir)
		if err != nil {
			return err
		}
		if in.Key != "" && before.keys[in.Key] {
			out.Summary, out.AlreadyBooked = before, true
			return nil
		}
		now := time.Now
		if s.Now != nil {
			now = s.Now
		}
		r := Record{V: 1, At: contract.FormatTime(now()), Job: in.Job, Session: in.Session, Model: in.Model, Tokens: in.Tokens, Key: in.Key}
		if in.Dollars != nil {
			r.Dollars = *in.Dollars
		}
		if in.Minutes != nil {
			r.Minutes = *in.Minutes
		}
		if err := before.add(r); err != nil {
			return err
		}
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		f, err := fslock.OpenNoFollow(filepath.Join(dir, "usage.jsonl"), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := f.Truncate(end); err != nil {
			return err
		}
		if _, err := f.Seek(end, io.SeekStart); err != nil {
			return err
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			return err
		}
		if err := atomicfile.SyncDir(dir); err != nil {
			return err
		}
		out.Summary = before
		return nil
	})
	return out, err
}
