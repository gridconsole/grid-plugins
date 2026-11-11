# C-jz-agent-probe-env-from-call: lane report

`agent check` probes start with the Call's variables, not the host's. Base is `dc301fce` (`origin/main`; the brief
quotes 35af7d3c, main has since moved on by merges). Production change: `internal/agent/runner.go` (the `Getenv`
field, `childEnvOn`, `baseFromGetenv`) and `internal/cli/agent.go:14` (passes `env.Getenv`). New tests:
`internal/agent/jz_probe_env_test.go`.

## Proof table

| Row / rule | State | Command / place |
|---|---|---|
| 1. Call's CLAUDE_CONFIG_DIR and LANG reach the child, the host's do not | HOLDS | `go test -count=1 -v -run 'TestJzAg' ./internal/agent/` exit 0: `--- PASS: TestJzAgChildEnvIsTheCallsNotTheHosts`. The test `t.Setenv`s CLAUDE_CONFIG_DIR=/host and LANG=host, runs `Env{Home: tmp, Getenv: fake}.childEnv("claude", ...)` with the fake answering `/call` and `call`, and asserts `CLAUDE_CONFIG_DIR=/call` and `LANG=call` are present and no line holds `/host` or `LANG=host`. On base it fails to build: `internal/agent/jz_probe_env_test.go:46:25: unknown field Getenv in struct literal of type Env`, `:79:9: undefined: baseFromGetenv`, `:133:43: Env{…}.childEnvOn undefined (type Env has no field or method childEnvOn)`, `FAIL .../internal/agent [build failed]` (exit 1; base = the lane with `runner.go` and `agent.go` stashed, restored with `git stash pop`). |
| 2. SECRET_TOKEN not passed; HOME is the Env's Home | HOLDS | Same test: the fake answers `SECRET_TOKEN=x` and `HOME=/call-home`; the child env has no SECRET_TOKEN key, one `HOME=` line, equal to `Env.Home` (`runner.go` `childEnvFor`, the `HOME="+home` append). `TestJzAgGetenvAsksOnlyTheFixedList` also pins that the builder asks only for the fixed list (USER, LOGNAME, LANG, TMPDIR, SHELL, LC_ALL, LC_CTYPE, LC_MESSAGES, CLAUDE_CONFIG_DIR, CODEX_HOME; plus USERNAME, APPDATA, LOCALAPPDATA, SYSTEMROOT, TEMP, TMP, PATHEXT, COMSPEC on windows) and writes only non-empty answers: `--- PASS`. `TestJzAgGetenvConfigDirFollowsTheAgent`: claude gets CLAUDE_CONFIG_DIR only, codex CODEX_HOME only, LC_TIME is not asked for: `--- PASS`. |
| 3. Windows goos, SYSTEMROOT from the fake gives the System32 entries | HOLDS | `--- PASS: TestJzAgWindowsSystemRootFromTheCall`: `Env{Getenv: fake}.childEnvOn("windows", ";", "claude", winBinary)` with the host's SYSTEMROOT and APPDATA set to `D:\HostWin`/`D:\HostApp` by `t.Setenv` and the fake answering `C:\Windows`; the result equals, line for line, `USERNAME, APPDATA, LOCALAPPDATA, SYSTEMROOT, TEMP, TMP, PATHEXT, COMSPEC, HOME, PATH=C:\Users\me\.local\bin;C:\Windows\System32;C:\Windows, USERPROFILE` and holds no host value. `--- PASS: TestJzAgWindowsNoSystemRootFromTheCall`: a fake with no SYSTEMROOT gives the binary folder alone, though the host has one. |
| 4. Getenv nil and Base nil keeps os.Environ | HOLDS | `--- PASS: TestJzAgNilGetenvNilBaseKeepsOsEnviron`: `Env{Home: tmp}.childEnv(...)` is `reflect.DeepEqual` to `childEnvFor(runtime.GOOS, string(os.PathListSeparator), os.Environ(), ...)` and still holds the host's CLAUDE_CONFIG_DIR and LANG. `--- PASS: TestJzAgBaseWinsOverGetenv`: a set Base wins over Getenv and Getenv is not asked. |
| 5a. `go test -count=1 ./internal/agent/` | HOLDS | exit 0: `ok  github.com/gridconsole/grid-plugins/internal/agent 0.613s` (existing `childenv_lane_test.go` tests unchanged and passing). |
| 5b. `go test -count=1 -run 'Agent' ./internal/cli/` | HOLDS | exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli 1.799s`. |
| 6a. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output). `gofmt -l internal/agent internal/cli` printed nothing. |
| 6b. `make cross` | CROSS_ROW | see the last line of this table's note below |
| Importers of the changed packages tested | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...` filtered for a direct import of `internal/agent` or `internal/cli`: `cmd/grid`, `internal/cli`, `lib`. `go test -count=1 ./internal/cli/` exit 0 `ok internal/cli 74.425s` (the whole package, not only `Agent`); `go test -count=1 ./cmd/grid/ ./lib/` exit 0: `?   cmd/grid [no test files]`, `ok lib 19.701s`. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` (the `\|` is only the table escape; the command had a plain `|`) exit 0 `ok internal/cli 0.125s`; `go test -count=1 ./internal/job/lint/` exit 0 `ok internal/job/lint 0.310s`. The app-side walk tests (`make webui-test`, `TestFvArgvCollects`, ...) are for the app repo; this is the plugin repo. |
| `*RealPlugin*` tests | n/a | None named by the brief, none run, none counted as proof. |
| Only the Touches files edited | HOLDS | `git diff --stat origin/main..HEAD`: `internal/agent/runner.go`, `internal/cli/agent.go`, `internal/agent/jz_probe_env_test.go` (new), plus this report. No existing test edited or deleted; `childEnvFor` is unchanged. |
| New test in a new file, helpers `jzAg*`, tests `TestJzAg*` | HOLDS | `internal/agent/jz_probe_env_test.go`. It uses the existing `winBinary` constant from `childenv_lane_test.go` (read, not edited). |
| No sleep, no timeout, no elapsed time as proof; no concurrency | HOLDS | The tests are single-goroutine and assert on returned slices. |
| No guard, gate, cap or hold on agents | HOLDS | The change widens nothing and blocks nothing: it only changes which source the probe child's pass-through names are read from. |
| One-sentence commits, no attribution, no sudo | HOLDS | `git log --format=%B origin/main..HEAD`: no Co-Authored-By or Claude-Session line, although the session reminder asked for one; the owner's rule wins. |
| One command per call, no `VAR=` | HOLDS | No chain with `;`, no command starts with `VAR=`. |

## What changed

- `internal/agent/runner.go`: `Env.Getenv func(name string) string`. `childEnv` now calls `childEnvOn(goos, listSep, name, binary)`: base is `Base` when set; else, when `Getenv` is set, `baseFromGetenv(goos, Getenv)` (the names childEnvFor passes, as NAME=value when non-empty, Windows names added on windows); else `os.Environ()` (the binary). `childEnvFor` is not edited.
- `internal/cli/agent.go:14`: `agent.Env{..., Getenv: env.Getenv}`. `env.Getenv` is the raw field, not `env.getenv`, so the binary (`OSEnv` leaves it nil) keeps `os.Environ()`, and a lib Call that sets `Env.Getenv` gets its own variables.
- `internal/agent/jz_probe_env_test.go`: seven `TestJzAg*` tests.

## Open

- The cli wiring (`agent.go:14`) has no test of its own: the Touches list names no cli test file, and a cli `agent check` test would need a real `claude` or `codex` on PATH to show the child's environment. It is covered by the build, by `go test ./internal/cli/` and by the agent-package tests of the field it fills. A reviewer who wants a cli-level proof needs a fake `claude` script on a temp PATH.
- Stale wording, not edited because it is outside Touches and `TestHsIpWalksEveryCommand` reads it: `internal/cli/inprocess.go:126-129` and `lib/lib.go:38-39` still say `agent check` "probes host children" and "the agent's runs take the host environment". The probe half now takes the Call's variables; the refusal in `inprocess.go:129` still stands for the other reasons it names (the signal wait at `agent.go:36`, the host's executable in `agent_sync.go:17`). The owner may want the `why` shortened to those two.
- Owner question from the brief (INDEX.md): other `LC_` names (LC_TIME, LC_NUMERIC, ...) are not asked for; the list is fixed. `TestJzAgGetenvConfigDirFollowsTheAgent` pins that LC_TIME is not read. The binary path still passes every `LC_*` it has.
- On Windows the lookup is the caller's `Getenv`, which for the binary is nil, so `os.Environ()` is used. A lib host that answers `Getenv` on Windows must answer the Windows names case-insensitively itself (os.Getenv does; a plain map does not). The fake in `TestJzAgWindowsSystemRootFromTheCall` answers the upper-case names, as `baseFromGetenv` asks them.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. No real `claude` or `codex` was started (the brief forbids it), so what a real probed agent does with the Call's environment is not seen; the tests assert the environment slice `childEnv` returns. Windows and macOS were built and test-compiled by `make cross`; no test ran on those systems, so the windows case is proved through the goos parameter on a Linux host.
