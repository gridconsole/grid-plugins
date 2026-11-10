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
- Tests: every test that used persongate, `testenv.Person` or the linked and digest errors still has to be deleted or rewritten. Candidates: `internal/cli/{person_gate,person_gate_invalid,project_person_gate,linked_code,linked_policy_closed,linked_workflow_gate,task_new_linked}_test.go`, `internal/orchestrator/plan_linked_test.go`, and the usage and signal gate tests. Each deletion needs a reason in this file: behaviour removed by the owner's direction in the simplify brief, row "Hard enforcement around those gates".
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

## S6b7
- TestGuestProjectParity: on base the person gate refused task delete, task restore, question withdraw and setup for a guest before any project was resolved, so every project state got one answer. Now `Run` in internal/cli/cli.go (`guestWriteVerb`) gives a guest role (GRID_ROLE=guest, no task lease) those same refusals before resolving, using the messages the stores already gave (exit 3 task edit rejected, exit 3 question role may not withdraw, exit 1 guests cannot change project settings). This is a role rule; nothing checks agents. The test file only gained accepted early answers for those four commands (`guestParityEarlyAnswer`); no row was removed or weakened.
- TestAgentWithdrawUnreadablePolicy/question expected the removed person gate's answer (exit 3, agent_env). It now expects exit 1 "cannot read task withdrawal policy", as the feed, job and mcp rows already did. Not a regression.
- gofmt -w on signal_inbox_test.go and jobcmd/command_test.go.
- New test TestRenderHardLinkedPlan (internal/job/jobcmd): fails on base 6ac08ed9 with `linked_file: a file of this task has more than one hard link` (exit 2), passes now.
- Row 1 (agent environment succeeds): TestApproveGateAgentApproves already exists. On base 6ac08ed9 it fails: `agent approve: exit 3: grid approve: a person must run this ... (reason: agent_env)`. `grid task new` and `grid job render` under GRID_AGENT/GRID_WORKER_ID already succeeded on base (no gate), so no new test for them.
- Base checks were made on scratch branches from 6ac08ed9 with the lane test files checked out onto them.
- Checks: go test -count=1 ./internal/... ./cmd/... exit 0; go vet ./... exit 0; gofmt -l internal cmd clean; GOOS=windows and GOOS=darwin go build ./... exit 0. make lint not run (not on allow list). UI: none.

# S6b8 (rework for the S6b7 review; branch lane-S6b8, from f3d3ee08)

## Production changes
1. No agent-environment refusal is left in task trash, question withdraw or the feed withdrawal post.
   - `internal/cli/task_write.go`: `grid task delete|restore` no longer passes `Agent` or `PersonalAllowed`. `agentShell` stays, with a new comment: only `task attach` uses it (internal/cli/task.go:211, the scope rule that binds an agent to its assigned task, kept as the brief says).
   - `internal/task/trash.go`: removed the `Agent` and `PersonalAllowed` fields of `TrashOptions`, the `o.Agent` refusal and `trashPersonal` ("Personal tasks require the person guard") with its two calls. The role policy (`DeleteAllowed`, `RestoreAllowed`), the empty-`Who.ID` refusal and the resolver's replaceable `PersonalAllowed` caller policy stay.
   - `internal/question/question.go`, `command.go` and `internal/cli/question.go`: removed `Store.AgentShell`, `Env.AgentShell`, the refusal "agent shell requires a task lease to withdraw" (it also covered writer roles "agent" and "cli"; `WithdrawAllowed(role)` still refuses a role the policy does not list) and the bare-writer branch in `questionEnv`.
   - `internal/cli/feed.go`, `mcp.go`, `orchestrator.go`: removed `refuseWithdrawalPost` and `withdrawalPostRefusal` ("agent shell requires a task lease fence to withdraw") with its three call sites. The lease and fence checks of the feed writer stay.
