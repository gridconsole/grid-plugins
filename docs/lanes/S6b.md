# S6b: remove person-by-ancestry, hard-link refusals and the workflow-digest hold

Status: INCOMPLETE. The branch does not build and no check was run. The budget ran out at about call 55 of 60, after the removal edits and before the first compile.

## Done (edits made, not compiled)
- Deleted `internal/persongate`, `internal/cli/person_gate.go`, `internal/testenv/person.go` and `person_test.go`.
- `internal/cli`: removed every `gateCommand` call (cli.go, signal.go, usage.go, settings.go), the `Procs`, `PersonDir`, `personVia` and `personSetupWrite` fields, `gateWorkflowPolicy`, `withPersonErrorCode` and `grid person check`. The workspace-budget caller-role check stays. `feed.go` takes the actor from `GRID_ACTOR` whenever it is set.
- Hard links: deleted `internal/task/linked*.go` and `internal/workflow/linked*.go`. Removed the `Linked` calls in task.go, write.go, history.go, hold.go and create.go, and the refuse-linked calls in workflow policy_file.go, files.go and write.go. Removed the linked handling in workflow read.go, person.go, project_person.go and inbox_workspace.go, in cli/signal.go and in `internal/feed/identity.go`. `internal/feed` is not on the may-touch list; the edit was forced by the removal of `workflow.RefuseLinked`.
- Digest hold: removed `WorkflowDigest`, `DefinitionDigest`, `ErrWorkflowChanged`, `sameDigest` and the `workflow_digest` field, with their use in hold.go, create.go, deliver.go and the `workflow_changed` mapping in cli/error_codes.go.
- The rest of `internal/task/hold.go` (`StepsFor`, `PlanStage*`, `PlanHeldWith`) stays: `internal/guard`, which lane S6a is changing, calls it.

## NOT done
- `internal/job/source.go` (`linkedPlanError`, the `task.Linked` call, `ErrPlanNeedsApproval`), the Linked block in `internal/orchestrator/plan.go`, and the mappings in `jobcmd/command.go:92` and `lint/lint.go:47`.
- The `feedjob.go` `loadJob` fallback rule.
- The publication notice text "workflow requires approval ... run grid approve".
- Compile fixes, removal of unused imports and the `linked_file` and `person_required` constants.
- Tests: every test that used persongate, `testenv.Person` or the linked and digest errors still has to be deleted or rewritten. Candidates: `internal/cli/{person_gate,person_gate_invalid,project_person_gate,linked_code,linked_policy_closed,linked_workflow_gate,task_new_linked}_test.go`, `internal/orchestrator/plan_linked_test.go`, and the usage and signal gate tests. Each deletion needs a reason in this file: behaviour removed by the owner's direction in /srv/gridwork/briefs/simplify.md, row "Hard enforcement around those gates".
- New tests for rows 1 to 3: an agent environment running `grid task new`, `grid approve` and `grid job render`; a hard-linked plan.md rendering its Jobs; an approval naming the plan digest.
- `internal/task/create.go` `taskFront` is now a trivial wrapper and can be inlined.

## Checks
None run. Exit codes: n/a. The per-package test list, lint, and the Windows and macOS builds are all NOT RUN. Rows 1 to 5 are NOT MET.

## Split suggestion
Continue on this branch: (1) job, orchestrator and jobcmd cleanup, (2) `go build ./...` and `go vet` fixes, (3) delete or rewrite the tests listed above with reasons, (4) add the three new tests.

# S6b2 (continuation, branch lane-S6b2)

Status: builds. Production code only; no test was touched.

## Done
- `internal/job/source.go`: removed `linkedPlanError`, the `task.Linked` call and `ErrPlanNeedsApproval`; `planReadable` now returns nil. Added `ErrJobNotInPlan`, wrapped by `LoadPlan` when plan.md names no such Job.
- `jobcmd/command.go` and `job/lint/lint.go`: removed the `ErrPlanNeedsApproval` mappings.
- `orchestrator/plan.go`: removed the Linked block and the "workflow requires approval ... run grid approve" notice.
- `jobcmd/feedjob.go` `loadJob`: falls back to the Feed only when plan.md is missing (`os.IsNotExist`) or does not name the Job (`ErrJobNotInPlan`); any other plan error is returned.
- Compile fixes: unused imports (workflow, feed, task, cli, job, orchestrator); `ErrWorkflowChanged` reference in task/hold.go; removed the `person_required`, `linked_file` and `workflow_changed` code constants; restored `jsonRequested` in cli/error_codes.go (it was in the deleted person_gate.go and `Run` still uses it); dropped stale ErrLinkedFile comments. `taskFront` left as is (optional).
- Approval record still carries `plan_digest` (task/gates.go:85,105,213).

## Checks
- `go build ./...`, `GOOS=windows go build ./...`, `GOOS=darwin go build ./...`: exit 0.
- grep for Linked|RefuseLinked|persongate|WorkflowDigest|ErrPlanNeedsApproval in non-test .go files: 0 matches.
- `go vet ./...`: fails only in test files (below). Tests, lint: NOT RUN (test files do not compile).

## For S6b3 (test files that no longer compile)
- internal/cli/person_gate_invalid_test.go: imports deleted internal/persongate. Also reference removed `CodePersonRequired`/`CodeLinkedFile`: signal_policy_code_test.go, linked_policy_closed_test.go, project_person_gate_test.go, linked_workflow_gate_test.go.
- internal/feed/linked_task_test.go: workflow.ErrLinkedFile removed.
- internal/guard/plan_hold_swap_test.go: taskstore.ErrWorkflowChanged removed.
- internal/job/approval_gaps_test.go: ErrPlanNeedsApproval removed.
- internal/job/approval_workflow_test.go: task.ErrLinkedFile removed.
- internal/job/linked_plan_test.go: task.ErrLinkedFile and ErrPlanNeedsApproval removed.
- internal/signal/turn_linked_test.go: task.ErrLinkedFile removed.
- internal/task/create_linked_test.go: ErrLinkedFile removed.
- internal/workflow/linked_resolve_test.go, linked_task_test.go: ErrLinkedFile removed.
(vet stops at the first error per package, so more may surface once these are fixed.)
