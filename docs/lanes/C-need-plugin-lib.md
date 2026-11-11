# C-need-plugin-lib: the plugin offers a public lib package

Plugin lane on main 55c46cbd (branch lane-C-need-plugin-lib). Head: the sha the hand-off names. Nothing pushed.

## Proof table

| # | Row | Result | Command / place |
|---|-----|--------|-----------------|
| 1 | `go test ./lib` fails on main | HOLDS | On base 55c46cbd with lib/ stashed: `go test ./lib` exit 1: `stat <clone>/lib: directory not found` / `FAIL ./lib [setup failed]` |
| 2 | TestLibFeedPostReadState: lib and binary give the same bytes | HOLDS | lib/lib_test.go TestLibFeedPostReadState. Project (git folder) and task made through `Run`; one `feed post`; then `feed read`, `feed read --json`, `feed state --json` through `Run` and through `go build ./cmd/grid` with the same args, same Environ (+ GRID_HOME) and same home: stdout, stderr and exit code equal. `go test -count=1 ./lib` exit 0 |
| 3 | TestLibParallelHomesDoNotSeeEachOther | HOLDS | `go test -race -count=3 ./lib/...` exit 0 (race detector ran; C compiler present). Two homes in parallel, host GRID_HOME/GRID_FEED_TASK/GRID_MEMBER/GRID_ROLE set to a third (decoy) home: each home's `feed read` holds only its own 8 records, `task list` shows only its own task title, the decoy home is not written |
| 4 | TestLibHostEnvironmentIsNotRead + TestLibEnvIsTheCallsAlone | HOLDS | Host has GRID_MEMBER=intruder, GRID_LEASE_EPOCH=broken, GRID_JOB=J-wrong, GRID_ROLE=guest, GRID_HOME=elsewhere. Environ call: the post's author is `member` and its job is `J-1`; Environ `GRID_LEASE=task, GRID_LEASE_EPOCH=3.4` gives exit 3 `stale lease` (a host `broken` would be exit 1 `lease epoch must be gen.fence`). Nil Environ with `--member member --role owner` flags: exit 0, the record has author `member`, no job, attempt, wake, gen 0, fence 0. In package lib: `Call{}.env(nil)` has `Getenv` not nil (host GRID_MEMBER shows as ""), GridHome and Machine "" with a host GRID_HOME set, no host temp root, `OpenURL` not nil and returns nil, `AppStart` nil, `Executable` nil; with a counting OpenURL the Env's OpenURL is that function (one call, the same address); Executable field is returned by `Env.Executable` |
| 5 | TestLibExitCodes, TestLibPanicIsExitTwo | HOLDS | usage (`nonsense`) 2 with `unknown command`; missing task (`task show T-99` from the project folder) 1; stale lease (GRID_LEASE=task, GRID_LEASE_EPOCH=1.1, lease never taken) 3 `stale lease`; a Stdout whose Write panics with `version`: exit 2 and Stderr exactly `grid: internal error\n`; a panicking Stderr too: exit 2 and no panic out of `Run`. No new verb, no internal seam |
| 6 | Binary and contract unchanged | HOLDS | `git diff --stat main -- cmd/grid contract go.mod go.sum internal` prints nothing (exit 0). No internal/cli edit was needed: no row-2 to row-5 test showed a value still read from the host in a call the lib makes. Existing tests unedited (none touched). No guard, gate, cap or limit added |
| 7a | Package tests | HOLDS | `go test -count=1 ./lib/... ./internal/cli/... ./internal/mcp/... ./internal/golden/... ./internal/packaging/... ./contract/...` exit 0 (lib, cli, mcp, golden, packaging, packaging/cmd, contract/formats ok) |
| 7b | Builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0; `GOOS=windows go vet ./lib` exit 0 (the tests compile for Windows) |
| 7c | Format and lint | HOLDS | `gofmt -l lib` empty; `make lint` (go vet ./... and gofmt -l .) exit 0 |
| R1 | Importers tested | HOLDS | `go list` of packages importing internal/cli or lib (code or tests): cmd/grid, contract/formats, internal/golden, internal/mcp, internal/packaging, internal/question, internal/settings, lib. Nothing imports lib yet. All tested: the six above in 7a, and `go test -count=1 ./cmd/... ./internal/question/... ./internal/settings/...` exit 0 (cmd/grid has no test files) |
| R2 | Whole-module test, cross-repo set | NOT RUN | The orchestrator runs them at landing |

## What changed

- New `lib/lib.go`: `lib.Run(lib.Call) int`. Call fields: Args, GridHome, Home, Dir, Environ, Stdin, Stdout, Stderr, Executable, OpenURL (no Member, Role or Lease fields; they travel in Environ or as the global flags in Args). Every choice of the brief is as written: Environ is the whole environment (last entry wins, names fold case on Windows, nil is empty) and `Env.Getenv` is always set from it; Home (field, then HOME or USERPROFILE on Windows from Environ, then `gridhome.Real`); GridHome (field over GRID_HOME in Environ, else `gridhome.Dir`, then `Real`, Machine from it); Dir empty is the Grid home; nil Stdin is empty, nil Stdout/Stderr discard; TempRoots TMPDIR, TMP, TEMP from Environ then /tmp and /var/tmp; Executable empty leaves `Env.Executable` nil; OpenURL always set (nil Call.OpenURL does nothing and returns nil); AppStart nil; a panic gives exit 2 and one line `grid: internal error`, and a panicking Stderr does not escape. The package doc names what a call reads, the host reads that stay (user name and `git config user.email` when no member is given, internal/account's nil fallback, CLAUDE_PLUGIN_DATA, os.Getppid in `ports claim`, `env up` and the task lease, children of land, agent and routine) and the exit codes.
- New `lib/lib_test.go` (package lib, one file, so the Env case of row 4 can see `Call.env`): 5 tests, all new. Redone on main from the P1-plib shape; that branch was not merged or rebased.
- No change in internal/, cmd/grid, contract/, go.mod.

## Notes

- The doc lists the host reads by file, not by line, since lines move (M-plugin-held-f-e moves cli.go).
- The nil `OpenURL` case of `grid open` itself is not run through `Run`; the Env-level test shows the function is set and returns nil. `internal/cli/open.go` and `open_start.go` read the host (`os.Getenv`, `os.Environ`) only when `OpenURL` is nil or `AppStart` is set; the lib sets one and leaves the other nil.
- Not shown: that each new test fails if its guarded line is removed (for example `Getenv` left nil). Row 1 is the only fail-on-base proof; the package did not exist.

## NOT VERIFIED on a live screen

- Nothing in this lane has a screen. NOT VERIFIED: Windows and macOS at run time (cross-builds and `go vet` only); other verbs under parallel calls beyond `feed`, `task list` and `project add`; how the app takes the package (a published version or a CI clone), which is an open owner question and a later brief.
