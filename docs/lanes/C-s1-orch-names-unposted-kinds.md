# C-s1-orch-names-unposted-kinds

The orchestrator's first result names each step-2 Job kind it did not post, with one line why (end review 0212Z S1).

## Proof table

| Row | State | Evidence |
|---|---|---|
| 1. Diff of agents/orchestrator.md step 7 and of any fixture | HOLDS | `agents/orchestrator.md:29` (below). No fixture reads step 7; the plan-jobs skill does not hold the list, so it is unchanged. |
| 2. `make lint` | HOLDS | `make lint`: exit 0 (`go vet ./...`, gofmt clean). |
| 2. Plugin agent/skill lint | HOLDS | `go test -count=1 ./internal/job/lint/`: ok (exit 0). The plugin has no other agent/skill lint target. |
| 3. docs/lanes/C-s1-orch-names-unposted-kinds.md | HOLDS | this file |
| New test fails on base, passes on lane | HOLDS | Base (agents/orchestrator.md stashed): `FAIL TestS1OrchestratorNamesUnpostedKinds`: `step 7 lacks "names each step-2 kind"`, `step 7 lacks "did not post"`, `step 7 lacks "one line why"`. Lane: `go test -count=1 ./internal/job/lint/`: ok. |
| Tests of changed packages and importers | HOLDS | Importers of the root package (embed.go): internal/cli, internal/job, internal/job/jobcmd. `go test -count=1 ./internal/job/... ./internal/cli/ .`: all ok. Also `./cmd/... ./lib/... ./internal/hook/ ./internal/mcp/ ./internal/orchestrator/`: ok. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...`: exit 0. |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...`: exit 0. |
| Walk tests: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | ok, exit 0. |
| Walk test: `go test -count=1 ./internal/job/lint/` | HOLDS | ok, exit 0. |
| `git fetch origin && git merge origin/main` | HOLDS | "Already up to date." |
| App-repo walk tests, realplugin guest walks | not run, not asked (plugin repo; the app-repo checks are not part of this lane) |
| Whole-module test, cross-repo set | not run, not asked (the orchestrator runs them at landing) |
| No new guard, gate, cap or hold | HOLDS | Instruction text and one text-pin test only. |

## What changed

Step 7 of `agents/orchestrator.md` (line 29) gains one sentence: "In the first Job of a task, the `outcome` also names each step-2 kind (research, plan, build, review, person) you did not post, with one line why (for example "no research Job: the brain already holds the answer")."

`internal/job/lint/s1_orchestrator_unposted_kinds_test.go` pins that sentence in step 7.

The brief names line 67 for the research judgement; on this main the step-2 list is lines 19-24 and step 7 is line 29.

## NOT VERIFIED on a live screen

Whether a live orchestrator now names its unposted kinds in the first result. That needs a real run, for example an eval run like round 8.
