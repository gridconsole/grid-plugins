# C-ve-task-merge-verb: `grid task merge --task <id> --json` merges a task's branch into its base locally, never a push

Base: 44c7de5b (plugin main). Lane commits: the verb, the table rows, the move into task.go, the prose row, this report.

Base check: on 44c7de5b, the new test file does not build, so every new test fails:

```
internal/cli/ve_task_merge_test.go:70:9: undefined: taskMergeGit
internal/cli/ve_task_merge_test.go:71:2: undefined: taskMergeGit
internal/cli/ve_task_merge_test.go:75:21: undefined: taskMergeGit
FAIL	github.com/gridconsole/grid-plugins/internal/cli [build failed]
```

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| A1 | ve_task_merge_test.go on a temp git repo: a clean merge prints the sha and base has it; a conflict exits non-zero, names it, porcelain empty after; dirty checkout refused before any merge; no push or fetch | HOLDS | `go test -count=1 -v -run TestVeMerge ./internal/cli/` exit 0: `--- PASS: TestVeMergeCleanMergeFromTheTaskBranch`, `TestVeMergeCleanMergeOnMainWithProject`, `TestVeMergeConflictAbortsAndLeavesTheCheckout`, `TestVeMergeDirtyCheckoutRefusedBeforeAnyMerge`, `TestVeMergeTaskWithoutBranch`, `TestVeMergeUsage`; `ok internal/cli 0.517s`. The fixture runs the real `grid deliver` in the repo to write `branch:`/`base:`, then the route's argv `task merge --task T-01 --json`. Clean: stdout `{"ok":true,"merged":"<sha>"}` == `rev-parse main`, two parents with the task tip second, `main:b.txt` present, HEAD back on grid/T-01. Conflict: exit 1, one stderr line naming `conflicts in a.txt` and `grid task merge again`, porcelain empty, HEAD grid/T-01, both tips unchanged, no MERGE_HEAD. Dirty: no `merge` or `checkout` argv, the edit kept. No push/fetch: every git argv is recorded through `taskMergeGit` and none is push/fetch/pull/remote/ls-remote; no FETCH_HEAD; the bare origin has no refs. |
| A2 | `go test -count=1 -run 'TestHsIpWalksEveryCommand' ./internal/cli/` | HOLDS | run as `-run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok internal/cli 0.051s`, exit 0. |
| A3 | `go test -count=1 ./internal/cli/ ./internal/contract/ ./internal/deliver/...` | HOLDS | `./internal/deliver/...` does not exist in the plugin (deliver is internal/task/deliver.go), so it is dropped. `go test -count=1 ./internal/contract/... ./internal/cli/`: `ok internal/contract 1.245s`, `ok internal/cli 45.500s`, exit 0. |
| R1 | Tests of changed packages and every importer | HOLDS | Changed: internal/cli, internal/contract (one test row), contract/ (commands.json). `go list` importers of internal/cli: `cmd/grid`, `lib`. `go test -count=1 ./internal/cli/ ./internal/contract/... ./contract/... ./lib/... ./cmd/grid/... ./internal/job/lint/ .`: contract ok, contract/formats ok, lib ok, cmd/grid no test files, internal/job/lint ok, root ok. internal/contract was red in that run (TestHeMvMessagesNameRealSubs, see below), then fixed and rerun: ok. |
| R2 | Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| R3 | Lint, Windows and macOS builds | HOLDS | `make lint` (go vet ./... and gofmt) exit 0, no output. `make cross`: all four stages ran (darwin/arm64 and windows/amd64 vet and test compile), no FAIL. |
| R4 | New tests fail on base, pass on lane | HOLDS | Base check above; pass lines in A1. |
| R5 | No existing test edited to pass, none deleted | HOLDS with the table rows below | No assertion removed or weakened; each existing test file got one table row. |
| R6 | No guard, gate, cap, hold or hook | HOLDS | The verb applies no role rule (guest row below shows that). Its refusals are the brief's causes: a dirty checkout, a conflict, no branch, plus no base branch and no project folder. |
| R7 | `make generate` | HOLDS | Ran it; it changed contract/commands.json only (the `task merge` entry and the task summary line). |
| R8 | Commits, no attribution, no personal paths | HOLDS | One-sentence messages, lane identity, no Co-Authored-By. |
| R9 | *RealPlugin* | not run | none named by the brief. |

## What changed

- `internal/cli/task.go`: usage line `grid task merge --task <task> [--project <p>] [--json]`, the `merge` dispatch, and `runTaskMerge`/`taskMerge` with helpers. The branch is task.md's `branch:` as `grid deliver` writes it (recordBranch, internal/task/deliver.go). The base is task.md's `base:` when it names a local branch, else the first of `main`, `master`, the order deliver uses. The repo is the project folder from the `where` resolver, or the one repo of the project that has the branch. Steps: `status --porcelain` (non-empty is refused), checkout the base, `merge --no-ff --no-edit -m`, `rev-parse HEAD`, checkout the starting branch. On failure it names the conflicting files, runs `merge --abort` when MERGE_HEAD exists, and checks out the start again. A branch already in the base answers the base tip and makes no commit. All git calls go through `taskMergeGit`. Exit 1 with one stderr line naming the cause and the next step; a missing `--task` exits 2.
- `internal/cli/inprocess.go`: `"merge": {}` under task, like deliver, which also runs git.
- `contract/commands.json`: regenerated by `make generate`.
- `testdata/golden/task-help.golden`: the usage line. `testdata/golden/help.golden`: the task line.
- `internal/cli/ve_task_merge_test.go` (new).

## Changes outside Touches (the brief cannot land without them)

A new task subverb is read by repo walk tests that fail without a row for it:
- `internal/cli/cli.go:135`: the task help line. All 17 task verbs need 121 columns and TestExHelpLinesFitOneLine requires under 120, so the line now ends `(grid task help lists all)`, the form TestExHelpSummariesNameEveryVerb allows (as brain does). It names new, show, list, status, lane, merge, stop, close, reopen, history, attach, move.
- `internal/contract/he_mv_message_verbs_test.go`: `"internal/cli/cli.go:task help"` in heMvProse, which TestHeMvMessagesNameRealSubs asks for when a message is prose.
- `internal/cli/contract_inventory_test.go`: `task merge` annotation with required `--task` (make generate refuses a route without one).
- `internal/cli/gg_unknown_task_walk_test.go`: `task merge` in ggTaskWalked (T-99 answers no_such_task).
- `internal/cli/guest_parity_test.go`: `task merge` parity argv.
- `internal/cli/fv_guest_task_test.go`: the guest `merge` row as seen: exit 1, `no branch` line, nothing changed.

No `docs/` file lists task verbs (only lane reports), and skills/deliver/SKILL.md does not say how a branch is merged, so neither changed. contract/README.md documents task verbs but is outside Touches, so it is unchanged. A section is open below.

## Open items

- A guest is not refused by the verb. The brief puts who may merge in the app's route. The guest row in fv_guest_task_test.go only shows that refusal is left to the route.
- contract/README.md has no `grid task merge` section (outside Touches).
- The verb checks out the base in the project checkout and returns. If the base is checked out in another worktree, checkout fails and the verb exits 1 naming it, with nothing changed.

## NOT VERIFIED on a live screen

The Code tab Merge button (app lane C-ve-task-merge-route-app) was not clicked. The verb's argv and JSON shape match the route's read, `{"ok":true,"merged":"<sha>"}` (TestVeMergeCleanMergeOnMainWithProject compares the exact stdout).
