package contract

import "strings"

// GuestAddressee reports whether an unleased answer by a guest to a person Job is
// the answer of the person the Job waits for. It is the one rule the CLI writer
// choice, AnswerPersonWith, the holder's pass, the Feed's admission of a guest
// answer and the fold all read. It is true only when the role is a guest; the
// writer holds no lease; the author and the Job's intent.owner are both set and
// equal as written (no trimming, no case folding); the Job's role is Person; and
// the Job's kind is not approve, which is a plan approval and stays with the team.
// The caller has already found that the record is an answer to the Job's first
// job record, and passes the Job's own role, intent.owner and kind.
func GuestAddressee(role, author string, leased bool, jobRole, owner, kind string) bool {
	return NormalizeRole(role) == "guest" &&
		!leased &&
		author != "" && author == owner &&
		strings.EqualFold(strings.TrimSpace(jobRole), "Person") &&
		!strings.EqualFold(strings.TrimSpace(kind), "approve")
}
