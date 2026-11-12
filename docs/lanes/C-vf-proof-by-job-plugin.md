# C-vf-proof-by-job-plugin

Base: 2b47f621 (plugin main). Commit identity: Grid lane. No attribution lines.

## Proof table

| Row | Result |
|---|---|
| Base check: GRID_JOB=J-2 via `h.env.Getenv`, no `--by` | HOLDS: on base the record printed `"by":"grid"` (`TestVfByJobWhenNoBy`: `by "grid", want J-2`; `TestVfByFileProof`: `by "grid", want J-3`, both FAIL on base). |
| 1. `go test -count=1 -v -run 'TestVfBy' ./internal/cli/` | HOLDS, exit 0: PASS TestVfByJobWhenNoBy (J-2), TestVfByExplicitWins (alex), TestVfByExplicitGridWins, TestVfByNoJobKeepsGrid (grid), TestVfByFileProof (J-3). Code: internal/cli/proof.go:42-52. |
| 2. `go test -count=1 -run 'TestProof\|TestDoctorGolden\|TestContractCommands' ./internal/cli/` | HOLDS, exit 0. `git diff main --stat -- contract internal/cli/testdata` is empty. |
| 3. `go test -count=1 ./internal/proof/` | HOLDS, exit 0. |
| 4. `make lint` | HOLDS, exit 0 (go vet ./...). |
| 4. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| 4. diff names only proof.go and the new test | HOLDS for code: internal/cli/proof.go and internal/cli/vf_proof_by_job_test.go. This report docs/lanes/C-vf-proof-by-job-plugin.md is also committed because the runbook requires it. |
| Importers of changed package | HOLDS: internal/cli is imported by cmd/grid only (no tests); `make cross` compiles every package's tests. |
| Walk: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0. |
| Walk: `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0. |
| New tests fail on base, pass on lane | HOLDS (see base check). |
| No existing test edited or deleted; no guard/gate/cap | HOLDS. |
| No personal paths, hosts or emails | HOLDS. |

## What changed

`grid proof add` with no `--by` takes the author from GRID_JOB when it is set. An explicit `--by` (including `--by grid`) wins. Flag, default and Usage are unchanged.

## NOT VERIFIED on a live screen

Nothing here is UI. The app Result tab half is a separate brief.
