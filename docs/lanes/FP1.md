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

# FP1R: writers refuse linked files; one lock order; orchestrator and workflows.md checks

Base lane-FP1b 5bbf22cd, already on plugin main 5cf3215f (the rebase is a no-op). Review round 1 must-fixes 1-4, as decided. Where this section and the FP1 sections above disagree, this section holds: Grid writers no longer replace a linked file. They refuse it.

## What changed

- MF1, writers refuse a linked task:
  - `AppendHistory` takes `.task.lock`, calls `Linked(dir)` and returns `ErrLinkedFile` without writing. The hook logs the error and the task stays held.
  - `commitWriteLines` (and so `commitWrite`, which every `withWrite` writer, trash receipt and Note uses) checks `Linked` before the journal is written.
  - `recoverWrite` checks `Linked` before it replays a journal. The journal stays in place, and the task held, until the link is removed.
  - `TestAppendHistoryLeavesAPlantedLinkOnTheOldInode` now asserts refusal and the same bytes and inode on both names, as decided.
- MF2, one lock order: `.task.lock` is never taken while the session lock is held.
  - hook/brain.go brain read and brain recall decide and mark under `UpdateActivity`, then call `AppendHistory` after it returns.
  - The grep found a third nested site, `bashHistory` in hook/history.go. It now collects its lines under the session lock and appends them afterwards.
  - The other `UpdateActivity` closures (events.go, verify.go, brain.go planner task) take no task lock.
  - A failed append after the mark is logged and not retried.
- MF3: orchestrator `publishPlanWithReader` calls `task.Linked` under `.task.lock` before it reads task.md. A linked task sets `publicationBlocked` and posts no Jobs.
- MF4, workflows.md:
  - `workflow.readFile` counts the links of the opened descriptor. That is fstat `Nlink` on unix and `NumberOfLinks` on Windows; elsewhere the count is an error, so the file fails closed. A linked workspace or project workflows.md returns `workflow.ErrLinkedFile`.
  - `TaskSteps` returns an error. Corrected in FP1R2: its only caller is `RaisePreview`, so a linked workflows.md blocks raising a preview; it does not hold a task's steps or its plan (PlanStage reads task.md). `ProjectPersonVerbs` keeps `ErrLinkedFile` in the chain.
  - `workflow.Write` refuses a linked workflows.md too.
  - `task.ErrLinkedFile` is now the same value as `workflow.ErrLinkedFile` (task imports workflow, not the reverse), so both map to the CLI code `linked_file`.
  - The dead `<task>/workflows.md` entry is gone from `protectedNames`, and the contract/README.md `linked_file` row is reworded.
- cli person gate: when a workflows.md it reads is linked, the gate applies the default replaceable policy. A verb under the floor or that policy is then refused for a person and an agent alike: exit 1, stderr names the file, and the `--json` envelope carries `error.code` `linked_file`. Other verbs run as before. (Superseded in FP1R2: this let a verb that only the linked file listed run without a person. The gate now refuses every verb whose gate read the linked file.)

## Writer inventory after FP1R

| Writer | Linked task or file |
|---|---|
| `AppendHistory` (history and brain hooks, `grid land`) | refuses under `.task.lock`, writes nothing |
| `commitWrite`/`commitWriteLines` (Approve, Decide, Deliver, RaisePreview, Rename, Attach, Change, Note, trash receipts) | refuses before the journal |
| `recoverWrite` (withWrite, trash) | refuses before the replay; the journal stays |
| `workflow.Write` | refuses a linked workflows.md |
| plan writers (internal/plan), `feed.EnsureTaskID` | unchanged: follow-up below |

## Acceptance rows

