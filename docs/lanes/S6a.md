# S6a: remove the shell text guard

Base: lane-S5R (6ac08ed9). Removal list: simplify.md, "The shell text guard" row.

## What changed
- Deleted `internal/guard/host_targets.go` (1,268 lines: shell tokenizer, heredoc/subshell reader, write-target scan, floor scan).
- `internal/guard/host.go` is mixed. Removed the Bash branch of `EvaluateHost`, the `~/.grid-person` Bash check, and everything only they used: `gateVerb`, `feedAnswer`, `gateName`, `workflowName`, `projectWorkflowName`, `switchVerb`, `settingsName`, `personDirName`, `namesHost`, `namesFeed`, `readOnly`, `findWrites`, `shellPathWord`, `flatten`, `flattened`, `runsVerb`, `writesNamed`, `onlyReads`, `readerRuns`, `writesProjectWorkflow`, `writesWorkflowIn`, `floorReason`, `historyAt`, `workflowReason`. A Bash command now gets no decision from `EvaluateHost`.
- Kept in `host.go`: the file-tool branch of `EvaluateHost` (Write/Edit/MultiEdit/NotebookEdit on `.host`, Feed, gate files, settings files), `gridRoots`, `settingsAt`, `gateAt`, `projectWorkflowAt`, `hostFolder`, `taskAtFold`, `gateReason`, `switchReason`. Those refuse file tools, not shell text.
- `internal/hook` needed no code change: it only calls `guard.EvaluateHost`, and no hook path read shell text itself. Other Bash branches (`plan.go` recall gate, `home.go` own-script check) are not on the removal list and stay.
- Edit deny rules: `grid setup` code is in `internal/cli` and is untouched. `TestClaudeDenyRulesEqualWriterGuard` and `TestSetupClaudeCLI` (internal/cli/setup_claude_test.go) pass and prove the deny rules are written.
- New test `TestHostNeverRefusesBash` (host_gates_test.go) asserts that Bash commands that used to be refused now get no decision. It fails on base (base denies them) and passes here.

## Deleted tests (behaviour removed by the owner's direction: simplify.md, "The shell text guard ... Remove it")
- Whole files: `host_write_targets_test.go`, `host_write_targets_case_test.go`, `..._case_monotonic_test.go`, `..._fail_closed_test.go`, `..._quote_groups_test.go`, `..._rework_test.go`, `host_word_floor_test.go`, `host_history_test.go` (TestHostFencesBashWritesToTaskHistory), `host_main_denied_test.go` (TestHostLaneDeniesWhatMainDenies), `host_ordinary_reads_test.go` (TestHostOrdinaryReadsAllowed, ...StayDenied), `host_reads_back_test.go` (TestHostReadsBackStayDenied, ...RefusedVariable, TestHostPlainVarRules, TestHostTildeOpaque), `person_test.go` (TestHostBashPersonDirectory), `internal/hook/history_fence_test.go` (TestAgentBashAppendToHistoryDoesNotReleaseThePlan and its `bash` helper).
- Bash-only parts of mixed files: `host_approval_test.go` (Bash loops in TestHostApprovalReceipts and TestHostProtectsTaskWorkflow; the file-tool halves stay), `host_workflow_test.go` (TestHostAgentWorkflowShell; TestHostAgentWorkflowFileTools stays), `host_preview_test.go` (Bash loops in TestHostPreviewApprovalReceipt; the Write/Edit/MultiEdit/NotebookEdit cases stay; restored by lane S6aR), `host_project_workflow_test.go` (Bash loops in TestHostAgentProjectWorkflows; the Write/Edit/MultiEdit cases on the direct path and the symlink alias stay; restored by lane S6aR), `host_brief_test.go` (Bash table in TestHostRules), `task_gates_test.go` (Bash loop), `linked_test.go` (TestHostBashHistoryAndPreviewApprovalAreGateFiles).
- `host_gates_test.go`: removed TestHostDeniesTaskWriteVerbs, ...GateAndSwitchVerbsInEveryForm, ...QuotedVerbWords, TestHostAllowsOtherGridVerbsAndLookalikes, TestHostBashGatePaths, TestHostBashSettingsPaths, TestHostGateShellChecksNeedAGridFolder, TestHostDeniesFeedAnswerPosts, ...WithoutRole, ...WithdrawalPosts, the Bash subtest of TestAgentCannotWriteGuardOffIntoSettings, and the shared verb tables. File-tool tests stay: TestHostFileToolsGatePaths, TestHostFileToolsSettingsPaths, and the Write/Edit part of TestAgentCannotWriteGuardOffIntoSettings.

