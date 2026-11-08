package task

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/question"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// AttachOptions identifies the terminal's parent process and runtime session.
// Agent callers must supply their assigned task or run inside that task folder.
type AttachOptions struct {
	Cwd, ID, SessionID, Runtime, PIDStart string
	PID                                   int
	Who                                   contract.Author
	Agent                                 bool
	BoundTask                             string
}

// Attachment reports task facts and the session key. Holders are advisory only;
// neither attaching nor reading these facts grants or breaks a lease.
type Attachment struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	State         string      `json:"state"`
	Approved      bool        `json:"approved"`
	OpenQuestions int         `json:"open_questions"`
	Session       session.Key `json:"session"`
	Holders       []string    `json:"-"`
}

// Attach resumes a terminal session and appends one history receipt under the
// task writer lock. It never changes task.md, plan.md, or a lease.
func (s Store) Attach(o AttachOptions) (Attachment, error) {
	var result Attachment
	if o.Agent {
		var boundPath string
		if idPattern.MatchString(o.BoundTask) {
			bound, err := s.Show(o.Cwd, o.BoundTask)
			if err == nil {
				boundPath = bound.Path
			}
		} else {
			bound, err := s.Resolver.Resolve(where.Query{Cwd: o.Cwd, Task: o.BoundTask})
			if err == nil {
				boundPath = bound.TaskPath
			}
		}
		if boundPath == "" {
			return result, fmt.Errorf("agent cannot attach %s without its assigned task folder", o.ID)
		}
		target, err := s.Show(o.Cwd, o.ID)
		if err != nil {
			return result, err
		}
		if gridhome.Real(boundPath) != gridhome.Real(target.Path) {
			return result, fmt.Errorf("agent cannot attach %s: assigned to another task's folder", o.ID)
		}
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if t.State == "closed" {
			return fmt.Errorf("task %s is closed; use grid task reopen %s", t.ID, t.ID)
		}
		if o.Runtime != "" && o.Runtime != "claude" {
			return errors.New("only Claude session records are supported")
		}
		if o.PID <= 0 || o.PIDStart == "" {
			return errors.New("cannot identify the terminal parent process and its start time")
		}
		if o.SessionID == "" {
			var err error
			o.SessionID, err = feed.NewID()
			if err != nil {
				return err
			}
		}
		machine, err := feed.MachineID(s.Resolver.GridHome)
		if err != nil {
			return err
		}
		key := session.Key{SessionID: o.SessionID, MachineID: machine}
		sessions := session.Store{TaskDir: t.Path, Now: s.Now}
		records, err := sessions.List()
		if err != nil {
			return err
		}
		result = Attachment{ID: t.ID, Title: t.Title, State: t.State, Approved: Approved(t.Path), Session: key}
		for _, record := range records {
			// Start removes an old Job heartbeat when reusing its key. Refuse that
			// key rather than silently turning a Job session into a terminal one.
			if record.Key == key {
				if record.Lease != "" {
					return fmt.Errorf("session %s belongs to Job lease %s; use a different --session", key.SessionID, record.Lease)
				}
				continue
			}
			// Unknown Job liveness is not evidence that its tenure is free.
			liveJob := record.Lease != "" && record.State == session.LivenessUnknown
			if record.Ended != nil || (record.State != session.Running && !liveJob) {
				continue
			}
			if record.Lease != "" || record.MachineID != machine {
				holder := record.Holder
				if holder == "" {
					holder = record.SessionID + "@" + record.MachineID
				}
				result.Holders = append(result.Holders, holder)
			}
		}
		questions, err := (question.Store{TaskDir: t.Path}).List()
		if err != nil {
			return err
		}
		for _, q := range questions {
			if q.State == "open" {
				result.OpenQuestions++
			}
		}
		// Preflight the history destination before publishing the session.
		if _, err := readOptional(filepath.Join(t.Path, "history.md")); err != nil {
			return err
		}
		if _, err := sessions.Start(session.Record{Key: key, Runtime: o.Runtime, Where: "terminal", Cwd: o.Cwd, PID: o.PID, PIDStart: o.PIDStart}); err != nil {
			return err
		}
		event := s.event(t, WriteOptions{Action: "attach", Note: key.SessionID, Who: o.Who}, t.State)
		return commitWrite(t.Path, event, map[string][]byte{})
	})
	return result, err
}