| Row | Test | Fails on 5bbf22cd |
|---|---|---|
| P1: approved task, `ln history.md`, one `AppendHistory`: still held, both names unchanged | `TestP1AppendHistoryKeepsALinkedApprovedTaskHeld` (task) | yes |
| P2: no receipt, forged `plan accepted` through the link, one `AppendHistory`: still held, both unchanged | `TestP2AppendHistoryKeepsAForgedLinkedTaskHeld` (task) | yes |
| P1 and P2 with one PostToolUse Write hook run through the built binary (an unlinked control run proves the hook appends) | `TestPostToolUseHookThroughTheBinaryKeepsALinkedTaskHeld` (task) | yes, both subtests |
| writer refuses, decided wording | `TestAppendHistoryLeavesAPlantedLinkOnTheOldInode` (task) | yes |
| journal writers refuse: commitWrite on linked task.md, history.md, approval.json; recoverWrite keeps the journal | `TestTaskWritersRefuseALinkedFile` (task) | yes, all 4 subtests |
| P4: bash history, brain read and brain recall hooks against attach's lock order, 10 s timeout: both finish | `TestHookAndAttachLockOrderFinishes` (hook) | yes, all 3 subtests (deadlock at 10 s) |
| a linked task.md (and plan.md) posts no Jobs; the unlinked control posts 1 | `TestLinkedTaskPostsNoJobs` (orchestrator) | yes, both linked subtests |
| linked workspace and project workflows.md: TaskSteps fails (so no preview can be raised), ProjectPersonVerbs refuses, show reads nothing, Write refuses, bytes and inode unchanged | `TestLinkedWorkflowsFileIsUnreadable` (workflow) | yes, both subtests |
| Write refuses a linked workflows.md even with the right revision | `TestWriteRefusesALinkedWorkflowsFile` (workflow) | yes |
| a person verb refuses with `linked_file` (workspace and project, person and agent); a verb outside the policy still runs | `TestLinkedWorkflowsFileRefusesPersonVerbs` (cli) | yes, the 4 refusal subtests (the 2 `agent note` controls pass on both) |

P4 note: the attach side holds `.task.lock` and then calls `session.Store.Start`, the order and lock that `Attach` uses (withWrite, then sessions.Start). Calling `task.Attach` itself cannot be paused between its two locks, so the deadlock could not be forced deterministically that way.