2. The workflow policy's role lists are enforced again: new `internal/cli/role_policy.go`, called from `Run` right before the command runs (after the guest write-verb answers, so a guest gets the same answer for every project state). For each policy that names the verb (task-local `workflow.md`, project or workspace `workflows.md`, signal act aliases as on base), `GRID_ROLE` (default `member`) must be in its role list, otherwise the refusal is exit 3 with the base text `grid <verb>: a person must run this, from the Grid app or your own terminal outside Claude Code or Codex (reason: agent_env).` and, with `--json`, `{"error":"person_required","reason":"agent_env","hint":...}` on stdout. It reads the role only: no process, no ancestry, no agent variable.
   - Kept from the base gate: hook commands (except `hook level`) are not checked; a standalone `--help`/`-h` is not checked; `question withdraw` with `GRID_LEASE=task` is not checked (the lease rule); a policy that cannot be read adds no role rule.
   - Not restored (they belonged to the person floor, not the role list): the person-only floor verbs, the `setup` preview exemption (with a role list on `setup`, the preview is checked too), and the warning "invalid person_verbs; ignoring the key and keeping the person-only floor" (adding a variant of it failed three existing stderr tests, so the key is ignored silently; `grid workflow show` reports the bad file).

## Test changes (each changed expectation or deletion)
- `internal/task/trash_test.go`: `trashOptions` lost `PersonalAllowed: true`. `TestTrashPersonalAndPersonGuards` is renamed `TestTrashRequiresAWho` and keeps only the "empty Who.ID refused" row; the "personal" (PersonalAllowed=false) and "agent" (Agent=true) rows are removed: behaviour removed by this brief (must-fix 1). Roles are still covered by `TestTrashPolicies`; the resolver's Personal policy by `TestTrashResolverPersonalPolicy`.
- `internal/cli/task_trash_test.go`: rows "agent" and "restore_agent" removed from `TestTaskTrashRefusals` (they expected exit 3 for an owner with `GRID_AGENT=worker`; removed by this brief). New `TestTaskTrashAgentEnvironmentAllowed`: delete then restore exit 0 for an owner with `GRID_AGENT` or `GRID_WORKER_ID` set.
- `internal/cli/withdraw_post_test.go`: `TestFeedPostAgentWithdrawalRefused`, `TestJobPostAgentWithdrawalRefused`, `TestMCPFeedPostAgentWithdrawalRefused`, `TestAgentWithdrawalPostVariants` and the helper `assertWithdrawalUnchanged` deleted: they asserted only the removed refusal (behaviour removed by this brief; the role and lease rules stay covered by `TestAgentWithdrawalWithTaskFence`, `TestWithdrawPolicyRefusals` and `TestAgentWithdrawUnreadablePolicy`). New `TestAgentEnvWithdrawalPostSucceeds`: a withdrawal post by `grid feed post` and MCP `feed_post` under `GRID_AGENT` or `GRID_WORKER_ID` closes the question.
- `internal/question/question_test.go` `TestWithdrawPolicyRefusals`: rows "agent cannot bypass policy" and "worker shell" and the `agent` field removed (the refusal is gone); "default guest" and "custom owner only" stay.
- `internal/question/withdraw_cli_test.go` `TestPersonWithdrawCLI`: rows "agent shell", "worker id" now expect a successful withdrawal (renamed "agent env member withdraws", "worker id owner withdraws"). Row "worker" is renamed "worker mismatch" and expects exit 1 `GRID_WORKER must match GRID_MEMBER and the local machine id`, the feed writer's identity check, which is unchanged.
- New `internal/cli/role_policy_test.go`: `TestPersonAnswerScopeAndWorkflowRoles` restores the rows guest_other_question, guest_workflow_denied, lead_workflow_denied, guest_workflow_allowed and lead_workflow_allowed for `answer` and `question answer` from 6ac08ed9 (no process-tree setup), adds member_workflow_allowed and default_role_is_member, and checks exit 3 with the `person_required` JSON code and no file change on a refusal. `TestWorkflowRoleListIgnoresAgentEnvironment`: `person_verbs: {answer: [member]}` admits a member and refuses a guest with `GRID_AGENT` set, so the rule looks at the role only.

