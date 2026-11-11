# A-win-renames-followup

Base: stacked on lane-A-win-atomic-replace (merged into this branch; it adds `atomicfile.Rename` at internal/atomicfile/rename_unix.go:9 and rename_windows.go:13).

## Proof table

| Row | Result | Proof |
|---|---|---|
| Pre-check: atomicfile.Rename exists | HOLDS | `git grep -n "func Rename" internal/atomicfile/` shows rename_unix.go:9 and rename_windows.go:13, exit 0 |
| 1. No bare os.Rename at the seven named lines | HOLDS | `git grep -n "os.Rename(" -- internal/plan/store.go internal/task/create.go internal/intent/files.go internal/settings/claude_user.go internal/settings/claude.go internal/settings/budget.go` prints only `internal/task/create.go:143` (the staged task folder, left on purpose). Replaced lines: internal/plan/store.go:133, internal/task/create.go:415, internal/intent/files.go:190, internal/settings/claude_user.go:264, internal/settings/claude.go:433, internal/settings/claude.go:443, internal/settings/budget.go:810, each now `atomicfile.Rename(` |
| 2. One test per changed package: write lands over an existing file; a non-empty folder at the target gives an error and nothing else changes | HOLDS | plan: internal/plan/plan_rename_test.go (TestRenameWriteReplacesExistingPlan, TestRenameAtomicWriteReplacesFile, TestRenameAtomicWriteOverFolderFails). task: internal/task/task_rename_test.go (TestRenameAppendHistoryReplacesExisting, TestRenameAtomicWriteReplacesFile, TestRenameAtomicWriteOverFolderFails). intent: internal/intent/intent_rename_test.go (TestRenameRenewReplacesClaim, TestRenameReplaceJSONReplacesFile, TestRenameReplaceJSONOverFolderFails). settings: internal/settings/settings_rename_test.go (ClaudeSet, RunClaudeSetup, BudgetApply, each a replace test and an over-a-folder test). `go test ./internal/plan/ ./internal/task/ ./internal/intent/ ./internal/settings/ -count=1 -run Rename -v` exit 0, all PASS. Two limits, see below |
| 3. go test of the five packages | HOLDS | `go test ./internal/plan/ ./internal/task/ ./internal/intent/ ./internal/settings/ ./internal/atomicfile/ -count=1` exit 0, 5 ok, 0 fail |
| 4a. GOOS=windows go vet | HOLDS | `GOOS=windows go vet` on the five packages, exit 0 |
| 4b. Windows and macOS test builds | HOLDS (compiled) | `GOOS=windows go test -c -o /dev/null` and `GOOS=darwin go test -c -o /dev/null` on the five packages, exit 0 each |
| 4c. make lint | HOLDS | `make lint` (go vet ./...) exit 0 |
| Importers of the changed packages | HOLDS | Packages whose Deps include plan, task, intent, settings or atomicfile (from `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...`): cmd/grid, account, brain, braincmd, cli, doctor, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, localdata, mcp, migrate, orchestrator, packaging, packaging/cmd, people, ports, projectmd, projects, proof, question, routine, session, signal, usage, worker, workflow. `go test -count=1` on all of them except packaging, packaging/cmd (no plan/task/intent/settings change reaches them beyond atomicfile): exit 0, all ok |
| Rule: no guard, gate, cap or hold added | HOLDS | The diff swaps seven calls and adds imports and test files; no hook, cap or check |
| Rule: no existing test edited or deleted | HOLDS | `git diff 4760b007..HEAD --stat` (4760b007 is the stacked merge of A-win-atomic-replace; origin/main..HEAD also carries that lane's files) touches only the six source files and the four new `*_rename_test.go` files, plus this report |
| Rule: new tests fail on base | NOT DONE, cannot be shown | On Linux a bare os.Rename succeeds as atomicfile.Rename does (rename_unix.go:9 is os.Rename), so these tests pass on base too. The Windows retry itself is tested inside atomicfile by A-win-atomic-replace |
| Rule: small commits, one sentence, no attribution | HOLDS | two commits on top of the merge: "Move the seven hot-path file renames ..." and "Add the rename tests ...", no trailer lines |
| Rule: no personal paths, hosts or emails in the repo | HOLDS | test files use t.TempDir only |

Windows host run: for the orchestrator: `go test ./internal/plan/ ./internal/settings/ -count=1 -run Rename -v`
(plan's Store.Write test skips on Windows because fslock is not implemented there; the two atomicWrite tests in plan run. The settings tests that need no lock run.)

## What changed

- os.Rename -> atomicfile.Rename at the seven call sites above; import added in plan/store.go, task/create.go, intent/files.go (settings already imported it).
- Four new test files, one per package.

## Notes on the failure tests (row 2)

- In plan, settings and the budget apply, a folder at the target is refused by the regular-file check that precedes the rename, so the test proves the error surfaces and nothing under the folder changes, not that the rename itself failed.
- In task (atomicWrite) and intent (replaceJSON) the failure test calls the changed unexported function directly, because it is the only way to reach the rename with a folder at the target (Renew and Release read the target first and are refused there). That differs from "through the public caller"; the success tests do go through public callers (AppendHistory, Renew, Store.Write, ClaudeSet, RunClaudeSetup, BudgetApply).

## Left unchanged on purpose

- internal/task/create.go:143 renames the staged task folder, not a file (brief).
- No call site relies on *os.LinkError (only intent_test.go:379 builds one for a link, not a rename).

## Further bare os.Rename, candidates, unchanged

`git grep -n "os.Rename(" -- 'internal/**/*.go' ':!*_test.go'` (excluding atomicfile itself):
internal/artifact/artifact.go:382, internal/eval/results.go:29, internal/job/jobcmd/save.go:75, internal/migrate/apply.go:221,227,229, internal/migrate/recover.go:214,223, internal/migrate/rollback.go:236,241 (these are folder renames of the Grid home), internal/signal/move.go:127, internal/task/create.go:143 (folder), internal/task/trash.go:361, internal/version/check.go:168. File-writes among them (artifact, eval/results, jobcmd/save, version/check) are the likeliest next candidates.

## NOT VERIFIED on a live screen

Nothing here is UI. The Windows behaviour (retry on a file another process holds open) is not run on a Windows host; it needs the orchestrator's host run above.
