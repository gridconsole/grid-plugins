# PLUG-words

## Changed
- Row 1: the role refusal (`internal/cli/role_policy.go`) already said `role_not_allowed` and no longer said "outside Claude Code"/`agent_env`. It now also names the roles the policy allows ("; allowed: member, lead"). `role_not_allowed` is already a reason in use; no contract reason table lists it, so no generated file changes. `contract/README.md` line 366 still said `agent_env`; it now says `role_not_allowed` (hand-written file).
- Row 2: `guestWriteVerb`'s early refusals no longer copy wording. `question.WithdrawRefusalText` and `task.MoveRefusal` are used by both the stores and `cli.go`; `guestSettingsRefusal` is one const used by the five CLI sites that said "guests cannot change project settings".
- Row 3: NOT DONE, nothing to change on this base. There is no hard-link hold notice reading "workflow requires approval ... run grid approve" and no `ErrPlanNeedsApproval` anywhere (grep over the tree); `jobcmd/command.go:92` and `lint/lint.go:47` hold unrelated code. The hard-link refusal was removed (`jobcmd/command_test.go:43`). The brief seems written against a different base; point me at the text if it lives elsewhere.

## Row 4: remaining texts naming a shell verb
- internal/task/gates.go:305 "run grid approve to post it" (error text)
- internal/hook/hook.go:160 "grid approve in a terminal outside Claude Code or Codex" (pinned by internal/hook/hook_test.go:76)
- comments only: internal/cli/task.go:132, internal/cli/orchestrator.go:362, internal/task/hold.go:135
Not changed (outside rows 1-3).

## New tests (fail on base, pass on lane)
- `TestRoleRefusalNamesAllowedRoles` (internal/cli/role_words_test.go): base prints no "allowed:" list. Not run on base.
- `TestGuestWriteRefusalMatchesStores` (same file) and `TestTrashRefusalIsMoveRefusal` (internal/task/move_refusal_test.go): fail on base by build error (no `WithdrawRefusalText`/`MoveRefusal`). Not run on base.

## Checks (exit 0 each)
- go build ./...; GOOS=windows go build ./...; GOOS=darwin go build ./...; gofmt -l (empty); go vet on cli, task, question
- go test: internal/cli, task, question, braincmd, guard, hook, job/..., land, signal, orchestrator, lease, packaging/..., cmd/...  (importers of task/question)
- make lint not run (per brief). Goldens to regenerate: none expected.

## NOT VERIFIED
Row 3; whole-module test; the new-tests-fail-on-base run.
