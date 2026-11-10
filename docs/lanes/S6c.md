# S6c1: the person Job result carries the plan digest (additive)

Base: lane-S6b8, b8378c46. Branch lane-S6c1. Status: **WORK IN PROGRESS, not ready to land.** The new tests pass, but two goldens still need regenerating (see Open items) and the whole-package run exits 1.

## What changed (production)

- `internal/task/write.go`: `WriteOptions` gains the field `JobResult func(Event) error`. This is one additive field outside the "gates.go" listed in the brief. It is the only way to carry the hook through `land.Store.Approve`, which passes `WriteOptions` through unchanged, and internal/land is not in scope.
- `internal/task/gates.go`:
  - `Approve` is extended in place. After `WithDigest` returns nil (a new receipt, or a repeat press whose receipt already matches), it calls `o.JobResult(event)`. That call happens while `.task.lock` is still held but after `.plan.lock` is released. The reason: the orchestrator takes `.orchestrator.lock` before `.plan.lock` (`orchestrator/plan.go` `WithHash`), so posting while holding `.plan.lock` could deadlock. A stale hash or the missing-plan error returns first, so nothing is written.
  - New `JobApproval` type and `ApprovalFromJob(dir)`, placed next to `Approved`. It is lock-free and reads the Feed with `ReadOnly`. It looks for the newest accepted control (by greatest ULID) that targets a person Job, is accepted in the fold, and carries `plan_digest`. It returns nil when that result's `plan_hash` and `plan_digest` match plan.md now. Otherwise it returns "Changed since you approved: plan hash no longer matches", or "plan has not been approved". No caller changed except the tests.
- `internal/cli/orchestrator.go`:
  - `grid job result <job> --plan-hash <h>` on a person Job runs the same `land.Store.Approve` compare-and-set as `grid approve`. The `answer` and `note` come from `--body`. It then posts the result, and prints the approve event.
  - A Job that is already accepted, cancelled or rejected gets the same message AnswerPerson gives (`%s is already %s`), and this happens before anything is written. That is not a new refusal: AnswerPerson already refuses this case.
  - New helpers:
    - `planPersonJob`: the first open person Job with `kind: approve`.
    - `personApproval`: posts the Feed message the way AnswerPerson does, then the accepting control `{target, status: accepted, reason, answer, plan_hash, plan_digest, lands_hash?, note?}`, both through `orchestrator.Write`. A Job already accepted with the same digest gets nothing new.
  - `internal/orchestrator` is not in scope, so `AnswerPerson` is unchanged.
- `internal/cli/task_write.go`: when the task has an open person Job of an approve step, `grid approve` sets `o.JobResult`. If the Feed writer cannot be built (for example no `GRID_MEMBER`), it prints "`<job>` stays open: ..." and writes approval.json as before. This adds no new refusal.
- `contract/schemas/control.json`: the status-control branch gains five optional properties: `answer`, `plan_hash` (6 hex), `plan_digest` (64 hex), `lands_hash` and `note`. All are additive. No required field and no other branch changed. `cmd-approve.json` is unchanged.

approval.json is written exactly as before (Q9). The Job result is written next to it, in the same `withWrite` call.

## Removed non-test lines

None. `git diff b8378c46 --stat` (production, before tests and this doc):

```
 contract/schemas/control.json | 20 +++++++++
 internal/cli/orchestrator.go  | 97 +++++++++++++++++++++++++++++++++++++++++++
 internal/cli/task_write.go    | 10 +++++
 internal/task/gates.go        | 63 ++++++++++++++++++++++++++++
 internal/task/write.go        |  5 +++
 5 files changed, 195 insertions(+)
```

## Tests added (internal/cli/approve_job_test.go)

- `TestApprovePostsPersonJobResultWithPlanDigest` (row 1): `grid approve --plan-hash <current>` accepts J-2 (a person Job with `kind: approve`) with `plan_hash`, `plan_digest` and `answer: approved`, and approval.json carries the same hash and digest.
- `TestApproveStaleHashWritesNoJobResult` (row 2): runs both `grid approve --plan-hash 000000` and `grid job result J-2 --plan-hash 000000`. Each exits 1 with "plan hash no longer matches", the same as today's stale approve. Neither writes an accepted control or approval.json.
- `TestJobResultPlanHashAndApprovalFromJob` (row 3): before any result, `ApprovalFromJob` errors. After `grid job result --plan-hash <digest>` it matches (job, hash, digest, answer, note). After plan.md changes it reports "Changed since you approved".

On base these tests do not compile, because `taskstore.ApprovalFromJob` is undefined, and `--plan-hash` is an unknown `job result` flag there. I did not run them on base within the budget, so I have no failing-on-base output to paste. NOT VERIFIED.

No test was deleted, and no existing test or expectation was edited.

## Checks

- `go test -count=1 -run 'TestApprovePostsPersonJobResultWithPlanDigest|TestApproveStaleHashWritesNoJobResult|TestJobResultPlanHashAndApprovalFromJob|TestPersonJob' -v ./internal/cli/`: exit 0, all 4 PASS.
- `go test -count=1 ./internal/... ./cmd/...`: **exit 1**. Two visible failures:
  - `internal/cli TestContractCommandsParserInventory`: `contract/commands.json` lacks the new `--plan-hash` flag on the job verbs (`line 698: inventory "--project"; parser "--plan-hash"`).
  - `TestSessionGolden` (server_test.go:466, golden `mcp-session`, the Feed MCP server package): `feed_types` prints control.json, which now has the new optional fields.

  Both are goldens that gain only the new optional field. The output after the MCP failure was truncated in my view, so the pass or fail state of the packages listed after it (orchestrator, task, plan, cmd/...) is NOT VERIFIED.
- `go vet ./...`: exit 0.
- `gofmt -l internal cmd`: exit 0, no output.
- `GOOS=windows go build ./...`: exit 0. `GOOS=darwin go build ./...`: exit 0.

## Open items

1. Regenerate `contract/commands.json` with `UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$' -count=1`. The agent allow list refuses that env prefix. Then regenerate the `mcp-session` golden with its update flag, and check that both diffs only add `--plan-hash` and the five control fields.
2. Re-run `go test -count=1 ./internal/... ./cmd/...` and paste the tail. Run the new tests on base and paste the failing lines.
3. The Job importers list (`go list -deps`/reverse deps of internal/task and internal/cli) was not pasted, for budget reasons.
4. "The plan Job" is defined here as a person Job with `kind: approve`, the first open one in Feed order, for `grid approve`. `ApprovalFromJob` accepts any accepted person Job whose result carries `plan_digest`. S6c2 should confirm both definitions when it moves the readers.
5. The `grid approve` fallback when no Feed writer exists warns and leaves the Job open. The owner may prefer that it fails instead.
6. The cross-repo set and the app are not checked.
