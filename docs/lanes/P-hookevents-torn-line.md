# P-hookevents-torn-line: a torn event line costs only the torn record

Base: bacb3f76e40836129a3a4c299887cc8653c569d6 (main; `git fetch origin && git merge origin/main` said "Already up to date" at the start and again before the last run).

## What changed

- `internal/hookevents/hookevents.go`: `Append` opens the log `O_CREATE|O_APPEND|O_RDWR` (was `O_WRONLY`), and `endsTorn` reads the last byte. When the file is not empty and that byte is not `'\n'`, a `'\n'` is put in front of `buf`, so it is one `Write`. A file it cannot stat or read counts as not torn. No lock, no retry loop; `Append` stays best-effort and silent (the `recover` and the ignored write error are unchanged).
- `internal/hookevents/hookevents_test.go`: three new tests, no existing test edited.

## Proof table

| Row | Result |
|---|---|
| 1. Torn log: after one Append, Read returns the record and skips only the torn line; fails on base | HOLDS. `TestAppendAfterTornLineLosesOnlyTheTornRecord`. On base bacb3f76 (source change stashed): `--- FAIL: TestAppendAfterTornLineLosesOnlyTheTornRecord` / `hookevents_test.go:196: Read returned rules [first], want after,first`, exit 1. On the lane it passes. |
| 2. Log ending in '\n' gets no blank line | HOLDS. `TestAppendAfterNewlineAddsNoBlankLine` passes (it also passes on base: it guards the new code against a regression). |
| 3. Empty or missing log gets no leading newline | HOLDS. `TestAppendToEmptyOrMissingLogAddsNoLeadingNewline` (both cases) passes (also passes on base, same reason). |
| 4. Existing hookevents and logshift tests pass with `-race`; no test edited | HOLDS. `go test -race -count=1 ./internal/hookevents/... ./internal/logshift/...` exit 0: `ok .../hookevents 113.290s`, `ok .../logshift 2.187s`. `git diff` on the test file only adds lines. |
| 5. No new lock, no retry loop; best-effort and silent | HOLDS. hookevents.go `Append` and `endsTorn`: one stat, one `ReadAt` of one byte, one `Write`; errors return false or are ignored. |
| 6. This doc; last line is DONE | HOLDS. |
| Tests of packages that import hookevents (`go list`: internal/cli, internal/hook, internal/job/jobcmd, and through them internal/packaging, internal/packaging/cmd, cmd/grid, lib) | HOLDS. `go test -count=1 ./internal/hook/... ./internal/job/jobcmd/... ./internal/packaging/... ./cmd/grid/... ./lib/...` exit 0 (hook 32.4s, jobcmd 24.2s, packaging 25.0s, packaging/cmd 1.1s, lib 41.8s; cmd/grid has no test files). `go test -count=1 ./internal/cli/...` exit 0 (324.2s). |
| Lint | HOLDS. `make lint` exit 0 (`go vet ./...`). |
| Windows build | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS. `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| Merge origin/main before the last proof run | HOLDS. `git fetch origin && git merge origin/main`: Already up to date. |
| App-repo checks (`make webui-test`, realplugin guest walks), `*RealPlugin*` tests | not run, not asked (plugin repo; the brief names none) |
| Whole-module test, cross-repo set | not run, the orchestrator runs them at landing |

## Not verified on a live screen

Nothing here has a screen. A real full-disk cut was not reproduced; the torn tail is seeded by the test.

## Limits

Two Appends racing on a torn tail can each add a `'\n'`, which leaves at most one blank line; Read skips blank lines. A record written between the last-byte read and the write is still appended whole by `O_APPEND`.

DONE