## Checks
| Check | Result |
|---|---|
| `go test -count=1 ./internal/guard/...` | ok, 1.1s (exit 0) |
| `go test -count=1 ./internal/hook/...` | ok, 2.1s (exit 0) |
| Importers of guard: hook; of hook: cli, packaging (go list) | hook ok, packaging ok, cli FAIL (see below) |
| `go test -count=1 ./internal/... ./cmd/...` | exit 1: two failures, both in packages I may not touch |
| `make lint` (go vet ./...) | exit 0 |
| `GOOS=windows go build ./...` | exit 0 |
| `GOOS=darwin go build ./...` | exit 0 |

## Open items (outside my packages)
1. `internal/cli` `TestInvalidPersonVerbsHookProbe/Bash` (person_gate_invalid_test.go:48) expects the hook to deny `grid approve T-01` from a builder through Bash. That is the removed behaviour. Lane S6b owns internal/cli: delete or change the Bash case there.
2. `internal/golden` `TestGoldenCorpus/plan-guard/bash-recall`: "listed in known-differences.txt but matches the recording; remove the line". The case now matches the recording. Remove that line from the known-differences list. I did not touch internal/golden.
Both failures follow from this change. Row 5 of the brief is not fully met until they are fixed.

## NOT VERIFIED
- No real Claude Code session was run to confirm the deny rules block an Edit on the real settings.
- Windows and macOS were built, not run.

## S6aR rework
- Golden: removed the `plan-guard/bash-recall` line from `golden/known-differences.txt` (the file is at the repo root `golden/`, not `internal/golden/`); the test now matches the recording.
- `go test -count=1 ./internal/guard/ ./internal/hook/ ./internal/golden/`: exit 0 (guard 1.3s, hook 2.1s, golden 0.2s).
- `go test -count=1 ./internal/... ./cmd/...`: exit 1, only `internal/cli` `TestInvalidPersonVerbsHookProbe/Bash` fails (`person_gate_invalid_test.go:48: unexpected end of JSON input`). It stays failing on this lane on purpose: lane S6b deletes that whole file (it imports the removed internal/persongate), and S6a lands after S6b.
- `make lint`: exit 0.

## S6aB (S6a re-applied on lane-S6b8, head b8378c46)
- `git cherry-pick 6ac08ed9..lane-S6aR` applied the three S6a commits; the only conflict was `internal/guard/linked_test.go`, in the first commit.
- Resolution: S6a deleted its only remaining test, `TestHostBashHistoryAndPreviewApprovalAreGateFiles` (the Bash case). S6b had already replaced the file's content with the hard-link test `TestALinkedTaskFileHoldsTheGuard`, and then removed that too. Kept: nothing, because the file held no file-tool cases on either side. Dropped: both tests, so the file is deleted.
- No other conflicts. `TestInvalidPersonVerbsHookProbe` is absent (S6b deleted it); S6a does not bring it back.
- `go vet ./...`: exit 0. `gofmt -l internal cmd`: exit 0, no output. `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`: exit 0.
- `go test -count=1 ./internal/guard/ ./internal/cli/ ./internal/hook/`: exit 0 (`ok guard 1.231s`, `ok cli 13.252s`, `ok hook 2.057s`). `./internal/packaging/` (imports hook): exit 0.
