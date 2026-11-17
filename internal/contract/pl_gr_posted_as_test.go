package contract

import (
	"encoding/json"
	"strings"
	"testing"
)

// plGrAt is a fixed post time; no test here reads the clock.
const plGrAt = "2026-03-01T10:00:00Z"

func plGrGrants() []Grant {
	return []Grant{{
		Link:   "grid/people/dana/grants/answers",
		Person: "dana",
		Role:   "member",
		Agents: []string{"agent-7@01J000000000000000000000AA"},
		Posts:  []string{"answer"},
		Kinds:  []string{"decide", "approve"},
		Tasks:  []string{"all"},
		Until:  "2026-03-02T00:00:00Z",
	}, {
		Link:   "grid/people/gus/grants/answers",
		Person: "gus",
		Role:   "guest",
		Agents: []string{"worker"},
		Posts:  []string{"answer"},
		Until:  "2026-03-02T00:00:00Z",
	}}
}

func plGrAgent() Author { return Author{ID: "agent-7@01J000000000000000000000AA", Role: "worker"} }

func plGrBody(person, by, grant string) map[string]any {
	body := map[string]any{"answer": "yes"}
	for k, v := range map[string]string{"for": person, "by": by, "grant": grant} {
		if v != "" {
			body[k] = v
		}
	}
	return body
}

// Row 1 (V22): an answer with for, by and a covering grant counts as the person's.
func TestPlGrGrantedAnswerIsThePersons(t *testing.T) {
	agent := plGrAgent()
	for _, link := range []string{"[[grid/people/dana/grants/answers]]", "[[grid/people/dana/grants/answers|Dana's answers]]", "grid/people/dana/grants/answers.md"} {
		who, why := PostedAs("answer", agent, "T-01", plGrAt, plGrBody("dana", agent.ID, link), "decide", plGrGrants())
		if who != (Author{ID: "dana", Role: "member"}) || why != "" {
			t.Fatalf("grant %s: got %+v %q, want dana as member", link, who, why)
		}
	}
	// The post was made before until; folded later it is still the person's,
	// because the expiry is read against the post's own at, never the clock.
	body, _ := json.Marshal(plGrBody("dana", agent.ID, "[[grid/people/dana/grants/answers]]"))
	p := Post{Type: "answer", Task: "T-01", Author: agent, At: "2026-03-01T23:59:59Z", Body: body}
	if who, why := PostAs(p, "approve", plGrGrants()); who.ID != "dana" || why != "" {
		t.Fatalf("PostAs: got %+v %q, want dana", who, why)
	}
}

// Row 2 (V22): an expired, wrong-agent or missing grant leaves the agent's own author.
func TestPlGrUngrantedAnswerIsTheAgents(t *testing.T) {
	agent := plGrAgent()
	link := "[[grid/people/dana/grants/answers]]"
	other := Author{ID: "agent-9@01J000000000000000000000BB", Role: "worker"}
	narrow := plGrGrants()
	narrow[0].Kinds = []string{"decide"}
	narrow[0].Tasks = []string{"T-02"}
	for _, c := range []struct {
		name   string
		author Author
		at     string
		body   map[string]any
		kind   string
		grants []Grant
		why    string
	}{
		{"expired at until", agent, "2026-03-02T00:00:00Z", plGrBody("dana", agent.ID, link), "decide", plGrGrants(), "expired"},
		{"expired after until", agent, "2026-04-01T10:00:00Z", plGrBody("dana", agent.ID, link), "decide", plGrGrants(), "expired"},
		{"wrong agent", other, plGrAt, plGrBody("dana", other.ID, link), "decide", plGrGrants(), "does not name"},
		{"by is another agent", agent, plGrAt, plGrBody("dana", other.ID, link), "decide", plGrGrants(), "is not the post's author"},
		{"no grant note", agent, plGrAt, plGrBody("dana", agent.ID, "[[grid/people/dana/grants/none]]"), "decide", plGrGrants(), "no grant note"},
		{"no grants at all", agent, plGrAt, plGrBody("dana", agent.ID, link), "decide", nil, "no grant note"},
		{"no grant key", agent, plGrAt, plGrBody("dana", agent.ID, ""), "decide", plGrGrants(), "all needed"},
		{"no by key", agent, plGrAt, plGrBody("dana", "", link), "decide", plGrGrants(), "all needed"},
		{"another person's grant", agent, plGrAt, plGrBody("erin", agent.ID, link), "decide", plGrGrants(), "not erin's"},
		{"guest's grant", agent, plGrAt, plGrBody("gus", agent.ID, "[[grid/people/gus/grants/answers]]"), "", plGrGrants(), "not of a person of the team"},
		{"kind not named", agent, plGrAt, plGrBody("dana", agent.ID, link), "approve", narrow, "person Jobs"},
		{"task not covered", agent, plGrAt, plGrBody("dana", agent.ID, link), "decide", narrow, "does not cover task"},
		{"person posts for another", Author{ID: "erin", Role: "member"}, plGrAt, plGrBody("dana", "erin", link), "decide", plGrGrants(), "only an agent"},
	} {
		who, why := PostedAs("answer", c.author, "T-01", c.at, c.body, c.kind, c.grants)
		if who != c.author || !strings.Contains(why, c.why) {
			t.Errorf("%s: got %+v %q, want the post's own author and a reason with %q", c.name, who, why, c.why)
		}
	}
	// A post that asks for no person keeps its author with no reason.
	if who, why := PostedAs("answer", agent, "T-01", plGrAt, map[string]any{"answer": "yes"}, "decide", plGrGrants()); who != agent || why != "" {
		t.Errorf("plain answer: got %+v %q", who, why)
	}
}
