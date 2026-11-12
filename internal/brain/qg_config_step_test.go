package brain

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"testing"
)

// qgGridCall finds `grid <verb>` and, when a plain word follows, `grid <verb> <sub>`.
// A placeholder such as <project> or a flag after the verb gives no sub.
var qgGridCall = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./-])grid ([a-z][a-z0-9-]*)(?: ([a-z][a-z0-9-]*))?`)

// qgCommands reads verb -> subs from contract/commands.json.
func qgCommands(t *testing.T) map[string]map[string]bool {
	t.Helper()
	b, err := os.ReadFile("../../contract/commands.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name string `json:"name"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]bool{}
	for _, c := range doc.Commands {
		out[c.Name] = map[string]bool{}
		for _, s := range c.Subcommands {
			out[c.Name][s.Name] = true
		}
	}
	return out
}

// TestQgConfigFindingNamesAVerb: the step a config_topic finding names is a
// command the plugin has, for every topic check treats as config.
func TestQgConfigFindingNamesAVerb(t *testing.T) {
	cmds := qgCommands(t)
	topics := make([]string, 0, len(configProposalTopics))
	for topic := range configProposalTopics {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	if len(topics) == 0 {
		t.Fatal("configProposalTopics is empty")
	}
	for _, topic := range topics {
		s := chkStore(t)
		chkProposal(t, s, "p1", topic, "proposed", "x\n")
		r, err := s.Check()
		if err != nil {
			t.Fatal(err)
		}
		chkWant(t, r, CheckConfigTopic)
		msg := r.Findings[0].Message
		if len(msg) <= len("config: ") || msg[:len("config: ")] != "config: " {
			t.Errorf("%s: finding %q does not read `config: <step>`", topic, msg)
		}
		for _, m := range qgGridCall.FindAllStringSubmatch(msg, -1) {
			verb, sub := m[1], m[2]
			subs, ok := cmds[verb]
			if !ok {
				t.Errorf("%s: finding %q names `grid %s`, which is not a verb in contract/commands.json", topic, msg, verb)
				continue
			}
			if sub != "" && len(subs) > 0 && !subs[sub] {
				t.Errorf("%s: finding %q names `grid %s %s`, which is not a sub in contract/commands.json", topic, msg, verb, sub)
			}
		}
	}
}
