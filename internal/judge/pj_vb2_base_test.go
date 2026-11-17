package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// pjVb2Remote is a bare remote in a temp folder with main and release pushed, and a work clone
// that pushes to it. It answers the remote, the clone and the SHAs of main and release.
func pjVb2Remote(t *testing.T) (remote, work, mainSHA, releaseSHA string) {
	t.Helper()
	testgit.Isolate(t)
	root := t.TempDir()
	remote, work = filepath.Join(root, "remote.git"), filepath.Join(root, "work")
	testgit.Run(t, root, "init", "-q", "--bare", "-b", "main", remote)
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	testgit.Init(t, work, remote, true)
	mainSHA = pjVb2Commit(t, work, "main", "refs/heads/main")
	releaseSHA = pjVb2Commit(t, work, "release", "refs/heads/release")
	return remote, work, mainSHA, releaseSHA
}

// pjVb2Commit makes one empty commit in work, pushes it to ref on origin and answers its SHA.
func pjVb2Commit(t *testing.T, work, msg, ref string) string {
	t.Helper()
	testgit.Run(t, work, "commit", "-q", "--allow-empty", "-m", msg)
	testgit.Run(t, work, "push", "-q", "origin", "HEAD:"+ref)
	return testgit.Run(t, work, "rev-parse", "HEAD")
}

// pjVb2Base is the base of the rework of builder after review, or the test fails.
func pjVb2Base(t *testing.T, st State, builder, review, target string) string {
	t.Helper()
	plan, err := Rework(st, builder, review, "", target)
	if err != nil {
		t.Fatalf("rework of %s after %s (target %q): %v", builder, review, target, err)
	}
	if len(plan.Posts) != 2 || plan.Posts[0].Type != "job" {
		t.Fatalf("rework of %s posts %+v, want the rework Job and its control", builder, plan.Posts)
	}
	return str(plan.Posts[0].Body["base"])
}

// Row 8 (V27): the review Job's base is the SHA the reviewed attempt submitted; a rework Job's
// base is the SHA refs/grid/<T>/head points at on the task's remote now, not the Feed's newest
// head post, and the target's tip there when the task has no head. A rework of a rework, whose
// own base is a SHA, starts from the branch of the Job it reworks.
func TestPjVb2ReworkBaseIsTheRemoteHeadElseTheTargetTip(t *testing.T) {
	remote, work, mainSHA, releaseSHA := pjVb2Remote(t)
	const submitted = "4444444444444444444444444444444444444444"
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
	st.Bodies["J-1"]["base"] = "main"
	pjVbRecord(&st, "S1", "submit", "J-1", 1, map[string]any{"sha": submitted, "base_sha": mainSHA})
	review, err := Review(st, "J-1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := review.Posts[0].Body["base"]; got != submitted {
		t.Fatalf("review base %v, want the submitted SHA %s", got, submitted)
	}
	pjVbApply(t, &st, review.Posts)
	pjVbRecord(&st, "V1", "result", "J-1.1", 1, map[string]any{"outcome": "REJECTED\nMust-fix:\n- row 1"})

	// The Feed's newest head post names an older head; the remote's ref is the current one.
	headSHA := pjVb2Commit(t, work, "head", "refs/grid/T-1/head")
	pjVbRecord(&st, "H1", "head", "", 0, map[string]any{"sha": mainSHA, "ref": "refs/grid/T-1/head"})
	st.Head = "H1"
	st.Tip, st.HeadRef = RemoteTip("", remote), HeadRef("T-1")
	if st.HeadRef != "refs/grid/T-1/head" {
		t.Fatalf("HeadRef(T-1) = %q", st.HeadRef)
	}
	if got := pjVb2Base(t, st, "J-1", "J-1.1", "release"); got != headSHA {
		t.Fatalf("rework base %s with a task head on the remote, want the head %s", got, headSHA)
	}
	newHead := pjVb2Commit(t, work, "head moved", "refs/grid/T-1/head")
	if got := pjVb2Base(t, st, "J-1", "J-1.1", ""); got != newHead {
		t.Fatalf("rework base %s after the head moved, want the current head %s", got, newHead)
	}

	// No head on the remote: the target's tip, the builder's branch when no target is named.
	testgit.Run(t, work, "push", "-q", "origin", ":refs/grid/T-1/head")
	if got := pjVb2Base(t, st, "J-1", "J-1.1", ""); got != mainSHA {
		t.Fatalf("rework base %s with no head, want the tip of the builder's main %s", got, mainSHA)
	}
	if got := pjVb2Base(t, st, "J-1", "J-1.1", "release"); got != releaseSHA {
		t.Fatalf("rework base %s with target release, want its tip %s", got, releaseSHA)
	}
	movedMain := pjVb2Commit(t, work, "main moved", "refs/heads/main")
	if got := pjVb2Base(t, st, "J-1", "J-1.1", ""); got != movedMain {
		t.Fatalf("rework base %s after main moved, want main's tip now %s", got, movedMain)
	}

	// The rework J-2 is based on a SHA; its own rework reads main from J-1 along the chain.
	plan, err := Rework(st, "J-1", "J-1.1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	pjVbApply(t, &st, plan.Posts)
	if st.Bodies["J-2"]["base"] != movedMain || st.Jobs["J-2"] == nil {
		t.Fatalf("rework J-2 body %v, want base %s", st.Bodies["J-2"], movedMain)
	}
	pjVbJob(&st, "J-1.2", "reviewer", "submitted", 1, "J-2", "")
	pjVbRecord(&st, "V2", "result", "J-1.2", 1, map[string]any{"outcome": "REJECTED\nstill fails"})
	if got := pjVb2Base(t, st, "J-2", "J-1.2", ""); got != movedMain {
		t.Fatalf("rework of the rework J-2 has base %s, want main's tip %s", got, movedMain)
	}
}

// Row 8: a base the remote cannot give is an error and nothing is posted: a target branch the
// remote does not hold, a builder that names no branch, and a remote that cannot be read.
func TestPjVb2ReworkBaseTheRemoteCannotGiveFails(t *testing.T) {
	remote, _, _, _ := pjVb2Remote(t)
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
	pjVbJob(&st, "J-1.1", "reviewer", "submitted", 1, "J-1", "")
	pjVbRecord(&st, "V1", "result", "J-1.1", 1, map[string]any{"outcome": "REJECTED\nMust-fix:\n- row 1"})
	st.Tip, st.HeadRef = RemoteTip("", remote), HeadRef("T-1")
	for _, c := range []struct{ name, target, base, remote, want string }{
		{"missing branch", "nope", "", remote, "refs/heads/nope"},
		{"no branch named", "", "", remote, "--target"},
		{"unreadable remote", "", "main", filepath.Join(t.TempDir(), "absent.git"), "git ls-remote"},
	} {
		t.Run(c.name, func(t *testing.T) {
			st.Bodies["J-1"]["base"] = c.base
			st.Tip = RemoteTip("", c.remote)
			plan, err := Rework(st, "J-1", "J-1.1", "", c.target)
			if err == nil || !strings.Contains(err.Error(), c.want) || len(plan.Posts) != 0 {
				t.Fatalf("rework = %+v, %v; want no posts and an error naming %q", plan.Posts, err, c.want)
			}
		})
	}
}
