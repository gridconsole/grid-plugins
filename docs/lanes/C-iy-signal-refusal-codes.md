# C-iy-signal-refusal-codes

Base: 31824184 (origin/main at build time; lane-C-ia-write-role-exit 6feacb3a is an ancestor: `git merge-base --is-ancestor 6feacb3a HEAD` exit 0). `git fetch origin && git merge origin/main` before the last test run: "Already up to date". `CodeGuestRefused` / `guest_refused` was absent on that main (grep over internal and contract found none), so this lane declares it exactly as il does, with il's row at the end of the table.

Base check, `git grep -n 'RefusedError{' HEAD -- internal ':!*_test.go'` on 31824184: the sixteen literals of the brief and no others (cli/signal.go:67, :139, :157; signal/act.go:98, :119, :127, :129, :133, :137; signal/move.go:40, :69, :75, :78, :81, :90, :146). None had a Code.

## Proof table

| Row | State | Proof |
| --- | --- | --- |
| 1 TestIySigStore | HOLDS | `go test ./internal/signal -count=1 -run 'TestAct\|TestMove\|TestApply\|TestHkSig\|TestIySig' -v` exit 0; 18 subtests PASS: guest-apply, guest-move (guest_refused); apply-missing-note, move-missing-note (no_such_note); add-unknown-task (no_such_task); move-decided and the eight act cases of act_lane_test.go:141-148 (note_state); move-same-project, move-id-in-target (note_exists); move-notes-full, 4000 files (notes_full); move-unknown-project, move.go:146 (no_such_project). Each also checks the message is unchanged. |
| 2 TestIySigCLI | HOLDS | `go test ./internal/cli -count=1 -run '...\|TestIySig' -v` exit 0: guest, wake-new, note-done, move-same-project PASS; each exits 3 with and without `--json`, stderr bytes equal, plain stdout empty, JSON stdout keys exactly code/error/message, error "refused", message as today, code as the brief. |
| 3 TestIySigWalk | HOLDS | PASS: `source` walks every non-test .go file under internal/signal and internal/cli with go/parser, reports by file:line any `RefusedError`/`signal.RefusedError` literal with no Code or Code ""; fails if it sees none; holds no list. `reports` feeds a source string and asserts exactly `iy.go:8:11` (no Code) and `iy.go:10:11` (Code "") are reported, the coded ones are not. |
| 4 TestIySigConstants | HOLDS | PASS: seven signal constants equal their cli twins; guest_refused, note_state, note_exists, notes_full each found once in the README table (via hfFsDocCodes). TestHfFsCodeTable and TestHfFsKnownOpen PASS with hf_failure_shapes_test.go:288-289 set to `false`. |
| 5 guest parity and named runs | HOLDS | guest_parity_test.go:432 expects `"code":"guest_refused"`; the cli run of row 2 (TestGuestProjectParity, TestStableErrorCodes, TestErrorCodeIgnoresWording, TestSignal*, TestHfFs*, TestIySig*) exit 0 and the signal run of row 1 exit 0. act_lane_test.go, refusals_test.go, hk_signal_add_test.go untouched (not in `git diff origin/main --stat`). |
| 6 lint, cross, diff | HOLDS | `make lint >/dev/null 2>&1 && echo` -> "make lint exit 0"; `make cross >/dev/null 2>&1 && echo` -> "make cross exit 0". `git diff origin/main --stat` before this report: contract/README.md, internal/cli/{error_codes.go, guest_parity_test.go, hf_failure_shapes_test.go, iy_signal_refusal_codes_test.go, signal.go}, internal/signal/{act.go, iy_refusal_codes_test.go, move.go} — all in Touches. The only other file is this report, docs/lanes/C-iy-signal-refusal-codes.md, which the runbook requires (same convention as every lane in docs/lanes). |
| New tests fail on base | HOLDS | production files stashed to base (`git stash push -- act.go move.go signal.go error_codes.go README.md`), `go test ./internal/signal ./internal/cli -run TestIySig`: `internal/signal/iy_refusal_codes_test.go:42:51: undefined: CodeGuestRefused` ... `FAIL github.com/gridconsole/grid-plugins/internal/signal [build failed]`, and `# github.com/gridconsole/grid-plugins/internal/cli [...cli.test]` build failure (signal.Code* and cli Code* undefined). Stash popped; tree clean before commit. |
| Existing tests edited only where the brief says | HOLDS | guest_parity_test.go:432 and hf_failure_shapes_test.go:288-289 only (diff stat 2 and 4 lines). No test deleted. |
| Packages changed and importers tested | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... \| grep -E 'internal/(signal\|cli)( \|$)'` -> cmd/grid, internal/cli, internal/signal, lib. `go test -count=1 ./internal/signal ./internal/cli ./cmd/grid ./lib`: ok signal 1.9s, ok cli 107.5s, cmd/grid no test files, ok lib 127.3s (exit 0). |
| No guard, gate, cap or hold | HOLDS | Only a field and codes added; the 4000-file refusal keeps its number and message (move.go:89-90). |
| Messages, exit 3, stderr unchanged | HOLDS | TestIySigCLI compares stderr with and without --json and checks messages; TestSignalActErrorOutput, TestSignalMoveRefusalsCLI and TestGuestProjectParity (stderr) pass unchanged. |
| Commits small, no attribution, no personal paths | HOLDS | `git log origin/main..HEAD`: three one-sentence commits, no trailers. |
| Tests: temp home, no clock, no sleep, no skip | HOLDS | fixtures use t.TempDir via fixture/moveFixture/actFixture; no time, sleep or t.Skip in the new files. |
| RealPlugin | n/a | No row names a *RealPlugin* test. |

## What changed

- internal/signal/act.go: `RefusedError` gains `Code string`; seven exported constants `CodeGuestRefused`, `CodeNoteState`, `CodeNoteExists`, `CodeNotesFull`, `CodeNoSuchNote`, `CodeNoSuchTask`, `CodeNoSuchProject`; the six act literals carry a Code.
- internal/signal/move.go: the seven move literals carry a Code.
- internal/cli/signal.go: :67 `CodeGuestRefused`; :139 and :157 `CodeNoSuchProject`.
- internal/cli/error_codes.go: `CodeNoteState`, `CodeNoteExists`, `CodeNotesFull`, `CodeGuestRefused` after `CodeWorkflowFileChanged` (string literals, so TestHfFsCodeTable sees them); errorCode keeps its cases first, then returns `RefusedError.Code` when non-empty.
- contract/README.md: shape 2 sentence says `refused` always carries a code and names them; rows note_state, note_exists, notes_full after no_such_note and il's guest_refused row last; "The first six constants" -> "The first ten constants".

## Open items

- Shared lines: C-il-guest-refusal-code also adds `CodeGuestRefused` and the same README row; whichever lands second drops its copy (this lane used il's exact constant and row text). C-ic-job-post-exists and C-if-brain-partial-code also touch error_codes.go and the table.
- The cli fail-on-base output was cut after its package header by `head`; the signal package lines are pasted in full above.

## NOT VERIFIED

Nothing: no UI in this lane.

DONE
