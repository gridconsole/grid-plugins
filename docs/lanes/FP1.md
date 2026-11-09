# FP1: protected files refuse planted hard links; writers rename into place; stopgap names

Base 5cf3215f (plugin ref main), rebased from 68e6020e. Pushed as lane-FP1b, because the rebase means lane-FP1 cannot fast-forward.

## What changed

1. `internal/task/linked.go` is new, with `linked_unix.go`, `linked_windows.go` and `linked_other.go`. `Linked(dir)` checks these files in a task folder: task.md, history.md, approval.json, approval-ship.json, approval-preview.json, plan.md and workflows.md. It returns `ErrLinkedFile` (`linked_file`) naming the first one with more than one hard link.
   - Unix reads `Stat_t.Nlink`.
   - Windows opens the file and reads `NumberOfLinks` from `GetFileInformationByHandle`. This is stdlib `syscall`, so no build-tag skip is needed.
   - Other platforms return an error, which holds the task.
2. Readers:
   - `task.Read` refuses a linked task first. So `grid task show`, every `withWrite` writer (approve included), `ApprovalForGate`, `Approved`/`ApprovedDigest` and Deliver (through `Show`) refuse it.
   - `PlanHeld` returns held when `Linked` returns any error. So the guard's `held`, `isHeld` and `recallHeld` and Deliver's plan check hold the task.
   - In hold.go this is a 3-line insertion at the top of `PlanHeld` and nothing else.
3. `AppendHistory` (history.go) no longer appends in place. It locks `.task.lock` rather than history.md, because a rename replaces history.md's inode. It reads history.md and writes the new version with `atomicWrite`: a temp file in the same folder, then a rename. A planted link keeps the old inode.
4. Host guard: `approval-preview.json` is added to `hostProtectedNames` (host_targets.go). Main's 8ea3160 already had the rest: history.md in `gateName` and `hostProtectedNames`, and approval-preview.json in `gateName`. After the rebase, host.go is main's unchanged.
5. `CodeLinkedFile = "linked_file"` is added in `internal/cli/error_codes.go`, mapped from `task.ErrLinkedFile`, with a row in the stable code table in contract/README.md.

## Behaviour change

- The host guard refuses an agent's shell write to history.md. This includes a one-line `>> history.md` append, such as the `brain proposed` line that agent instructions still ask for. Main's 8ea3160 brought this refusal in; this lane keeps it.
- Agents add a history line with `grid history add "<line>" --task T-NN` instead. That is not a person verb. It writes through Grid's journal and a rename (`Store.Note`, then `commitWrite`).
- The agent and skill text that still asks for a shell append is not changed in this lane.

## Writer inventory (protected task files)

| Writer | Files | Method |
|---|---|---|
| `commitWrite`/`commitWriteLines` (write.go) through `recoverWrite` and `atomicWrite` (create.go) | task.md, history.md, approval*.json, refusal-R-n.json | journal, then temp + rename per file (unchanged) |
| `Approve`, `Decide` receipts (gates.go) | approval.json, approval-ship.json, approval-preview.json, refusal-R-n.json | `commitWriteLines`, temp + rename (unchanged) |
| Deliver, RaisePreview, Rename, Attach, Change, Note (`grid history add`) | task.md, history.md | `commitWrite`, temp + rename (unchanged) |
| `AppendHistory` (history.go), called by the history and brain hooks and `grid land` | history.md | was O_APPEND in place under a lock on history.md; now temp + rename under `.task.lock` (changed) |
| `grid land` first line (land.go) | history.md, only when missing | O_CREATE\|O_EXCL new file (unchanged; cannot open an existing link) |
| `Create` `writeSynced` (create.go) | new task.md, intent.md, history.md | O_EXCL new files (unchanged) |
| plan writers (internal/plan store.go `atomicWrite`, write.go) | plan.md, plan revisions | temp + rename (unchanged) |
| workflow writer (internal/workflow write.go, `atomicfile.WriteRoot`) | workflows.md | temp + rename (unchanged) |
| Feed task id (internal/feed identity.go, `atomicfile.Write`) | task.md | temp + rename (unchanged) |
| trash writability probe (trash.go) | history.md | opens O_APPEND only to probe, writes nothing (unchanged) |
| migrate (tree.go, recover.go) | migrated task files | O_EXCL new files (unchanged) |

## Acceptance rows and the tests that cover them

| Row | Test | Fails on base 5cf3215f |
|---|---|---|
| `ln task.md t`: the hold reader holds the task, show refuses and names the file, Deliver refuses | `TestALinkedProtectedFileHoldsTheTask/task.md` (internal/task) | yes |
| same for history.md | `TestALinkedProtectedFileHoldsTheTask/history.md` | yes |
| same for approval.json | `TestALinkedProtectedFileHoldsTheTask/approval.json` | yes |
| the guard denies (all three names), and Deliver refuses through the guard's fixture | `TestALinkedTaskFileHoldsTheGuard` (internal/guard) | yes |
| `linked_file` is the CLI code | `TestLinkedFileHasAStableCode` (internal/cli) | yes (build failure: `undefined: task.ErrLinkedFile`) |
| a Grid writer leaves a planted link on the stale inode | `TestAppendHistoryLeavesAPlantedLinkOnTheOldInode` | yes |
| appenders serialize on `.task.lock` | `TestAppendHistoryKeepsEveryConcurrentLine` | no: base locks history.md and appends in place, which also keeps every line |
| shell writes to history.md and approval-preview.json are gate writes | `TestHostBashHistoryAndPreviewApprovalAreGateFiles` | no: main's 8ea3160 already fences them. The 4 history rows failed on the first base, 68e6020e |
| a real `grid approve` still releases a normal task | the existing `TestDeliverStillReleasesAnApprovedPlan` (guard), the approve gate tests, and the app real-plugin set (rule 18) | n/a |

