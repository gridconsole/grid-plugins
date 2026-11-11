package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/question"
)

func runQuestion(args []string, env Env) int {
	if len(args) > 0 && args[0] == "answer" {
		return runAnswer(args[1:], env)
	}
	return question.Run(args, questionEnv(env))
}

func questionEnv(env Env) question.Env {
	return question.Env{Cwd: env.Cwd, Resolver: resolver(env), Stdout: env.Stdout, Stderr: env.Stderr, Jobs: questionJobs(), RecordError: env.recordError, Writer: func(control bool) (feed.Writer, error) {
		if control {
			return feedWriter(env, env.getenv("GRID_LEASE"), env.getenv("GRID_LEASE_EPOCH"))
		}
		return feedWriter(env, "", "")
	}}
}
func runAnswer(args []string, env Env) int { return question.RunAnswer(args, questionEnv(env)) }

// questionJobs wires the question store to the orchestrator's person Job: ask
// posts the Job, and an answer, a default or a withdrawal closes it.
func questionJobs() question.JobPort {
	return question.JobPort{Post: postQuestionJob, Close: closeQuestionJob}
}

// postQuestionJob posts the person Job under the task lease when the caller holds
// it and without a lease when a person of the team asks. Any other writer (an
// agent with no lease) gets no Job and no refusal: the question is still posted.
func postQuestionJob(f feed.Store, lease, person feed.Writer, body map[string]any) (string, error) {
	snap, err := orchestrator.Load(f)
	if err != nil {
		return "", err
	}
	top := 0
	for id := range snap.State.Jobs {
		n, _ := strconv.Atoi(strings.TrimPrefix(strings.SplitN(id, ".", 2)[0], "J-"))
		top = max(top, n)
	}
	id := fmt.Sprintf("J-%d", top+1)
	body["id"] = id
	in := feed.Input{Type: "job", Body: body, Job: id}
	switch {
	case lease.Lease == "task":
		_, err = f.Post(in, lease)
	case orchestrator.PersonWriter(person):
		_, err = orchestrator.PostPerson(f, person, in)
	default:
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// closeQuestionJob gives a question's person Job its result. A Job that is not a
// question Job, is closed already, or has no writer that may close it is left alone.
func closeQuestionJob(f feed.Store, lease, person feed.Writer, id, result string) error {
	snap, err := orchestrator.Load(f)
	if err != nil {
		return err
	}
	j := snap.State.Jobs[id]
	ask := false
	for _, it := range snap.Items {
		ask = ask || it.ID == id && it.IsPerson() && it.Kind == question.JobKind
	}
	if j == nil || !ask || j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected {
		return nil
	}
	w := person
	if lease.Lease == "task" {
		w = lease
	}
	if w.Lease != "task" && !orchestrator.PersonWriter(w) {
		return nil
	}
	result = strings.TrimSpace(result)
	if orchestrator.CheckPersonAnswer(result) != nil {
		result = string([]rune(result)[:200]) + "…"
	}
	_, err = orchestrator.AnswerPerson(f, w, id, result, time.Now())
	return err
}
