# D-dep-release: release a waiting Job on a passing submit, not only on accepted

Repo plugin, branch lane-D-dep-release, from main (plugin eb833a2d).

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1a. `after` Jobs all submitted with a passing verify (or accepted) are ready | HOLDS | `internal/jobgraph/graph.go` `Released`, `VerifyPassed`, used at the `done` case in `Ready`; `go test ./internal/jobgraph/` exit 0; `TestSubmittedWithPassingVerifyReleases` (9 cases) |
| 1b. Rejected `after` Job puts an unstarted dependent back to waiting | HOLDS | `TestRejectedBeforeStartWaitsAgain` (jobgraph); `TestReadyReleasesOnSubmittedWithPassingVerify` (orchestrator, submit then reject); exit 0 |
| 1c. Started dependent gets a note in its Feed, not a kill | HOLDS | `graph.go` returns `Readiness.Notes` and never moves a started Job to Waiting (`TestRejectedAfterStartNotesNotKills`); `internal/orchestrator/release.go` posts the note once, called from `tick.go` before the waits and assigns; `TestStartedDependentGetsOneNoteOnRejection` checks one note post carrying the Job and zero controls; exit 0 |
| 2a. `contract/README.md` says it in one place | HOLDS | `contract/README.md` bullet before "Result arrival alone is not acceptance" |
| 2b. `docs/job-contract.md` says it in one place | HOLDS | `docs/job-contract.md`, "Dependencies and review", paragraph after the gate sentence |
| 2c. Graph tests cover submitted-ready, rejected-before-start, rejected-after-start, each failing on main | HOLDS | `internal/jobgraph/release_test.go`; failing lines on base below |
| 3a. `go test` of changed packages and importers | HOLDS | importers from `go list` (graph and orchestrator importers): `internal/cli`, `internal/jobgraph`, `internal/orchestrator`; `go test ./internal/cli/ ./internal/jobgraph/ ./internal/orchestrator/` exit 0 |
| 3b. `go test ./...` whole module | NOT RUN | left to the orchestrator at landing, per the builder rules |
| 3c. `make lint` | HOLDS | `make lint` (`go vet ./...`) exit 0; `gofmt -l internal/jobgraph internal/orchestrator` prints nothing |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| No existing test edited or deleted | HOLDS | `git diff origin/main --stat` touches only new `release_test.go` files; `graph_test.go:186` ("submitted is not done", no verify) still passes |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The note is a message post; a started Job gets no control, hold or stop. A dependent that has not started waits for a dependency, as before |
| No attribution lines, no personal paths, hosts or emails | HOLDS | commit messages are one sentence; no attribution lines |

### Failing on base (new graph tests, with only the empty `Note` type and `Notes` field added so they compile)

```
--- FAIL: TestSubmittedWithPassingVerifyReleases/bare,_verify_true
    release_test.go:45: got {Ready:[] Waiting:[{Job:J-2 Target:J-1 Kind:done Reason:waiting for J-1 to be accepted}] Notes:[]}
--- FAIL: TestSubmittedWithPassingVerifyReleases/done,_verify_pass
--- FAIL: TestRejectedBeforeStartWaitsAgain
    release_test.go:57: not released while submitted: {Ready:[] Waiting:[{Job:J-2 ...}] Notes:[]}
--- FAIL: TestRejectedAfterStartNotesNotKills
    release_test.go:84: started "assigned": got {Ready:[] Waiting:[] Notes:[]}
```

The orchestrator tests were not run on base: there the package does not compile (no `Notes`, no `noteStartedDependents`).

## What changed

- `internal/jobgraph/graph.go`: `VerifyPassed` and `Released`; a `done` or bare edge is met by accepted, or by submitted with a passing verify. A rejected target gives the wait reason "waiting for J-n, which was rejected". `Readiness.Notes` lists started Jobs (assigned, running, submitted, parked) whose `done` target was rejected; they are not held.
- `internal/orchestrator/ready.go`: the held-Job wait uses `jobgraph.Released`.
- `internal/orchestrator/release.go`, `tick.go`: one message post (with `job` set) per started dependent per rejection; the post is its own dedupe marker.
- `contract/README.md`, `docs/job-contract.md`: the rule, once each.

## Open items

- What counts as "verify passing": the read has one verify outcome, `contract.Job.Verified`, the `verified` object of the latest result (`pass` true or `"pass"`). The submit post has no verify field. Work that never posts a `verified` object (for example L0 docs and tests) is not released by its submit; it waits for accepted as before. If the owner wants a worker's own verify run to release it, the submit or result needs a field for that outcome. That is a contract change, so it is not guessed here.
- `failed` and `children-done` edges are unchanged: they still wait for rejected-with-no-attempts and for accepted children.
- The note is a `post` message with `job` set and `ref: task`. The post schema's `ref` cannot name a Job, so the Job is carried in the record's `job` field. I did not check that the app's Feed view shows posts by `job`.
- Budget: I used about 62 tool calls against 60, because a first graph read missed that a Feed post cannot reference a Job and the fixture needed a second look.

## NOT VERIFIED on a live screen

- That the dependent's Feed in the app shows the note, and that D-dep-start starts the dependent worktree from the submitted branch.
