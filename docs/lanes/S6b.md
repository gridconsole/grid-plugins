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

## S6b3 test changes
Reason for every row unless noted: behaviour removed: simplify.md row Hard enforcement around those gates.
- internal/workflow/linked_resolve_test.go: whole file (TestTaskWorkflowKeepsTheLinkSentinel).
- internal/workflow/linked_task_test.go: whole file (TestLinkedTaskWorkflowFileIsUnreadable, TestInboxWorkspaceLinkedFile).
- internal/feed/linked_task_test.go: whole file (TestEnsureTaskIDRefusesALinkedTaskMD, TestQuestionWithdrawPolicyRefusesALinkedWorkflowFile).
- internal/task/create_linked_test.go: whole file (TestNewRefusesALinkedWorkflowsFile; it also asserted workflow_digest, which New no longer writes).
- internal/task/linked_writers_test.go: whole file (TestP1/TestP2 AppendHistory linked, TestTaskWritersRefuseALinkedFile, TestPostToolUseHookThroughTheBinaryKeepsALinkedTaskHeld).
- internal/job/linked_plan_test.go: whole file (TestLinkedPlanIsHeldWithLinkedFile).
- internal/job/approval_workflow_test.go: TestPlanOmittedAnchorsRefused keeps the omitted/covered anchor rows; the hard-link tail (linked_file rows, refused approve) removed.
- internal/job/approval_gaps_test.go: TestSourceSwapUsesIdentifiedHandle no longer accepts ErrPlanNeedsApproval as a safe outcome; Render must succeed and return the identified snapshot.
- internal/signal/turn_linked_test.go: rewritten as TestTurnMakesATask (unlinked turn makes a task and marks the note done); linked rows and the workflow_digest assertion removed.
- internal/cli/linked_workflow_gate_test.go, linked_policy_closed_test.go: whole files.
- internal/cli/project_person_gate_test.go: whole file (person_verbs policy, persongate).
- internal/cli/signal_policy_code_test.go: TestSignalMovePersonGateCode removed; TestSignalMoveInvalidPolicyCode kept.
- internal/cli/person_gate_invalid_test.go: TestInvalidPersonVerbsHookProbe and TestInvalidPersonVerbsCommands removed; TestHookCommandsBypassPersonPolicy kept.
- internal/cli/person_gate_test.go: all person-gate tests removed (TestPersonVerbRefusals ... TestPersonAnswerScopeAndWorkflowRoles); fileBytes and TestPersonAnswerAuthorship kept; testProcs and personTestEnv removed with Env.Procs/PersonDir.
- internal/task/mode_test.go: seedHistory helper restored (it was in the deleted linked_test.go; TestRewriteKeepsTheFileMode still uses it).

## S6b3 not finished (budget)
internal/cli tests still do not vet: they use Env.Procs, Env.PersonDir, testProcs, personTestEnv, personHint, CodePersonRequired, CodeLinkedFile. Files: signal_inbox_test, delivery_test, usage_workspace_gate_test, usage_budget_gate_test, plan_policy_test, task_new_linked_test, withdraw_post_test, error_codes_test, land_test, agent_test, workflow_policy_test, cli_test, settings_access_test. Outside the allowed packages and also broken: internal/settings/claude_cli_rework_test.go and internal/question/withdraw_cli_test.go (persongate, Env.Procs), and testenv.Person is gone (non-test file in internal/testenv). Not run: go test of any package, lint, Windows/macOS builds. Acceptance rows 1, 2 and 4 are NOT met; the workspace-budget caller-role test is in usage_workspace_gate_test.go and was not touched.

## S6b3 additional deletions (found by running tests)
- internal/task/workflow_digest_test.go: whole file (TestWorkflowDigestReadsNameKindGateAndOrder); WorkflowDigest removed, behaviour removed: simplify.md row Hard enforcement around those gates.
- internal/orchestrator/plan_linked_test.go: whole file (TestLinkedTaskPostsNoJobs); hard-link hold removed, same reason.
- internal/orchestrator/plan_approval_test.go: TestPlanPublicationNoticeRepeatsOnlyAfterUnblocked removed (its only hold was a hard-linked plan.md), same reason.

## S6b3 checks
- go vet ./internal/feed ./internal/job/... ./internal/signal ./internal/task/... ./internal/workflow ./internal/orchestrator: exit 0.
- go test -count=1 of the same packages: ok for feed, job, job/frontmatter, job/jobcmd, job/lint, signal, task, workflow; orchestrator re-run after the deletions was not repeated (vet only).
- internal/cli, internal/settings, internal/question: do not vet (see above). NOT run: lint, Windows/macOS builds, importer-package tests.

