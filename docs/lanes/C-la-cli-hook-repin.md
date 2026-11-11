# C-la-cli-hook-repin

The CLI hook tests no longer lean on the host guard's deny. The proof that the PreToolUse hook ran is now the Bash snapshot record.

## Proof table

| Row | Result |
|---|---|
| 1. no `host-guard` in cli_test.go, kg_writer_deny_test.go | HOLDS: `git grep -n 'host-guard' -- internal/cli/cli_test.go internal/cli/kg_writer_deny_test.go` prints nothing, exit 1 (no match) |
| 2. TestLaCliHookRecordsBash | HOLDS: internal/cli/la_cli_hook_record_test.go. `Run([]string{"hook","pretooluse"})` with Bash, session-demo, `use-1`, cwd in a `task.Store.New` task: exit 0, no `"deny"`, snapshots == [use-1]. A second call with an empty tool use id leaves no snapshot. |
| 3. `go test -count=1 -v -run 'TestHookPreToolUse\|TestKgWriterHookNoLongerRefuses\|TestLaCli' ./internal/cli/` | HOLDS, exit 0: `--- PASS: TestHookPreToolUse`, `--- PASS: TestKgWriterHookNoLongerRefuses`, `--- PASS: TestLaCliHookRecordsBash` |
| 4. holds after the host drop | HOLDS: with `guard.EvaluateHost(in, o.Cwd, env),` deleted from internal/hook/hook.go:358 in the working tree, the row 3 command exits 0 with the same three PASS lines. `git checkout internal/hook/hook.go` done. `git diff main --stat` names only cli_test.go, kg_writer_deny_test.go and la_cli_hook_record_test.go. |
| 5. `go test -count=1 ./internal/cli/` after `git merge main` | HOLDS, exit 0 (`git merge main`: Already up to date; `ok internal/cli 33.075s`) |
| 6. `make lint`, `make cross` | HOLDS, both exit 0 (go vet; darwin/arm64 and windows/amd64 vet and test compile) |
| Packages importing internal/cli (go list) | `cmd/grid`, `lib`. cmd/grid has no tests; `lib` passed in the `make cross` compile; this lane changed only test files of internal/cli. |
| Walk tests (plugin) | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| New tests fail on base | NOT APPLICABLE: the lane changes tests only, and the record it pins (bashHistory) already exists on main. TestLaCliHookRecordsBash passes on base too. Its non-vacuous half is the empty-tool-use-id call, which must leave no snapshot. |
| No existing test edited to pass, no deletions | Only TestHookPreToolUse and TestKgWriterHookNoLongerRefuses were edited, as the brief names. No test deleted. |
| No guard, gate, cap or hold added | HOLDS: test files only, no production file touched. |
| RealPlugin tests | None named by the brief. |

## What changed

- cli_test.go TestHookPreToolUse: dropped the settings.json host-guard write. It now wants exit 0 and no `"permissionDecision":"deny"`.
- kg_writer_deny_test.go TestKgWriterHookNoLongerRefuses: levels loop is only-writer. The `.host` control became the Bash PreToolUse record, through the shared helper. The test now builds its Grid home with `newHarness` (same root, home and Grid home layout). The comment is rewritten.
- la_cli_hook_record_test.go (new): helper `laCliBashHook` and `TestLaCliHookRecordsBash`.

## NOT VERIFIED on a live screen

Nothing here needs a screen.
