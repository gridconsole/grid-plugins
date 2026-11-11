# C-le-job-writers-retired-fields

Lane head: see the last line of the report. Repo: plugin. Base: main a7809a58.

## Proof table

| Row | Result |
|---|---|
| 1. TestLeRwNoRetiredWriters walks internal/ and fails outside receipt.go | HOLDS. `go test -count=1 -v -run 'TestLeRw' ./internal/job/` exit 0: `--- PASS: TestLeRwNoRetiredWriters`. On base (production edits stashed) it failed naming approvefix/approvefix.go, cli/deliver_keeper_job.go, cli/task_new_first_job.go, orchestrator/replan.go, question/job.go (`FAIL github.com/gridconsole/grid-plugins/internal/job`). Test: internal/job/le_retired_writers_test.go:17. |
| 2. TestLeRwBodies, TestLeRwQuestionBody | HOLDS. `go test -count=1 -v -run TestLeRw ./internal/cli/ ./internal/question/` exit 0: `--- PASS: TestLeRwBodies`, `--- PASS: TestLeRwQuestionBody`. On base: `firstJob writes the retired key "review"` / `"max_rounds"`, `curatorJobBody ...`, `ReplanJobBody writes the retired key "max_rounds"`, `approvefix.JobBody ...`, and `jobBody writes the retired key "review"` / `"max_rounds"`; both packages `FAIL`. |
| 3a. `go test -count=1 ./internal/question/ ./internal/approvefix/ ./internal/orchestrator/` | HOLDS, exit 0 (question ok, approvefix no test files, orchestrator ok 76s). |
| 3b. `go test -count=1 ./internal/cli/ -run 'TestOrchestratorText\|TestOrchestratorExample\|TestJobPost\|TestGq\|TestJw\|TestIu\|TestKb3\|TestQuestion'` | HOLDS, exit 0 (`ok internal/cli 2.609s`). |
| 4. hfJrKnownOpen keeps receipt.go and fold.go; contract doc rows | HOLDS. internal/job/hf_record_doc_test.go:21-24; docs/job-contract.md:89-90 name planned.go (review only) and migrate/receipt.go and say the other builders no longer write it. `go test -count=1 -v -run 'TestHfJr' ./internal/job/` exit 0, all five TestHfJr* PASS. |
| 5. diff names only Touches; receipt.go unchanged | DIFFERS by one file. `git diff main --stat -- internal/migrate/receipt.go` is empty (receipt.go unchanged). The stat also names internal/cli/deliver_keeper_job_test.go, which is not in Touches (see below). Every other file is in Touches. |
| 6. `make lint` / `make cross` | HOLDS, both exit 0. `make lint` first said `gofmt needed: internal/cli/task_new_first_job.go` (alignment after my edit); fixed with gofmt, rerun printed only `go vet ./...` and exited 0. `make cross` ran darwin/arm64 and windows/amd64 vet and test compile, exit 0. |
| Packages that import the changed ones (test) | HOLDS. Importers from `go list` of question/orchestrator/approvefix/cli/job: cli, mcp, migrate, task, lib, cmd/grid, job/... . `go test -count=1 ./internal/task/ ./internal/mcp/ ./internal/migrate/ ./internal/job/... ./internal/approvefix/ ./internal/question/ ./lib/ ./cmd/...` exit 0. `go test -count=1 ./internal/cli/` exit 0 after the test fix below. |
| Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. The app walk tests are not part of this plugin lane. |
| New tests fail on base, pass on lane | HOLDS (rows 1 and 2). |
| No guard, gate, cap, hold or blocking hook added | HOLDS. Only two keys removed from five builders. |
| *RealPlugin* tests | NOT RUN. The brief names none; nothing here posts through the built plugin beyond what the cli tests cover. |
| Commits: small, one sentence, no attribution lines, no personal paths | HOLDS. |

## What changed

- The five builders no longer write `review` or `max_rounds`: internal/cli/task_new_first_job.go:32, internal/cli/deliver_keeper_job.go:77, internal/question/job.go:44, internal/approvefix/approvefix.go:123, internal/orchestrator/replan.go:162. internal/migrate/receipt.go is untouched.
- hfJrKnownOpen: the five entries left the list.
- docs/job-contract.md rows `review` and `max_rounds`.
- Three new test files (le_retired_writers_test.go, le_retired_fields_test.go in cli and question).

## One edit outside Touches (needs the orchestrator's call)

`go test ./internal/cli/` was red after the writer change: `TestDeliverPostsOneCuratorJobForWaitingProposals` (internal/cli/deliver_keeper_job_test.go:87) asserted `j.Review != "L0"` on the curator Job, so it fails with `job fields: {... Review: ...}`. The brief's row 3 filter (`TestJw|TestIu|...`) does not match that test, so the brief did not list it. I removed the single clause `j.Review != "L0" ||` and nothing else. That assertion covered behaviour the owner removed (docs/simplify.md row 10, answers 11 Nov q-levels and v-10); the test still checks id, skills, after, title, intent, read, acceptance and verify. If the orchestrator wants Touches strictly kept, revert that one commit (`Drop the retired review assertion from the curator Job test.`); the cli package is then red.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: the app rigs and cross-repo set (orchestrator at landing); the whole-module test.