## S6b4 test changes
Reason for every deletion unless noted: behaviour removed: simplify.md row Hard enforcement around those gates.
- internal/cli/task_new_linked_test.go: whole file (TestTaskNewFromTaskFolderRefusesLinkedWorkflows; hard-link refusal and workflow_digest).
- internal/cli/linked_code_test.go: whole file (TestLinkedFileHasAStableCode; task.ErrLinkedFile removed).
- internal/cli/person_gate_invalid_test.go: whole file (TestHookCommandsBypassPersonPolicy; person_verbs policy removed).
- internal/cli/signal_inbox_test.go: TestSignalMoveInboxRetainsPersonGate removed; TestSignalMoveNamedIgnoresInboxPolicy stays.
- internal/cli/plan_policy_test.go: TestApproveWorkflowGuestGateFirst removed (person gate first); digest-table test stays.
- internal/cli/workflow_policy_test.go: TestWorkflowPolicyPreservesGateRefusal removed (person_required on workflow show).
- internal/cli/error_codes_test.go: rows person-gate and person-check removed; the other rows stay.
- internal/cli/settings_access_test.go: TestSettingsPersonalWritesRefused now runs role member only; the guest/unknown/external/lead rows asserted the person refusal.
- internal/cli/guest_parity_test.go: guestParityPersonRefusal and the "floor" branch of guestParityEarlyAnswer removed (see failures below).
- internal/cli/usage_budget_gate_test.go: TestUsageBudgetPersonGate and TestUsageBudgetGateKeepsSyntaxAndWorkspaceAnswers replaced by TestUsageBudgetSetAndClear and TestUsageBudgetSyntaxError.
- internal/cli/usage_workspace_gate_test.go: person-gate, workflow-policy and runWorkspaceBudgetGated-person tests removed; TestUsageWorkspaceBudgetCallerRole (worker/builder refused with exit 3, ""/owner allowed, syntax exit 2) and TestUsageWorkspaceBudgetShowStaysOpen kept. usage_test.go: runWorkspaceBudgetGated call lost its person func argument.
- personTestEnv/testProcs lines dropped from agent_test, cli_test, land_test, delivery_test, withdraw_post_test, error_codes_test.

## S6b4 checks
- go vet ./internal/cli/: exit 0.
- go test -run 'TestUsageWorkspaceBudgetCallerRole|TestUsageWorkspaceBudgetShowStaysOpen|TestUsageBudget' ./internal/cli/: ok (all PASS).
- go test -count=1 ./internal/cli/: exit 1, 11.2s. NOT met.

## S6b4 not finished (budget ran out)
Failing, all unexamined; most look like leftovers of removed person gate / workflow_digest / hard-link, but the first two lines may show a production difference:
- TestApproveGateAgentRefused: exit 3 as wanted but message is now "Approval and refusal decisions require a person or the app..." (a different gate that still exists; check expected text).
- TestContractCommandsParserInventory: annotation for removed route "person check" still in the inventory file.
- TestDeliverWorkflowChangedThroughCLI, TestSizedLaterKeepsWorkflowDigest, TestInlineRewriteOfStampedTaskNotHeld, TestPlanDigestRoundTrip, TestBlockedPublicationNoticeUsesCLIStderrOnce: workflow_digest / workflow_changed hold, likely delete.
- TestGuestProjectParity (many rows: task restore, question withdraw, approve...), TestGuestProjectSettingsWrites: my removal of the "floor" branch left the guest rows that used to expect person_required; they need the new expected answer from the product.
- TestPersonalBypassTaskCommands, TestTaskPersonGates, TestSettingsPersonalAllowedCallers, TestSettingsRefusalCreatesNothing, TestAnswerFeedAndRecovery, TestTaskRenameDecline, TestTaskWriteGolden, TestAgentWithdrawUnreadablePolicy: not looked at.
Not run: lint, Windows/macOS builds, importer-package tests.

