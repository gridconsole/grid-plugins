# S6tidy

Base: lane-S6c4 (898766c4).

## Changed
1. internal/guard/host.go: `gateReason` and `switchReason` no longer name shell verbs or `cat, grep ...`; they state the file-tool case and keep the phrase "Reading is allowed." (host_gates_test.go matches it).
2. internal/cli/role_policy.go: role refusal reason is `role_not_allowed`, hint is "your role is not in this workflow's list for this command". JSON error code `person_required` and exit 3 unchanged. internal/cli/role_policy_test.go rows updated to the new reason.
3. `agentShell`: only one use is left, task.go:211 (task attach). No use in trash, question, feed or task other than that; nothing to remove. What it does: for an agent env (GRID_AGENT, GRID_WORKER_ID, GRID_WORKER, or a role other than owner/member/cli), `task attach` must resolve GRID_TASK to a task folder and the target must be that same folder, else it errors ("agent cannot attach ... "). That is a scope rule by caller kind, not a shell-verb refusal. Left as is.

## Checks
- go test ./internal/... ./cmd/... : exit 1, only internal/cli fails, all from S6c4's gate-flag removal, none from this lane:
  TestContractCommandsInventory (45 vs 42), TestContractCommandsParserInventory (commands.json, `--gate`), TestGuestProjectParity (missing refusal decide, decide, preview rows), TestTaskOperandHelp/approve (golden approve-help still has --gate).
  Goldens/files to regenerate: contract/commands.json, the approve-help golden, the guest parity table, the command table in contract_commands_test.go.
- Packages importing the changed ones: cli, guard, hook (guard imported by hook; cli imported by cmd). All pass except the four above.
- GOOS=windows go build ./... exit 0; GOOS=darwin go build ./... exit 0; gofmt -l clean. make lint not run (orchestrator).

## Not done / notes
- contract/README.md:366 still says `agent_env` for a role the person gate refuses (references internal/persongate/gate.go, which no longer applies); contract rows naming the role refusal reason: README only, no JSON contract file matched.
- internal/hook/hook.go:160 and hook_test.go:76 still say "terminal outside Claude Code or Codex" in the hook text; outside the brief rows, left.
- No new test: the changed reason is covered by the edited role_policy_test rows (they failed with the old reason string against the new code before the edit).
