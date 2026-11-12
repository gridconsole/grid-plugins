# C-vs-doctor-upgrade-rows

Base: plugin main 792b8d6e53bb5a1c84a82c2e5da65bd638babfac. Base `internal/doctor/installation.go:99-103`:

    if _, err := lookPath("grid"); err != nil {
        add("grid on PATH", false, "grid was not found on PATH")
    } else {
        add("grid on PATH", true, "grid is on PATH")
    }

Base prints no version, so the lane goes ahead. C-vr-interrupted-migrate-gate: `git -C /srv/gridwork/ref/plugin.git merge-base --is-ancestor lane-C-vr-interrupted-migrate-gate main` exit 128 (`fatal: Not a valid object name lane-C-vr-interrupted-migrate-gate`): the branch is not in the ref repo, so no `migration` row is in this tree. The README sentence and the goldens carry no `migration` text; the orchestrator runs `git merge main` after C-vr lands (see open items).

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | Red on base, green on lane for "grid on PATH" differing / same | HOLDS. Base (new tests before the code), `go test -count=1 -v -run 'TestVsDoc' ./internal/doctor/` exit 1: `vs_upgrade_rows_test.go:48: a different grid on PATH must fail: {Name:grid on PATH OK:true Detail:grid is on PATH Fix:}`, `vs_upgrade_rows_test.go:69: got {Name:grid on PATH OK:true Detail:grid is on PATH Fix:}`. Base detail: `grid is on PATH`. Lane, same command exit 0: `--- PASS: TestVsDocPathVersionDiffers`, `--- PASS: TestVsDocPathVersionSame` (detail `grid is on PATH; version 0.1.0`; differing detail holds `/x/bin/grid`, `0.0.1-vs`, version.Version, and the fix `Update or remove the grid at the path above, or put the current plugin's bin folder first on PATH.`). |
| 2 | GridVersion nil or error: ok true, `grid is on PATH; version not read`, Report.OK stays true | HOLDS. Base failed (`vs_upgrade_rows_test.go:87: got {... Detail:grid is on PATH ...}`); lane `--- PASS: TestVsDocPathVersionNotRead/nil`, `/error` (same command, exit 0). `TestVsDocPathNotFoundUnchanged` PASS: not-found detail and fix unchanged. |
| 3 | `launcher` check: none without bin/grid; symlink, other version fails and names the resolved target; same version ok | HOLDS. Base: `vs_upgrade_rows_test.go:155: no "launcher" check in [...]` (exit 1). Lane PASS: `TestVsDocLauncherAbsentHasNoCheck`, `TestVsDocLauncherVersions` (also `version not read` with GridVersion nil). Code `internal/doctor/vs_upgrade.go:30-57`. |
| 4 | `worker` check with worker.lock held by a second handle (fslock.OpenNoFollow + TryLockExclusive): 0.0.9 not ok with detail == version.AppWarning("", "", "0.0.9"); 0.1.0 and 9.9.9 ok `running, version <v>`; no lock: no check | HOLDS. Base: `vs_upgrade_rows_unix_test.go:39: no "worker" check in [...]` (exit 1). Lane PASS: `TestVsDocWorkerOld`, `TestVsDocWorkerCurrent`, `TestVsDocWorkerNoLockNoCheck`, `TestVsDocWorkerStoppedHasNoCheck` (a stale status file alone adds no check). Code `vs_upgrade.go:59-79`. |
| 5a | `go test -count=1 -run 'TestDoctorGolden' ./internal/cli/` | HOLDS, exit 0 (run before the goldens were edited: exit 1, all five differ only on the `grid on PATH` line, `got` = `grid is on PATH; version not read`; after the hand edit: exit 0). |
| 5b | `go test -count=1 -run 'TestGjReadmeDoctorChecksMatch' ./internal/contract/` | HOLDS, exit 0. |
| 5c | `go test -count=1 ./internal/doctor ./internal/cli ./internal/contract` | HOLDS, exit 0: `ok .../internal/doctor 2.236s`, `ok .../internal/cli 97.792s`, `ok .../internal/contract 0.865s`. First README wording failed `TestGbDocsVerbsAreImplemented` and `TestGoReadmeSubverbsInCommandsJSON` (a backtick span starting with `grid is`); reworded so no span starts with `grid `, no new allow-list entry needed. |
| 6a | `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| 6b | `make cross` | HOLDS, exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, all packages `ok`. |
| 6c | `git diff main --stat` names only Touches | HOLDS: contract/README.md, internal/cli/doctor.go, internal/doctor/{doctor.go,installation.go,vs_upgrade.go,vs_upgrade_rows_test.go,vs_upgrade_rows_unix_test.go}, five testdata/golden/doctor-*.golden, plus this report. |
| R1 | Packages that import the changed ones | `go list` over `./...`: the only package importing `internal/doctor` is `internal/cli`; `internal/worker` and `internal/version` are only read, not changed. Tested: doctor, cli, contract (5c). |
| R2 | Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|TestDoctorGolden'` exit 0 (`ok .../internal/cli 0.545s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok 1.059s`). |
| R3 | No guard, gate, cap or hold on agents | HOLDS: the new rows only report; the only timeout is 5 s on the `version --json` child process that doctor runs. |
| R4 | Commit rules | HOLDS: one commit, one-sentence message, no attribution lines, no personal paths. |
| R5 | CLI `gridVersionAt` runs the real child | HOLDS by a throwaway test (not committed, removed with `git clean`): a script printing `{"version":"1.2.3"}` for `version --json` gave `1.2.3`; exit 3 and empty output gave errors. |
| R6 | `*RealPlugin*` tests | Not applicable: no test named RealPlugin is part of the rows. |

## What changed

- `internal/doctor/doctor.go`: `Options.GridVersion func(path string) (string, error)`.
- `internal/doctor/installation.go`: "grid on PATH" detail with the version (`grid is on PATH; version <v>`, `...; version not read`, or the failing `grid on PATH (<path>) is version <x>; this grid is <version.Version>`); fixHint for it, for `launcher` and for `worker`; checkInstallation then calls checkLauncher and checkWorker (right after "grid on PATH", before "plugin hooks").
- `internal/doctor/vs_upgrade.go`: `gridVersionAt`, `checkLauncher`, `checkWorker`. A launcher whose symlink cannot be resolved is a failing `launcher` row (`launcher points at a target that cannot be found`); an empty version from the worker reads `running, version not read`.
- `internal/cli/doctor.go`: `gridVersionAt` runs `<path> version --json` (os/exec, no shell, 5 s limit, no network) and decodes `version`.
- `contract/README.md:476`: one sentence after the "Checks, in order:" sentence; the check list is unchanged.
- Five doctor goldens: the `grid on PATH` line.
- New tests `internal/doctor/vs_upgrade_rows_test.go`, `vs_upgrade_rows_unix_test.go` (`//go:build !windows`).

## NOT VERIFIED on a live screen

- `grid doctor` against a real stale grid on PATH, a real launcher symlink, or a worker started by the app: only the fakes above ran. The Windows `bin\grid.exe` branch compiles (`make cross`) but ran only as a build.

## Open items

- C-vr-interrupted-migrate-gate is not in the ref repo, so its `migration` row, README text and goldens are not in this lane. When it lands, `git merge main` here; the conflicts are expected in doctor.go Options, installation.go fixHint, README.md:476 and the five goldens (keep both).
