# C-hv-plan-unreadable-tick-goes-on

Base: main 91fadeff. Base check: on base, tick.go:40-41 returns the publishPlan error, so a plan fault ends the tick. The tick does not already continue past a plan fault.

## Proof table

| Row / rule | Status | Proof |
|---|---|---|
| 1 TestHvPlanScalarJobsGoesOn | HOLDS | `go test ./internal/orchestrator/ -count=1 -run 'TestPlanPublication\|TestHvPlan' -v`: `--- PASS: TestHvPlanScalarJobsGoesOn`, exit 0. Test at hv_plan_unreadable_test.go:55. |
| 2 TestHvPlanOtherJobStillAssigned | HOLDS | Same run: `--- PASS: TestHvPlanOtherJobStillAssigned`. Checks `out.Assigned == 1` and contract.Assigned (hv_plan_unreadable_test.go:79). |
| 3 TestPlanPublicationValidatesBeforeWriting, all 5 cases | HOLDS | Same run: PASS for cycle, duplicate, missing_field, invalid_contract and missing_dependency. 0 FAIL, exit 0. Tick returns nil, planJobRecords is empty, and exactly one notice names each case's cause (plan_test.go:93-125). |
| 4 TestHvPlanFixedPublishes | HOLDS | Same run: `--- PASS: TestHvPlanFixedPublishes`. Checks one J-1 record, no new notice, and `plan_publication_blocked` false or absent in `.orchestrator.json` (hv_plan_unreadable_test.go:97). |
| 5a `go test ./internal/orchestrator/ ./internal/plan/ -count=1` exit 0 | HOLDS | Exit 0: `ok .../internal/orchestrator 79.886s`, `ok .../internal/plan 0.274s`. Fix round 1 widened Touches to plan_approval_test.go:85-87. That check now reads `if err := <-badResult; err != nil { t.Fatalf("malformed plan stopped its tick: %v", err) }`. The rest of TestMalformedPlanDoesNotBlockOtherTasks is unchanged: the good task is still assigned, the bad task publishes nothing, and fixing it lets it assign. `--- PASS: TestMalformedPlanDoesNotBlockOtherTasks`. |
| 5b `make lint` | HOLDS | `make lint` exit 0 (go vet ./...). gofmt -l is clean. |
| 5c `git diff main --stat` | HOLDS | Names only internal/orchestrator/hv_plan_unreadable_test.go, plan.go, plan_test.go, plan_approval_test.go (:85-87, widened by fix round 1), plus this report. |
| New tests fail on base | HOLDS | With plan.go stashed back to base: `hv_plan_unreadable_test.go:58: tick stopped on an unpublishable plan: plan line 2: each jobs entry must be an object`, `:86` (same), `:100: plan line 2: each jobs entry must be an object`, and `plan_test.go:117: invalid plan stopped the tick:` for all 5 causes. FAIL. |
| Importers tested | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep internal/orchestrator` gives cmd/grid, internal/cli, internal/migrate, internal/orchestrator, lib. Rerun after fix round 1: `go test ./internal/cli/ ./internal/migrate/ ./lib/ ./cmd/grid/ -count=1` gives cli ok, migrate ok, lib ok, and cmd/grid has no test files. Exit 0. orchestrator: see 5a. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0. `GOOS=darwin go build ./...` exit 0. |
| No existing test edited except the ones the brief and fix round 1 allow | HOLDS | Two tests changed. In TestPlanPublicationValidatesBeforeWriting the `err == nil` check flipped, the "no Jobs published" check stays, and a cause check was added. In TestMalformedPlanDoesNotBlockOtherTasks only lines :85-87 changed, as fix round 1 directs. |
| No guard, gate, cap or hold | HOLDS | The change removes a stop: the tick goes on and posts one notice. |
| tick.go untouched | HOLDS | Not in the diff. |
| Error paths unchanged | HOLDS | Read and I/O errors (plan.go:32-34, :47-49), lock errors (:37-45), memory save errors, a failed notice Post (plan.go:78) and a failed Job Post (postPlanJobs) still return an error. |
| No attribution lines, no personal paths | HOLDS | Commit messages are single sentences. The diff has no paths, hosts or emails. |

## What changed

- plan.go: `publishPlanText` is split into `planJobInputs` (parses the text, checks each Job and the whole graph, posts nothing) and `postPlanJobs`. If `planJobInputs` returns an error, `planUnpublishable` sets `PlanPublicationBlocked` true and posts `{ref: task, kind: message, text: "plan.md cannot be published: <oneLine(err)>. No Job from plan.md is posted until it is fixed."}`. The post is skipped when the snapshot already holds a task-lease message with the same ref and text (the warnDuplicateGates rule). The tick then goes on as on a task without plan.md. When the plan is valid, blocked goes back to false and nothing extra is posted.
- Behaviour note: the dedup is by exact text. If the same fault comes back after a fix, no second notice is posted, because the first one is still on the Feed.

## NOT VERIFIED

Nothing here needs a live screen. The worker stderr and app `task_skipped` paths are no longer reached for a plan-text fault. Nothing in the sandbox exercises that end to end.

## Open items

- None. The plan_approval_test.go:85-87 item is closed by fix round 1 (commit 85d60c41).

DONE
