# P-hookevents-keep-3h-2

Why: plugin check 2026-11-16T0250Z issue 1. This lane carries P-hookevents-keep-3h (every row HOLDS in its review) and adds what that review asked for: the dependent test no longer writes the limit out, the contract states the new limit, and the bound test measures exactly. `git merge origin/main`: already up to date.

## What changed

- `internal/hook/ruleruns_test.go:170`: `const limit = hookevents.Limit` (import added at line 12). A later Limit change no longer breaks `TestRuleRunsLogIsBounded`. The package already imports `hookevents` in production code, so there is no new dependency.
- `contract/README.md:667`: the log shifts at 2 MiB, at most three files of 2 MiB stay (6 MiB, about 60,000 lines, over 3 hours at 250 lines a minute).
- `internal/hookevents/hookevents_test.go` `TestFilesStayWithinFilesTimesLimitPlusOneLine`: the folder is read after every append (was every 97th), and the bound uses the measured longest line (the last line written, since the numbers rise) instead of an assumed 120 bytes.
- Note on the bound: `logshift.Shift` shifts at the next append once a file holds Limit bytes or more, so each file stops less than one line past Limit. The true bound is therefore Files x (Limit + one line), not Files x Limit + one line. The first run of the every-append check against the brief's wording failed with `the files reached 6291656 bytes together, more than Files x Limit plus one line = 6291576` (200 bytes, about 2 lines over): three files can each be one line past Limit at once. The test asserts the exact bound: no file at or above Limit + longest line, the total under Files x (Limit + longest line), and the total at least (Files-1) x Limit so the test did fill the log. No production code changed.

## Proof table

| Row | Result |
|---|---|
| 1. ruleruns_test.go uses `hookevents.Limit` | HOLDS. `ruleruns_test.go:170`. `go test -count=1 ./internal/hook/` exit 0 (`ok internal/hook 16.397s`). |
| 2. contract line states the new limit | HOLDS. `contract/README.md:667`. `go test ./contract/... ./internal/contract/` exit 0. |
| 3. A3 test checks the size after every append | HOLDS. `hookevents_test.go:116-141` (the closure runs after each `Append`). It found the true bound (see Note above). |
| `go test ./internal/hookevents/ ./internal/logshift/ ./internal/hook/` | HOLDS, exit 0: `ok internal/hookevents 13.881s`, `? internal/logshift [no test files]`, `ok internal/hook 8.525s`. |
| `grep -rn '256 << 10' internal/ contract/` | HOLDS: no output, exit 1 (no match). A wider grep for `256 KiB` finds only `hookevents.go:28` (comment naming the old figure as history) and `docs/lanes/P-hook-events-read.md`, `P-hookevents-keep-3h.md` (history). |
| `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| Windows build | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS: `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Importers' tests (`go list`: `hookevents` is imported by `internal/hook` and `internal/cli`; through them `internal/packaging`, `internal/packaging/cmd`, `cmd/grid`, `lib`) | HOLDS, exit 0: `go test -count=1 ./internal/hookevents/ ./internal/hook/ ./internal/cli/ ./internal/packaging/... ./cmd/grid/ ./lib/ ./contract/... ./internal/contract/`: hookevents 22.5s, hook 16.4s, cli 115.9s, packaging 10.4s, packaging/cmd 0.6s, cmd/grid no test files, lib 30.0s, contract 0.3s, formats 0.5s, internal/contract 2.4s. |
| Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok 0.121s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.228s`). |
| New tests fail on base | The changed A3 test is a tightening of an existing test, not a new behaviour: it passes on base `Limit` for the right reason (the bound is a property of `logshift`). Its earlier, looser form (every 97th append, assumed 120-byte line) is what passed before; the brief's wording of the bound fails on the exact check (line above). Not run on base: no production change in this lane. |
| Mismatch proof | not run, not asked |
| `*RealPlugin*` tests, webui | not run, not asked (no route, UI or plugin binary change) |
| No guard, gate, cap, hold or blocking hook | HOLDS: only two test files and one contract line changed. |
| Existing test edited to pass | `TestRuleRunsLogIsBounded` changed one line (the constant), as the brief row 1 directs; same shift, dropped-oldest and fresh-file checks. `TestFilesStay...` was tightened, not weakened. |
| No attribution lines, no personal paths | HOLDS. |

## NOT VERIFIED on a live screen

Nothing here is screen-visible.

## Open items

- The brief's phrase "Files x Limit plus one line" is slightly loose: the real bound is Files x (Limit + one line), about 200 bytes more on 6 MiB. The contract line says "6 MiB, about 60,000 lines", which holds either way.
- The last `git fetch origin && git merge origin/main` was done at the start; main did not move.

DONE
