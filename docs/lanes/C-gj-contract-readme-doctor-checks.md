# C-gj-contract-readme-doctor-checks

Base: b8324c53a776fefe1faadb5b7094d3a4e6f7598a. The README line on base named 7 checks (no deny block, no sandbox), so the lane applies.

| Row | Result |
| --- | --- |
| 1. README names the 9 checks in doctor.go order, says when deny block is skipped, gives the 3 sandbox forms | HOLDS: contract/README.md:396. `git diff main -- contract/README.md \| grep -c refusal_message` prints 0 |
| 2. TestGjReadmeDoctorChecksMatch compares README names with a doctor run; sub-test on an edited copy fails when a name is removed | HOLDS: `go test -count=1 -v -run TestGjReadmeDoctorChecksMatch ./internal/contract/` exit 0, both PASS. On base README (README change stashed): `FAIL ... README names ["grid home" ... "allow block"], doctor reports [... "allow block" "deny block" "sandbox"]`, exit 1 |
| 3. Home rule: HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, GRID_HOME under t.TempDir() via t.Setenv; t.Fatalf when HOME not under root; no sleep | HOLDS: gj_readme_doctor_checks_test.go:48-58 |
| 4. `go test -count=1 ./internal/contract/ ./internal/doctor/` | HOLDS: exit 0 |
| 4. `go test -count=1 -run TestGbDocs ./internal/cli/` | HOLDS: exit 0 |
| 4. `make lint` | HOLDS: exit 0 |
| 4. diff names only the two Touches files | HOLDS: plus this lane note under docs/lanes/ |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0 |
| Importers' tests | No production code changed. The only new code is a test file in package contract_test; cli is the only importer of doctor and TestGbDocs ran. The whole-module run is the orchestrator's. |
| New test fails on base, passes on lane | HOLDS (row 2) |
| No existing test edited or deleted; no guard added | HOLDS |
| RealPlugin tests | none involved |

## What changed
- contract/README.md `### grid doctor --json`: the Checks line names `deny block` and `sandbox`; the skip text covers both allow and deny block; the deny block `not checked:` / `missing rules:` forms and the three sandbox details are given.
- New internal/contract/gj_readme_doctor_checks_test.go. It is an external test package (contract_test) because doctor imports settings, which imports contract; an in-package test would be an import cycle.

## NOT VERIFIED on a live screen
Nothing here is a UI row.
