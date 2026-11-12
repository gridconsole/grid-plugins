# C-vr2-doctor-count-ten

Change: `internal/contract/gj_readme_doctor_checks_test.go` lines 80 and 81, 9 to 10. No assertion changed. The order check at :83 passes, so `contract/README.md` is untouched.

C-vs-doctor-upgrade-rows is not on main (main is 792b8d6e); the count is 10 for the migration check only.

| Row | Status | Proof |
|---|---|---|
| 1 TestGj | HOLDS | `go test -count=1 -run TestGj -v ./internal/contract/` exit 0: `--- PASS: TestGjReadmeDoctorChecksMatch (0.00s)` and `--- PASS: TestGjReadmeDoctorChecksMatch/a_removed_name_is_caught` |
| 2 C-vr rows 1-4 | HOLDS | `go test -count=1 -v -run TestVrPend ./internal/migrate/ ./internal/cli/ ./internal/doctor/` exit 0, all `--- PASS`. cli: `grid task new x: exit 1, stdout "", stderr "A migration of ~/.absent stopped part way. Run: grid migrate --apply\n"`; `grid hook sessionstart: exit 0, stdout "", stderr ""`. |
| 2 C-vr rows 5-6 | HOLDS | `go test -count=1 ./internal/contract/ ./internal/migrate/ ./internal/cli/ ./internal/doctor/` exit 0 (contract, migrate, cli 41.9s, doctor all ok); `make lint` exit 0; `make cross` exit 0. |
| 3 packages | HOLDS | Same run as above, exit 0. Importers of the changed test package: none (test-only file). |
| 4 diff stat | HOLDS | Local branch `lane-C-vr-interrupted-migrate-gate` does not exist here; against HEAD (the C-vr head) `git diff HEAD --stat` names only `internal/contract/gj_readme_doctor_checks_test.go` (2 lines), plus this doc. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| Windows/macOS | HOLDS | `make cross` exit 0. |
| No guard on agents | HOLDS | None added. |

NOT VERIFIED on a live screen: nothing here is UI.
