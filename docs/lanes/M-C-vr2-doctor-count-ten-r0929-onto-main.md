# M-C-vr2-doctor-count-ten-r0929-onto-main: lane C-vr2-doctor-count-ten onto plugin main 31c7f182

Base: plugin main 31c7f182. Carries: C-vr2-doctor-count-ten. Merge commit 47044927 (origin/main merged into the lane); the head sha is in the report.

Verdict: DONE.

## What changed

- Merged origin/main into the lane. One conflict, in `docs/lanes/C-vr-interrupted-migrate-gate.md`, the `Verdict:` line: the lane wrote "CLOSED BY C-vr2-doctor-count-ten", main wrote "CLOSED BY C-vr-interrupted-migrate-gate". Both are kept as "earlier wordings" in one line that now reads "CLOSED BY M-C-vr2-doctor-count-ten-r0929-onto-main: ...", as the brief asks for the carried verdict. No other line of either side was touched, and no row, test or error code was dropped.
- Main already holds the code half of the lane: `internal/contract/gj_readme_doctor_checks_test.go:80-81` says `!= 10` and `want 10` (set by M-C-vr-interrupted-migrate-gate-r0911-onto-main). So the lane's remaining diff against main is docs only: `docs/lanes/C-vr2-doctor-count-ten.md` and the closed verdict line.
- C-vs-doctor-upgrade-rows is not on main (doctor prints ten checks; the pin is ten).
- No NOT DONE verdict, row or last line remains in docs/lanes/ for the lane carried (`grep -n -i 'NOT DONE' docs/lanes/C-vr2-doctor-count-ten.md docs/lanes/C-vr-interrupted-migrate-gate.md` prints nothing).

## Proof table

| Row | Result | Command and result |
|---|---|---|
| 1. main is an ancestor; diff names only lane files | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0 (merge 47044927). `git diff origin/main --stat`: `docs/lanes/C-vr-interrupted-migrate-gate.md` (1 line, the verdict) and `docs/lanes/C-vr2-doctor-count-ten.md` (new); both are files the carried lane touched. |
| 2a. C-vr2 row 1: TestGj | HOLDS | `go test -count=1 -run TestGj -v ./internal/contract/` exit 0: `--- PASS: TestGjReadmeDoctorChecksMatch (0.00s)`, `ok .../internal/contract 0.005s`. |
| 2b. C-vr2 row 2: every row of C-vr-interrupted-migrate-gate | HOLDS | `go test -count=1 -v -run TestVrPend ./internal/migrate/ ./internal/cli/ ./internal/doctor/` exit 0: `--- PASS: TestVrPendCutBetweenTheRenamesLeavesNoHomeAndAPendingJournal`, `TestVrPendAHomeMadeInTheGapStopsTheRecovery`, `TestVrPendIsTheJournalFileWhateverItsType`, `TestVrPendGridLeavesTheHomeAloneWhileAJournalIsOpen`, `TestVrPendNoJournalLeavesTheCommandsAsTheyWere`, `TestVrPendDoctorShowsAnInterruptedMigrationWhenTheHomeIsAbsent`, `TestVrPendDoctorMigrationRowIsOKWithNoJournal`, `TestVrPendDoctorShowsTheJournalBesideAHomeThatExists`; `go test -count=1 -run 'TestMigrate\|TestHookSessionStartGolden\|TestVersionGolden\|TestDoctorGolden' ./internal/cli/` -> `ok .../internal/cli 0.539s`; lint and cross are rows 4a/4b. |
| 2c. C-vr2 row 3: three packages | HOLDS | `go test -count=1 ./internal/contract/ ./internal/migrate/ ./internal/cli/ ./internal/doctor/` exit 0: contract ok 1.264s, migrate ok 5.964s, cli ok 43.217s, doctor ok 2.136s. Importers: `go list` shows cli, hook, brain, job, task, workflow and others import these packages, but this lane changes no Go package against main (docs only), so the packages that changed are none. |
| 2d. C-vr2 row 4: diff names only Touches | HOLDS | Touches of C-vr2: the count test (already on main), contract/README.md (not needed: the order check passes), docs/lanes/C-vr2-doctor-count-ten.md. Against main only the lane doc and the one carried verdict line differ. |
| 3. repo-wide walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok .../internal/cli 0.062s`; `go test -count=1 ./internal/job/lint/` -> `ok .../internal/job/lint 0.108s`. 0 fail. |
| 4a. lint | HOLDS | `make lint` exit 0 (`go vet ./...`, no output). |
| 4b. cross | HOLDS | `make cross` exit 0: `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`; no FAIL line. |
| Rule: new tests fail on base, pass on lane | HOLDS (none added) | This lane adds no test. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | None added; the diff is two docs. |
| Rule: carried NOT DONE verdicts closed | HOLDS | `docs/lanes/C-vr-interrupted-migrate-gate.md:5` reads `Verdict: CLOSED BY M-C-vr2-doctor-count-ten-r0929-onto-main: ...`. |
| Rule: no personal paths, hosts or emails in the doc | HOLDS | Paths in this doc are repo-relative. |

## NOT VERIFIED on a live screen

Nothing here is a UI change; there is no live-screen item.
