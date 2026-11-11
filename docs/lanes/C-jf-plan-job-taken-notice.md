# C-jf-plan-job-taken-notice

## Proof table

| Row | Result |
|---|---|
| 1 TestJfTakenMemberPostGetsOneNotice | HOLDS: `go test -count=1 -v -run TestJfTaken ./internal/orchestrator/` PASS, exit 0. On base (plan.go stashed) it fails: `jf_taken_test.go:69: want one notice, got 0: []` |
| 2 TestJfTakenHolderPublicationGetsNone | HOLDS: PASS (same run). TestPlanPublicationRetriesWithoutResettingJobs unchanged and green in the full package run. |
| 3 TestJfTakenSameTitleAndRoleGetsNone | HOLDS: PASS (same run). |
| 4 `go test -count=1 ./internal/orchestrator/` | HOLDS: ok, exit 0. `go vet ./internal/orchestrator/` exit 0. `gofmt -l internal/orchestrator` empty. Diff names only plan.go and jf_taken_test.go (plus this report). |
| Extra: TestJfTakenPersonCopyOfTextDoesNotSilence | HOLDS: a person's unleased post with the notice text does not silence it (dedup counts only a leased task message). Fails on base: `jf_taken_test.go:109: want one leased notice, got 0`. |
| New tests fail on base, pass on lane | HOLDS (lines above). |
| No existing test edited or deleted | HOLDS. |
| Importers of orchestrator (go list): internal/cli, internal/migrate | HOLDS: `go test -count=1 ./internal/migrate/ ./internal/cli/` ok, exit 0. |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0. |
| Lint | go vet and gofmt only; no separate make lint target was run. |
| No guard, gate, cap, hold | HOLDS: the skip stays, one message is added. |
| *RealPlugin* tests | none touched; not run. |

## What changed

- plan.go `planJobInputs` also returns notice texts. At the skip, `planJobTaken` takes the earliest `job` record with that Job id; if it has no lease and its title or role differs from the plan Job, it keeps `plan.md's Job <id> was not posted: <author id> already posted a Job with that id.`
- `postTakenNotices` runs after `postPlanJobs`, posts each text as a task message with s.Writer, and skips it when a leased task message with that exact text is already on the Feed.
- The text holds the id and the stamped author id only, no title or path.
- Not put in the Needs list.

## NOT VERIFIED on a live screen

How guest and developer-link readers render the message in the app.
