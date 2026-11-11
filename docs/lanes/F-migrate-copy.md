# F-migrate-copy: grid migrate on a copy shaped like the owner's home, with rollback

Repo: plugin. Branch: lane-F-migrate-copy. Base: 842f006e.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1a | Where an Unmapped entry makes --apply refuse | HOLDS: `internal/migrate/apply.go:94-95` (`len(p.Unmapped) > 0` returns "migration blocked"). The CLI dry run also exits 1 on Unmapped at `internal/cli/migrate.go:84-85` (not touched). |
| 1b | Where ids become ULIDs | HOLDS: tasks at `internal/migrate/plan.go:612-618` (`T-NN` becomes the display number; the id is the stored `ulid`, else `legacyID("task:...")`); projects at `internal/migrate/plan.go:564-566` (`legacyID("project:...")`). The id map is filled at `plan.go:584` (project) and `plan.go:634` (task). |
| 2 | A task folder with no task.md is skipped, kept in place, copied untouched; --apply goes on; the plan names it in one line | HOLDS: skip branch `plan.go:591-596`; the line is printed at `plan.go:661` as `skip: <folder>: no task.md; kept in place, copied untouched`. The folder is already copied by `copyPrefix` of the whole project, so it arrives byte for byte. Tests: `TestTaskFolderWithoutRecordIsSkipped` (one skip line, 0 unmapped, Apply ok) and cli `TestMigrateSkipsTaskFolderWithoutRecord` (dry run exit 0, apply exit 0, folder unchanged). |
| 2b | No other Unmapped kind changes behaviour | HOLDS: only a directory with no task.md is skipped. `TestOtherUnmappedKindsStillBlock` shows a stray file under tasks/, a task.md with no frontmatter and a project without project.md each still give 1 Unmapped and "migration blocked". The existing tests in plan_test.go and apply_test.go pass unchanged. |
| 3 | Owner-shaped home migrates in a temp copy: ULID for every task, every old id in the id map, source not written | HOLDS: `TestOwnerHomeMigratesOnACopy` on `internal/migrate/testdata/owner-home/` (two projects, T-346 without task.md, notes/, knowledge/ with index.md and backlinks.json, feed jsonl). It checks every migrated task id against `ulidRE`, `display` equal to the old id, the id map in migration.log (2 tasks and 2 projects, 4 entries) and that the source tree (path, mode, checksum, mtime) is the same after apply. |
| 4 | rollback on the copy restores the source byte for byte | HOLDS: `TestOwnerHomeRollbackRestoresTheTree` (in place, the cutover shape: hash of every path, mode and file before apply equals the hash after Rollback) and `TestOwnerHomeRollbackFromACopyLeavesTheSource`. |
| 5 | Running --apply twice is a no-op the second time | HOLDS: `TestOwnerHomeApplyTwiceIsANoOp` (second Scan is Current, Apply gives Changed=false and no backup path; the first plan applied again also does nothing; home, backup folder and siblings are identical, mtimes included) and cli `TestMigrateSkipsTaskFolderWithoutRecord` (second apply prints "already at format 1; no changes"). |
| 6 | `go test ./internal/migrate/... ./internal/cli/...` and `make lint`: 0 fail | HOLDS: go test: `ok internal/migrate 17.776s`, `ok internal/cli 209.638s`, exit 0. `make lint` exit 0 (`go vet ./...`; it first failed on gofmt of my new cli test, fixed with gofmt, lint rerun printed no error). |
| R1 | New tests fail on base | HOLDS: with plan.go and apply.go from 842f006e, the cli test fails: `--- FAIL: TestMigrateSkipsTaskFolderWithoutRecord ... migrate_test.go:87: dry run: 1: ... unmapped: projects/dw6/tasks/T-346-unsent: missing task.md`. The migrate package test does not build on base: `owner_home_test.go:116:11: p.Skipped undefined (type *Plan has no field or method Skipped)`, `owner_home_test.go:204:23: undefined: IDMapping`, `man.IDs undefined`. |
| R2 | No edit to an existing test; no deletion | HOLDS: `git diff 842f006e --stat` shows only added test code (a new file `owner_home_test.go` and one new test in `cli/migrate_test.go`); nothing deleted. |
| R3 | No guard, gate, cap, hold or blocking hook added | HOLDS: the change removes a block (Unmapped for a task folder with no task.md); it adds none. |
| R4 | Packages that import the changed package | HOLDS: `go list` of importers of `internal/migrate`: `internal/cli` (and migrate itself). Both ran in row 6. |
| R5 | Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./... && GOOS=darwin CGO_ENABLED=0 go build ./...` printed builds-ok, exit 0. |
| R6 | Small commits, one-sentence messages, no attribution lines, no personal paths, hosts or emails | HOLDS: see `git log 842f006e..`. The fixture uses `/example/...` paths and `example@example.com`. |
| R7 | Only the brief's packages touched | HOLDS: `internal/migrate/` (plan.go, apply.go, tests, testdata), `internal/cli/migrate_test.go`, this file. Not `cli/migrate.go`, not the task package. |

## What changed

- `plan.go`: a directory under `tasks/` with no task.md goes to the new `Plan.Skipped` list instead of `Unmapped`; `WriteText` prints one `skip:` line each. A plain file under `tasks/` is still unmapped, as before.
- `plan.go`: new `IDMapping` type and `Plan.IDs`: the old id and the new id of every project and task (a project with no id uses its folder name as the old id).
- `apply.go`: `migration.log` (the manifest) now carries `ids` and `skipped` (both omitted when empty, so old logs still read). Rollback reads the manifest with plain `json.Unmarshal`, so the new fields do not affect it.
- `internal/migrate/testdata/owner-home/`: the fixture. `owner_home_test.go`: the tests above. `internal/cli/migrate_test.go`: one CLI test.

## Open items

- There was no id map in the code before. I defined it as `Plan.IDs` and the `ids` list in migration.log; the brief row "every old id is in the id map" is tested against that. If the board wants another shape (for example a separate file), that is a follow-up.
- The Plan result line (`result: N actions, 0 conflicts, 0 unmapped`) is unchanged, so it does not count skips; the `skip:` lines carry them.
- A stray non-folder file under `tasks/` (for example `.DS_Store`) still blocks apply. Left as it was, since the brief says no other Unmapped kind changes. The real home was reported to have only the T-346 folder.
- Tool-call budget: I went about 5 calls over 60 (about 3 of them to fix a gofmt lint failure and a refused `;` chain).

## NOT VERIFIED on a live screen

- No UI change. `grid migrate` was not run against the owner's real home; the fixture is shaped like it (two projects, one folder with no task.md, notes/, knowledge/ with a brain index, feed files). The cutover run on a copy of the real home is for the orchestrator.
