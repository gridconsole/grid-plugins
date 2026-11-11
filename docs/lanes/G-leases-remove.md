# G-leases-remove: stopped at row 0, no code changed

Base: main. Lane branch: lane-G-leases-remove. This lane adds only this file.

## Why it stopped

Row 0 is a guard: the F-leases-map map must be on main. It is not.

- `docs/lanes/F-leases-map.md` is not in this tree, and `git ls-tree -r --name-only origin/main docs/lanes/F-leases-map.md` prints nothing.
- `git grep -n -i "leases-map"` finds no reference in the tree.
- The map exists only on `origin/lane-F-leases-map` (commit daafee06). `git branch -a --contains origin/lane-F-leases-map` lists no other branch, so it has not been merged into main or into this lane's base.
- A copy of the lane report is at `/srv/gridwork/tmp/hand/F-leases-map-report.md`. That is a hand-off file, not main.

The brief says to stop with NOT DONE when the map is not on main, so no file was removed.

## The removable list, quoted from the unlanded map (section E)

> 1. **Merge slot only** (`lease take|renew|release|status [merge]`): `lease/lease.go`, the merge branch of `cli/liveness.go:20-107`, and its `contract` lease schema use if unshared. Nothing in the plugin calls it and no second machine can read it. Delete only after the app owner confirms the app does not call it.
> 2. **Nothing else.** The rest is either a cross-machine fence (keep) or the OS flock/process table already doing the job (the code is the built-in).

The map also says "Whether the app runs `grid lease take` without `task` (the merge slot) is NOT KNOWN from this repo."

## Two points for the orchestrator before this lane is re-run

1. The brief's Touches line lists `cli/task_lease.go`, `internal/lease/` as a whole, the lease flags on job commands, and the `--lease` and `--lease-epoch` flags. The map marks the task lease, the job lease, the takeover file and the fence counter as CROSSES, keep, and says nothing else is removable. Those are the "used" items the brief says to keep. If the map lands as written, the removable set is only the merge slot. That is `internal/lease/lease.go` plus the merge branch of `internal/cli/liveness.go`, and their tests. The brief should say that.
2. The map makes the merge-slot removal conditional on the app owner confirming the app does not call `grid lease take` without `task`. Removing the verb is hard to reverse for the app, and the repo cannot answer that question. I did not guess.

Read-only facts I checked on this base so the re-run fits in budget:
- `contract/commands.json:1764` has the `lease` command.
- `internal/lease` is imported by `internal/cli/liveness.go`, `internal/cli/task_lease.go`, `internal/orchestrator/{stream,tick,write}.go`, `internal/hook/events.go`, `internal/ports/ports.go` and several tests. A merge-slot-only removal touches `lease/lease.go` and `cli/liveness.go`, and the shared task lease code stays.

## Proof table

| Row / rule | Result |
|---|---|
| 0 map on main, quote removable list | NOT DONE: map is not on main (`git ls-tree -r --name-only origin/main docs/lanes/F-leases-map.md` prints nothing, exit 0). The removable list is quoted above from the lane branch |
| 1 each removable item deleted, `git grep` shows no caller | NOT DONE: blocked by row 0, nothing deleted |
| 2 old lease files in a task folder load and are ignored (Go test) | NOT DONE: blocked by row 0. The task-folder lease files are marked keep by the map, so this row may not apply once the map lands |
| 3 removed flag gives the unknown-flag usage error (Go test) | NOT DONE: blocked by row 0, no flag removed |
| 4 `make golden` | NOT DONE: no change to regenerate from |
| 5 `go test ./...` and `make lint` | NOT RUN: no package changed, so the `go list` importer set is empty. The orchestrator runs the module at landing |
| Windows and macOS builds | NOT RUN: no code changed |
| New tests fail on base, pass on lane | NOT APPLICABLE: no new test, no test edited or deleted |
| No guard, gate, cap or hold on agents | HOLDS: nothing added |
| Commits one sentence, no attribution lines | HOLDS: see `git log` on the lane |
| Public repo: no personal paths, hosts or emails | HOLDS: this file names only repo paths. The `/srv/gridwork/tmp/hand/` path above is in the report for the orchestrator and can be dropped before landing |

## What changed

Only this file, `docs/lanes/G-leases-remove.md`.

## NOT VERIFIED on a live screen

Nothing here touches a screen.

## To re-run

Land `lane-F-leases-map` on main, then either confirm the app owner's answer on the merge slot or restate the brief as "merge slot only". The expected removal is `internal/lease/lease.go`, the merge branch of `internal/cli/liveness.go`, the tests the map lists, and a `make golden`.
