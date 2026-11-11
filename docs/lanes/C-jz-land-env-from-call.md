# C-jz-land-env-from-call

Base: dc301fce757d6374de26d8d1b68de26575341b32 (main of the lane clone). Lane commits before this report: b944383f (product), 7096da4d (tests).

A lib `land` now runs the deploy command with the Call's variables, not the host's. `land.Store` has `Getenv func(string) string` ("nil reads the host, as the binary does"). With it set, `landEnvironment` asks Getenv for each allowed name (the fixed list for the goos, then the rule's `env` names) and passes `NAME=value` when the answer is not empty, with the allow list's own spelling. With it nil, the `os.Environ` filter runs as before. `internal/cli/land.go` passes the raw `env.Getenv`.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1 getenv builder, linux, darwin, windows | HOLDS | `go test -count=1 -v -run 'TestJzLd' ./internal/land/` exit 0: `--- PASS: TestJzLdAllowedEnvironmentFromGetenv/linux`, `/darwin`, `/windows`. Test: internal/land/jz_land_env_test.go:30-97. Each case compares the exact slice, checks no `SECRET=` and compares the exact list of names the fake was asked (each once, none outside the allow list). Windows: the fake answers `Path` and `systemroot` case-blind and the rule's names are `JZ_MARK`, `Path`, `SYSTEMROOT`; the result has `PATH=` and `SystemRoot=` once each. |
| 2 Getenv answers `call`, host `host` | HOLDS | Same run: `--- PASS: TestJzLdLandEnvFromCall (0.15s)` (internal/land/jz_land_env_unix_test.go:50-59). Rule `env: [JZ_MARK]`, command `printf '%s' "$JZ_MARK" > outcome`, `t.Setenv("JZ_MARK","host")`, Getenv answers `call`; outcome holds `call`. Also `--- PASS: TestJzLdLandEnvHostValueDoesNotLeak` (host HOME does not reach the command when the Call has none) and `--- PASS: TestJzLdLandEnvPathFromCall`. |
| 2 fails on base | HOLDS | With the four product files stashed (`git stash push -- internal/land/environment.go internal/land/land.go internal/land/process_windows.go internal/cli/land.go`), `go test -count=1 -v -run 'TestJzLd' ./internal/land/` exit 1: `internal/land/jz_land_env_unix_test.go:19:4: s.Getenv undefined (type Store has no field or method Getenv)`, `internal/land/jz_land_env_test.go:76:11: undefined: allowedEnvironmentGetenv`, `FAIL github.com/gridconsole/grid-plugins/internal/land [build failed]`. Then `git stash pop`. |
| 3 Getenv nil reads the host | HOLDS | Same run: `--- PASS: TestJzLdLandEnvNilReadsHost (0.07s)` (jz_land_env_unix_test.go:61-66): `JZ_MARK=host` in the host, Store.Getenv nil, outcome `host`. `--- PASS: TestJzLdLandEnvironmentUsesGetenv` (jz_land_env_test.go:100-131) also checks the nil path through `landEnvironment`, and that `GRID_LAND_*` come once each from the plan even when the Call answers `GRID_LAND_BRANCH`. |
| 4 land package | HOLDS | `go test -count=1 ./internal/land/` exit 0: `ok .../internal/land 1.596s` (`environment_test.go:38` unchanged and passing). |
| 4 cli Land | HOLDS | `go test -count=1 -run 'Land' ./internal/cli/` exit 0: `ok .../internal/cli 0.970s`. Whole package `go test -count=1 ./internal/cli/` exit 0: `ok .../internal/cli 105.797s`. |
| 5 lint | HOLDS | `make lint` exit 0 (`go vet ./...` and `gofmt -l .` print nothing). |
| 5 cross | HOLDS | `make cross` exit 0 (it ran past the 600 s foreground limit and finished in the background; the output has no FAIL line): `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `ok .../internal/land 0.004s`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, `ok .../internal/land 0.001s`. process_windows.go and its Windows tests build. |
| Importers tested | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E '(^\| )...internal/(land\|cli)( \|$)'` lists: cmd/grid, contract/formats, internal/account, internal/braincmd, internal/cli, internal/golden, internal/jobgraph, internal/land, internal/mcp, internal/packaging, internal/question, internal/settings, lib. The only package that imports `internal/land` in product code is `internal/cli` (tested, exit 0 above). The rest import `internal/cli` and none of them calls a function whose signature changed (only `landEnvironment`, package-private). `go test -count=1 ./lib ./internal/mcp ./cmd/grid ./internal/land/...` exit 0: `ok .../lib 143.327s`, `ok .../internal/mcp 5.522s`, `cmd/grid [no test files]`, `ok .../internal/land 33.596s`. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0: `ok .../internal/cli 0.629s`. `go test -count=1 ./internal/job/lint/` exit 0: `ok .../internal/job/lint 0.366s`. |
| Edit only Touches | HOLDS | Product files changed: internal/land/environment.go, internal/land/land.go, internal/land/process_windows.go, internal/cli/land.go. New tests only in internal/land/jz_land_env_test.go and jz_land_env_unix_test.go. No existing test file changed. process.go, process_unix.go and process_other.go are unchanged. |
| Naming, new test files | HOLDS | Helpers `jzLdFake`, `jzLdLand`, `jzLdStore`, `jzLdMarkCommand`; tests `TestJzLd...`. jz_land_env_unix_test.go starts with `//go:build unix`. No row added to a shared test table. |
| No guard, gate, cap, limit | HOLDS | None added. The change only decides where the variables come from; the allow list is the same list. |
| No sleep, timeout or elapsed time as proof; no `VAR=` command | HOLDS | None in the new tests or in the commands above. |
| No test deleted or edited | HOLDS | `git diff dc301fce --stat -- '*_test.go'` shows the two new files only. |
| RealPlugin | n/a | No row names a `*RealPlugin*` test. |
| Commits | HOLDS | b944383f, 7096da4d and the report commit: one-sentence messages, no attribution lines. |

## What changed

- internal/land/environment.go: `allowedKeysFor(goos, extra)` is the allow list (split out of `allowedEnvironmentFor`, whose result is unchanged). New sibling `allowedEnvironmentGetenv(goos, getenv, extra)` asks getenv for each allowed name once (a rule name that folds to one already asked, such as `Path` on windows, is skipped, so the fixed list's spelling wins) and emits `NAME=value` for non-empty answers. New `allowedEnvironmentFrom(getenv, extra)`: nil is `allowedEnvironment(extra)`, otherwise the sibling with `runtime.GOOS`. `landEnvironment(p, getenv)` takes the getenv (no test called the one-argument form).
- internal/land/land.go: `Store.Getenv` and its use at the `runProcess` call.
- internal/cli/land.go: `Getenv: env.Getenv` (the raw field; nil in the binary).
- internal/land/process_windows.go: the killer's environment (`killer.Env`) is `killerEnvironment(cmd)`.

## Where this differs from the brief

The brief says to pass the same getenv to the Windows killer. `killProcessTree(cmd)` is reached through `cmd.Cancel` and `cleanupProcess(cmd)`, and existing tests call `configureProcess(cmd)`, `cleanupProcess(cmd)` and `runProcess(ctx, argv, dir, env)` with those signatures (process_windows_test.go:106-155, process_proctree_windows_test.go:53, process_unix_test.go:111, jp_land_drain_unix_test.go:94,146). A getenv argument would change those signatures and force edits to existing tests, and to process.go, process_unix.go and process_other.go. So the killer takes the fixed allow list from `cmd.Env`, which `runProcess` has already set from the same getenv (process.go:134, before `configureProcess`): `allowedEnvironmentFor(runtime.GOOS, cmd.Env, nil)`. With a Getenv that is the Call's variables, with nil it is the host's `os.Environ` filter, as before, and with no `cmd.Env` at all (the bare `exec.Cmd` the Windows tests pass to `configureProcess`) it falls back to `allowedEnvironment(nil)`, as before. The result is the fixed list only, as `allowedEnvironment(nil)` was; the rule's names and `GRID_LAND_*` are dropped. The `SystemRoot` read that locates taskkill (process_windows.go:39, `os.Getenv("SystemRoot")`) stays on the host: it names where the OS's own tool lives, and the brief names only the `killer.Env` line.

## NOT VERIFIED

- Windows at run time: `process_windows.go` is compiled and vetted by `make cross`, and its tests are compiled with `-exec /bin/true`; none was run on a Windows machine. The windows getenv builder itself is tested on every host with a case-blind fake.
- macOS at run time: compiled and vetted by `make cross`; the unix tests ran on Linux only.

## Open items

- The comments that still name the host environment as the reason a lib `land` runs out of process, lib/lib.go:41-43 and internal/cli/inprocess.go:101-102, are outside Touches and unchanged. The env half of that reason is gone; the signal half was already gone. Moving `land` in process is a separate brief.
- `Store.Getenv` is set only by `runLand`; `Approve` does not run the command, so it needs none.

DONE
