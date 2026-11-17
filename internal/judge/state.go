// Package judge holds the mechanical parts of judging a Job, ported from the app's Judge
// (internal/kernel/worker/judge.go): reading a reviewer's verdict, and building the review Job,
// the rework Job with the control that closes the reworked one, and the person Job. It decides
// nothing: the orchestrator reads a verdict and chooses the verb, and every function here only
// answers the posts that choice makes. It holds no loop, no cap and no count of its own; the
// number of review rounds is the workflow's, and verdict only reports how many are used.
package judge

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// State is what the verbs read of one task: the folded Jobs, the job record body of each Job,
// the counted records by id (orchestrator.Snapshot holds all three), every Job id a job record
// names, and the record id of the task's newest head post (contract.State.Head).
//
// Tip, when set, reads the task's git remote as it is now: the SHA a full ref points at there,
// "" when the remote has no such ref (RemoteTip). HeadRef is the task head's ref on that remote,
// refs/grid/<T>/head (HeadRef). With Tip nil the verbs read no remote.
type State struct {
	Task    string
	Jobs    map[string]*contract.Job
	Bodies  map[string]map[string]any
	Records map[string]feed.Record
	IDs     []string
	Head    string
	Tip     func(ref string) (string, error)
	HeadRef string
}

// Plan is what a verb posts, in order. Note says why a verb posts nothing, or less than its
// usual posts; it is empty when every post is there.
type Plan struct {
	Posts []Post
	Note  string
}

type feedRecord = feed.Record

func str(v any) string { s, _ := v.(string); return s }

// role is the Job's role without the plugin prefix, in lower case.
func role(j *contract.Job) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(j.Role)), "grid-")
}

func isReviewer(j *contract.Job) bool { r := role(j); return r == "reviewer" || r == "checker" }

func closed(j *contract.Job) bool {
	return j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected
}

// bodyOf is the job record body of a Job; the fold keeps the first record of an id.
func (s State) bodyOf(id string) map[string]any { return s.Bodies[id] }

// afterOf reads a job body's after, whether it lists ids or {job, kind} objects.
func afterOf(body map[string]any) []string {
	var out []string
	list, _ := body["after"].([]any)
	for _, e := range list {
		switch v := e.(type) {
		case string:
			out = append(out, v)
		case map[string]any:
			if id, ok := v["job"].(string); ok {
				out = append(out, id)
			}
		}
	}
	return out
}

// reviewsOf are the review Jobs of a builder Job: those discovered from it, and those posted with
// it in their after. A review that has not started and waits on the builder cannot be the review
// that accepts it, so it is left out and counts neither as open nor as a round.
func (s State) reviewsOf(id string) []*contract.Job {
	var out []*contract.Job
	for _, j := range s.Jobs {
		if !isReviewer(j) || j.ID == id {
			continue
		}
		waits := slices.Contains(afterOf(s.bodyOf(j.ID)), id)
		if waits && (j.Status == contract.Posted || j.Status == contract.Ready) {
			continue
		}
		if j.DiscoveredFrom != nil && *j.DiscoveredFrom == id || waits {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// reviewedBy is the Job a review judges: the Job it was discovered from, else the first Job in
// its after that is not a review.
func (s State) reviewedBy(r *contract.Job) *contract.Job {
	if r.DiscoveredFrom != nil {
		if b := s.Jobs[*r.DiscoveredFrom]; b != nil && !isReviewer(b) {
			return b
		}
	}
	for _, id := range afterOf(s.bodyOf(r.ID)) {
		if b := s.Jobs[id]; b != nil && !isReviewer(b) {
			return b
		}
	}
	return nil
}

// chainOf is a builder Job and the Jobs it reworks, newest first: a rework Job continues the Job
// it reworks and is discovered from the review that rejected it. A loop in the Feed ends the walk.
func (s State) chainOf(id string) []string {
	out, seen := []string{}, map[string]bool{}
	for id != "" && !seen[id] {
		seen[id], out = true, append(out, id)
		body := s.bodyOf(id)
		from, _ := body["discovered_from"].(string)
		if j := s.Jobs[from]; j == nil || !isReviewer(j) {
			break
		}
		id, _ = body["continues"].(string)
	}
	return out
}

// rootOf is the first Job of a builder's chain: the reviews and the person Job of every round are
// numbered under it, so the work keeps one id from the first review to the person's decision.
func (s State) rootOf(id string) string {
	c := s.chainOf(id)
	return c[len(c)-1]
}

// roundsOf counts the review rounds over a builder's chain, not over one id.
func (s State) roundsOf(id string) int {
	n := 0
	for _, c := range s.chainOf(id) {
		n += len(s.reviewsOf(c))
	}
	return n
}

// childID is the first unused numeric child of a Job: J-1 gives J-1.1, J-1.2 and so on.
func (s State) childID(parent string) string {
	for n := 1; ; n++ {
		id := parent + "." + strconv.Itoa(n)
		if s.Jobs[id] == nil && !slices.Contains(s.IDs, id) {
			return id
		}
	}
}

// nextTopID is the next free top-level J-<n>, counting every Job of the fold and every id a job
// record names: the rule of the plugin's nextJobID (internal/cli/deliver_keeper_job.go).
func (s State) nextTopID() string {
	highest := 0
	count := func(id string) {
		if n, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(id, ".", 2)[0], "J-")); err == nil && n > highest {
			highest = n
		}
	}
	for id := range s.Jobs {
		count(id)
	}
	for id := range s.Bodies {
		count(id)
	}
	for _, id := range s.IDs {
		count(id)
	}
	return "J-" + strconv.Itoa(highest+1)
}

// personFrom is a person Job discovered from id, the one that already holds the decision.
func (s State) personFrom(id string) *contract.Job {
	for _, k := range sortedIDs(s.Jobs) {
		j := s.Jobs[k]
		if role(j) == "person" && j.DiscoveredFrom != nil && *j.DiscoveredFrom == id {
			return j
		}
	}
	return nil
}

// newest is the last record of ids (the fold lists them in Feed order) that matches keep.
func (s State) newest(ids []string, keep func(feed.Record) bool) (feed.Record, bool) {
	for i := len(ids) - 1; i >= 0; i-- {
		if r, ok := s.Records[ids[i]]; ok && keep(r) {
			return r, true
		}
	}
	return feed.Record{}, false
}

func attemptOf(r feed.Record) int {
	if r.Attempt == nil {
		return 0
	}
	return *r.Attempt
}

// Answered is the record that already answers the wake of p: a record of p's type carrying the
// same wake event, about the same Job (a control's target, a Job's discovered_from). The Feed
// writes a wake once (orchestrator.ErrDuplicateWake); a judge verb called again with the wake it
// already answered builds the post it would build now, which may name another Job id, so the
// check is made on what the post is about, before the write.
func (s State) Answered(p Post) (feed.Record, bool) {
	if p.Wake == "" {
		return feed.Record{}, false
	}
	event, _, _ := strings.Cut(p.Wake, "/")
	ids := make([]string, 0, len(s.Records))
	for id := range s.Records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := s.Records[id]
		if r.Wake == nil || r.Type != p.Type {
			continue
		}
		if e, _, _ := strings.Cut(*r.Wake, "/"); e == event && about(r.Type, r.Body) == about(p.Type, p.Body) {
			return r, true
		}
	}
	return feed.Record{}, false
}

// about is the Job a judge post is about: the target of a control, the source of a new Job.
func about(typ string, body map[string]any) string {
	if typ == "control" {
		return str(body["target"])
	}
	return str(body["discovered_from"])
}
