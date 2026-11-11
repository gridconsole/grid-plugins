# A-tests-signal-act: proof

Change: one new file, `internal/signal/act_lane_test.go`. No code change, no edit to an existing test, nothing deleted.

## Proof table

| Row / rule | Status | Proof |
|---|---|---|
| 1. Validate table, each rule gives `*BadInputError` with the text at its line; `errors.As` finds it | HOLDS | `TestActValidate` act_lane_test.go:17 (cases quote act.go:48, :53, :56, :59, :62, :65, :73 in line comments; `errors.As` at :45). `go test ./internal/signal/ -count=1` exit 0 |
| 1. Valid snooze with `2026-11-12T09:30` and with RFC3339 validate; stored as local minute | HOLDS | `TestActValidateAccepts` (local, `Z`, `+02:00` forms; stored value compared to `.In(time.Local)`), `TestActSnoozeStoresLocalMinute` |
| 2. Refusals: guest | HOLDS | `TestActRefusals/guest` (act.go:95; `where.Caller{Role: "guest"}`), file byte-identical |
| 2. Missing note: `errors.Is(ErrNoNote)` and `errors.As *RefusedError` | HOLDS | `TestActRefusals/missing-note` (act.go:116) |
| 2. wake on new, reopen on new, snooze on dismissed ("already decided (dismissed)") | HOLDS | `TestActRefusals/wake-new`, `reopen-new`, `snooze-dismissed` (act.go:124, :126, :130); each compares note bytes before and after. Extra: reopen-snoozed, wake-dismissed, note-dismissed, dismiss-kept, add-kept, bad-input-leaves-note |
| 3. snooze, wake, dismiss, reopen on one note, file read after each; note twice gives "unchanged" | HOLDS | `TestActTransitions` (snoozed+until, new+until cleared, dismissed, new+became empty; second `note` is `unchanged` and the file is not rewritten; add then reopen clears became) |
| 4. add --task T-7: done, became T-7, no tasks/ folder | HOLDS | `TestActAddLinksTaskWithoutCreatingOne` (`os.Stat` of tasks/ is NotExist) |
| 5. `go test ./internal/signal/ -count=1` 0 fail | HOLDS | `ok ... 0.444s`, exit 0 |
| 5. cover figure | HOLDS | `coverage: 82.3% of statements` (was 76.2% on main) |
| 5. `go tool cover -func`: BadInputError.Error, RefusedError.Unwrap, Validate, normalized 100%; Apply > 80% | HOLDS | act.go:19 Error 100.0%, :28 Unwrap 100.0%, :45 normalized 100.0%, :81 Validate 100.0%, :93 Apply 81.5%. act.go:27 RefusedError.Error 100.0% |
| 5. GOOS=windows / darwin `go test -c -o /dev/null ./internal/signal/` | HOLDS | both exit 0 |
| 5. `make lint` | HOLDS | prints `go vet ./...`, exit 0 |
| Packages importing signal | HOLDS | `go list` imports: only `internal/cli`. `go test ./internal/cli/ -count=1`: ok 33.3s, exit 0 |
| New tests fail on base, pass on lane | NOT APPLICABLE | The lane adds tests for unchanged code, so they pass on base too. The base gap is the coverage: Validate, BadInputError.Error and RefusedError.Unwrap were at 0% and `normalized` at 50% on main; they are now at 100%. No failing lines to paste. |
| No existing test edited or deleted | HOLDS | `git diff origin/main --stat`: one new file plus this report |
| No guard, gate, cap, hold on agents | HOLDS | test-only change |
| CRLF refusal not pinned | HOLDS | no CRLF test written |
| No attribution lines in commits | HOLDS | commit message is one sentence |
| Windows host run | Not a row | for the orchestrator: `GOOS=windows go test -c -o signal.test.exe ./internal/signal/` and run it on the host |
| macOS host run | Not a row | for the orchestrator: `go test ./internal/signal/ -count=1` on macOS |

Ten Apply statements stay uncovered, as the brief says (lock, store, encoder and task-creation errors, plus the turn path).

## What changed

New `internal/signal/act_lane_test.go`: `TestActValidate` (17 cases), `TestActValidateAccepts`, `TestActRefusals` (guest, missing note, 8 state refusals, bad input leaves the note), `TestActTransitions`, `TestActSnoozeStoresLocalMinute`, `TestActAddLinksTaskWithoutCreatingOne`. It uses the `fixture` helper of signal_test.go and `Store.Add`.

No bug found in act.go.

## NOT VERIFIED on a live screen

Nothing here is UI. The Windows and macOS runs are compiled only.

DONE
