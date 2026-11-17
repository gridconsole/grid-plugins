package brain

import (
	"testing"
)

// 6.3: a person note is listed before the agent note that supersedes it, and the
// agent note is marked conflict in search and in the listing; notes that do not
// conflict keep the score order.
func TestPbAdPersonNoteRanksBeforeTheAgentNoteThatSupersedesIt(t *testing.T) {
	s := pbAdVault(t)
	// The person's note matches the query once; the agent's note matches it far better
	// and would come first by score alone.
	pbAdEdit(t, s, "Notes/Old.md", "A person's note about the cache.\n")
	agent := "---\ndescription: cache rules\nlinks:\n  - to: Notes/Old\n    rel: supersedes\n    why: newer\n---\nThe cache rules.\n"
	if _, err := s.Write("home", "Notes", "Newer", "none", []byte(agent), pbAdAgent(), "why", false); err != nil {
		t.Fatal(err)
	}
	// An unrelated agent note with the best score of all.
	if _, err := s.Write("home", "Notes", "Free", "none", []byte("---\ndescription: cache rules\n---\nOther cache rules.\n"), pbAdAgent(), "why", false); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search("cache", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, h := range hits {
		pos[h.ID] = i
	}
	person, newer, free := "brain:home/Notes/Old", "brain:home/Notes/Newer", "brain:home/Notes/Free"
	if _, ok := pos[person]; !ok {
		t.Fatalf("hits = %+v", hits)
	}
	if pos[person]+1 != pos[newer] {
		t.Errorf("the person note is at %d and the agent note that supersedes it at %d; want it right after", pos[person], pos[newer])
	}
	if c := hits[pos[newer]].Conflict; len(c) != 1 || c[0] != person {
		t.Errorf("conflict on the agent hit = %v", c)
	}
	if c := hits[pos[free]].Conflict; len(c) != 0 || pos[free] != 0 {
		t.Errorf("the note that does not conflict is at %d with conflict %v; want first by score", pos[free], c)
	}
	rows := pbVrRows(t, s)
	if c := rows[newer].Conflict; len(c) != 1 || c[0] != person {
		t.Errorf("listing conflict of the agent note = %v", c)
	}
	if c := rows[person].Conflict; len(c) != 1 || c[0] != newer {
		t.Errorf("listing conflict of the person note = %v", c)
	}
	if len(rows[free].Conflict) != 0 {
		t.Errorf("listing conflict of the free note = %v", rows[free].Conflict)
	}
}