The `approval-preview.json` entry added to `hostProtectedNames` has no row that fails on base. The shell forms tried are already refused through `gateName`.

## Mutations (run on the first head; the code under test is unchanged by the rebase)

- `Linked` returns nil at its top: 15 failing rows (9 in `TestALinkedProtectedFileHoldsTheTask`, 6 in `TestALinkedTaskFileHoldsTheGuard`).
- `AppendHistory` writes in place (`os.WriteFile`) instead of renaming: 2 failing rows in `TestAppendHistoryLeavesAPlantedLinkOnTheOldInode`.

## Checks

These ran on the VM, in a fresh clone as user grid, with GOFLAGS=-timeout=30m, on head b0af8d20. The pushed head differs from it only in this report.

| Check | Exit |
|---|---|
| `go test -race -count=1 ./internal/task ./internal/guard ./internal/cli ./internal/hook ./internal/land` | 0 |
| `make lint` | 0 |
| `make generate`, then `git status --porcelain` | 0, 0 lines |
| `GOOS=windows go build ./...` | 0 |
| `GOOS=darwin go build ./...` | 0 |
| rule 14: `go test -count=1 ./internal/cli/...` on the lane | 0 |
| rule 14: the same on base 5cf3215f | 0 |
| rule 18: app ref main a1158a8c, plugin built from b0af8d20, `go test -count=1 -v -run RealPlugin ./...` under `script` (pty) | 0 (114 passed, 0 failed, 0 skipped) |

## Fails on base (5cf3215f)

The three new test files were copied into a detached base worktree and run per package with `-run 'TestALinked|TestAppendHistory|TestHostBashHistory|TestLinkedFile'`:

```
--- FAIL: TestALinkedProtectedFileHoldsTheTask (0.01s)
        linked_test.go:28: PlanHeld = false after a link to task.md
        linked_test.go:31: Show err = <nil>; want linked_file naming task.md
        linked_test.go:34: Deliver err = <nil>; want linked_file
        linked_test.go:28: PlanHeld = false after a link to history.md
        linked_test.go:31: Show err = <nil>; want linked_file naming history.md
        linked_test.go:34: Deliver err = <nil>; want linked_file
        linked_test.go:28: PlanHeld = false after a link to approval.json
        linked_test.go:31: Show err = <nil>; want linked_file naming approval.json
        linked_test.go:34: Deliver err = <nil>; want linked_file
--- FAIL: TestAppendHistoryLeavesAPlantedLinkOnTheOldInode (0.00s)
    linked_test.go:57: the planted link still aliases history.md after a Grid write
    linked_test.go:60: the planted link changed: "- 2026-01-02T03:04 · task created · by grid\n- 2026-11-09T17:20 · files changed · notes.md · by grid\n"
FAIL	github.com/gridconsole/grid-plugins/internal/task	0.019s
--- FAIL: TestALinkedTaskFileHoldsTheGuard (0.02s)
        linked_test.go:31: guard after a link to task.md: "", want deny
        linked_test.go:34: Deliver err = <nil>; want linked_file
        linked_test.go:31: guard after a link to history.md: "", want deny
        linked_test.go:34: Deliver err = <nil>; want linked_file
        linked_test.go:31: guard after a link to approval.json: "", want deny
        linked_test.go:34: Deliver err = <nil>; want linked_file
FAIL	github.com/gridconsole/grid-plugins/internal/guard	0.025s
internal/cli/linked_code_test.go:11:60: undefined: task.ErrLinkedFile
FAIL	github.com/gridconsole/grid-plugins/internal/cli [build failed]
```

## Rebase

Rebased onto 5cf3215f. internal/guard/host.go and host_targets.go conflicted. The resolution takes main's comment and `gateName`, and main's `hostProtectedNames` plus `"approval-preview.json"`. The test row that named a person now uses a neutral word.

## NOT VERIFIED and open

- Windows link count: compiled (GOOS=windows build) but not run on Windows.
- macOS Seatbelt behaviour for a link planted before launch is still the lane 4 probe row.
- Lane-table rows (a), the prose `plan accepted` release, and (b), a task folder moved to another project, are not in this brief's Build and are not built here. Until (a) lands, there is a gap:
  - When `AppendHistory` replaces a linked history.md, text an agent already wrote through the link is carried into the new single-link file.
  - On a task with no approval.json, the prose release in `PlanStage` would honour a forged line there.
- `hook/brain.go` and `hook/events.go` call `PlanStage` on text they read themselves, not `PlanHeld`, so a linked file does not change the stage they report.
- `TestAppendHistoryKeepsEveryConcurrentLine` passes on base. It pins the `.task.lock` choice, not a base defect. The reviewer decides whether it stays.