Failing lines on 5bbf22cd (the lane's test files checked out on top of the base; long byte dumps cut):

```
--- FAIL: TestAppendHistoryLeavesAPlantedLinkOnTheOldInode
    linked_test.go:53: AppendHistory err = <nil>; want linked_file
--- FAIL: TestP1AppendHistoryKeepsALinkedApprovedTaskHeld
    linked_writers_test.go:90: P1: the task was released after one AppendHistory while the link exists
    linked_writers_test.go:93: Linked = <nil> after AppendHistory; want linked_file
    linked_writers_test.go:95: history.md was replaced by a new inode; the link evidence is gone
--- FAIL: TestP2AppendHistoryKeepsAForgedLinkedTaskHeld
    linked_writers_test.go:116: P2: the forged release went live after one AppendHistory
    linked_writers_test.go:118: history.md was replaced by a new inode; the link evidence is gone
--- FAIL: TestTaskWritersRefuseALinkedFile/commitWrite/task.md
    linked_writers_test.go:148: commitWrite err = <nil>; want linked_file
    linked_writers_test.go:156: task.md was replaced; the link evidence is gone
--- FAIL: TestTaskWritersRefuseALinkedFile/commitWrite/history.md
    linked_writers_test.go:156: history.md was replaced; the link evidence is gone
--- FAIL: TestTaskWritersRefuseALinkedFile/commitWrite/approval.json
    linked_writers_test.go:151: commitWrite changed files: ...
--- FAIL: TestTaskWritersRefuseALinkedFile/recoverWrite
    linked_writers_test.go:173: recoverWrite err = <nil>; want linked_file
    linked_writers_test.go:178: history.md bytes changed: "...- replayed\n"
--- FAIL: TestPostToolUseHookThroughTheBinaryKeepsALinkedTaskHeld/P1_approved
    linked_writers_test.go:250: P1 approved: the task was released by one PostToolUse hook while the link exists
--- FAIL: TestPostToolUseHookThroughTheBinaryKeepsALinkedTaskHeld/P2_forged
    linked_writers_test.go:250: P2 forged: the task was released by one PostToolUse hook while the link exists
--- FAIL: TestHookAndAttachLockOrderFinishes/bash_history (10.50s)
    lockorder_test.go:138: P4: attach still blocked on the session lock after 10 s while the hook waits for .task.lock
--- FAIL: TestHookAndAttachLockOrderFinishes/brain_read (10.51s)
    lockorder_test.go:143: P4: attach still blocked on the session lock after 10 s while the hook waits for .task.lock
--- FAIL: TestHookAndAttachLockOrderFinishes/brain_recall (10.51s)
    lockorder_test.go:148: P4: attach still blocked on the session lock after 10 s while the hook waits for .task.lock
--- FAIL: TestLinkedTaskPostsNoJobs/linked_task.md
    plan_linked_test.go:37: jobs=1 assigned=1 want=0 tick err=<nil> read err=<nil>
--- FAIL: TestLinkedTaskPostsNoJobs/linked_plan.md
    plan_linked_test.go:37: jobs=1 assigned=1 want=0 tick err=<nil> read err=<nil>
--- FAIL: TestLinkedWorkflowsFileIsUnreadable/workspace
    linked_test.go:40: TaskSteps = [{check-it check}], <nil>; want a linked_file error that holds the task
    linked_test.go:46: ProjectPersonVerbs err = <nil>; want ErrPersonVerbs wrapping linked_file
--- FAIL: TestLinkedWorkflowsFileIsUnreadable/project
    linked_test.go:40: TaskSteps = [{ship run}], <nil>; want a linked_file error that holds the task
--- FAIL: TestWriteRefusesALinkedWorkflowsFile
    linked_test.go:94: Write err = <nil>; want linked_file
--- FAIL: TestLinkedWorkflowsFileRefusesPersonVerbs/workspace/person
    linked_workflow_gate_test.go:50: exit 0; want 1 (linked_file): {"result":"updated",...
--- FAIL: TestLinkedWorkflowsFileRefusesPersonVerbs/workspace/agent
    linked_workflow_gate_test.go:50: exit 0; want 1 (linked_file): {"result":"updated",...
--- FAIL: TestLinkedWorkflowsFileRefusesPersonVerbs/project/person
--- FAIL: TestLinkedWorkflowsFileRefusesPersonVerbs/project/agent
    linked_workflow_gate_test.go:50: exit 0; want 1 (linked_file): {"result":"updated",...
```

## Mutations (on the lane, each restored after)

- Drop the `Linked` check in `AppendHistory`: 5 rows red: `TestAppendHistoryLeavesAPlantedLinkOnTheOldInode`, P1, P2, and both binary subtests. The writer rows stay green, because they exercise commitWrite and recoverWrite.
- Restore the nested lock (hook/brain.go and hook/history.go from 5bbf22cd): 3 rows red, all P4 subtests, each at the 10 s timeout.
- Drop the `refuseLinked` call in `workflow.readFile`: 6 rows red, the 2 `TestLinkedWorkflowsFileIsUnreadable` subtests and the 4 cli refusal subtests. `TestWriteRefusesALinkedWorkflowsFile` stays green because it covers the writer's own check.

## Checks

These ran on code head fd6586fe in a fresh clone; this report commit changes docs only.

| Check | Exit |
|---|---|
| `go test -race -count=1 -timeout=30m ./internal/task ./internal/guard ./internal/cli ./internal/hook ./internal/land ./internal/orchestrator ./internal/workflow` | 0 (task 11.8 s, guard 8.1 s, cli 53.4 s, hook 5.4 s, land 4.9 s, orchestrator 64.6 s, workflow 10.7 s) |
| `make lint` | 0 |
| `make generate`, then `git status --porcelain` | 0, 0 lines |
| `GOOS=windows go build ./...` | 0 |
| `GOOS=darwin go build ./...` | 0 |
| `GOOS=windows go vet ./internal/workflow ./internal/task`; `GOOS=solaris go build ./...` | 0, 0 |
| rule 14: `go test -count=1 ./internal/cli/...` on the lane | 0 |
| rule 14: the same on 5bbf22cd and on main 5cf3215f | 0, 0 |
| rule 18: app ref main a1158a8c, plugin built from fd6586fe, `go test -count=1 -v -run RealPlugin ./...`, no pty | 1 (58 passed, 38 failed, 0 skipped) |
| rule 18 control: the same with a plugin built from 5bbf22cd | 1 (58 passed, 38 failed, 0 skipped; the same failing tests) |

Rule 18 ran inside the builder's agent session, without the pty and clean login the brief asks for. The failures are the plugin person gate refusing the app's person writes with `person_required` (reason `agent_env`). The base binary fails the identical set, and `linked_file` appears nowhere in the lane run.

## Follow-ups (outside this brief's packages)

- The plan.md writers in internal/plan (store.go `atomicWrite`, write.go) and `feed.EnsureTaskID` (task.md) still replace a linked file. Both packages are imported by task, so they cannot call `task.Linked`; a link check needs a lower shared package. The planner decides.
- routine/workflow.go reads workflows.md without `workflow.readFile`, so it has no link check.
- `Linked` still skips non-regular names (a symlink at a protected name), as noted in the review.

## NOT VERIFIED

- Rule 18 under a pty with a clean, non-agent environment: not run here (see above). That the 38 person-gate failures pass there is not verified for this head.
- The checks ran as the builder user in a fresh local clone, not as user grid. `-timeout=30m` was passed as a flag instead of through GOFLAGS. Nothing was pushed; the lane is delivered as a bundle.
- Link counts on Windows and darwin are compiled but not run there. The fail-closed `linked_other.go` in workflow (plan9, js, wasip1) was not compiled.
- TOCTOU between `Linked` and the rename is unchanged (review note: it needs an unsandboxed process).

# FP1R2: the person gate fails closed on a linked policy; remaining writers

Brief: FP1R2-gate-fails-closed. Review: FP1R (must-fix 1, notes 1 and 4). Decisions: FP1R review round 2, items 1-4 and 6 built here; item 5 stays a follow-up.

## Base and rebase

- `lane-FP1R2` started from `lane-FP1R` 71bd49c9 and was rebased onto plugin main 9764ce4e (L155U landed in between).
- Three files conflicted. Both sides were kept:
  - contract/README.md and internal/cli/error_codes.go: main added `workflow_changed` and FP1 added `linked_file`. Both codes stay, and the `errorCode` switch maps both.
  - internal/task/hold.go: main renamed the core of `PlanHeld` to `PlanHeldWith(lookup, ...)`, and `PlanHeld` now wraps it. FP1's `Linked(dir)` hold moved into `PlanHeldWith`, so the guard, Deliver and the hook helpers all hold a linked task.
- After the rebase, every FP1 and FP1R row passes (the race run below covers all of their packages).

## What changed

1. **Person gate fails closed (decision 1, option a)**, in internal/cli/person_gate.go:
   - When a policy file the gate reads is linked, `apply` records the error and adds no policy. It no longer substitutes `workflow.DefaultPersonVerbs()`.
   - The `linked_file` refusal now runs before `persongate.Required`. Every verb whose gate read a linked file refuses, for a person and an agent alike: exit 1, stderr names the file without a machine path, and `--json` gets `error.code` `linked_file`.
   - Behaviour change: FP1R's `agent note` control rows in `TestLinkedWorkflowsFileRefusesPersonVerbs` flip from exit 0 to exit 1 `linked_file`, as decision 1 says. That test is FP1R's own (not on main), and the two subtests' expectation was changed to match the decision. No test on main was changed.
   - What a linked file now blocks (the gate reads the file):
     - A workspace or project workflows.md blocks signal verbs in any scope and every verb run in a project scope with no task. The project file is read first; the workspace file is read only when the project file sets no `person_verbs`.
     - A task's workflow.md blocks every verb run in that task's context, including `task show` and `grid mcp`.
   - Hook commands skip the gate, so hooks keep running. A person clears the refusal by removing the extra link.
2. **`feed.EnsureTaskID` refuses a linked task.md (decision 2).** Under `.task.lock`, after the regular-file check and before it reads or writes, it counts links on the open descriptor with the new `workflow.RefuseLinked`, the exported form of the workflows.md check. It refuses with or without an existing id. feed cannot import task, which imports feed, so the check reuses workflow's link count, which feed already imports.
3. **`signal move --inbox` on a linked workspace workflows.md reports `linked_file` (decision 3).** `workflow.InboxWorkspace` returns the path-free `ErrLinkedFile` error instead of `ErrInboxWorkspace`. The handler prints `grid signal move: linked_file: workflows.md has 2 hard links; ...`, exits 1, and `--json` gets `{"error":{"code":"linked_file",...}}` instead of the `invalid_policy` object.
4. **Task-local workflow.md (decision 4).** `readTaskPolicy` (internal/workflow/policy_file.go) counts links on the opened regular file and refuses a linked one with `ErrLinkedFile`, wrapped as an `*os.PathError` read error. A directory is not counted, since its link count is its entries; reading it fails as before. Its readers:
   - `PersonVerbs` keeps `ErrLinkedFile` in the chain without the task path, so the gate refuses (item 1).
   - `GatePostRoles`: `feed.Store.Post` of a gate record fails with `read gate post policy: ...`. `QuestionWithdrawPolicy` fails closed like an unreadable file: a write exits 1, and the read-side projection allows no person withdrawal. It no longer falls back to the defaults.
   - `PlanHashToken` fails closed with its existing message (`grid approve` exit 3). The gate refuses `approve` first with `linked_file`.
   - `UsageNoticeThresholds` warns with `cannot read workflow.md: linked_file: ...` and the defaults apply. Thresholds only decide notices, not gates.
5. **Report and contract (decision 6).** The FP1R section above is corrected in place: a linked workflows.md makes `TaskSteps` fail, which blocks raising a preview, and does not hold TaskSteps generally. The contract/README.md `linked_file` row now describes the fail-closed gate, the task's workflow.md, `signal move --inbox` and the preview. The `readFile` comment is corrected the same way.

## Writer and reader inventory after FP1R2

| Writer or reader | Linked file |
|---|---|
| `feed.EnsureTaskID` (feed post, question post, task lease, orchestrator, usage notice) | refuses under `.task.lock`, writes nothing, inode unchanged |
| person gate, every verb whose gate read a linked workflows.md or workflow.md | refuses with `linked_file` before `Required` |
| `workflow.InboxWorkspace` (`signal move --inbox`) | `linked_file` |
| `readTaskPolicy`: PersonVerbs, GatePostRoles, PlanHashToken, UsageNoticeThresholds | refuses: see item 4 |
| plan writers (internal/plan), routine/workflow.go | unchanged: follow-ups (decision 5) |

## Acceptance rows (new in FP1R2)

| Row | Test | Fails on 71bd49c9 |
|---|---|---|
| The review probe: workspace and project, linked and unlinked, agent and person, real file adds `signal act note`. Linked → exit 1 `linked_file`, no file changed. Unlinked agent → exit 3 `person_required`; unlinked person → exit 0 | `TestLinkedPolicyRefusesTheVerbItAdds` (cli) | yes, the 4 linked subtests; the 4 unlinked controls pass on both |
| FP1R's `agent note` rows flipped by decision 1 | `TestLinkedWorkflowsFileRefusesPersonVerbs/{workspace,project}/agent_note` (cli) | yes, both |
| Linked task workflow.md: `task show` (the verb it adds) for agent and person, and `task close` (floor) for a person → exit 1 `linked_file`, no file changed. Unlinked: agent exit 3, person exit 0 | `TestLinkedTaskWorkflowFileRefusesVerbs` (cli) | yes, the 3 linked subtests; the 2 unlinked controls pass on both |
| `signal move --inbox` on a linked workspace workflows.md → `linked_file`, not `invalid_policy`, nothing moved | `TestSignalMoveInboxLinkedPolicy` (cli) | yes |
| Linked task workflow.md: PersonVerbs, GatePostRoles, UsageNoticeThresholds return `ErrLinkedFile` without the path; PlanHashToken refuses; bytes and inode unchanged | `TestLinkedTaskWorkflowFileIsUnreadable` (workflow) | yes |
| `InboxWorkspace` on a linked file returns `ErrLinkedFile`, not `ErrInboxWorkspace` | `TestInboxWorkspaceLinkedFile` (workflow) | yes |
| `EnsureTaskID` on a linked task.md, without and with an id: refused, inode and bytes unchanged | `TestEnsureTaskIDRefusesALinkedTaskMD` (feed) | yes, both subtests |
| A linked task workflow.md fails the withdrawal policy closed, not to the defaults | `TestQuestionWithdrawPolicyRefusesALinkedWorkflowFile` (feed) | yes |

Failing lines on 71bd49c9, with the lane's four test files checked out on top (`go test -count=1 -run '<the 8 tests>' ./internal/cli/ ./internal/workflow/ ./internal/feed/`, exit 1; temp paths and long JSON cut):

```
--- FAIL: TestLinkedPolicyRefusesTheVerbItAdds/workspace/linked/agent
    linked_policy_closed_test.go:74: exit 0; want 1 (linked_file): {"result":"updated","id":"S-01ARZ3NDEKTSV4RRFFQ69G5FAV","project":"example","note":"S-01ARZ3NDEKTSV4RRFFQ69G5FAV.md","status":"kept","became":"T-09"}
--- FAIL: TestLinkedPolicyRefusesTheVerbItAdds/workspace/linked/person
    linked_policy_closed_test.go:74: exit 0; want 1 (linked_file): {"result":"updated",...,"status":"kept","became":"T-09"}
--- FAIL: TestLinkedPolicyRefusesTheVerbItAdds/project/linked/agent
    linked_policy_closed_test.go:74: exit 0; want 1 (linked_file): {"result":"updated",...,"status":"kept","became":"T-09"}
--- FAIL: TestLinkedPolicyRefusesTheVerbItAdds/project/linked/person
    linked_policy_closed_test.go:74: exit 0; want 1 (linked_file): {"result":"updated",...,"status":"kept","became":"T-09"}
--- FAIL: TestLinkedTaskWorkflowFileRefusesVerbs/linked/agent/task_show
    linked_policy_closed_test.go:126: exit 3; want 1 (linked_file): {"code":"person_required","error":"person_required",...,"reason":"agent_ancestor"}
--- FAIL: TestLinkedTaskWorkflowFileRefusesVerbs/linked/person/task_show
    linked_policy_closed_test.go:126: exit 0; want 1 (linked_file): {"id":"T-01","title":"Fix the label",...}
--- FAIL: TestLinkedTaskWorkflowFileRefusesVerbs/linked/person/task_close
    linked_policy_closed_test.go:126: exit 0; want 1 (linked_file): {"via":"terminal","task":"T-01",...,"what":"close","from_state":"queued","to_state":"closed",...}
--- FAIL: TestSignalMoveInboxLinkedPolicy
    linked_policy_closed_test.go:152: refusal JSON = {"error":"invalid_policy","key":"inbox_workspace","message":"invalid inbox_workspace in workspace workflow (only personal is supported)","code":"invalid_policy"}
         (json: cannot unmarshal string into Go struct field .error of type struct { Code string "json:\"code\"" }); want error.code linked_file
--- FAIL: TestLinkedWorkflowsFileRefusesPersonVerbs/workspace/agent_note
    linked_workflow_gate_test.go:45: exit 0; want 1 (linked_file): {"result":"updated",...,"status":"kept","became":"T-09"}
--- FAIL: TestLinkedWorkflowsFileRefusesPersonVerbs/project/agent_note
    linked_workflow_gate_test.go:45: exit 0; want 1 (linked_file): {"result":"updated",...,"status":"kept","became":"T-09"}
FAIL	github.com/gridconsole/grid-plugins/internal/cli
--- FAIL: TestLinkedTaskWorkflowFileIsUnreadable
    linked_task_test.go:31: PersonVerbs = map[task show:[]], <nil>; want ErrPersonVerbs wrapping linked_file, no path
    linked_task_test.go:34: GatePostRoles = map[ci:[...] job:[...] person:[...] timer:[...] withdraw:[owner]], <nil>; want linked_file
    linked_task_test.go:37: PlanHashToken = "digest", <nil>; want a refusal
    linked_task_test.go:40: UsageNoticeThresholds = map[tokens:[50 100]], <nil>; want linked_file, no path
--- FAIL: TestInboxWorkspaceLinkedFile
    linked_task_test.go:67: InboxWorkspace = "", invalid inbox_workspace in workspace workflow (only personal is supported); want linked_file, not invalid inbox_workspace, no path
FAIL	github.com/gridconsole/grid-plugins/internal/workflow
--- FAIL: TestEnsureTaskIDRefusesALinkedTaskMD/no_id
    linked_task_test.go:34: EnsureTaskID = "01M70WESR1CCDG97Y8W6Q9B2T7", <nil>; want linked_file, no path
    linked_task_test.go:39: task.md was replaced; the link evidence is gone
    linked_task_test.go:43: task.md changed: "---\ntitle: probe\nworkflow: ''\nulid: 01M70WESR1CCDG97Y8W6Q9B2T7\n---\nBody.\n"
--- FAIL: TestEnsureTaskIDRefusesALinkedTaskMD/with_id
    linked_task_test.go:34: EnsureTaskID = "01ARZ3NDEKTSV4RRFFQ69G5FAV", <nil>; want linked_file, no path
--- FAIL: TestQuestionWithdrawPolicyRefusesALinkedWorkflowFile
    linked_task_test.go:64: QuestionWithdrawPolicy = true, <nil>; want a path-free linked_file refusal
    linked_task_test.go:67: the read policy allowed a withdrawal from a linked workflow.md
FAIL	github.com/gridconsole/grid-plugins/internal/feed
```

## Mutations (on the lane, each restored with `git checkout --`, porcelain empty after)

| Mutation | Package run | Red rows |
|---|---|---|
| Move the `linkedErr` refusal back after `persongate.Required` (person_gate.go) | `go test -count=1 ./internal/cli/` | **12**: the 4 linked probe subtests, 2 task-local `task show` subtests, and all 6 `TestLinkedWorkflowsFileRefusesPersonVerbs` subtests. `task close` stays green because the floor makes it Required |
| Drop the `RefuseLinked` check in `EnsureTaskID` (`false &&`) | `./internal/feed/` | **2**: both `TestEnsureTaskIDRefusesALinkedTaskMD` subtests |
| Drop the link check in `readTaskPolicy` | `./internal/workflow/ ./internal/feed/ ./internal/cli/` | **5**: `TestLinkedTaskWorkflowFileIsUnreadable`, `TestQuestionWithdrawPolicyRefusesALinkedWorkflowFile`, 3 linked `TestLinkedTaskWorkflowFileRefusesVerbs` subtests |
| Drop the `ErrLinkedFile` branch in the `signal move` handler (run together with the previous one; separate tests) | `./internal/cli/` | **1**: `TestSignalMoveInboxLinkedPolicy` |

## Checks

These ran on code head 5612551e in a fresh clone of `lane-FP1R2`. This report commit changes docs/lanes/FP1.md only.

| Check | Exit |
|---|---|
| `go test -race -count=1 -timeout=30m ./internal/task/... ./internal/guard/... ./internal/cli/... ./internal/hook/... ./internal/land/... ./internal/orchestrator/... ./internal/workflow/... ./internal/feed/...` | 0 (task 12.3 s, guard 10.8 s, cli 57.0 s, hook 5.6 s, land 4.9 s, orchestrator 66.8 s, workflow 10.8 s, feed 8.7 s) |
| `make lint` (`go vet ./...`) | 0 |
| `make generate`, then `git status --porcelain` | 0, 0 lines |
| `GOOS=windows go build ./...` | 0 |
| `GOOS=darwin go build ./...` | 0 |
| `GOOS=windows go vet ./internal/workflow ./internal/feed ./internal/cli ./internal/task` | 0 |
| rule 14: `go test -count=1 -timeout=30m ./internal/cli/...` on the lane head | 0 |
| rule 14: the same on base main 9764ce4e | 0 |
| new rows on 71bd49c9 (above) | 1, as intended |
| hygiene: the FP1R2 diff grepped for /srv, /Users, /home/, emails, IPs, ssh and local names | no hits |

## Follow-ups (decision 5)

- Plan writers in internal/plan (write.go and store.go `atomicWrite`): `grid plan write` resolves the task with `where.Resolve`, so no `Linked` check runs. On a linked plan.md it archives the bytes written through the link and renames a new plan.md into place. No plan release, since the approval binds the digest.
- routine/workflow.go reads workflows.md through `safefile.Read`, with no link check. A forged file can raise the routine limits. No plan release.
- `PlanHashToken` on a linked workflow.md gives its generic "must be any or digest" message, not `linked_file`. The gate refuses `approve` first, so the message only shows to a direct caller.
- Unchanged from FP1R: `Linked` skips a symlink at a protected name, and the TOCTOU between the check and the rename.

## NOT VERIFIED

- Rule 18 (the app real-plugin set with a plugin built from this head) was not run; the operator runs it. This lane changes CLI output only when a policy file has a second hard link: exit 1 `linked_file` where a verb used to run or exit 3, and `linked_file` instead of `invalid_policy` for `signal move --inbox`.
- The checks ran as the builder user in a fresh local clone, not as user grid. `-timeout=30m` was passed as a flag, not through GOFLAGS. Nothing was pushed; the lane is delivered as a bundle.
- Link counts on Windows and darwin were compiled and vetted, not run.
- The probe ran as committed test rows. The reviewer's scratch probe file was not run as is.
