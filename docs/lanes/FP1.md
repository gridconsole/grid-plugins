# FP1: protected files refuse planted hard links; writers rename into place; stopgap names

Base 68e6020e (plugin ref main). Branch lane-FP1.

## What changed

1. `internal/task/linked.go` (new, with `linked_unix.go`, `linked_windows.go`, `linked_other.go`): `Linked(dir)` checks task.md, history.md, approval.json, approval-ship.json, approval-preview.json, plan.md and workflows.md in a task folder and returns `ErrLinkedFile` (`linked_file`) naming the first one with more than one hard link. Unix reads `Stat_t.Nlink`; Windows opens the file and reads `NumberOfLinks` from `GetFileInformationByHandle` (stdlib `syscall`, no build-tag skip); other platforms return an error, which holds the task.
2. Readers: `task.Read` refuses a linked task first, so `grid task show`, every `withWrite` writer (approve included), `ApprovalForGate`, `Approved`/`ApprovedDigest` and Deliver (through `Show`) refuse it. `PlanHeld` (hold.go, a 3-line insertion at the top of the function, nothing else in hold.go) returns held when `Linked` returns any error, so the guard's `held`, `isHeld` and `recallHeld` and Deliver's plan check hold the task.
3. `AppendHistory` (history.go) no longer appends in place. It locks `.task.lock` (not history.md, whose inode a rename replaces), reads history.md, and writes the new image with `atomicWrite` (temp file in the same folder, then rename). A planted link keeps the old inode.
4. Host guard stopgap: `history\.md` added to `gateName` (host.go); `approval-preview.json` and `history.md` added to `hostProtectedNames` (host_targets.go). `approval-preview.json` was already in `gateName`.
5. `CodeLinkedFile = "linked_file"` in `internal/cli/error_codes.go`, mapped from `task.ErrLinkedFile`; a row in contract/README.md's stable code table.

## Writer inventory (protected task files)

| Writer | Files | Method |
|---|---|---|
| `commitWrite`/`commitWriteLines` (write.go) through `recoverWrite` and `atomicWrite` (create.go) | task.md, history.md, approval*.json, refusal-R-n.json | journal, then temp + rename per file (unchanged) |
| `Approve`, `Decide` receipts (gates.go) | approval.json, approval-ship.json, approval-preview.json, refusal-R-n.json | `commitWriteLines`, temp + rename (unchanged) |
| Deliver, RaisePreview, Rename, Attach, Change, Note | task.md, history.md | `commitWrite`, temp + rename (unchanged) |
| `AppendHistory` (history.go), called by the history and brain hooks and `grid land` | history.md | was O_APPEND in place under a lock on history.md; now temp + rename under `.task.lock` (changed) |
| `grid land` first line (land.go) | history.md, only when missing | O_CREATE\|O_EXCL new file (unchanged; cannot open an existing link) |
| `Create` `writeSynced` (create.go) | new task.md, intent.md, history.md | O_EXCL new files (unchanged) |
| plan writers (internal/plan store.go `atomicWrite`, write.go) | plan.md, plan revisions | temp + rename (unchanged) |
| workflow writer (internal/workflow write.go, `atomicfile.WriteRoot`) | workflows.md | temp + rename (unchanged) |
| Feed task id (internal/feed identity.go, `atomicfile.Write`) | task.md | temp + rename (unchanged) |
| trash writability probe (trash.go) | history.md | opens O_APPEND only to probe, writes nothing (unchanged) |
| migrate (tree.go, recover.go) | migrated task files | O_EXCL new files (unchanged) |

## Acceptance rows and the tests that cover them

| Row | Test | Fails on base |
|---|---|---|
| `ln task.md t`: hold reader held, show refuses naming it, Deliver refuses | `TestALinkedProtectedFileHoldsTheTask/task.md` (internal/task) | yes |
| same for history.md | `TestALinkedProtectedFileHoldsTheTask/history.md` | yes |
| same for approval.json | `TestALinkedProtectedFileHoldsTheTask/approval.json` | yes |
| guard denies (all three names), Deliver refuses through the guard's fixture | `TestALinkedTaskFileHoldsTheGuard` (internal/guard) | yes |
| `linked_file` is the CLI code | `TestLinkedFileHasAStableCode` (internal/cli) | yes (build failure: `undefined: task.ErrLinkedFile`) |
| a Grid writer leaves a planted link on the stale inode | `TestAppendHistoryLeavesAPlantedLinkOnTheOldInode` | yes |
| appenders serialize on `.task.lock` | `TestAppendHistoryKeepsEveryConcurrentLine` | no (base locks history.md and appends in place, which also keeps every line) |
| history.md shell writes are gate writes | `TestHostBashHistoryAndPreviewApprovalAreGateFiles` (4 history rows) | yes; the approval-preview.json rows pass on base, already in `gateName` |
| a real `grid approve` still releases a normal task | existing `TestDeliverStillReleasesAnApprovedPlan` (guard), the approve gate tests, and the app real-plugin set (rule 18) | n/a |

