# Lane C-kk-probe-environ: the account probe child takes its environment from account.Env

Repo: plugin. Base: `21f0273607ae73e7aa8494035743afecdd19f8fa` (lane-Pint115 merge, `main`). Model: sonnet.
Base check: `account.Env` on the base has Home, GridHome, Getenv, Now, GOOS and no environment field (account.go:50-57). No lane on the base added one.

## Proof table

| # | Row / rule | Result | Proof |
|---|---|---|---|
| 1 | `go test -count=1 -v -run 'TestKkProbe' ./internal/account/` has a `--- PASS:` for each test | HOLDS | exit 0; `--- PASS:` for TestKkProbeEnvironHome, TestKkProbeProviderEnv, TestKkProbeTwoEnvsDoNotSeeEachOther, TestKkProbeEmptyEnvironIsNotTheHost, TestKkProbeEnvironReadAtCallTime, TestKkProbeEnvironSignInAndCodex. Home test: Environ's FAKE_HOME gives `a@example.com` signed in; Environ nil gives signed out (host FAKE_HOME). internal/account/kk_probe_environ_test.go:71 |
| 2 | `providerEnv` windows/linux/dir cases, order kept | HOLDS | TestKkProbeProviderEnv (kk_probe_environ_test.go:95): windows `{"Path=x","Claude_Config_Dir=/wrong","A=1"}` with dir "" gives `{"Path=x","A=1"}`; linux gives the list whole; dir "/d" ends in `CLAUDE_CONFIG_DIR=/d`; input slice unchanged; result never nil |
| 3 | Two goroutines, two Envs, start barrier, `Status(env, true)` | HOLDS | TestKkProbeTwoEnvsDoNotSeeEachOther: A's registry holds only `a@example.com`, B's only `b@example.com`; closed channel start, no sleep, no clock, no timeout |
| 4 | `go test -count=1 ./internal/account/` | HOLDS | exit 0, `ok ... internal/account 2.217s` |
| 5a | `make lint` | HOLDS | exit 0 (`go vet ./...`, then the gofmt check, no output) |
| 5b | `make cross` | HOLDS | exit 0; stages `cross: darwin/arm64 vet`, `darwin/arm64 test compile`, `windows/amd64 vet`, `windows/amd64 test compile` all ran, no FAIL. Also `GOOS=windows CGO_ENABLED=0 go vet ./internal/account/` and the darwin one: exit 0 |
| 6a | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, `ok internal/cli 0.196s` |
| 6b | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, `ok internal/job/lint 0.696s` |
| 6c | `git diff main --stat` names only account.go and the new test file | HOLDS | `internal/account/account.go (58 lines)`, `internal/account/kk_probe_environ_test.go (249 lines)`, 2 files changed (run before this report was committed; the report commit adds only docs/lanes/C-kk-probe-environ.md, the runbook's own file) |
| R1 | New tests fail on base | HOLDS | on the unchanged account.go: `kk_probe_environ_test.go:81:6: env.Environ undefined (type Env has no field or method Environ)`, `:119:10: undefined: providerEnv`, `:144:4: unknown field Environ in struct literal of type Env`, `FAIL internal/account [build failed]` |
| R2 | The tests catch the fault, not only compile | HOLDS | mutation run (not committed): `providerEnv(os.Environ(), ...)` plus exact-only name compare made all six TestKkProbe tests `--- FAIL`; restored with `git checkout -- internal/account/account.go`; tree clean |
| R3 | Tests of changed package and every importer | HOLDS | `go list -test` shows `internal/cli` is the only importer of `internal/account`. `go test -count=1 ./internal/account/` exit 0; `go test -count=1 ./internal/cli/` exit 0 (`ok 78.404s`); `go vet ./internal/account/ ./internal/cli/` exit 0 |
| R4 | No existing test edited or deleted | HOLDS | `git diff main --stat` shows no other test file |
| R5 | Only the Touches changed; no edit under internal/cli, lib/, internal/settings | HOLDS | see 6c |
| R6 | Package helpers start `kkProbe`, tests `TestKkProbe`, no TestMain | HOLDS | kk_probe_environ_test.go: kkProbeFakeCodex, kkProbeHome, kkProbeGetenv, kkProbeHomeFolder; no TestMain |
| R7 | No guard, gate, cap, hold or hook on agents | HOLDS | none added; the 30-second probe context and exit-code handling are untouched |
| R8 | exec.LookPath left as it is; nothing else changes | HOLDS | account.go:205 unchanged (`exec.LookPath(bin)` on the host PATH) |
| R9 | `*RealPlugin*` tests, `make webui-test` and other app walks | NOT APPLICABLE | no row names a RealPlugin test; the app-only walks are not in the plugin repo |
| R10 | No attribution lines in commits; public repo has no personal paths, hosts or emails | HOLDS | two commit messages of one sentence each; the test uses `example.com` emails and `t.TempDir()` |

## What changed

- `Env.Environ func() []string` (account.go:57-60, the field is line 60): nil reads `os.Environ()`. `Env.environ()` reads it at each call, never cached.
- `providerEnv(environ, dirVar, dir, goos)` (account.go:185): pure; drops the entries named `dirVar` (`strings.EqualFold` when goos is "windows", exact otherwise), keeps order, appends `dirVar=dir` when dir is not "". It never returns nil, because a nil `cmd.Env` makes `exec.Cmd` hand the host environment to the child.
- `runProvider(env, bin, ...)` builds `cmd.Env` from `providerEnv(env.environ(), dirVar, dir, env.goos())`. `readClaude`, `readCodex` (gains `env`) and `SignIn` pass their env.
- Tests: six in internal/account/kk_probe_environ_test.go, the four the brief lists plus the empty list (not the host's), read-at-call-time, and SignIn plus the codex probe.

## Notes for the orchestrator and the T14 mount

- internal/cli/inprocess.go:137-141 still cites `account/account.go:183`; that line number is stale now. Not edited here (no edit under internal/cli); the T14 mount edits that file.
- A malformed entry with no `=` is kept as before (the old prefix test `dirVar=` never dropped it).

## NOT VERIFIED on a live screen or machine

- Windows case folding runs only as the pure function `providerEnv` (and through `Env.GOOS = "windows"` on Linux); no Windows machine ran the child.
- The fake providers are shell scripts, so TestKkProbe tests that start a child skip on Windows (via installFakeClaude); the pure `providerEnv` test runs everywhere.
- `-race` was not run.
