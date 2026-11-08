package job

import "encoding/json"

// Record encodes a Job for the Feed job-record contract, leaving the Job's
// brief and job.yaml representation unchanged. Callers still validate the
// result with internal/contract; this projection does not repair invalid Jobs.
func (j Job) Record() ([]byte, error) {
	j.FollowUp = ""
	// These legacy brief states do not establish a Feed lifecycle transition.
	// In particular, a worker's "done" does not establish acceptance.
	switch j.Status {
	case "briefed", "done", "blocked":
		j.Status = ""
	}
	if j.PostsBack == nil {
		j.PostsBack = []string{}
	}
	// Override only the record's zero min_tests encoding. A command still needs
	// a positive count; a kind verifier cannot carry min_tests at all.
	type recordVerify struct {
		Verify
		MinTests int `json:"min_tests,omitempty"`
	}
	return json.Marshal(struct {
		Job
		Verify recordVerify `json:"verify"`
	}{Job: j, Verify: recordVerify{Verify: j.Verify, MinTests: j.Verify.MinTests}})
}
