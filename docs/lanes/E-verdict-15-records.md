# E-verdict-15-records: record hooks never block

Verdict v-15 (11 Nov): hooks that only record never block or deny, even when the write fails.

## Row 1: record components, the function each calls, and whether it can return non-zero or deny (base, before this lane)

Dispatch is `remaining` (internal/hook/events.go:83, a `func(Options) int`) and `quiet(sessionStart)` (events.go:37). Components map: events.go:72.

| Component / event | Function | file:line | Non-zero code or deny on base? |
|---|---|---|---|
| SessionStart | sessionStart | hook.go:55 | No. Returns nothing; `quiet` returns 0 (events.go:68). Errors only to `o.log`. |
| heartbeat | heartbeat | events.go:232 | No. Returns nothing; error to `o.log`. |
| worker-exit (and SessionEnd, SubagentStop tail) | workerExit | events.go:261 | No. Returns nothing; error to `o.log`. |
| history-log | history, bashHistory | history.go:20, :80 | No. Returns nothing; error to `o.log`. |
| usage-log | recordSession | events.go:186 | No. Returns nothing; error to `o.log`. |
| brain-read | brainRead | brain.go:29 | No. Returns nothing; error to `o.log`. |
| brain-recall | recall | brain.go:121 | No code. Writes only `additionalContext`, never a decision. |
| home-allow counter | CountHomeAllow | count.go:14 | No. Returns nothing; every error is a plain `return`. |
| lib-* helpers | helper | events.go:364 | No. Diagnostics only. |
| stop-check | stopCheck | stop.go:109 | YES, returns a code (events.go:117, :132). It is a guard, not a record; out of scope by the brief ("not stop.go"). |
| permission-forward | toolUse | hook.go:223 | YES, can deny. It is a guard; out of scope ("not hook.go toolUse"). |

One path outside "returns a code": a Go panic in any record function exits the process with code 2, and Claude Code reads exit 2 as a block. No function panicked in any input I tried (row 3), but nothing stopped one. Fixed in this lane.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. List of record components | HOLDS | Table above, with file:line. |
| 2. Each record function returns 0 and no decision on any error | HOLDS | Base already did, for every error I could provoke (unwritable folder, bad JSON, missing task). Hardened: every record call now runs inside `Options.record` (events.go:146), which turns a panic into a log line; `CountHomeAllow` has the same recover (count.go:16). Calls: events.go:37, :105-:115, :125-:129, :136. |
| 3. One table-driven test: unwritable home and malformed input, exit 0, empty decision | HOLDS | `TestRecordEventsNeverBlockOrDeny` in internal/hook/records_test.go: 3 homes (under a file, read-only, missing) x 7 inputs (well formed, not JSON, null, array, empty, wrong types, truncated) x 13 events (SessionStart, UserPromptSubmit, SubagentStart, PostToolUse, PreCompact, SessionEnd, PostToolUseFailure, and the six record components) = 273 cases. `go test ./internal/hook/ -run 'TestRecord\|TestCountHomeAllow'` exit 0. |
| New tests fail on base | PARTLY | On base `TestRecordPanicIsLoggedNotRaised` fails: `internal/hook/records_test.go:95:4: o.record undefined (type Options has no field or method record)` then `FAIL ... [build failed]`. The table test passes on base logic: the behaviour it holds was already true. I did not weaken it to force a failure. |
| 4. go test ./internal/hook/... and make lint | HOLDS | Package list that imports hook (go list): internal/cli, internal/hook, internal/packaging. `go test ./internal/hook/... ./internal/cli/... ./internal/packaging/...` exit 0: hook ok 2.975s, cli ok 195.662s, packaging ok 0.899s. `make lint` (go vet ./...) exit 0. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| No guard, gate, cap, hold or blocking hook added | HOLDS | The diff only adds `recover` that lets a hook carry on; no new deny, code or limit. |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat` shows one new test file and two changed source files. |
| Only allowed files touched | HOLDS | events.go, count.go, a new test. history.go needed no change: it has no non-zero or deny path. |

## What changed

- internal/hook/events.go: `Options.record` runs a record function and logs a panic; `remaining` and the SessionStart table row use it.
- internal/hook/count.go: `CountHomeAllow` recovers from a panic.
- internal/hook/records_test.go: the table test, a panic test, and a counter test.

## Open items

- Stop, SubagentStop (`stopCheck`) and PreToolUse/PermissionRequest (`toolUse`) are guards and can return 2 or deny by design. Untouched, as the brief says.
- `recall` and `history` called from `toolUse` (hook.go:256-257) are not wrapped, because hook.go is out of scope. A panic there would exit 2.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a live Claude Code session with a real unwritable Grid home.
