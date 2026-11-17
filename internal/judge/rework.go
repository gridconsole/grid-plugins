package judge

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Rework builds the rework of a builder Job its review rejected (the app's rework): a new Job
// under the next free top-level id that continues the builder and is discovered from the review,
// with the builder's body and one more acceptance row holding the review's must-fix, and then the
// control that rejects the builder as reworked by it. A rework already on the Feed is not posted
// again; only the control is. The rework's base is a SHA read from the task's remote now: the
// task head's (refs/grid/<T>/head), else the target branch's tip (reworkBase). The wake goes on
// the control, the last post.
func Rework(st State, builder, review, wake, target string) (Plan, error) {
	_, src, err := st.jobOf(builder)
	if err != nil {
		return Plan{}, err
	}
	r := st.Jobs[review]
	if r == nil {
		return Plan{}, fmt.Errorf("no review Job %s on the task's Feed", review)
	}
	round := st.roundsOf(builder)
	var posts []Post
	id := st.reworkOf(builder, review)
	if id == "" {
		id = st.nextTopID()
		_, text, mustFix := st.verdictOf(r)
		if len(mustFix) == 0 {
			if text == "" {
				text = "the review " + review + " gave no must-fix"
			}
			mustFix = []string{text}
		}
		note := fmt.Sprintf("Must-fix from review round %d:\n- %s", round, strings.Join(mustFix, "\n- "))
		body := copyBody(src)
		rows, _ := src["acceptance"].([]any)
		rows = append(append([]any{}, rows...), map[string]any{"id": nextRowID(rows), "text": note, "traces": "owner"})
		for _, k := range []string{"status", "created_at", "order"} {
			delete(body, k)
		}
		body["id"] = id
		body["acceptance"] = rows
		body["continues"] = builder
		body["discovered_from"] = review
		body["attempt"] = 1
		body["review_rounds"] = round
		body["checkpoint"] = nil
		body["close_reason"] = nil
		base, err := st.reworkBase(builder, src, target)
		if err != nil {
			return Plan{}, err
		}
		if base != "" {
			body["base"] = base
		}
		posts = append(posts, Post{Task: st.Task, Type: "job", Job: id, Body: body})
	}
	posts = append(posts, Control(st.Task, builder, "rejected", fmt.Sprintf("rework after review round %d", round), wake,
		map[string]any{"close_reason": "reworked as " + id}))
	return Plan{Posts: posts}, nil
}

// reworkBase is the base of a rework Job. With the task's remote (Tip), it is the SHA the task
// head HeadRef points at there now, else the tip of the target branch there (targetOf); a remote
// that cannot be read, or a target it does not hold, is an error and nothing is posted. With no
// remote it is the sha of the task's newest head post, else target, else the builder's own target
// or base, as named.
func (s State) reworkBase(builder string, src map[string]any, target string) (string, error) {
	if s.Tip != nil {
		if s.HeadRef != "" {
			sha, err := s.Tip(s.HeadRef)
			if err != nil || sha != "" {
				return sha, err
			}
		}
		name := s.targetOf(builder, target)
		if name == "" {
			return "", fmt.Errorf("the task has no head on its remote and %s names no target branch: name it with --target", builder)
		}
		ref := name
		if !strings.HasPrefix(ref, "refs/") {
			ref = "refs/heads/" + name
		}
		sha, err := s.Tip(ref)
		if err == nil && sha == "" {
			err = fmt.Errorf("the task has no head on its remote and the remote has no %s to start the rework from", ref)
		}
		return sha, err
	}
	if head, ok := s.Records[s.Head]; ok && s.Head != "" {
		if sha := str(head.Body["sha"]); sha != "" {
			return sha, nil
		}
	}
	for _, v := range []string{target, str(src["target"]), str(src["base"])} {
		if strings.TrimSpace(v) != "" {
			return v, nil
		}
	}
	return "", nil
}

// targetOf is the branch a rework starts from when the task has no head: target, else the first
// target or base along the builder's chain that names a branch. A rework's own base is a SHA,
// so a rework of a rework reads the branch of the Job it reworks.
func (s State) targetOf(builder, target string) string {
	if t := strings.TrimSpace(target); t != "" {
		return t
	}
	for _, id := range s.chainOf(builder) {
		body := s.bodyOf(id)
		for _, v := range []string{str(body["target"]), str(body["base"])} {
			if v = strings.TrimSpace(v); v != "" && !shaPattern.MatchString(v) {
				return v
			}
		}
	}
	return ""
}

// shaPattern is a full SHA-1 or SHA-256 object name.
var shaPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// HeadRef is the ref of the task head on the task's remote: refs/grid/<T>/head, where task is the
// task's T-<n> id.
func HeadRef(task string) string { return "refs/grid/" + task + "/head" }

// RemoteTip reads remote as it is now with git ls-remote, run in dir: remote is a remote of the
// checkout at dir (origin), a URL or a path, as git takes it. The answer for a full ref is the
// SHA the remote holds for exactly that ref, "" when it holds none.
func RemoteTip(dir, remote string) func(ref string) (string, error) {
	return func(ref string) (string, error) {
		args := []string{"ls-remote", remote, ref}
		if dir != "" {
			args = append([]string{"-C", dir}, args...)
		}
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("git ls-remote %s %s: %s", remote, ref, strings.Join(strings.Fields(stderr.String()+" "+err.Error()), " "))
		}
		for _, line := range strings.Split(string(out), "\n") {
			if sha, name, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok && name == ref {
				return sha, nil
			}
		}
		return "", nil
	}
}

// reworkOf is the rework of builder after review already on the Feed: the Job that continues the
// builder and is discovered from that review, "" when there is none.
func (s State) reworkOf(builder, review string) string {
	for _, id := range sortedIDs(s.Jobs) {
		j := s.Jobs[id]
		if j.Continues != nil && *j.Continues == builder && j.DiscoveredFrom != nil && *j.DiscoveredFrom == review {
			return id
		}
	}
	return ""
}

// nextRowID is the next free acceptance row id A<n> after rows.
func nextRowID(rows []any) string {
	highest := 0
	for _, e := range rows {
		if row, ok := e.(map[string]any); ok {
			if n, err := strconv.Atoi(strings.TrimPrefix(str(row["id"]), "A")); err == nil && n > highest {
				highest = n
			}
		}
	}
	return "A" + strconv.Itoa(highest+1)
}
