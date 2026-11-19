package orchestrator

import (
	"fmt"
	"regexp"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

var headBaseID = regexp.MustCompile(`^J-[0-9]+(\.[0-9]+)*$`)

// ValidHeadBase reports whether base is head: followed by a Job id.
func ValidHeadBase(base string) bool {
	id, ok := job.HeadBaseTarget(base)
	return ok && headBaseID.MatchString(id)
}

// WorkerBranch is the branch a worker makes for one attempt of a Job. It is the name
// runner.Branch gives (the runner imports this package, so the format is held here too;
// a test keeps the two equal).
func WorkerBranch(task, job string, attempt int) string {
	return fmt.Sprintf("grid-%s-%s-a%d", task, job, attempt)
}

// ResolveBase turns a Job's base into the ref its worktree starts from. A plain base
// (or none) is answered as it is. head:J-n becomes the branch of the accepted head of
// J-n's continues chain, with that head's accepted attempt. When the chain has no
// accepted head the error names J-n; the caller must not fall back to HEAD.
func ResolveBase(s Snapshot, task, base string) (string, error) {
	id, ok := job.HeadBaseTarget(base)
	if !ok {
		return base, nil
	}
	if !ValidHeadBase(base) {
		return "", fmt.Errorf("base %q does not name a Job id", base)
	}
	if _, posted := s.State.Jobs[id]; !posted {
		return "", fmt.Errorf("base %s waits for %s, which is not posted", base, id)
	}
	jobs := jobgraph.Snapshot{Jobs: map[string]contract.Job{}}
	for k, j := range s.State.Jobs {
		jobs.Jobs[k] = *j
	}
	head, state := jobgraph.Head(jobs, id)
	if state != jobgraph.HeadAccepted {
		return "", fmt.Errorf("base %s waits for %s, whose continues chain has no accepted head (%s at %s)", base, id, state, head)
	}
	attempt := s.State.Jobs[head].Attempt
	if attempt < 1 {
		return "", fmt.Errorf("base %s waits for %s, whose head %s has no accepted attempt", base, id, head)
	}
	return WorkerBranch(task, head, attempt), nil
}

// headBaseWait is the reason a Job whose base is not yet resolvable stays unspawned.
func headBaseWait(err error) string {
	return "waiting: " + err.Error()
}
