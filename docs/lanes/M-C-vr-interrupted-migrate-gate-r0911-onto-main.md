# M-C-vr-interrupted-migrate-gate-r0911-onto-main: lane C-vr-interrupted-migrate-gate merged onto plugin main 219ca2bb

Carries: C-vr-interrupted-migrate-gate (its lane doc: docs/lanes/C-vr-interrupted-migrate-gate.md). Merge commit 5ca93131 merged origin/main (219ca2bb) into the lane.

Verdict: DONE. Every row HOLDS on the merged head. One judgement call is open for the reviewer: item 1 under Open items.

## Conflicts and how each was resolved

- `contract/README.md` (the "Checks, in order:" line): kept main's whole sentence (the `grid on PATH` version text and the `launcher` and `worker` checks) and added the lane's two parts: `migration` right after `format version` in the list, and the sentence on what the migration check reports, placed after the "Writability and format checks are skipped" sentence. Nothing of either side was dropped.
- `testdata/golden/doctor-ok-json.golden`, `doctor-missing-home-json.golden`: took main's line (`grid is on PATH; version not read`) and put the lane's `migration` row back in the same place (after `format version`; after `grid home` when the home is absent). The three plain goldens merged without conflict.
- `internal/contract/gj_readme_doctor_checks_test.go:80-81`: not a textual conflict, but with the merge `go test ./internal/contract/` failed on the pinned count (`doctor reports 10 checks ..., want 9`). Changed `9` to `10` and `want 9` to `want 10`; no assertion changed. This is the fix the carried lane doc wrote out (its open item 1) and it is what closes that lane's NOT DONE rows 5b and 5c.
- The carried lane's NOT DONE rows (5b, 5c) and the importers cell now read `CLOSED BY M-C-vr-interrupted-migrate-gate-r0911-onto-main: <what decided it>` in docs/lanes/C-vr-interrupted-migrate-gate.md; its verdict line already read CLOSED. No text was removed.

## Proof table

| Row | Result | Command and result |
|---|---|---|
| 1. origin/main is an ancestor; diff names only the lane's files | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0. `git diff origin/main --stat` (15 files): contract/README.md, docs/lanes/C-vr-interrupted-migrate-gate.md, internal/cli/cli.go, internal/cli/vr_migrate_pending_test.go, internal/contract/gj_readme_doctor_checks_test.go, internal/doctor/doctor.go, internal/doctor/installation.go, internal/doctor/vr_migration_row_test.go, internal/migrate/vr_pending.go, internal/migrate/vr_pending_test.go, five doctor goldens. All are files the lane touched, plus the count pin (item 1 below). This lane doc is added at the end. |
| 2.1 migrate rows (carried row 1) | HOLDS | `go test -count=1 -v -run TestVrPend ./internal/migrate/` exit 0: `--- PASS: TestVrPendCutBetweenTheRenamesLeavesNoHomeAndAPendingJournal`, `--- PASS: TestVrPendAHomeMadeInTheGapStopsTheRecovery`, `--- PASS: TestVrPendIsTheJournalFileWhateverItsType`. |
| 2.2 cli rows (carried rows 2, 3) | HOLDS | `go test -count=1 -v -run TestVrPend ./internal/cli/` exit 0: `--- PASS: TestVrPendGridLeavesTheHomeAloneWhileAJournalIsOpen`, `--- PASS: TestVrPendNoJournalLeavesTheCommandsAsTheyWere`. `go test -count=1 -run 'TestMigrate\|TestHookSessionStartGolden\|TestVersionGolden' ./internal/cli/` exit 0 (run together with the golden and walk tests below: `ok .../internal/cli 0.200s`). The base output of `grid task new x` is in the carried lane doc, row 2. |
| 2.3 doctor rows (carried row 4) | HOLDS | `go test -count=1 -v -run TestVrPend ./internal/doctor/` exit 0: `--- PASS: TestVrPendDoctorShowsAnInterruptedMigrationWhenTheHomeIsAbsent`, `--- PASS: TestVrPendDoctorMigrationRowIsOKWithNoJournal`, `--- PASS: TestVrPendDoctorShowsTheJournalBesideAHomeThatExists`. |
| 2.4 goldens, README, four packages (carried row 5) | HOLDS | `go test -count=1 -run TestDoctorGolden ./internal/cli/` exit 0. `go test -count=1 ./internal/contract/` exit 0 (`ok .../internal/contract 0.505s`; includes `TestGjReadmeDoctorChecksMatch`, whose README-names-versus-doctor comparison now runs). `go test -count=1 ./internal/cli ./internal/doctor ./internal/migrate ./internal/contract`: `ok` for all four (`cli 52.019s`, `doctor 4.599s`, `migrate 8.813s`, `contract 1.640s`). |
| 2.5 lint, cross, Touches (carried row 6) | HOLDS | See rows 4 and 1. |
| New tests fail on base, pass on lane | HOLDS | Unchanged from the carried lane; base failures are pasted in its rows 1, 2 and 4. No new test was added by this merge. |
| Importers of the changed packages | HOLDS | `go test -count=1 ./internal/cli ./internal/doctor ./internal/migrate ./internal/contract ./internal/packaging ./internal/mcp ./internal/settings ./internal/golden ./internal/jobgraph ./internal/question ./internal/account ./internal/orchestrator ./internal/braincmd ./contract/... ./lib ./cmd/grid`: every package `ok` (cmd/grid `[no test files]`); the list is the carried lane's importer list. |
| 3. repo-wide walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok .../internal/cli 0.200s` with the golden tests); `go test -count=1 ./internal/job/lint/` exit 0 (`ok .../internal/job/lint 0.089s`). The app-side walk tests do not apply to this repo. |
| 4. make lint | HOLDS | `make lint` ran `go vet ./...` and printed nothing else; no error. |
| 4. make cross | HOLDS | `make cross` printed `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, every package `ok`, no FAIL or error. |
| Tests edited or deleted | see Open items 1 | No test deleted. One existing test edited: the count pin in `internal/contract/gj_readme_doctor_checks_test.go:80-81`, `9` to `10`. |
| commits | HOLDS | One-sentence messages, no attribution line (the owner's rule wins over the reminder), no personal path, host or email. |
| no guard on agents | HOLDS | None added; the lane's change extends the existing format refusal only. |
| *RealPlugin* tests, mismatch proof, UI rows | n/a | No row names one. |
| budget | HOLDS | About 35 of the 30-call brief plus the expected extras (proof runs, write-up). |

## What changed

- The merge itself (conflict resolutions above), the count pin `9` to `10`, the carried lane doc's NOT DONE rows closed, and this doc. No production file was changed beyond what the carried lane already changed.

## Open items

1. **Existing test edited.** `internal/contract/gj_readme_doctor_checks_test.go:80-81` pins doctor's check count; the lane's `migration` check makes it 10, so the pin was moved. The owner's rule says not to edit an existing test to make it pass; the reviewer runbook line 12 says a walk table the changed code forces counts as inside Touches. If the owner does not accept that reading, revert this one hunk, and `TestGjReadmeDoctorChecksMatch` is red until the count moves. Note C-vs-doctor-upgrade-rows adds checks too and will have to move the same count again.
2. The carried lane's open items 2 to 4 (symlinked `GRID_HOME`, journal present while a migrate runs, unreadable journal) are unchanged.

## NOT VERIFIED

Nothing here has a screen. All proof runs through the in-process harness; I did not run a built grid binary against a real interrupted migration.

DONE
