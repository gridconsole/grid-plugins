# D-release-path: a marketplace install works on a machine without Go

Repo plugin, lane `lane-D-release-path`, from main eb833a2d. All commands ran in the clone; exit 0 unless stated.

## Proof table

| Row | Verdict | Evidence |
|---|---|---|
| 1. `make dist` writes dist/plugin with prebuilt binaries for darwin-arm64, darwin-amd64, linux-amd64, linux-arm64, windows-amd64 | HOLDS | `make dist` exit 0. `ls -l dist/plugin/bin`: grid-darwin-amd64 17955456, grid-darwin-arm64 16523426, grid-linux-amd64 17643884, grid-linux-arm64 16045049, grid-windows-amd64.exe 17763840 bytes. Targets at `internal/packaging/dist.go:17`, name rule `dist.go:20-26`; `TestTargetsShipEveryPlatform` |
| 1. `bin/grid` (sh) entry runs the right binary; test with a fake uname per OS checks the chosen path | HOLDS | `internal/packaging/release_test.go` `TestLauncherChoosesBinaryPerOS`: Darwin arm64 and x86_64, Linux x86_64 and aarch64, MINGW64_NT, MSYS_NT, CYGWIN_NT, each with all five binaries present and only the right one allowed to run; `go test ./internal/packaging -count=1` exit 0. Windows arm in `internal/packaging/grid.sh:30-36` |
| 1. `bin/grid.cmd` entry | HOLDS (content only) | `internal/packaging/grid.cmd`, written with CRLF by `layout.go` (`files["bin/grid.cmd"]`); `TestWindowsEntryIsCmd` checks CRLF, the `.exe` beside it, `%*`, the exit code, no build, no download. Not run in a real cmd.exe: see NOT VERIFIED |
| 1. No Go needed at install time | HOLDS | `TestDistPluginRunsWithoutGo` copies the real `dist/plugin` to a scratch dir, sets PATH=/usr/bin:/bin (no go) and runs the hook entry; output under row 4 |
| 2. Manifest version is 0.1.<commit count>, stamped by dist, one place; a test checks it | HOLDS | `go run ./internal/packaging/cmd -release` printed `0.1.1221` (`git rev-list --count HEAD` = 1221 on the base). `make dist` wrote `"version": "0.1.1221"` into `.claude-plugin/plugin.json`, `.codex-plugin/plugin.json` and `.claude-plugin/marketplace.json`. One input: `Makefile` `RELEASE` to `packaging.Build(version)` to `Files(version)`. Tests: `TestVersionStampedInEveryManifest`, `TestCommitVersion`, `TestReleaseVersionCountsCommits` (temp repo, 1, 2, 3 commits give 0.1.1, 0.1.2, 0.1.3) |
| 3. docs/release.md: exact steps, "owner runs this; not done by agents", what goes in, what stays out | HOLDS | `docs/release.md` (marked at the top; steps 1-5; "What goes into the public repo"; "What stays out" names evals/ as internal, D-privacy-audit). `docs/packaging.md` and `README.md` updated to match. Fix round 1: step 5 now sets `H=$(mktemp -d)` once and runs both `claude` lines with `HOME=$H`, then runs the installed `$H/.claude/plugins/cache/grid/grid/0.1.$N/bin/grid version` with `PATH=/usr/bin:/bin` |
| 4. Install dist/plugin into a scratch HOME with `claude plugin`, PATH lacking go, `grid version` through the hook entry | PARTLY: `claude` step NOT DONE | The shell's allow list refuses `claude` (`claude plugin --help` was denied), so `claude plugin marketplace add` and `install` were not run. Done instead: `TestDistPluginRunsWithoutGo` (exit 0): copy of dist/plugin, PATH=/usr/bin:/bin, HOME scratch, CLAUDE_PLUGIN_ROOT set, the command prefix every hook in hooks/hooks.json uses, run with `version`: `grid 0.1.1221`, `contract 1`, `app_min 0.1.0` |
| 5. `go test ./...` 0 fail | HOLDS | `go test ./... -count=1` exit 0 |
| 5. `make lint` 0 fail | HOLDS | `make lint` exit 0 (after `gofmt -w` on my test file) |
| Rule: new tests fail on base, pass on the lane | HOLDS | On base eb833a2d with `release_test.go` copied in, `go test ./internal/packaging`: `release_test.go:18:21: undefined: binaryName`, `:52:12: undefined: CommitVersion`, `:71:15: undefined: ReleaseVersion`, `FAIL ... [build failed]`. On the lane: `ok  github.com/gridconsole/grid-plugins/internal/packaging` |
| Rule: no existing test edited or deleted | HOLDS | `git diff origin/main --stat -- '*_test.go'` shows only the new `release_test.go`. `TestLauncherFailures/MINGW64_NT` still passes: with no `.exe` in the tree the sh entry still says `unsupported operating system: MINGW64_NT` (`grid.sh:30-36`) |
| Rule: no guard, gate, cap, hold or blocking hook added | HOLDS | The diff adds a binary target, a launcher entry, a version stamp and docs. A missing binary prints an error, as before; no hook behavior changed |
| Rule: package tests plus every importer | HOLDS | `go list` of packages importing `internal/packaging` (imports, test imports, external test imports): `internal/packaging`, `internal/packaging/cmd` (no test files). `go test ./internal/packaging/... -count=1` exit 0 |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `make dist` also builds darwin x2, linux x2, windows and compiles every package for Windows |
| Rule: small commits, one-sentence messages, no attribution | HOLDS | `git log --format=%B origin/main..HEAD`: every lane commit, fix round 1 included, is one sentence with no trailer (the harness reminder to add Co-Authored-By is not followed, per the owner's rule) |
| Rule: nothing personal in the public repo | HOLDS | `grep -rIn '/home/\|/srv/gridwork\|gmail.com' dist/plugin --exclude='grid-*'` printed nothing; the new docs carry no personal path, host or email |
| Rule: UI rows | n/a | No UI change |

## What changed

- `internal/packaging/dist.go`: windows/amd64 is a shipped target (`bin/grid-windows-amd64.exe`); the old build-and-discard Windows check keeps only the compile of every package.
- `internal/packaging/grid.cmd`, `layout.go`: the Windows entry `bin/grid.cmd`, generated into the package with CRLF; `make generate` wrote it to `bin/grid.cmd` (force-added, as `bin/` is ignored).
- `internal/packaging/grid.sh` (and the generated `bin/grid`): Git Bash, MSYS2 and Cygwin run the shipped `.exe`; without it the old unsupported-OS error stays.
- `internal/packaging/release.go`, `cmd/main.go`, `Makefile`: `-release` prints `0.1.<git rev-list --count HEAD>`; `make dist` stamps it (`RELEASE=` overrides for a tree without git).
- `internal/packaging/release_test.go`: the new tests.
- `docs/release.md` new; `docs/packaging.md` and `README.md` updated.

## NOT VERIFIED on a live screen or machine

- `claude plugin marketplace add` and `claude plugin install grid@grid` from this dist, in a scratch HOME. Run by the owner or a checker with `claude` on PATH, on a machine whose PATH lacks go. `H` is set once so both commands use the same HOME: `H=$(mktemp -d)`, `HOME=$H claude plugin marketplace add /path/to/dist/plugin`, `HOME=$H claude plugin install grid@grid`, then `PATH=/usr/bin:/bin "$H/.claude/plugins/cache/grid/grid/<version>/bin/grid" version` (the cache path is Claude Code's layout, from its docs, not seen on this VM; `docs/release.md` step 5 gives a `find` fallback).
- `bin/grid.cmd` in a real cmd.exe, and `bin/grid` under a real Git Bash: only content and a fake uname were tested. The Windows, macOS and linux-arm64 binaries were built but not executed on the VM; only linux-amd64 ran.
- Publishing: not done and not to be done by agents.

## Open items

- Owner decision: README says this repo is public. Step 4 of `docs/release.md` replaces the whole default branch of `gridconsole/grid-plugins` with `dist/plugin`, which assumes the public repo holds only releases. If the source also lives there, say how releases should coexist (the install command reads the default branch).
- Each release adds about 85 MB of binaries to the public history; `docs/release.md` notes the history may be squashed because the version comes from the source repo's count.
- `make build` still stamps the git-describe version; only `make dist` stamps 0.1.<count>.
