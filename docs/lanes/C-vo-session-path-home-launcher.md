# C-vo-session-path-home-launcher: the SessionStart hook puts the Grid home's bin folder on PATH when the stable launcher exists

Base: plugin main 792b8d6e53bb5a1c84a82c2e5da65bd638babfac. Lane commits: a1020ac9 (tests, red on base), then the implementation on top.

Base check: HOLDS. On 792b8d6e `internal/hook/hook.go:152` is `if err := writeEnv(envFile, o.Resolver.GridHome, res.Task, pluginPathLine(o.Getenv("CLAUDE_PLUGIN_ROOT"), o.Getenv("PATH"))); err != nil {` and `pluginPathLine` (hook.go:187-200 on base) reads only `root`; nothing names the Grid home's bin folder. The hook did not already do this.

## Proof table

| Row | Result | Command and output |
|---|---|---|
| 1 red on base | HOLDS | Tests committed alone at a1020ac9, run on base production code: `go test -count=1 -v -run 'TestVoPath' ./internal/hook/` printed `--- FAIL: TestVoPathHomeLauncherWinsOverPluginRoot`, `--- FAIL: TestVoPathLauncherLinkToFileCounts`, `--- FAIL: TestVoPathHomeWithSpaceAndQuoteResolvesGrid`, `FAIL ... internal/hook`. The base line it names is the plugin root's: `export PATH='/tmp/TestVoPathHomeLauncherWinsOverPluginRoot2737783842/002/plug/bin':"$PATH"` where the test wants `export PATH='/tmp/TestVoPathHomeLauncherWinsOverPluginRoot2737783842/001/data/bin':"$PATH"`. |
| 1 green on lane | HOLDS | `go test -count=1 -v -run 'TestVoPath' ./internal/hook/`, exit 0: `--- PASS: TestVoPathHomeLauncherWinsOverPluginRoot`, `TestVoPathNoLauncherKeepsPluginRootLine`, `TestVoPathHomeBinAlreadyOnPathWritesNoHomeLine`, `TestVoPathLauncherFallsBackToPluginRootLine`, `TestVoPathLauncherDirectoryWritesPluginRootLine`, `TestVoPathLauncherLinkToFileCounts`, `TestVoPathLauncherBrokenLinkWritesPluginRootLine`, `TestVoPathHomeWithSpaceAndQuoteResolvesGrid`, `TestVoPathLineDirect`, then `ok .../internal/hook`. The first test runs `Run` for sessionstart with `CLAUDE_PLUGIN_ROOT` a folder holding `bin/grid` and `GridHome` a home holding `bin/grid`; the env file holds exactly one PATH line, `export PATH='<home>/bin':"$PATH"`, and no `<root>/bin`. |
| 2 no launcher is today's bytes | HOLDS | `TestVoPathNoLauncherKeepsPluginRootLine` builds the expected file with `writeEnv(want, home, "", pluginPathLine(root, "/usr/bin"))` and compares the whole env file byte for byte. It passes on base and on the lane (a regression pin, so it is not among the three red tests). |
| 2 home bin already on PATH | HOLDS | `TestVoPathHomeBinAlreadyOnPathWritesNoHomeLine`: PATH `/usr/bin:<home>/bin/` (trailing slash, Clean compare) writes no home line; with no plugin root there is no `export PATH=` line at all. See the note on the fallback below. |
| 2 `<home>/bin/grid` a directory | HOLDS | `TestVoPathLauncherDirectoryWritesPluginRootLine`: the plugin root's line is written, the home line is not. Also: `TestVoPathLauncherLinkToFileCounts` (a link to a file is the launcher; `os.Stat` follows it) and `TestVoPathLauncherBrokenLinkWritesPluginRootLine`. |
| 3 space and quote home | HOLDS | `TestVoPathHomeWithSpaceAndQuoteResolvesGrid`, home `<tmp>/a b'c`: env file holds `export PATH='<tmp>/a b'\''c/bin':"$PATH"` (spelled out in the test, not through shellQuote), and `sh -c '. "$1"; command -v grid' sh <envfile>` with PATH `/usr/bin:/bin` printed `<home>/bin/grid`. Skips when `sh` is missing or on Windows. |
| 4 hook package | HOLDS | `go test -count=1 ./internal/hook/` exit 0: `ok .../internal/hook 15.655s`. hook_test.go is unedited (`git diff main --stat` below names no test file but the new one). |
| 4 golden | HOLDS | `go test -count=1 -run 'TestHookSessionStartGolden' ./internal/cli/` exit 0: `ok .../internal/cli 2.127s`. `git diff main --stat -- testdata/golden` printed nothing. |
| 5 lint | HOLDS | `make lint && echo LINT_EXIT_0` printed `go vet ./...` and `LINT_EXIT_0` (gofmt -l is part of the target and printed nothing). |
| 5 cross (Windows and macOS builds) | HOLDS | `make cross && echo CROSS_EXIT_0` printed `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, every package `ok`, then `CROSS_EXIT_0`. |
| 5 Touches | HOLDS | `git diff main --stat`: `internal/hook/hook.go | 6 +-`, `internal/hook/vo_path_line.go | 28 ++++`, `internal/hook/vo_path_line_test.go | 256 +++`, plus this lane report under docs/lanes. Helpers, types and variables of the new files start with `voPath`; test functions start with `TestVoPath`. `pluginPathLine` and its signature are not edited. |
| Importers tested | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep -E ' .../internal/hook( \|$)'` names: cmd/grid, internal/cli, internal/packaging, internal/packaging/cmd, lib. `go test -count=1 ./cmd/grid/ ./internal/cli/ ./internal/packaging/... ./lib/` exit 0: `ok internal/cli 144.149s`, `ok internal/packaging 24.797s`, `ok internal/packaging/cmd 2.183s`, `ok lib 81.765s`, `cmd/grid [no test files]`. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok .../internal/cli 0.085s`; `go test -count=1 ./internal/job/lint/` -> `ok .../internal/job/lint 0.208s`. The app-side walk tests do not apply to this repo. |
| New tests fail on base | HOLDS for the three that test new behaviour (row 1 above); the other six pin behaviour that is unchanged or only exists with the new function (`TestVoPathLineDirect` calls `voLauncherPathLine`, which base lacks, so it does not compile on base). |
| No existing test edited or deleted | HOLDS | `git diff main --stat` names no existing test file. |
| `*RealPlugin*` tests, mismatch proof | n/a | No row names one. |
| Cross-repo | HOLDS | `git -C /srv/gridwork/ref/app.git grep -n 'CLAUDE_ENV_FILE' main` printed nothing: no app rig reads the PATH line; no app brief needed. |
| No guard, gate, cap or hold on agents | HOLDS | The change only chooses which folder the existing PATH line names; nothing blocks, limits or asks. |
| No personal path, host or email in the plugin repo | HOLDS | `grep -n -E '/home/\|/srv/\|@[a-z]\|gridwork'` over the two new files printed nothing. |

## What changed

- `internal/hook/vo_path_line.go` (new): `voLauncherPathLine(home, pathEnv string) string` returns `export PATH='<home>/bin':"$PATH"\n` (folder quoted by `shellQuote`), or "" on Windows, with an empty home, when `<home>/bin/grid` is missing or a directory, or when `<home>/bin` is already on PATH (`filepath.Clean` compare). It clones `pluginPathLine`'s checks with `home` for `root`.
- `internal/hook/hook.go:152-156`: SessionStart takes `voLauncherPathLine(GridHome, PATH)` and, when that is "", `pluginPathLine(CLAUDE_PLUGIN_ROOT, PATH)` as before. With no `<home>/bin/grid` the env file is byte for byte the same as base.
- `internal/hook/vo_path_line_test.go` (new): tests through `Run` (env file read back) and one direct test of the function.

## Notes for the reviewer

- When `<home>/bin/grid` exists and `<home>/bin` is already on PATH, `voLauncherPathLine` returns "" and the fallback writes the plugin root's line, as the brief words it ("when that is not "", else pluginPathLine as today"). That prepends the versioned cache folder in front of the home folder; after an update the shell skips the missing folder and finds the home launcher, so `grid` keeps working. `TestVoPathLauncherFallsBackToPluginRootLine` pins this so it reads as the spec and not an oversight. If the owner wants no PATH line at all in that case, that is a one-line change in hook.go.
- The empty-home branch cannot be reached through `Run` (hook.go:105 returns when `GridHome` is ""); `TestVoPathLineDirect` covers it by calling the function.
- Which `grid` an agent's shell resolves to now depends on a file under the Grid home. The hook only checks that `<home>/bin/grid` is a file (or a link to one), as `pluginPathLine` does for the plugin root; it does not check the executable bit or who owns the folder (no guard was asked for or added).

## NOT VERIFIED on a live screen

- A real Claude Code session on the VM that started before a plugin update and still runs `grid` afterwards. Nothing creates `<home>/bin/grid` yet: that is C-bl-timer-stable-launcher, which is not on main. Until it lands the hook output is byte for byte today's.
- Windows and macOS runtime output: only vetted and test-compiled by `make cross`; the Windows path returns "" by `runtime.GOOS`.

Verdict: DONE