## S6b5 test changes
Production: internal/cli/task_write.go `personWrite` removed; `grid approve` and `grid refusal decide` no longer refuse when GRID_AGENT, GRID_WORKER_ID or GRID_WORKER is set (behaviour removed: simplify.md row Hard enforcement around those gates). `agentShell` stays: trash, question, feed and task.go still pass it to the task/question packages (not looked at further; those packages are S6b6's).

Deleted:
- deliver_workflow_changed_test.go (TestDeliverWorkflowChangedThroughCLI, TestSizedLaterKeepsWorkflowDigest, TestInlineRewriteOfStampedTaskNotHeld): workflow_digest / workflow_changed hold; behaviour removed: simplify.md row Hard enforcement around those gates.
- TestBlockedPublicationNoticeUsesCLIStderrOnce: its only hold was a hard-linked plan.md; behaviour removed by S6b (hard-link).
- TestTaskPersonGates: checked the agent-env refusal of approve/refusal decide; replaced by TestApproveGateAgentApproves.
- "person check" row in the inventory test and contract/commands.json (route removed).
- question_test.go: the "lease-marked answer exits 3" step in TestAnswerFeedAndRecovery (removed refusal; the answer step after it still covers answering).
Changed expectations:
- TestApproveGateAgentRefused -> TestApproveGateAgentApproves: agent env approves, event names plan_hash. Failed on base with exit 3.
- TestPreviewAgentRaisesButCannotApprove -> TestPreviewAgentRaisesAndApproves: agent approve exits 0 and writes the preview receipt.
- TestPersonalBypassTaskCommands/stop: hidden and missing task both exit 1 (was 3, the person gate).
- TestSettingsPersonalAllowedCallers: every caller role may write Personal settings (no exit 3).
- TestSettingsRefusalCreatesNothing, TestTaskRenameDecline guest rows, TestGuestProjectSettingsWrites: guest refusal is exit 1 with the ordinary message ("task not found" / "guests cannot change project settings"), not exit 3 agent_env; nothing is written.
- Receipt/stdout key "via" dropped from testdata/contract/plan-digest/transcript.json and the six task-*-json goldens (no code writes it; removed with the gate).
- contract/commands.json is outside the may-touch list but is the inventory file the test compares; edited by hand.

## S6b5 not finished
- TestGuestProjectParity (task delete/restore, question withdraw, setup rows, invited guest): the invited guest now gets different answers than the unknown-project answer (exit 3 "a person with an allowed role must perform this task move", exit 2 usage / bad question id, exit 1 "guests cannot change project settings"). That may leak project existence; it is a product question, so the test is left failing.
- TestAgentWithdrawUnreadablePolicy/question: "cannot read task withdrawal policy (workflow.md): is a directory" exit 1; not examined (withdraw policy code is in internal/question, S6b6's area).
- gofmt -l lists internal/cli/signal_inbox_test.go (not touched by this lane).
- Importer packages: only internal/cli and cmd importers by go list; their tests were not run.
## S6b6 test changes (outside internal/cli)
- internal/question/withdraw_cli_test.go TestPersonWithdrawCLI: kept, rewritten. Dropped terminalProcs, Env.Procs, PersonDir, testenv.Person and the agent_env override of the expected stderr; rows now expect the question package's own refusals (kept behaviour).
- internal/settings/claude_cli_rework_test.go: kept; removed terminalProcs, personEnv and the persongate import (behaviour removed: simplify.md row Hard enforcement around those gates).
- internal/guard/plan_hold_swap_test.go (TestNamedWorkflowSwappedProjectFolder, swapRows and helpers mustRename/copyInto/rebuild, used nowhere else): deleted; behaviour removed: simplify.md row Hard enforcement around those gates (workflow_digest hold, ErrWorkflowChanged).
- internal/guard/linked_test.go TestALinkedTaskFileHoldsTheGuard: deleted; behaviour removed: simplify.md row Hard enforcement around those gates (hard-link refusal). TestHostBashHistoryAndPreviewApprovalAreGateFiles stays.
- internal/guard/plan_stage_steps_test.go TestNamedWorkflowProposedEdit row "names a template with no plan gate": expected stage changed from "unreadable" to "" (no digest mismatch to make it unreadable); the guard still denies the edit, so want stays "deny".
- internal/orchestrator tests re-run: pass (17s).

## S6b6 checks
- go vet ./...: exit 0 (clean, including internal/cli).
- go test -count=1 ./internal/... ./cmd/...: only FAIL is internal/cli (13.0s); every other package ok. Importers of the changed packages (guard, settings, question, orchestrator) pass.
- GOOS=windows go build ./...: exit 0. GOOS=darwin go build ./...: exit 0.
- make lint: not run (not on the allow list).

## S6b6 internal/cli failures left for S6b5 (not touched)
TestApproveGateAgentRefused, TestContractCommandsParserInventory, TestDeliverWorkflowChangedThroughCLI, TestSizedLaterKeepsWorkflowDigest, TestInlineRewriteOfStampedTaskNotHeld, TestGuestProjectSettingsWrites, TestGuestProjectParity, TestBlockedPublicationNoticeUsesCLIStderrOnce, TestPersonalBypassTaskCommands, TestPlanDigestRoundTrip, TestAnswerFeedAndRecovery, TestSettingsPersonalAllowedCallers, TestSettingsRefusalCreatesNothing, TestTaskRenameDecline, TestTaskWriteGolden, TestTaskPersonGates, TestAgentWithdrawUnreadablePolicy.
