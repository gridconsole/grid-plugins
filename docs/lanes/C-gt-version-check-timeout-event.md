# C-gt-version-check-timeout-event

TestCheckTimeout now proves the timeout by the server seeing its request end, with no clock.

| Row | Result |
|---|---|
| 1. `go test -count=1 -run '^TestCheckTimeout$' -v ./internal/version/` | HOLDS, exit 0: `--- PASS: TestCheckTimeout (0.02s)`, `PASS`. `-count=20`: `ok  github.com/gridconsole/grid-plugins/internal/version 0.438s` |
| 2. `git grep -n -E 'time\.(Since\|Sleep)' -- internal/version/check_test.go` | HOLDS: no output. The only clock left is the 30 s `time.After` failure bound, which says "request never ended". |
| 3. Break: check.go:80 `context.WithCancel` instead of `WithTimeout` (plus `_ = timeout` so it compiles) | HOLDS: `go test -timeout 60s ...` exit 1, `panic: test timed out after 1m0s`, `running tests: TestCheckTimeout (1m0s)`. Restored with `git checkout -- internal/version/check.go`; `git diff main --stat` shows only check_test.go. |
| 4. `go test -count=1 ./internal/version/` | HOLDS: `ok`, exit 0 |
| 4. `git diff main --stat` | HOLDS: `internal/version/check_test.go \| 24 ++++++++++++++++++++----` |
| 4. `gofmt -l internal/version` | HOLDS: no output |
| 4. `go vet ./internal/version/` | HOLDS: exit 0 |
| Importers of version (go list): doctor, migrate, packaging/cmd, cli | HOLDS: `go test -count=1 ./internal/version/ ./internal/doctor/ ./internal/migrate/ ./internal/packaging/... ./internal/cli/` all `ok` |
| Windows and macOS builds | HOLDS: `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` both exit 0, no output |
| Lint | Covered by gofmt and go vet above. I did not run a separate lint target. |
| New test fails on base | Not applicable. This fixes a load-flaky test and the same code passes on both. The break row shows the test still fails when the pinned behaviour is broken. |
| No guard, gate or cap added; no test deleted, renamed or skipped | HOLDS |
| `*RealPlugin*` tests | None involved |

## What changed
`internal/version/check_test.go`, TestCheckTimeout only:
- The handler closes `entered` when called and `gone` when `r.Context().Done()` fires.
- Check returning with Status "unavailable" is the proof, because the handler never answers.
- Only if `entered` is closed does the test wait on `gone`, with a 30 s failure bound.
- The elapsed-time assertion is dropped.

## NOT VERIFIED on a live screen
Nothing here touches the UI.

DONE
