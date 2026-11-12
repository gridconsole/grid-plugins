# C-qi-worker-question-fallback

Base sha: 44c7de5bacbac31560de11e84afd4ff15980c05d (plugin main).

## Base check (the :18 fallback as written, kb3 env)

`job post --task <task> --person dana --body <personBody>` with GRID_ROLE=worker, GRID_LEASE=J-1, GRID_LEASE_EPOCH empty, `orchestratorHarness`:

- exit 1, stderr: `GRID_LEASE_EPOCH or --lease-epoch must be gen.fence; the task holder gets it from: grid lease take task`
- With the worker's own Job epoch set instead (`assignedExitHarness`): exit 3, stderr `stale lease`.
- No person Job posted either way. The refusal is a lease refusal, not the schema author-role refusal the brief names (see open items). The text is right to change.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 `TestQiOldFallbackRefused` | HOLDS | `go test -count=1 -v -run TestQi ./internal/cli/` exit 0: `--- PASS: TestQiOldFallbackRefused`; non-zero exit, `feed read --type job` holds only J-1. Also `TestQiOldFallbackRefusedWithJobEpoch` PASS. |
| 2 red on base | HOLDS | On base text: `--- FAIL: TestQiWorkerFallbackRuns` — `"grid job post --person": exit 2: flag needs an argument: -person`. On lane: `--- PASS: TestQiWorkerFallbackRuns` (runs `grid job result --task <path> --job J-1 --body <json>` as the `assignedExitHarness(t, true)` worker, GRID_ATTEMPT=1; exit 0; `feed read --task --job J-1 --type result --json` holds the `decisions_needed` entry; no new Job). |
| 3 grep + kb3 | HOLDS | `git grep -n 'it posts a person Job with ...' -- skills` prints nothing (exit 1). `go test -count=1 -v -run 'TestKb3\|TestSkillCommandsImplemented' ./internal/cli/ ./internal/packaging/` exit 0: `--- PASS: TestKb3QAskByWorkerIsRefusedNoJob`, `--- PASS: TestSkillCommandsImplemented`. |
| 4 lint, cross, stat | HOLDS | `make lint` exit 0 (go vet ./...). `make cross` exit 0 (darwin/arm64, windows/amd64 vet and test compile). `git diff main --stat`: `internal/cli/qi_worker_fallback_test.go`, `skills/questions/SKILL.md` only. |
| Tests of changed packages and importers | HOLDS | The skill is embedded by the root package, imported by internal/job. `go test -count=1 . ./internal/job/... ./internal/jobgraph ./internal/orchestrator ./internal/lease ./internal/hook ./internal/migrate ./internal/ports ./internal/packaging/... ./lib ./cmd/grid` exit 0 (all ok); `go test -count=1 ./internal/cli/` exit 0. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (PASS both); `go test -count=1 ./internal/job/lint/` exit 0. |
| New tests fail on base, pass on lane | HOLDS | Row 2 lines above. |
| No existing test edited, none deleted | HOLDS | diff touches only the two files. |
| No guard, gate, cap, hold | HOLDS | text and one test file only. |
| RealPlugin tests | not applicable | no row names one. |

## What changed

- `skills/questions/SKILL.md:18`: the one sentence "An agent without GRID_TASK_EPOCH ..." now says the worker may not post a person Job either, names the decision with options and recommendation in its result's `decisions_needed` (`grid job result --task <task> --job <job> --body <json>`), stops, and the orchestrator asks the person.
- `internal/cli/qi_worker_fallback_test.go`: `TestQiOldFallbackRefused`, `TestQiOldFallbackRefusedWithJobEpoch`, `TestQiWorkerFallbackRuns`. The last cuts the last backticked `grid job ...` command out of the sentence read through `gridplugins.SkillFor("questions")` and runs it.

## Open items

- The brief says the post is refused because a worker's author role fails job.json. In the harness the refusal comes earlier, from the lease check (`GRID_LEASE_EPOCH ... must be gen.fence`, or `stale lease` with the Job epoch). The outcome the text rests on (a worker cannot post a person Job) holds; the reason in the brief was not reproduced. No product code touched.
- `grid job result` needs an attempt (GRID_ATTEMPT in the test); the skill sentence follows the brief's command exactly, as `skills/run-job/SKILL.md:18` does.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run with a live agent.

Verdict: DONE
