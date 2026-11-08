package orchestrator

import (
	"fmt"
	"sort"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func gateRank(kind string) int {
	switch kind {
	case "person":
		return 2
	case "timer":
		return 1
	default:
		return 0
	}
}

// warnDuplicateGates runs under the pass lock. The warning itself is the
// durable deduplication marker, surviving lost local metadata and handovers.
// Ready remains a pure predicate for readers and worker spawn sizing.
func (s Store) warnDuplicateGates(snap Snapshot) error {
	counts := map[string]int{}
	for id := range snap.State.Questions {
		gate, _ := snap.Records[id].Body["gate"].(map[string]any)
		if gid := stringValue(gate["id"]); gid != "" {
			counts[gid]++
		}
	}
	warned := map[string]bool{}
	for _, r := range snap.Records {
		if r.Type == "post" && r.Lease != nil && *r.Lease == "task" && r.Body["kind"] == "message" {
			warned[stringValue(r.Body["ref"])+"\n"+stringValue(r.Body["text"])] = true
		}
	}
	duplicates := []string{}
	for gid, count := range counts {
		if count > 1 {
			duplicates = append(duplicates, gid)
		}
	}
	sort.Strings(duplicates)
	for _, gid := range duplicates {
		ref := "question:" + gid
		text := fmt.Sprintf("Warning: duplicate gate id %s; using the strictest kind (person > timer > other), with all equally strict questions required to open.", gid)
		if warned[ref+"\n"+text] {
			continue
		}
		if _, err := s.Feed.Post(feed.Input{Type: "post", Body: map[string]any{"ref": ref, "kind": "message", "text": text}}, s.Writer); err != nil {
			return err
		}
	}
	return nil
}
