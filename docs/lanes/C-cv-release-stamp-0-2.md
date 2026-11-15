# C-cv-release-stamp-0-2: make dist stamps 0.2.<commit count>

0.2.<count> sorts above the public 0.2.0 for any count of 1 or more; the next re-apply to the five logins moves them from 0.1.N to 0.2.N, which `claude plugin update` reads as newer.

## Proof table

Code commit: "make dist stamps the plugin 0.2.<commit count>..." (count at that commit: 3374; the lane doc commit after it makes the count 3375).

| Row | Result |
|---|---|
| 1. `-release` prints 0.2.<n>, n = `git rev-list --count HEAD` | HOLDS. Before the code commit: `go run ./internal/packaging/cmd -release` printed `0.2.3373`, `git rev-list --count HEAD` printed `3373` (both exit 0). After the code commit `make dist` ran `-dist -version "0.2.3374"`. |
| 2. Red on base, green after | HOLDS. With release_test.go:52 at `"0.2.1221"` and release.go unchanged: `go test -count=1 ./internal/packaging -run 'TestCommitVersion$'` exit 1: `--- FAIL: TestCommitVersion` / `release_test.go:53: got "0.1.1221"`. After the change, `go test -count=1 -v` prints `--- PASS: TestCommitVersion`. |
| 3. `make dist` exit 0, three manifests, binary | HOLDS. `make dist` exit 0 (five platforms built, "Building grid 0.2.3374 for ..."). `grep -h '"version"' dist/plugin/.claude-plugin/plugin.json dist/plugin/.codex-plugin/plugin.json dist/plugin/.claude-plugin/marketplace.json` printed three lines `"version": "0.2.3374"` (exit 0). The sandbox refused running `dist/plugin/bin/grid-linux-amd64 version` directly; `go test -count=1 -v ./internal/packaging -run TestDistPluginRunsWithoutGo` runs that same binary through the bin/grid entry with PATH=/usr/bin:/bin, passes (exit 0) and prints `grid 0.2.3374` as the first line (it asserts the prefix `grid <manifest version>\n`, release_test.go:192). |
| 4. `git grep -n -E '0\.1\.(<\|%d\|N\|\$N)' -- . ':!docs/lanes'` | HOLDS. Prints nothing (exit 1 = no match). |
| 5. Four tests PASS; package exits 0 | HOLDS. `go test -count=1 -v ./internal/packaging/... -run 'TestCommitVersion\|TestReleaseVersionCountsCommits\|TestVersionStampedInEveryManifest\|TestReleasePrintsCommitCountOfGitRepo'` printed `--- PASS:` for all four, exit 0. `go test -count=1 ./internal/packaging/...` exit 0 (packaging ok, packaging/cmd ok). |
| 6. Lane doc sentence | HOLDS: first paragraph above. |
| 6. `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| 6. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64: vet and test compile). Windows and macOS builds covered. |
| 6. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0. |
| 6. `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0. |
| Packages that import the changed ones | HOLDS. `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...`: only `internal/packaging/cmd` imports `internal/packaging`; nothing imports `internal/packaging/cmd`. Both tested (row 5). |
| `git fetch origin && git merge origin/main` before the last run | HOLDS. `Already up to date.` |
| Walk tests (plugin) | HOLDS. Same commands as row 6, run after the merge, exit 0. |
| App-repo walk tests, guest walk tables | not run, not asked (plugin repo lane). |
| Whole-module test, cross-repo set | not run, not asked (run by the orchestrator at landing). |
| New tests fail on base, pass on lane | HOLDS. No new test; the two existing pins were changed in place (release_test.go:52, cmd/main_test.go:92); the red run is row 2. |
| Touches | HOLDS. Changed: release.go, cmd/main.go, release_test.go (:52, :57 only), cmd/main_test.go (:92 only), Makefile:3, README.md:10, docs/packaging.md:41, docs/release.md (:14, :61, :71, :93, :115, :116, :122, :124), this doc. `git status --short` clean after `make dist` (dist/ is ignored). |
| No guard, gate, cap or hold added; nothing pushed | HOLDS. |

## What changed

- `internal/packaging/release.go`: `CommitVersion` formats `0.2.%d`; comment says 0.2.<commit count>.
- `internal/packaging/cmd/main.go`: `-release` flag text says 0.2.<commit count>.
- `release_test.go` TestCommitVersion wants `0.2.1221`; `cmd/main_test.go` TestReleasePrintsCommitCountOfGitRepo wants `0.2.2\n`.
- Text lines 0.1 to 0.2: Makefile, README.md, docs/packaging.md, docs/release.md.
- `internal/version/version.go` (`Version = "0.1.0"`, `AppMin`) and release_test.go:28 are left as the brief says.

## NOT VERIFIED on a live screen

- `claude plugin update` itself picking up 0.2.N over 0.1.N on a login: not run (no claude in the sandbox, brief forbids it).
- Running `dist/plugin/bin/grid-linux-amd64 version` by hand: refused by the sandbox; covered by TestDistPluginRunsWithoutGo above.
