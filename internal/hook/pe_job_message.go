package hook

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// peMsgDeliver hands a team person's message to the running Job's agent. The app's
// Agents composer posts {"kind":"message","text":"To J-n: ..."} to the task Feed
// and tells the person the worker reads it on its next turn; this is that read.
// On PostToolUse, for a session whose GRID_JOB is a valid Job id, it writes each
// message not yet shown as additionalContext, oldest first. Each is shown once
// per session: the session store's seen keys hold pemsg:<job>:<record id>. A body
// marked external is shown only inside the Feed's data fence (feed.MarshalData).
// It only reads and informs; it never blocks, and any error goes to the hook log.
func peMsgDeliver(o Options, in hookio.Input) {
	jobID := o.Getenv("GRID_JOB")
	if in.HookEventName != "PostToolUse" || in.SessionID == "" || !job.ValidID(jobID) || o.Stdout == nil {
		return
	}
	dir, err := taskDir(o, in)
	if err != nil {
		o.log("job message", err)
		return
	}
	if dir == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(dir, "feed")); errors.Is(err, os.ErrNotExist) {
		return
	}
	id, err := feed.TaskID(dir)
	if err != nil {
		// A task with no stable id has no Feed this store can read.
		return
	}
	read, err := feed.Store{TaskDir: dir, TaskID: id}.Read(feed.Filter{Type: "post"})
	if err != nil {
		o.log("job message", err)
		return
	}
	var due []feed.Entry
	for _, e := range read.Records {
		if peMsgForJob(e.Record, jobID) {
			due = append(due, e)
		}
	}
	if len(due) == 0 {
		return
	}
	store, err := activityStore(o, in)
	if err != nil {
		o.log("job message", err)
		return
	}
	var lines []string
	err = store.UpdateActivity(in.SessionID, func(a *session.Activity) error {
		for _, e := range due {
			key := "pemsg:" + jobID + ":" + e.ID
			if a.Seen[key] {
				continue
			}
			a.Seen[key] = true
			if e.Record.Data() {
				// Body marked external: the Feed's own data fence, never plain text.
				lines = append(lines, "Message from "+e.Author.ID+" [external]:\n"+feed.MarshalData(e.Record))
				continue
			}
			text, _ := e.Body["text"].(string)
			lines = append(lines, "Message from "+e.Author.ID+": "+text)
		}
		return nil
	})
	if err != nil {
		o.log("job message", err)
		return
	}
	if len(lines) == 0 {
		return
	}
	b, err := json.Marshal(map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName":     "PostToolUse",
		"additionalContext": boundContext(strings.Join(lines, "\n")),
	}})
	if err != nil {
		o.log("job message", err)
		return
	}
	_, err = o.Stdout.Write(append(b, '\n'))
	o.log("job message", err)
}

// peMsgForJob reports whether a Feed record is a team person's message for the
// Job: a post of kind message by a person of the team, not written under the
// task lease (the tick's own notes), that names the Job in its job field or
// starts with "To <job>:". A guest only answers and comments, so a guest's post
// never reaches the agent, marked external or not.
func peMsgForJob(r feed.Record, jobID string) bool {
	if r.Type != "post" || r.Body["kind"] != "message" {
		return false
	}
	if contract.TierFor(r.Author.Role) != contract.PersonTeam {
		return false
	}
	if r.Lease != nil && *r.Lease == "task" {
		return false
	}
	text, _ := r.Body["text"].(string)
	if text == "" {
		return false
	}
	return (r.Job != nil && *r.Job == jobID) || strings.HasPrefix(text, "To "+jobID+":")
}
