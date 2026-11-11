# C-iu-replan-keeper-held-job-id

The re-plan and keeper Jobs take a J-<n> no held job record already carries.

Commits on top of main: `e3e1e4af` (code), `e66744dc` (tests), then this report.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. `TestIuRkReplanSkipsHeldID`: held J-3 posted before the tick; State has no J-3; re-plan Job is J-4; the held record is the only J-3 record | HOLDS | `go test ./internal/orchestrator -count=1 -run TestIuRkReplanSkipsHeldID -v` exits 0 (`--- PASS: TestIuRkReplanSkipsHeldID`). Test: `internal/orchestrator/iu_replan_held_id_test.go`. Base failure below. |
| 2. `TestIuRkKeeperSkipsHeldID`: held J-1 posted first; curator Job is J-2 | HOLDS | `go test ./internal/cli -count=1 -run TestIuRkKeeperSkipsHeldID -v` exits 0 (`--- PASS: TestIuRkKeeperSkipsHeldID`). Test: `internal/cli/iu_keeper_held_id_test.go`. Base failure below. |
| 3. `TestGkReplanAfterCancel` and `TestDeliverPostsOneCuratorJobForWaitingProposals` still pass; `go test ./internal/orchestrator ./internal/cli -count=1` exits 0 | HOLDS | Both ran in the importer run below (`ok internal/orchestrator 122.041s`, `ok internal/cli 73.149s`) and singly with `-v` (`--- PASS` for both). No existing test was edited or deleted. |
| 4. `make lint` exits 0 | HOLDS | `make lint` (runs `go vet ./...`) exits 0, no output; `gofmt -l internal/orchestrator internal/cli` prints nothing. |
| 5. `git diff main --stat` names only the Touches files | HOLDS | Five files: `internal/orchestrator/ready.go`, `internal/orchestrator/replan.go`, `internal/cli/deliver_keeper_job.go`, `internal/orchestrator/iu_replan_held_id_test.go`, `internal/cli/iu_keeper_held_id_test.go`. This report file (`docs/lanes/`) is the one addition the runbook asks for. |
| Edits stay in the named lines | HOLDS | `ready.go`: Snapshot field (`:36-39`) and Load (`:63-73`); `replan.go`: `:78-98` (gkNextJobID); `deliver_keeper_job.go`: `:36-54` (nextJobID). |
| Tests of the changed packages and of every importer | HOLDS | `go list` importers of `internal/orchestrator` or `internal/cli`: `cmd/grid`, `contract/formats`, `internal/account`, `internal/cli`, `internal/golden`, `internal/jobgraph`, `internal/mcp`, `internal/migrate`, `internal/orchestrator`, `internal/packaging`, `internal/question`, `internal/settings`, `lib`. `go test -count=1` on all of them exits 0: every package `ok` (`cmd/grid` has no test files). |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exits 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exits 0. |
| New tests fail on base, pass on the lane | HOLDS | Failing lines pasted below; passing runs above. |
| `*RealPlugin*` tests | n/a | The brief names none; no plugin build was needed. |
| No test deleted, skipped or quarantined; no number only raised | HOLDS | `git diff main --stat` shows no deletion in a test file; the only test files are the two new ones. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change adds a Snapshot field and one more input to the next-id rule; nothing refuses, caps or holds an agent. |
| No sleep, no elapsed-time assertion in the new tests | HOLDS | `grep -n "Sleep\|time\.\|After("` over the two new test files finds none. |
| Small commits, one sentence each, no attribution lines | HOLDS | `git log --oneline main..HEAD`: two commits, no trailer. |
| Public repo: no personal path, host or email | HOLDS | This file and the diff name none. |

## Failing lines on base

Base behaviour was produced by restoring main's `internal/orchestrator/replan.go` and `internal/cli/deliver_keeper_job.go` over the lane (`git checkout main -- <both>`); `ready.go` kept the new field so the tests compile. The two files were then restored (`git checkout HEAD -- <both>`) and `git status --short` printed nothing before the report was written.

```
--- FAIL: TestIuRkReplanSkipsHeldID
    iu_replan_held_id_test.go:79: want one re-plan Job J-4 from J-1, got map[J-3:map[... discovered_from:J-1 id:J-3 ... role:orchestrator ... title:Re-plan after J-1 was cancelled: J-2 wait on it ...]] (counted Jobs [J-1 J-2 J-3])
--- FAIL: TestIuRkKeeperSkipsHeldID
    iu_keeper_held_id_test.go:58: want one curator Job J-2, got [{... ID:J-1 ... Role:curator ...}] (stdout "T-01 delivered; waiting for the person\n2 brain proposals wait for the curator (Job J-1)\n")
```

## What changed

- `Snapshot.JobIDs` (`internal/orchestrator/ready.go:36-39`): the Job id of every job record Load read, whatever its verdict. Load fills it before the counted check (`:63-73`). The id is the body id, as the brief says; a record whose body is not here yet (a `ref` body) has none, so its record `job` field names the id. State, Records, Items, Bodies and Created are unchanged, so no reader of them sees a held record.
- `gkNextJobID` (`internal/orchestrator/replan.go:95`) and `nextJobID` (`internal/cli/deliver_keeper_job.go:52`) count `snap.JobIDs` as well as Items and State.Jobs. The two stay separate functions with one rule, as `replan.go` says.
- Which Job id the orchestrator writes when the Feed holds a record it has not counted (review item, data format): the next free J-<n> above every job record on the Feed, held or counted. The held record keeps its id; when its target arrives the fold counts it alone, and no second Job takes the same id.

## Notes for the reviewer

- The brief says "the fold holds" the record. In this code the Feed reader gives a record with an unposted `after` the verdict `held` first (`feed/fold.go:40`: `posts()` passes only counted, stale and unknown records), so `State.Verdicts` never lists it. The tests therefore read the verdict from `Feed.Read` entries. `Load` ranges over `r.Records`, so it sees the held record either way.
- Open item, outside the allowed lines: `postCuratorJob`'s recheck under `WriteUnless` (`deliver_keeper_job.go:132`) tests `s.State.Jobs[id]` only. A held record for the chosen id that lands between Load and the post is not caught there; io W2 (Store refuses a job record whose id the Feed holds) is the cover for that window.
- Not changed, as the brief says: the fixed ids of `plan.go:120` and `events.go:162`; `postQuestionJob` in `internal/cli/question.go` (the C-io-question-ask-free-job-id lane).

## NOT VERIFIED on a live screen

Nothing: no UI row.