## Mutations

- `Linked` returns nil at its top: 15 failing rows (9 in `TestALinkedProtectedFileHoldsTheTask`, 6 in `TestALinkedTaskFileHoldsTheGuard`).
- `AppendHistory` writes in place (`os.WriteFile`) instead of renaming: 2 failing rows in `TestAppendHistoryLeavesAPlantedLinkOnTheOldInode`.

## Checks

Run on the VM in a fresh clone as user grid, GOFLAGS=-timeout=30m, on head acbba9c6 (the pushed head adds this report only):

| Check | Exit |
|---|---|
| `go test -race -count=1 ./internal/task ./internal/guard ./internal/cli` | 0 |
| `make lint` | 0 |
| `make generate`, then `git status --porcelain` | 0, 0 lines |
| `GOOS=windows go build ./...` | 0 |
| `GOOS=darwin go build ./...` | 0 |
| rule 14: `go test -count=1 ./internal/cli/...` on the lane | 0 |
| rule 14: the same on base 68e6020e | 0 |
| rule 18: app ref main a1158a8c, plugin built from acbba9c6, `go test -count=1 -v -run RealPlugin ./...` under `script` (pty) | 0 (114 passed, 0 failed, 0 skipped) |

Also run locally (callers of `AppendHistory`): `go test -count=1 ./internal/hook ./internal/land`: 0. And `./internal/task ./internal/guard` without race: 0.

## Fails on base (68e6020e)

The three new test files copied into a detached base worktree, run per package with `-run 'TestALinked|TestAppendHistory|TestHostBashHistory|TestLinkedFile'`:

```
--- FAIL: TestALinkedProtectedFileHoldsTheTask (0.38s)
        linked_test.go:28: PlanHeld = false after a link to task.md
        linked_test.go:31: Show err = <nil>; want linked_file naming task.md
        linked_test.go:34: Deliver err = <nil>; want linked_file
        linked_test.go:28: PlanHeld = false after a link to history.md
        linked_test.go:31: Show err = <nil>; want linked_file naming history.md
        linked_test.go:34: Deliver err = <nil>; want linked_file
        linked_test.go:28: PlanHeld = false after a link to approval.json
        linked_test.go:31: Show err = <nil>; want linked_file naming approval.json
        linked_test.go:34: Deliver err = <nil>; want linked_file
--- FAIL: TestAppendHistoryLeavesAPlantedLinkOnTheOldInode (0.06s)
    linked_test.go:57: the planted link still aliases history.md after a Grid write
    linked_test.go:60: the planted link changed: "- 2026-01-02T03:04 · task created · by grid\n- 2026-11-09T17:10 · files changed · notes.md · by grid\n"
FAIL	github.com/gridconsole/grid-plugins/internal/task	0.778s
--- FAIL: TestALinkedTaskFileHoldsTheGuard (0.40s)
        linked_test.go:31: guard after a link to task.md: "", want deny
        linked_test.go:34: Deliver err = <nil>; want linked_file
        linked_test.go:31: guard after a link to history.md: "", want deny
        linked_test.go:34: Deliver err = <nil>; want linked_file
        linked_test.go:31: guard after a link to approval.json: "", want deny
        linked_test.go:34: Deliver err = <nil>; want linked_file
--- FAIL: TestHostBashHistoryAndPreviewApprovalAreGateFiles (0.11s)
        linked_test.go:59: decision "", want "deny":
        linked_test.go:59: decision "", want "deny":
        linked_test.go:59: decision "", want "deny":
        linked_test.go:59: decision "", want "deny":
FAIL	github.com/gridconsole/grid-plugins/internal/guard	0.737s
internal/cli/linked_code_test.go:11:60: undefined: task.ErrLinkedFile
FAIL	github.com/gridconsole/grid-plugins/internal/cli [build failed]
```

## NOT VERIFIED and open

- Windows link count: compiled (GOOS=windows build) but not run on Windows.
- macOS Seatbelt behaviour for a link planted before launch is still the lane 4 probe row.
- Lane-table rows (a) the prose `plan accepted` release and (b) a task folder moved to another project are not in this brief's Build and are not built here. Until (a) lands: when `AppendHistory` replaces a linked history.md, text an agent already wrote through the link is carried into the new single-link file, and on a task with no approval.json the prose release in `PlanStage` would honour a forged line there.
- `hook/brain.go` and `hook/events.go` call `PlanStage` on text they read themselves, not `PlanHeld`, so a linked file does not change the stage they report.
- `TestAppendHistoryKeepsEveryConcurrentLine` passes on base (it pins the `.task.lock` choice, not a base defect); the reviewer decides whether it stays.
- The host guard now refuses shell writes to history.md, including an agent's own appended line (agent instructions still ask for a `brain proposed` line through the shell).
