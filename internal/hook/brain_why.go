package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// brainReadVia is the via field of a brain read: the Job id when the agent runs
// inside a Job, otherwise the session that read the note, so two sessions of
// one task tell apart.
func brainReadVia(jobID, sessionID string) string {
	if jobID != "" {
		return jobID
	}
	return "session " + sessionID
}

// brainLinkWhy returns the why of the link in the Job's saved job.yaml that
// names the note id, with any #anchor of the link's target cut. The file is
// opened with job.Within, as `grid job save` writes it, so a path or a symbolic
// link that leaves the task folder is never read. A Job id that is not valid, a
// missing, unreadable or invalid job.yaml and a Job with no link to the note
// all give "": the hook only records, so a failure here is not reported.
func brainLinkWhy(taskDir, jobID, id string) string {
	if !job.ValidID(jobID) {
		return ""
	}
	path, err := job.Within(taskDir, filepath.Join("jobs", jobID, "job.yaml"))
	if err != nil {
		return ""
	}
	// Within resolves links; a named pipe or device in its place is not a Job file.
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var j job.Job
	if err := json.Unmarshal(b, &j); err != nil {
		return ""
	}
	for _, l := range j.Links {
		to, _, _ := strings.Cut(l.To, "#")
		if to == id && strings.TrimSpace(l.Why) != "" {
			return l.Why
		}
	}
	return ""
}
