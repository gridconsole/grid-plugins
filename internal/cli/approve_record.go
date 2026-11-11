package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// approvalWriter is the Feed writer of a person's approval. A caller that holds a
// task lease epoch writes under it. A person of the team with no epoch writes
// unleased, as grid job post --person does, so the worker that holds the task
// lease never blocks the approval. Anyone else gets an error and nothing is
// written: grid approve is for a person.
func approvalWriter(env Env) (feed.Writer, error) {
	if epoch := env.getenv("GRID_LEASE_EPOCH"); epoch != "" {
		return orchestratorWriter(env, "task", epoch)
	}
	role := contract.NormalizeRole(env.getenv("GRID_ROLE"))
	switch role {
	case "":
		role = "member"
	case "cli":
		// The plain cli role is a person's shell; writeOptions records it as the owner.
		role = "owner"
	}
	if env.getenv("GRID_MEMBER") == "" || contract.TierFor(role) != contract.PersonTeam {
		return feed.Writer{}, errors.New("approving is for a person of the team: set GRID_MEMBER")
	}
	w, err := feedWriter(env, "", "")
	w.Author.Role = role
	return w, err
}

// recordApproval records e as the result of the task's approve person Job and
// nothing else: the Feed is the only record of an approval. With no Job yet (job
// is ""), or only an accepted one that a person without a lease cannot answer
// again, it posts a new approve Job first, giving the task a Feed identity when it
// has none. A leased writer posts the accepting control in one write; a person
// without a lease posts an answer carrying plan_hash, plan_digest, lands_hash and
// note, which the holder's next pass turns into the control. A plan that already
// holds this approval gets nothing new. The caller holds no task or plan lock.
func recordApproval(dir string, store feed.Store, w feed.Writer, job, answer string, e taskstore.Event) error {
	if answer = strings.TrimSpace(answer); answer == "" {
		answer = "approved"
	}
	unleased := orchestrator.PersonWriter(w)
	if job != "" && unleased {
		snap, err := orchestrator.Load(store)
		if err != nil {
			return err
		}
		if j := snap.State.Jobs[job]; j != nil && j.Status == contract.Accepted {
			if a, err := taskstore.ApprovalFromJob(dir); err == nil && a.PlanDigest == e.PlanDigest {
				return nil
			}
			job = ""
		}
	}
	if job == "" {
		id, err := feed.EnsureTaskID(dir)
		if err != nil {
			return err
		}
		store = feed.Store{TaskDir: dir, TaskID: id}
		_, owner, _ := taskstore.FeedIdentity(dir)
		if owner == "" {
			owner = w.Author.ID
		}
		if job, err = migrate.PostPlanJob(store, w, owner); err != nil {
			return err
		}
	}
	if !unleased {
		return personApproval(store, w, job, answer, e)
	}
	fields := map[string]any{"plan_hash": e.PlanHash, "plan_digest": e.PlanDigest}
	if e.LandsHash != "" {
		fields["lands_hash"] = e.LandsHash
	}
	if e.Note != "" {
		fields["note"] = e.Note
	}
	_, err := orchestrator.AnswerPersonWith(store, w, job, answer, fields, time.Now())
	return err
}
