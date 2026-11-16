package gridplugins

import "testing"

// Every listed skill's description is in the context of every turn of every
// session, an agent's too. The person-facing skills, which agents Grid starts
// do not run, keep a short description: what it does and the words that call it
// (docs/decisions/D-ctx-skill-listing.md).
func TestCtxPersonSkillDescriptionsShort(t *testing.T) {
	const most = 200
	for _, name := range []string{"setup", "task", "questions", "signals"} {
		d := erDescText(t, erDescCase{name: name, skill: true})
		if len(d) > most {
			t.Errorf("%s: description is %d bytes, want at most %d: %s", name, len(d), most, d)
		}
	}
}
