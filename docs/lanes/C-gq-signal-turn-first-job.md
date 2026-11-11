# C-gq-signal-turn-first-job: proof table

| Row | Result |
|---|---|
| 1. `signal act --act turn` leaves a task with J-1 Posted, reading `intent.md#request` | HOLDS: `go test ./internal/cli/ -run 'SignalTurn|SignalOtherActs' -count=1` exit 0; `TestSignalTurnPostsFirstJob` (gq_signal_turn_first_job_test.go) reads the Feed, finds J-1, role orchestrator, intent owner/goal `intent.md#request`, and `job tree` lists J-1 |
| 2. Other acts post no Job | HOLDS: `TestSignalOtherActsPostNoJob` (add, note, snooze, dismiss): no task folder made |
| 3. Guest refused, no task or Job | HOLDS: `TestSignalTurnGuestMakesNoTaskOrJob` exit 3, no task folder |
| 4. J-1 body passes postFindings with no finding | HOLDS: `TestSignalTurnPostsFirstJob` calls `postFindings` on the posted body and wants zero findings |
| 5. `go test ./internal/cli/ -run 'Signal' -count=1` | HOLDS: exit 0 (`ok internal/cli 1.300s`) |
| A turn that fails posts nothing | HOLDS: `TestSignalTurnThatFailsPostsNothing` (second turn refused, exit 3, still one task and one Job record) |
| New tests fail on base | HOLDS: with signal.go stashed: `--- FAIL: TestSignalTurnPostsFirstJob ... want J-1 posted on the turned task, got []` and `--- FAIL: TestSignalTurnThatFailsPostsNothing ... a refused turn posted a Job: 0 records` |
| No existing test edited or deleted | HOLDS: `git diff origin/main --stat` shows signal.go and the new test file only (plus this doc) |
| Tests of changed package and importers | HOLDS: importers of internal/cli per `go list`: cmd/grid, lib. `go test ./internal/cli/ ./cmd/grid/ ./lib/ -count=1` exit 0 |
| Lint | HOLDS: `make lint` (go vet, gofmt -l) exit 0 |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0 |
| Real-plugin test | Not asked by a row; none run |
| No guard, gate, cap or hold on agents | HOLDS: nothing added that refuses or limits anyone |

## What changed
- internal/cli/signal.go: after `--act turn` returns, `signalPostFirstJob` reads the new task (`task.Store.Show`, id from `Result.Became`) and calls `postFirstJob`, as task.go:136 does. A failed post is said out loud with exit 1, after the result is printed, like `grid task new`.
- The feed takes author roles lead, owner, member, cli. A signal turned by an agent (`GRID_ROLE=agent`, covered by the existing `TestSignalActRolesAndTaskAuthor/agent`) has owner_role `agent`; for the J-1 post only, the role is written as `cli`. The task's own owner_role is unchanged. Reviewer note: `grid task new` with `GRID_ROLE=agent` still fails to post J-1 for the same reason; that is outside this brief.

## NOT VERIFIED on a live screen
Nothing here is UI. No real binary, `claude` or live worker was run; the stream pick-up of J-1 for a turned task is covered by the same code as `grid task new` (`TestTaskNewFirstJobIsSpawnedByTheStream`), not re-run for a signal.
