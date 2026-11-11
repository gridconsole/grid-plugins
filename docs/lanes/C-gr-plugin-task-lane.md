# C-gr-plugin-task-lane: `grid task lane <task> <lane>`

Result: NOT DONE. The feature works and its own tests pass, but three existing
tests that list every contract member fail until a `task lane` row is added to
each, and the brief forbids editing those files.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `task lane T-01 doing --json` prints `"lane":"doing"`; task.md gains only `lane: doing`; history gains one line with `moved to Doing` | HOLDS | `go test -count=1 -run TestGrTaskLane ./internal/cli/` ok (TestGrTaskLaneMovesAndKeepsTheRest, internal/cli/gr_task_lane_test.go). It compares task.md byte for byte against the original plus the one `lane: doing` line. The fixture has a comment, odd spacing, a flow mapping and a body. |
| 2. Back to planning adds `moved back to Planning` and `plan and step kept`; plan file byte-equal | HOLDS | Same test: one new history line holds both phrases, plan.md is unchanged. |
| 3. Inbox: paused gets `state: queued` and `lane: inbox`; running is refused, task.md byte-equal | HOLDS | TestGrTaskLaneInbox. The key written is `state:`, not `status:` (see open items). A running task exits 3 and the stderr names `running`. |
| 4. Refusals leave the task folder byte-equal: closed, planner, same lane, lane `done`, guest, unknown task | HOLDS | TestGrTaskLaneRefusals (exit 3, 3, 3, 2, guest 1), TestGrTaskLaneNoSuchTask (`no_such_task` on stdout, same stderr with and without `--json`), TestGrTaskLaneProjectMismatch. Only the empty `.task.lock` may be added, as for rename and decline. |
| 5. `go test -run 'TestGgUnknown\|Contract' ./internal/cli/` PASS; no gg_*_walk_test.go edit | NOT DONE | The contract tests and `TestGgUnknownProjectWalk` pass with the new commands.json entry. `TestGgUnknownTaskWalkCoversTheContract` FAILS: `contract members not walked or skipped: [task lane]` (gg_unknown_task_walk_test.go:127). It reads commands.json, and any JSON subcommand with a `<task>` arg must be in ggTaskWalked or ggTaskSkipped. The brief's premise that the walks pass as they are does not hold. The brief forbids editing the file, so I stopped. |
| 6. `go test -count=1 ./internal/task/ ./internal/cli/` exit 0 | NOT DONE | `internal/task` ok. `internal/cli` FAILS with three tests: TestGgUnknownTaskWalkCoversTheContract, `TestFvGuestTaskVerbs` (fv_guest_task_test.go:280: "grid task lane is in contract/commands.json but in neither fvGuestVerbs nor fvGuestReadVerbs") and `TestGuestProjectParity` (guest_parity_test.go:350: "missing guest parity command: task lane"). Each needs one `task lane` row in an existing shared table. |
| 6. `make lint` | HOLDS | `make lint` printed only `go vet ./...`, no findings. I piped it through `tail`, so the exit code was not captured. |
| 6. Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` printed windows-ok (exit 0). |
| 6. macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` printed darwin-ok (exit 0). |
| 6. diff names only Touches files | PARTLY | Touched: internal/cli/task.go, cli.go, task_lane.go, contract_inventory_test.go, gr_task_lane_test.go; internal/task/lane.go; contract/commands.json; testdata/golden/help.golden and task-help.golden. task-help.golden is not in the brief's list: `TestTaskRenameDeclineHelp` compares it to `taskUsage`, which the brief says to change. No gg_*_walk_test.go file is touched. |
| Packages that import the changed ones | NOT DONE | I ran internal/task and internal/cli. I did not run `go list` for the importers. The orchestrator runs the whole module at landing. |
| New tests fail on base | NOT DONE | I did not run the new test file against main (out of tool budget). It uses only the CLI, so it compiles on base, where `task lane` is an unknown subcommand. |
| No guard on agents; nothing removed | HOLDS | `SetLane` has no role policy. The only refusals are the brief's. No test was deleted or edited. The one edit to an existing test file is the one annotation in contract_inventory_test.go that the brief names. |

## What changed

- `internal/task/lane.go`: `Lanes`, `ValidLane`, `Store.SetLane` and `Store.CheckProject`.
  `SetLane` rewrites task.md through `rewriteTask`, the writer `grid task status` uses.
  It sets `lane`, and sets `state: queued` for inbox only where `NextState` allows it.
  It commits task.md and one history event through `commitWrite`; the event's note is `moved to <Lane>` or
  `moved back to <Lane> (plan and step kept)`. Back is by the order inbox < planning < doing < delivering.
  A task with no `lane` yet is never "back". Closed, kind planner and same-lane tasks are refused with nothing written.
- `internal/cli/task_lane.go`: `grid task lane <task> <lane> [--project P] [--json]`. It prints
  `{"state":"applied","task":{"id","status","lane"}}`. Exit codes: 3 refused, 2 bad lane or usage, 1 other.
  A project that is not registered is answered before the lane is read, as `task list` and `task new` do, so `TestGgUnknownProjectWalk` passes.
- `cli.go`: `lane` is in the `task` summary and in `guestTaskMove`. A guest gets exit 1 and
  `role "guest" may not move a task to a lane`. The summary now reads `stop|close|reopen` to stay under the
  120-column help limit (`TestExHelpLinesFitOneLine`); every verb is still named.
- `contract/commands.json`: one `lane` entry and the new summary, hand-edited to the generator's format because the
  `UPDATE_CONTRACT_COMMANDS=1` env prefix is refused here. `TestContractCommandsParserInventory` and
  `TestContractCommandsInventory` pass against it.

## Open items for the orchestrator

1. Add `task lane` to the three existing tables, one row each: `ggTaskWalked` in
   gg_unknown_task_walk_test.go (the walk may need a valid lane operand), `fvGuestVerbs` in
   fv_guest_task_test.go, and the command list in guest_parity_test.go. Then
   `go test -count=1 ./internal/task/ ./internal/cli/` should be green. I did not touch them because the brief forbids it.
2. The task.md key for the run state is `state:`, not `status:` (`internal/task/task.go:35`, `write.go:126`). Row 3's "status: queued"
   is `state: queued` on disk. The `--json` task object uses `status`, as the brief says. The app must read `state` from
   task.md for the run state and `lane` for the lane.
3. The writer does not change the `updated` key: the brief says every other key stays.
4. `app` `fake.mjs` was not readable from here. The rules (closed, Planner, same lane, back/forward text) come from the brief only.

## NOT VERIFIED on a live screen

The task page's state menu was not exercised: no app, browser or `moveLane` call. The app's reading of `card.lane` was not checked here.
