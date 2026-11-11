# C-fy-plugin-guest-task-move

Stacked on C-fv-plugin-guest-task-new-approve (merge commit 88f88587, which is the "base" below; the `origin/lane-C-fv-...` ref is older than that merge, so the stat is taken against 88f88587). On the base `guestTaskMove` (internal/cli/cli.go:246) named new, close, reopen and stop, so a guest's `grid task move` reached runIntentWrite and enqueued an intent. This lane adds "move" there and nothing else in production code. No new permission, nothing removed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Invited guest `task move laptop --task T-01 --project example` (project with a git remote): exit 1, `role "guest" may not move a task; a guest answers and comments`, no intent file | HOLDS | `go test -count=1 -run TestFyGuestTaskMoveRefusedEarly ./internal/cli/` PASS, subtests `invited`, `invited_to_form` (`task move T-01 --to laptop`), `uninvited`, `unknown_project`: exit 1, stderr `grid task move: role "guest" may not move a task; a guest answers and comments` (guestTaskMoveRefusal, cli.go:252-254; the guard is cli.go:194-211), stdout empty, intent files listed before and after (fy_guest_task_verbs_test.go:41-55, :72-79) equal, project folder snapshot unchanged. The intents folder is `<task folder>/intents` (intent/files.go:28), not a folder under the home; the test lists that one. **Fails on base 88f88587** (new test file, cli.go as on base): `--- FAIL: TestFyGuestTaskMoveRefusedEarly/invited: exit 0, want 1:` and `/invited_to_form` the same; `/uninvited: stderr "grid: project \"example\" is not registered", want "grid task move: role \"guest\" may not move a task; ..."`; `/unknown_project: stderr "grid: project \"nosuch\" is not registered"`. |
| 2. Nine rows: rename, decline, move by guest, member, owner | HOLDS | `go test -count=1 -v -run TestFyGuestTaskVerbsByRole ./internal/cli/` PASS, 9 subtests. guest: rename `grid task rename: scope: role cannot rename tasks`, decline `grid task decline: scope: role cannot decline tasks` (exit 1, as on base), move the row 1 text (exit 1); the project folder snapshot is unchanged. member and owner: exit 0 and the change is there (rename: the new title is in task.md; decline: the reason is in the task folder; move: exactly one intent file). On base only `guest/move` fails: `guest task move: exit 0, want 1:`. |
| 3. Boundary walk: one walk only, in TestFvGuestTaskVerbs | HOLDS | The base has it (fv_guest_task_test.go, TestFvGuestTaskVerbs reads ../../contract/commands.json). Only its `move` row changed (exit 1, row 1 text, snapshot unchanged); no second walk was added. TestFvGuestTaskVerbs PASS in the full package run. The new file adds `TestFyGuestWriteVerbEarlyList`: guestWriteVerb("task", verb) is true for close, delete, move, new, reopen, restore, stop and false for show, list, history; PASS. On base it fails: `guestWriteVerb(task, move) is false`. |
| 4. `go test -count=1 ./internal/cli/` and `make lint` | HOLDS | `go test -count=1 ./internal/cli/`: `ok ... 55.532s`, exit 0 (guest_parity_test.go, guest_role_test.go, task_rename_decline_test.go all in it). The uninvited-guest part of parity (TestGuestProjectParity `task_move/*/invited`) now gets the early answer for move as for stop. `make lint` (`go vet ./...`): exit 0, no finding. |
| 5a. `GOOS=windows go vet ./internal/cli/` | HOLDS | exit 0, no output. |
| 5b. `git diff` names only the four Touches files | HOLDS | `git diff 88f88587 --stat`: internal/cli/cli.go (8 +-), internal/cli/fv_guest_task_test.go (17 +-), internal/cli/fy_guest_task_verbs_test.go (179 +), internal/cli/guest_parity_test.go (2 +-). This report is a fifth file in its own commit, as the runbook asks. |
| New tests fail on base | HOLDS | See rows 1 to 3 (base 88f88587: `exit 0, want 1` for an invited guest's move). |
| Tests of the changed package and its importers | HOLDS | `go list` importers of internal/cli: cmd/grid, lib. `go test -count=1 ./cmd/grid/ ./lib/`: cmd/grid `[no test files]`, lib `ok`, exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`: exit 0. `gofmt -l internal/cli/` prints nothing. |
| No guard, gate, cap or hold on agents | HOLDS | GRID_LEASE=task stays exempt (cli.go:194, unchanged). `TestFyGuestTaskMoveWithTaskLease` PASS: a guest with GRID_LEASE=task still enqueues one intent (passes on base too). Nothing removed. |
| *RealPlugin* tests | n/a | None named by the brief; none run. |

## What changed

- internal/cli/cli.go: `guestTaskMove` also returns true for `move`; its comment and `guestTaskMoveRefusal`'s name the verb.
- internal/cli/guest_parity_test.go: `guestParityEarlyAnswer` adds `task move` to the close/reopen/stop case, and also `task new` (see item 1 below).
- internal/cli/fv_guest_task_test.go: the `move` row of `fvGuestVerbs` is exit 1 with the row 1 text and an unchanged snapshot; doc comments updated; and the `move_with_remote` row of `TestFvGuestTaskVerbsWhenValid` (see item 2 below).
- internal/cli/fy_guest_task_verbs_test.go (new): `TestFyGuestTaskMoveRefusedEarly`, `TestFyGuestTaskMoveWithTaskLease`, `TestFyGuestTaskVerbsByRole`, `TestFyGuestWriteVerbEarlyList`.

## Rows that differ from the brief, as seen

1. TestGuestProjectParity was already red on the base, for `task new` (the base's own report names it as its "Blocker"): `go test -run TestGuestProjectParity/task_new` on 88f88587 plus only the new test file fails with `invited guest still gets unknown answer: {Code:1 ... grid task new: role "guest" may not new a task; ...}`. Row 4 needs parity green, so `task new` joined the same case of `guestParityEarlyAnswer` (the case's expected text is the same shape, `strings.TrimPrefix(command, "task ")`). It is a test-table change in the file and function Touches names; no production code.
2. `TestFvGuestTaskVerbsWhenValid/move_with_remote` pinned the gap this lane closes (guest move with a remote: exit 0, folder changed; its comment says "a follow-up"). Touches names only the `move` row of TestFvGuestTaskVerbs, but that row would otherwise fail, so it now pins the refusal (exit 1, row 1 text, folder unchanged). The behaviour it asserted was removed by this brief; `TestFyGuestTaskMoveRefusedEarly/invited` covers the same input. `attach_bound_task` and `status` still show guest writes the CLI layer does not refuse (not in this lane).
3. "no intent file under the home's intents folder": intents are written under the task folder (`<task>/intents`), so the test lists that folder (it does not exist before or after a refused move) and also snapshots the whole project folder.

## Notes for the reviewer

- `grid task move --help` as a guest is now refused early too (guestWriteVerb exempts only a bare `--help`), the same as `task close --help`.
- Rename and decline keep their after-resolution refusal; the new walk does not pin `guestWriteVerb` for them, because the owner question about them is open (INDEX.md).

## NOT VERIFIED on a live screen

Nothing in this lane is UI. Not run: a real built plugin (no row names *RealPlugin*), the whole-module test and the cross-repo set (the orchestrator runs them at landing).
