package brain

import "fmt"

// CheckRefusal is the error Accept returns when the proposal check finds
// something. Nothing was written: no entry, version, audit line or status change.
type CheckRefusal struct{ Result CheckResult }

// Error names the codes and files only; quoted proposal text is data and stays
// in Result.
func (e *CheckRefusal) Error() string {
	if len(e.Result.Findings) == 0 {
		return "proposal check failed"
	}
	f := e.Result.Findings[0]
	return fmt.Sprintf("proposal check found %d problem(s); first: %s in %s", len(e.Result.Findings), f.Code, f.File)
}

// AcceptCheckRequired is the replaceable host policy (key
// brain_accept_check_levels, default home and project) for which levels run the
// proposal check on Accept. Reject is never gated.
var AcceptCheckRequired = func(level string) bool { return level == "home" || level == "project" }

// afterProposalRead is a test seam, nil in production: Accept calls it right
// after it reads the proposal, so a test can change the file between that read
// and the check and tell whether the check and the publication use one read.
var afterProposalRead func()

// checkAccept runs under the level lock on the bytes decide read, the same
// bytes it publishes from, so what is checked is what is published.
func checkAccept(level, name string, n Note, b []byte) error {
	if !AcceptCheckRequired(level) {
		return nil
	}
	res := CheckResult{Findings: []Finding{}}
	if err := ProposalCheck(level, name, n, b, &res); err != nil {
		return err
	}
	if len(res.Findings) > 0 {
		return &CheckRefusal{Result: res}
	}
	return nil
}
