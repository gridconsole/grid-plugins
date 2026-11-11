# C-fc-contract-windows-locks

Base: 06047047ac70ebf6bba163d448a121c627a363a1. Base grep matched contract/README.md:356 and :739, so the work was needed.

## Proof table

| Row | Result |
|---|---|
| 1. `git grep -n 'not supported until the Windows Feed\|refuses operations requiring locks' contract/` | HOLDS: no output, exit 1 |
| 2. TestFcContractWindowsLocks fails on stale text, passes on new | HOLDS: on base README it fails with `contract/README.md still holds stale phrase "not supported until the Windows Feed is decided"` and `... "refuses operations requiring locks"`, plus two `lacks` lines. On the new README `go test -count=1 -v -run TestFcContractWindowsLocks ./internal/fslock/` prints `--- PASS`, exit 0 |
| 3. `GOOS=windows go vet ./internal/fslock/ ./internal/feed/` | HOLDS: exit 0 |
| 4. `go test -count=1 -run TestFcContractWindowsLocks ./internal/fslock/`; `git diff main --stat` | HOLDS: ok, 0 fail. The diff names only contract/README.md and internal/fslock/fc_contract_locks_test.go (plus this report in docs/lanes) |
| Tests of changed package and importers | HOLDS: `go test -count=1` on fslock and the 24 packages that import it (from `go list`): all ok, exit 0 |
| Lint | HOLDS: `make lint` (go vet ./...) exit 0 |
| Windows and macOS builds | HOLDS: `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| New test fails on base, passes on lane | HOLDS: see row 2 |
| No guard, gate, cap, hold or hook added | HOLDS: only README text and one test |
| No existing test edited or deleted | HOLDS |
| Commits | HOLDS: one commit, one sentence, no attribution lines |

## What changed
- contract/README.md:355: one sentence, flock on Unix and LockFileEx on Windows (`internal/fslock`).
- contract/README.md:737-738: Windows builds and its locks are implemented (LockFileEx), compiled by `make cross`; a run on a real Windows machine is still to do.
- The feed-contract.md citation was dropped: that file is not in this repo (`git ls-files | grep feed-contract` is empty), so it cannot be checked.
- New test internal/fslock/fc_contract_locks_test.go (package fslock_test) reads ../../contract/README.md.

## Deviation from the brief
The brief names the test file `fc_contract_windows_test.go`. Go reads the `_windows_test.go` suffix as a GOOS=windows build constraint, so on Linux the test never ran (`go test` printed `[no tests to run]`; `go list` listed it under IgnoredGoFiles). I named it `fc_contract_locks_test.go` so row 2 and row 4 can run on any platform.

## NOT VERIFIED
- A run on a real Windows machine, which the README text itself still marks as to do.
- The wording was not checked against other lanes' README edits; rebase after C-er-contract-readme-receipts or lane-G-contract-readme-drift lands.
