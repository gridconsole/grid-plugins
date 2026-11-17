package contract

import "testing"

// plGrfAt is plGrAt in the envelope's millisecond form; no test here reads the clock.
const plGrfAt = "2026-03-01T10:00:00.000Z"

// plGrfPosts is a person Job of dana's, kind kind, and an agent's answer to it
// that carries for, by and grant, posted at at.
func plGrfPosts(t *testing.T, kind, at string) (Post, Post) {
	t.Helper()
	j := sample(t, "job", 1)
	changeBody(t, &j, func(v map[string]any) {
		v["role"], v["kind"] = "person", kind
	})
	a := sample(t, "answer", 5)
	agent := plGrAgent()
	a.Author, a.Tier, a.Worker, a.Re, a.At = agent, Agent, agent.ID, &j.ID, at
	body(t, &a, plGrBody("dana", agent.ID, "[[grid/people/dana/grants/answers]]"))
	return j, a
}

// Fix row 2: contract.Fold counts an agent's answer as the person's only with
// for, by and a covering grant; without one it folds as the agent's own, and
// it is kept either way.
func TestPlGrfFoldCountsAGrantedAnswerAsThePersons(t *testing.T) {
	j, a := plGrfPosts(t, "decide", plGrfAt)
	state, loop := FoldLoop([]Post{j, a}, plGrGrants()...)
	if v := verdict(state, a.ID); v.Kind != "counted" {
		t.Fatalf("granted answer: verdict %+v, want counted", v)
	}
	if got := loop.PostedAs[a.ID]; got != (Author{ID: "dana", Role: "member"}) {
		t.Fatalf("granted answer folds as %+v, want dana as member", got)
	}
	if v := verdict(Fold([]Post{j, a}, plGrGrants()...), a.ID); v.Kind != "counted" {
		t.Fatalf("Fold with grants: verdict %+v, want counted", v)
	}

	for _, c := range []struct {
		name   string
		kind   string
		at     string
		grants []Grant
	}{
		{"no grant", "decide", plGrfAt, nil},
		{"expired at the post", "decide", "2026-03-02T00:00:00.000Z", plGrGrants()},
		{"kind the grant does not name", "approve", plGrfAt, plGrfDecideOnly()},
	} {
		j, a := plGrfPosts(t, c.kind, c.at)
		state, loop := FoldLoop([]Post{j, a}, c.grants...)
		if v := verdict(state, a.ID); v.Kind != "counted" {
			t.Errorf("%s: verdict %+v, want the answer kept and counted", c.name, v)
		}
		if got, ok := loop.PostedAs[a.ID]; ok {
			t.Errorf("%s: answer folds as %+v, want the agent's own", c.name, got)
		}
	}
}

func plGrfDecideOnly() []Grant {
	g := plGrGrants()
	g[0].Kinds = []string{"decide"}
	return g
}
