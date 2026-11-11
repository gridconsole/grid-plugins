# C-pcov6-routine-store-refusals

Test-only lane: one new file, `internal/routine/store_refusals_test.go`. Helpers carry the `rs` prefix (`rsStore`, `rsWant`, `rsNames`; tests `TestRS*`).

## Proof table

| Row | Result |
|---|---|
| 1. Coverage before | HOLDS. `go test -count=1 -coverprofile=.../C-pcov6-routine-store-refusals-before.out ./internal/routine/` exit 0, "coverage: 79.5% of statements". `go tool cover -func=... \| tail -1` prints 78.8% (func-table total, a different rounding of the same profile). Profile written outside the clone. |
| 2. Validate (5 cases) | HOLDS. `TestRSValidateRefusals` (name, machine, every, run, outcome), each asserts the exact error text; `go test -run TestRS -v` exit 0. Blocks store.go:103-104, 106-107, 109-110, 112-113, 117 count 1 in the after profile. |
| 3. decode / List project | HOLDS. `TestRSDecodeRefusals`: `Get("other")` error has prefix `front matter: `; `List()` with Project `bad name` returns exactly `project level needs a valid project name`. Blocks :129.3, :155.3 count 1. |
| 4. Name mismatch | HOLDS. `TestRSNameMismatch`: Get gives `routine name does not match its file`; List gives `routine other: routine name does not match its file`. Blocks :132.3, :168.4 count 1. |
| 5. Remove lock | HOLDS. `TestRSRemoveLockRefusal`: `invalid routine or project name`. Block :176.3 and :230.3 count 1. |
| 6. Add refusals | HOLDS. `TestRSAddRefusals`: name and empty-run errors exact; `List()` still returns only `daily`. Blocks :196.3, :202.3 count 1. |
| 7. Execute lock | HOLDS. `TestRSExecuteLockRefusal`. Block :263.3 count 1. |
| 8. Other machine | HOLDS. `TestRSExecuteOtherMachine`: error exact, result Name `daily`. Block :272.3 count 1. |
| 9. Missing executor | HOLDS. `TestRSExecuteNeedsExecutor`: error exact; Get shows LastRun zero, Outcome empty. Block :290.3 count 1. |
| 10. Disappeared | HOLDS. `TestRSExecuteDisappeared`: error contains `routine disappeared during run`; Get then fails. Block :242.4 count 1. |
| 11. Bad timer state | HOLDS. `TestRSExecuteBadTimerState`: `errors.As` finds `*json.SyntaxError` without and with onFailure; with onFailure the join also holds `rs signal` and onFailure ran once; the runner did not run. Block :86.3 count 1. |
| 12. Coverage after and gates | HOLDS. After: 82.6% by `go test` (82.0% by `go tool cover -func`), before 79.5% (78.8%). One `grep -E` over the after profile shows every block above at count 1 (the only 0 in the range is :199.3, ParseInterval in Add, which no row names). `go test -count=1 ./internal/routine/` exit 0; `make lint` exit 0 (go vet ./...); `GOOS=windows go vet ./internal/routine/` exit 0; `GOOS=darwin go vet ./internal/routine/` exit 0; `gofmt -l internal/routine` empty. |
| Importers' tests | HOLDS. `go list` shows the only importer of internal/routine is `internal/cli`; `go test -count=1 ./internal/cli/` exit 0. |
| New tests fail on base | NOT APPLICABLE as written: this lane tests behaviour that already exists on main, so the tests pass on base by design (they add coverage, not a fix). I did not mutate store.go to show a failing line; the brief records that each trigger was flipped by throwaway tests on a scratch clone. |
| No existing test edited or deleted | HOLDS. `git status` clean; the commit adds one file only. |
| No guard, gate, cap or hold added | HOLDS. No non-test file changed. |
| No Unix-only calls | HOLDS. No chmod, Mkfifo, syscall, or Symlink. OS error text is not asserted; `front matter: ` is asserted by prefix. |
| No approval.json / jobtest Approve | HOLDS. Not used. |
| Commit messages | HOLDS. One sentence, no attribution lines. |

## What changed

Added `internal/routine/store_refusals_test.go` (10 top-level tests). Nothing else.

## Found bugs

None. Every refusal behaved as the brief quoted.

## NOT VERIFIED on a live screen

Nothing here is UI; no live check needed.

## Result

DONE
