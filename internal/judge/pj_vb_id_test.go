package judge

import (
	"regexp"
	"testing"
)

// pjVbULID is the plugin's own shape of a record id (internal/feed/record.go ulidPattern).
var pjVbULID = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)

// pjVbPostID is the request id of one post; there is no epoch to give, and nothing in memory.
func pjVbPostID(t *testing.T, task, typ, job string, body map[string]any, wake, person string) string {
	t.Helper()
	id := Post{Task: task, Type: typ, Job: job, Body: body, Wake: wake, Person: person}.ID()
	if !pjVbULID.MatchString(id) {
		t.Fatalf("request id %q is not a record ULID the plugin takes", id)
	}
	return id
}

// Ported from the app's judge_post_request_id_test.go:43: two equal posts hold the same id; a post
// that differs in the task, the type, the Job id, the body, the wake or the person holds another.
func TestJudgePostRequestIDFollowsThePost(t *testing.T) {
	body := func(title string) map[string]any {
		return map[string]any{"id": "J-7", "title": title, "after": []any{"J-1"}, "intent": map[string]any{"item": "a#b"}}
	}
	const wake1, wake2 = "01ARZ3NDEKTSV4RRFFQ69G5FAV/1", "01ARZ3NDEKTSV4RRFFQ69G5FAV/2"
	base := pjVbPostID(t, "T-1", "job", "J-7", body("one"), wake1, "")
	if again := pjVbPostID(t, "T-1", "job", "J-7", body("one"), wake1, ""); again != base {
		t.Errorf("two equal posts hold id %s and %s, want the same", base, again)
	}
	for name, other := range map[string]string{
		"another body":   pjVbPostID(t, "T-1", "job", "J-7", body("two"), wake1, ""),
		"another Job id": pjVbPostID(t, "T-1", "job", "J-8", body("one"), wake1, ""),
		"another type":   pjVbPostID(t, "T-1", "control", "J-7", body("one"), wake1, ""),
		"another task":   pjVbPostID(t, "T-2", "job", "J-7", body("one"), wake1, ""),
		"another wake":   pjVbPostID(t, "T-1", "job", "J-7", body("one"), wake2, ""),
		"another person": pjVbPostID(t, "T-1", "job", "J-7", body("one"), wake1, "owner"),
	} {
		if other == base {
			t.Errorf("a post with %s holds the same id %s as the base post", name, base)
		}
	}
}

// Ported from the app's judge_post_request_id_test.go:66: every post carries the id, a control
// among them, and two equal controls hold one id.
func TestJudgeControlCarriesARequestID(t *testing.T) {
	a := Control("T-1", "J-2", "ready", "rework after review round 1", "", nil)
	b := Control("T-1", "J-2", "ready", "rework after review round 1", "", nil)
	if a.Type != "control" || a.Job != "J-2" || a.Body["target"] != "J-2" {
		t.Fatalf("control post %+v, want a control on J-2", a)
	}
	if !pjVbULID.MatchString(a.ID()) || a.ID() != b.ID() {
		t.Errorf("two equal controls hold id %q and %q, want one record ULID twice", a.ID(), b.ID())
	}
	// The rework's own control, the one that rejects the reworked Job, carries its id too.
	plan, err := Rework(pjVbChain(), "J-2", "J-1.2", "", "")
	if err != nil {
		t.Fatal(err)
	}
	last := plan.Posts[len(plan.Posts)-1]
	if last.Type != "control" || !pjVbULID.MatchString(last.ID()) {
		t.Fatalf("the rework's last post %+v holds id %q, want a control with a record ULID", last, last.ID())
	}
}
