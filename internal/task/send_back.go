package task

import (
	"errors"
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

// SendBackHash checks the plan hash a person sends a plan back on (D-ig) with the
// compare and the text of Approve: the 6-hex plan_hash or the 64-hex plan_digest
// of plan.md as it is now, under the plan_hash_token policy of the workflow. It
// returns the current 6-hex plan_hash and writes nothing.
func SendBackHash(dir, token, hash string) (string, error) {
	switch token {
	case "", "any", "digest":
	default:
		return "", fmt.Errorf("%w: plan_hash_token must be any or digest", ErrEditUsage)
	}
	if !planHashToken.MatchString(hash) {
		return "", fmt.Errorf("%w: plan hash must be 6 or 64 lowercase hex digits", ErrEditUsage)
	}
	if token == "digest" && len(hash) != 64 {
		return "", fmt.Errorf("%w: plan_hash_token digest requires 64 lowercase hex digits", ErrEditUsage)
	}
	var sent string
	err := plan.Store{TaskDir: dir}.WithDigest(func(current, digest string) error {
		match := current
		if len(hash) == 64 {
			match = digest
		}
		if hash != match {
			return fmt.Errorf("plan hash no longer matches: approved %q, current %q", hash, current)
		}
		sent = current
		return nil
	})
	if errors.Is(err, plan.ErrNoPlan) {
		return "", errors.New("plan.md is missing; there is no plan to send back")
	}
	return sent, err
}

// SendBack appends the history line of a plan sent back: what send_back, the
// plan_hash it was sent back on and who, from o. It never holds the person's
// words (they are the answer on the Feed) and changes no state: the task waits
// for the re-plan. The caller writes it after the answer is posted.
func (s Store) SendBack(o WriteOptions, hash string) (Event, error) {
	var event Event
	o.Action, o.Note = "send_back", ""
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		event = s.event(t, o, t.State)
		event.PlanHash = hash
		return commitWrite(t.Path, event, map[string][]byte{})
	})
	return event, err
}
