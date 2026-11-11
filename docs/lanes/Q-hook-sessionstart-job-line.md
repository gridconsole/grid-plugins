# Q-hook-sessionstart-job-line

Head: see `git log -1` on lane-Q-hook-sessionstart-job-line (code commit 9b658351). Base main: 330256ced425dd8bb87d68737039b9ab74f246f4.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Test fails on main through the SessionStart entry point | HOLDS | With hook.go reset to main (jobContext stubbed empty in a throwaway shim), `go test -count=1 -run 'SessionStartJob\|SessionStartInvalidJob\|SessionStartWithoutJob' ./internal/hook/` exit 1: `--- FAIL: TestSessionStartJobLines` / `sessionstart_job_test.go:62: job lines missing or out of order:` (the context has no `J-2`). Also FAIL: AttemptOmitted, NoBriefFile, SurviveTheBound ("no job lines"), CommandsExistInContract. Main sha 330256ced425dd8bb87d68737039b9ab74f246f4. The shim was removed after the run. |
| 2. Three lines in order, task id from `where`, attempt from GRID_ATTEMPT | HOLDS | `TestSessionStartJobLines`, `TestSessionStartJobAttemptOmitted` (empty, `x`, `0`, `-1`, `1.5` drop `(attempt N)`); `go test -count=1 ./internal/hook/` exit 0. Code: internal/hook/hook.go `jobContext`. |
| 3. No brief file: second line is `grid job show <T> <J>`; invalid GRID_JOB adds nothing, logs nothing | HOLDS | `TestSessionStartJobNoBriefFile`; `TestSessionStartInvalidJobAddsNothing` (`../x`, `J-0`, `J-00`, `J-0.1`, empty, `J-`, `j-2`, a path and a newline injection): context byte-equal to the no-GRID_JOB context. `jobContext` has no log call. Note: `job.ValidID` accepts `J-0`, so `jobContext` also rejects an all-zero first number. |
| 4. No GRID_JOB: exactly main's context | HOLDS | `TestSessionStartWithoutJobIsUnchanged` compares the output with the six literal lines of main's `sessionContext` plus the brain index text, byte for byte. |
| 5. Long context: Job lines present, `Context truncated.` after them | HOLDS, with a caveat | `TestSessionStartJobLinesSurviveTheBound`: a 600-row index through `assembleContext` (the real join and bound used by `sessionStart`) keeps the Job lines, then `Brain:`, then `Context truncated.` last. Caveat: the real brain index is capped at 1,200 bytes (internal/brain/index.go:166), so the 9,900 bound can only be filled end to end by a long Grid home path, which breaks on macOS PATH_MAX. I therefore extracted `assembleContext` and tested it directly instead of the full Run. |
| 6. Commands exist with those flags in the contract | HOLDS | `TestSessionStartJobCommandsExistInContract` reads `contractdata.Commands`; `grid job show` and `grid job result` exist and `--task --job --body` are flags of `result`. |
| 7. `go test -count=1 ./internal/hook/`, `go vet ./internal/hook/` | HOLDS | test `ok` exit 0; vet exit 0; `gofmt -l internal/hook/` empty. |
| Importers' tests | HOLDS | `go list` importers of internal/hook: internal/cli, internal/packaging. `go test -count=1 ./internal/cli/ ./internal/packaging/` both `ok`. |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests in a new file, no existing test edited | HOLDS | internal/hook/sessionstart_job_test.go is new; the only other change is internal/hook/hook.go. |
| No guard, gate, cap or hold on agents | HOLDS | The change adds context text only. |
| Commits: one sentence, no attribution lines | HOLDS | `git log -1`. |
| Whole-module test and cross-repo set | NOT RUN | Left to the orchestrator at landing, as the brief says. |

## What changed

- internal/hook/hook.go: `jobContext` builds the three lines when GRID_JOB is valid and a task resolved. `assembleContext` joins session text, Job lines and brain index, then bounds the whole, so only the index can be cut. `sessionStart` uses both. No other output changes.
- internal/hook/sessionstart_job_test.go: the tests listed above.

## NOT VERIFIED on a live screen

- A real worker-started Job agent's first turn (needs `claude`, not run here).
- The 9,900-rune bound end to end with real files (see row 5).
