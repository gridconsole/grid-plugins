# M-C-jz-inprocess-reasons-onto-main: lane C-jz-inprocess-reasons merged onto plugin main a7809a58

## What changed

`git merge origin/main` into the lane conflicted in one file, `lib/lib.go`, in the
"What a call reads" list (the bullet about what stays with the host).

- Lane side: replaced the user-name and account bullets with one bullet naming the
  provider child of `account list`, `status` and `scan` (`internal/cli/account.go:30`,
  `internal/account/account.go:68`, `:216`), and kept the migrate read as a clause of it.
- Main side (3f4f1593, d47663dd): the migrate plan no longer reads the host. The flag
  wins, else `CLAUDE_PLUGIN_DATA` of the Call's environment (`internal/cli/migrate.go:65-72`);
  the package doc says so at `lib/lib.go:19-21`. Main also kept "the user name ... names
  a new task's owner" and "internal/account has its own fallback".
- Resolution: the lane's account bullet, without the migrate clause, because the clause
  is false on main (`internal/migrate/plan.go` and `internal/cli/migrate.go` hold no
  `os.Getenv`). The user-name bullet stays dropped: `internal/task/create.go:192-201`
  (`ownerFromEnv`) reads the Call's USER, then USERNAME, when Getenv is set. The
  "own fallback" bullet is covered by the account bullet (`internal/account/account.go:64-68`
  is the Environ fallback; the CLI gives it Getenv, `account.go:30`). Nothing of main's
  other hunks in `lib/lib.go` (migrate paragraph, Feed member paragraph) was touched.
- Auto-merged without conflict: main's changes to `internal/cli/migrate.go`, `question.go`
  and the tests; `internal/cli/inprocess.go` and `hs_inprocess_test.go` have no main change.
- No test, row or error code of either side was dropped or edited.

Head: see report (merge commit on top of the lane head 32caf89d and origin/main a7809a58).

## Proof table

| Row | Result |
|---|---|
| 1. `git merge-base --is-ancestor origin/main HEAD` | HOLDS, exit 0. `git diff origin/main --stat` names `docs/lanes/C-jz-inprocess-reasons.md`, `internal/cli/hs_inprocess_test.go`, `internal/cli/inprocess.go`, `lib/inprocess.go`, `lib/lib.go`: the lane's own files (plus this lane doc). |
| 2a. `go test -count=1 -v -run 'TestHsIpWalksEveryCommand\|TestHsIpSafe\|TestHsIpBinaryOnly\|TestHsIpFailClosed' ./internal/cli/` | HOLDS, exit 0: `--- PASS: TestHsIpWalksEveryCommand` (walked 39 commands, 39 table entries), `--- PASS: TestHsIpBinaryOnly`, `--- PASS: TestHsIpSafe`, `--- PASS: TestHsIpFailClosed`. |
| 2b. `go test -count=1 ./lib/` | HOLDS, exit 0 (`ok  .../lib  4.783s`). |
| 2c. `git grep -n 'host-environment' HEAD -- internal/cli/inprocess.go` | HOLDS: one hit, `inprocess.go:146` for `account sign-in`, a verb that stays binary-only (the provider child takes os.Environ, `account/account.go:216`). No flipped verb. |
| 2d. cited lines in `internal/cli/inprocess.go` exist on HEAD | HOLDS. Main changed none of the cited files (`git diff HEAD^1 HEAD` touches only `migrate.go` and `question.go` among internal/cli sources; neither is cited), so the lane's citations are the ones its own report checked. `grep -noE '[a-z_/]+\.go:[0-9]+'` lists them at inprocess.go lines 35-223. |
| 2e. `git diff origin/main -- lib/lib.go lib/inprocess.go` | HOLDS: only the "What a call reads" bullets and the `lib/inprocess.go` doc sentence differ. The Context comment is not in the diff. Cited in the list: `internal/cli/account.go:30`, `internal/account/account.go:68`, `:216`, `internal/cli/task_new_first_job.go:84`, `internal/env/env.go:117`, `internal/cli/task.go:241`, `internal/cli/task_lease.go:85`; all read from the files on HEAD (account.go:30, account/account.go:68 and :216 confirmed in this run). |
| 2f. `make golden`, then `git status --short golden/` | HOLDS: golden test run, `git status --short golden/` printed nothing. |
| 3. Walk tests | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.033s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.073s`). The app-only walk tests do not apply to this repo. |
| 4. `make lint` | HOLDS, exit 0 (`go vet ./...`, no output). `make cross` HOLDS, exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok`. |
| Tests of changed packages and importers | HOLDS: changed `internal/cli`, `lib`; importers by `go list`: `internal/cli` is imported by `cmd/grid` and `lib`; `lib` by none. `go test -count=1 ./internal/cli/ ./lib/ ./cmd/grid/` exit 0 (`ok internal/cli 91.5s`, `ok lib 24.2s`, `cmd/grid` no test files). |
| New tests fail on base, pass on lane | Not applicable: the merge adds no test. |
| Rules: no guard, no personal data, no attribution lines, nothing removed | HOLDS: the merge commit message has no attribution line; no guard added. |

## NOT VERIFIED on a live screen

Nothing here is UI. The cross builds were compiled and vetted, not run on Windows or macOS.

Whole-module test and the cross-repo set are run by the orchestrator at landing.

Report ends: DONE