## Acceptance rows
1. Row 1, grep of non-test .go files (`agentShell|GRID_AGENT|GRID_WORKER_ID|GRID_WORKER\b|PersonVerbs|AgentShell`), one line per hit:
   - internal/cli/task_write.go:205-209 `agentShell`: reads GRID_AGENT, GRID_WORKER_ID, GRID_WORKER and a non-person GRID_ROLE; used only by task attach.
   - internal/cli/task.go:211 `Agent: agentShell(env)`: task attach binds an agent to its assigned task folder (scope rule, kept by the brief).
   - internal/cli/feed.go:328-329 `GRID_WORKER`: the feed writer's identity check, refuses a GRID_WORKER that is not `<member>@<machine>`. It does not decide who is a person and is on base; NOT in the brief's removal list. See open items.
   - internal/hook/events.go:258,266 `GRID_WORKER`: the hook's own test for a worker session with a Job; refuses nothing. Outside the may-touch list.
   - internal/cli/role_policy.go:117,120 and internal/workflow/person.go, project_person.go `PersonVerbs`, `ProjectPersonVerbs`, `DefaultPersonVerbs`: read the role lists; the only caller is the role check.
2. Row 2: `TestTaskTrashAgentEnvironmentAllowed` and `TestAgentEnvWithdrawalPostSucceeds`. On f3d3ee08 (scratch branch, lane test files checked out onto it) they fail: `task_trash_test.go:92: delete exit = 3: grid task delete: task edit rejected: a person with an allowed role must perform this task move` (both markers) and `withdraw_post_test.go:44: withdrawal under GRID_AGENT: exit=3 ... stderr=grid feed: agent shell requires a task lease fence to withdraw`; the MCP rows fail with an `isError` reply.
3. Row 3: `TestPersonAnswerScopeAndWorkflowRoles` and `TestWorkflowRoleListIgnoresAgentEnvironment`. On f3d3ee08 the denied rows fail: `role_policy_test.go:41: answer exit 0: {"id":"Q-1",... "state":"answered","answer":"Yes", ... "role":"guest" ...}` (also lead, both verbs) and `role_policy_test.go:70: exit 0, want 3` for the guest. They pass on the lane.
4. Row 4: see Checks.

## Checks (on the lane head)
- `go build ./...`: exit 0. `go vet ./...`: exit 0. `gofmt -l internal cmd`: no output. `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`: exit 0 (one chain, printed CHAIN_OK).
- `go test -count=1 ./internal/... ./cmd/...`: every package `ok` or `[no test files]`, no FAIL line; 55 packages ok. The count came from `go test ... | grep -cE "^ok "`, so the printed exit status was grep's; the first run piped through `grep -vE "^ok "` showed no FAIL.
- `go test -count=1 ./internal/cli/ ./internal/task/ ./internal/question/`: ok (cli 13.2s).
- Importers of internal/cli, internal/task, internal/question (go list): cmd/grid, braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, orchestrator, signal, task, question; all inside the whole-module run above.
- `make lint`: exit 0 (its target prints `go vet ./...`).

## NOT VERIFIED
- UI: none changed.
- The exit status of the whole `go test` run itself (see above); the orchestrator's landing run is the check.
- The setup preview under a `setup` role list (not restored; no test).

## Open items for the orchestrator
- The role refusal keeps the base wording and the reason `agent_env`, as the brief says. The text now misleads (it says "a person must run this ... outside Claude Code" for a role that is simply not listed). Suggest new wording and a `role_not_allowed` reason in a later lane; the JSON code `person_required` is the same on base.
- `internal/cli/feed.go:328` still refuses a `GRID_WORKER` that does not match `<member>@<machine>`. It is an identity check on the writer, on base, and not in this brief's list; say if it should go.
- `internal/cli/task.go:211` keeps `agentShell` for task attach as the brief directs; it is the one remaining place an agent variable changes behaviour.
