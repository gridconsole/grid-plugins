# Lane C-kk-note-then-environ: `account note --then` runs the person's command with account.Env's environment

Repo: plugin. Base: `c98c8409af0cf4003f52d75f72dd06a3066feed5` (the merge of `origin/lane-C-kk-probe-environ` into this branch; stacked start, the probe lane is already in). Code head before this report: `1083fc4ca0d93f7117d2e9e22e20038872b0772c`. Model: sonnet.
Base check: `git grep -n 'func (e Env) environ' HEAD -- internal/account/` gave `HEAD:internal/account/account.go:64:func (e Env) environ() []string {`. It exists, so the lane went on.

## Proof table

| # | Row / rule | Result | Proof |
|---|---|---|---|
| 1 | `go test -count=1 -v -run 'TestKkNote' ./internal/account/` has a `--- PASS:` for each test; the environment test fails on base | HOLDS | exit 0; `--- PASS:` for TestKkNoteThenEnvironReplacesHost, TestKkNoteThenEnvironNilReadsHost, TestKkNoteThenEnvironEmptyListIsNotTheHost, TestKkNoteThenEnvironKeepsNumbersStdinAndExit. Row-1 test: host `KK_NOTE_X=host` (t.Setenv), Environ `{"KK_NOTE_X=call","PATH=<host PATH>"}`, `NoteThen(env, "{}", dir, "echo \"$KK_NOTE_X\"", ...)` returns 0, stdout `call\n`, stderr empty. internal/account/kk_note_then_environ_test.go:49 |
| 2 | Environ nil: same call prints `host\n` | HOLDS | TestKkNoteThenEnvironNilReadsHost (kk_note_then_environ_test.go:61) `--- PASS`. It also passes on base, by design: the binary path is unchanged |
| 3 | `go test -count=1 ./internal/account/` exits 0, TestNoteThenKeepsNumbersAndPassesTheirOutput and TestNoteThenSurvivesAPayloadItCannotParse still pass | HOLDS | exit 0, `ok internal/account 2.187s`; the two tests are in that package run (ei_statusline_test.go:329, :354), not edited |
| 4a | `make lint` | HOLDS | exit 0 (`go vet ./...`, then the gofmt check, no output); `gofmt -l internal/account/` empty |
| 4b | `make cross` | HOLDS | exit 0; `cross: darwin/arm64 vet`, `darwin/arm64 test compile`, `windows/amd64 vet`, `windows/amd64 test compile` all ran, every package `ok`, no FAIL |
| 5a | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, `ok internal/cli 0.125s` |
| 5b | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, `ok internal/job/lint 0.171s` |
| 5c | diff names only statusline.go and the new test file | HOLDS | `git diff c98c8409 --stat` (the base): `internal/account/kk_note_then_environ_test.go (108 lines)`, `internal/account/statusline.go (1 line)`, 2 files changed. Run before this report was committed; the report commit adds only docs/lanes/C-kk-note-then-environ.md. See the note below on `git diff lane-C-kk-probe-environ` |
| R1 | New tests fail on base | HOLDS | test file written first, run on the unchanged statusline.go: `kk_note_then_environ_test.go:57: with Environ set the child reads that list: stdout "host\n", want "call\n"` / `--- FAIL: TestKkNoteThenEnvironReplacesHost`; `kk_note_then_environ_test.go:83: an empty Environ is an empty environment, not the host's: stdout "host\n"` / `--- FAIL: TestKkNoteThenEnvironEmptyListIsNotTheHost`; `kk_note_then_environ_test.go:99: the exit code is the command's: 1, want 3` / `--- FAIL: TestKkNoteThenEnvironKeepsNumbersStdinAndExit` (the child on base sees `KK_NOTE_X=host`, so the `test "$KK_NOTE_X" = call` guard fails). TestKkNoteThenEnvironNilReadsHost passes on base by design (row 2) |
| R2 | Tests of changed package and every importer | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep internal/account` shows `internal/cli` is the only importer of `internal/account`. `go test -count=1 ./internal/account/` exit 0; `go test -count=1 ./internal/cli/` exit 0 (`ok internal/cli 66.993s`) |
| R3 | No existing test edited or deleted | HOLDS | `git diff c98c8409 --stat` shows no other test file |
| R4 | Only the Touches changed; no edit under internal/cli or lib/ | HOLDS | see 5c; statusline.go:127 only, inside NoteThen |
| R5 | Package helpers start `kkNote`, tests `TestKkNote`, no TestMain | HOLDS | kkNotePayload, kkNoteEcho, kkNoteShellOK, kkNoteEnv, kkNoteRun; four `TestKkNote...` tests; no TestMain. The helpers `shellOK` and `payload42` live in the external package `account_test`, so this file defines its own prefixed ones |
| R6 | The shell lookup stays on the host PATH; nothing else in NoteThen changes | HOLDS | statusline.go:122 `exec.LookPath(name)` unchanged; the diff of statusline.go is the one added line `cmd.Env = env.environ()` before `cmd.Run()` |
| R7 | Windows skip names its reason | HOLDS | kkNoteShellOK: `t.Skip("the --then command text is sh; Windows runs it with cmd /C")` (kk_note_then_environ_test.go:25) |
| R8 | No guard, gate, cap, hold or hook on agents | HOLDS | none added; the exit code, the 127 path and the payload-kept-first order are untouched |
| R9 | `*RealPlugin*` tests, app walks (`make webui-test`, fv/fi/hn, slices Imports) | NOT APPLICABLE | no row names a RealPlugin test; the app-only walks are not in the plugin repo. The plugin walks are 5a and 5b |
| R10 | One-sentence commits, no attribution lines, no personal paths, hosts or emails | HOLDS | one code commit and one report commit, one sentence each; tests use `t.TempDir()` only |

## What changed

- `NoteThen` (internal/account/statusline.go:127): `cmd.Env = env.environ()` before `cmd.Run()`. The child, `sh -c` or `cmd /C` by OS, takes the environment the Env holds; `Environ` nil reads `os.Environ()`, as the binary does. `exec.LookPath(name)` for the shell still reads the host PATH (owner question 1 in INDEX.md).
- internal/account/kk_note_then_environ_test.go: four tests (replaces the host, nil reads the host, empty list is an empty environment and not the host's, and with Environ set the payload is still kept first, stdin is the same bytes, the exit code is the command's, an unreadable payload does not stop the command).

## Notes for the orchestrator and the T14 mount

- Row 5 as written (`git diff lane-C-kk-probe-environ --stat` names two files): there is no local branch of that name, and `origin/lane-C-kk-probe-environ` is not an ancestor-line of this lane's base: this branch was cut from the Pint118 integration line and the probe branch was merged into it. `git diff origin/lane-C-kk-probe-environ --stat` therefore lists 83 files (other lanes already on the Pint118 line, none of them from this lane); `account.go` and `kk_probe_environ_test.go` are not in that list, so the probe lane is in HEAD as is (`git merge-base --is-ancestor origin/lane-C-kk-probe-environ HEAD` exits 0). The lane's own diff is the two files of 5c.
- internal/cli/inprocess.go:147-148 still says `--then` runs a shell command as a host child and keeps `account note --then` binary-only; that comment and entry are now stale. Not edited here (no edit under internal/cli); the T14 mount edits that file.
- An `Environ` func that returns a nil slice hands the host environment to the child (a nil `cmd.Env` means the host's, in os/exec); an empty non-nil slice is an empty environment (test 3). The probe lane's `providerEnv` never returns nil for that reason. The brief asks for exactly `cmd.Env = env.environ()`, so NoteThen does not turn nil into an empty list; a caller that means "no environment" returns `[]string{}`. If the owner wants nil read as empty, that is one line in `Env.environ()` (account.go, the probe lane's file), not in NoteThen.
- The bundle `origin/main..lane-C-kk-note-then-environ` carries the probe commits and the Pint118 line too (stacked start).

## NOT VERIFIED on a live screen or machine

- Windows: the child there is `cmd /C`; only `make cross` (vet and test compile for windows/amd64) ran. The four tests skip on Windows with a reason, because the command text is sh. No Windows machine ran `cmd /C` with a replaced environment.
- macOS: darwin/arm64 compiled and vetted through `make cross`; the tests were not run on a Mac.
- The status line command of a real Claude Code session was not started (no claude run, per the rules).

DONE
