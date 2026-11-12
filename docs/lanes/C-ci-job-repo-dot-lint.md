# C-ci-job-repo-dot-lint: a Job with `repo: "."` passes lint when the project lists named repos

Base: plugin main 23867517. Base check: `jobRepo` (internal/job/lint/lint.go:390-400 on base) compared the raw name with each entry of repos and had no case for "."; `RepoFolder` (internal/job/jobcmd/command.go:855,861-862) cleans the name and takes "." as `f.Folders[0]`. So the gap was real.

## Proof table

| Row | Result |
|---|---|
| 1 red on base | HOLDS. On base, `go test -count=1 -v -run TestCiDot ./internal/job/lint/` printed (exit 1): `ci_dot_repo_lint_test.go:17: repo "." has findings: [{Rule:job.repo ... Message:Repo "." is not a repo of the project; it has web ...}]`, the same for `"./"`, `--- FAIL: TestCiDotRepoPassesLint`. |
| 1 green | HOLDS. `go test -count=1 -v -run 'TestCiDot' ./internal/job/lint/` exit 0: `--- PASS: TestCiDotRepoPassesLint` (subtests `.` and `./` PASS). |
| 2 | HOLDS. `--- PASS: TestCiDotUnknownRepoStillRefused` (one `job.repo` finding, message holds "nope" and "web"). |
| 3 | HOLDS. Base: `ci_dot_repo_lint_test.go:57: repo ".": 1 job.repo findings, want 0 (options repos [web])`. Lane: `go test -count=1 -v -run 'TestCiDot' ./internal/job/jobcmd/` exit 0: `--- PASS: TestCiDotProjectRecordLint` (`prRecord(t, e, ".", "web")`, `LintOptions(prTask(e), e)`; "." 0, "web" 0, "nope" 1). |
| 4 | HOLDS. `go test -count=1 -v -run 'TestVcRepo\|TestRootsRepoPaths' ./internal/job/lint/ ./internal/job/jobcmd/` exit 0, no FAIL line (the Windows subtest SKIPs as on base). `w.repos` is untouched, so the subtest at project_record_test.go:305 still holds. |
| 5 | HOLDS. `git grep -n 'project folder' -- docs/job-contract.md` shows docs/job-contract.md:90. `go test -count=1 -run TestHfJr ./internal/job/` exit 0. |
| 6 make lint | HOLDS. `make lint` (go vet ./...) exit 0. |
| 6 make cross | HOLDS. `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| 6 walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0. |
| 6 job tests | HOLDS. `go test -count=1 ./internal/job/...` exit 0 (job, frontmatter, jobcmd, lint ok). |
| 6 diff | HOLDS with the lane doc. `git diff main --stat`: docs/job-contract.md, internal/job/jobcmd/ci_dot_repo_lint_test.go, internal/job/lint/ci_dot_repo_lint_test.go, internal/job/lint/lint.go, plus this lane doc. |
| importers of the changed package | `go list` importers of internal/job/lint: internal/cli, internal/job/jobcmd. `go test -count=1 ./internal/job/... ./internal/cli/...` exit 0. |
| main moved | `git fetch origin && git merge origin/main`: Already up to date. |
| gofmt | `gofmt -l internal/job` prints nothing. |
| No guard/gate/cap | HOLDS. Lint accepts one more name; nothing is blocked that was allowed, no hook, no agent limit. |
| App-repo checks (webui-test, realplugin) | not applicable, plugin repo. |
| Mismatch proof | not asked by the brief. |

## What changed

- `internal/job/lint/lint.go` `jobRepo`: cleans the name with `filepath.ToSlash(filepath.Clean(name))` as `RepoFolder` does; a cleaned "." returns; a cleaned name in repos returns; anything else gives the same `job.repo` finding, quoting the name as written. The comment says "." is the project folder even when project.md lists only named repos, as `RepoFolder` takes it. A side effect, per the brief's rule to clean the name: `web/` now passes like `web`.
- `docs/job-contract.md:90`: `.` names the project folder and always passes.
- Two new test files with `TestCiDot*`.
- `w.repos` and `roots` unchanged; the refusal text and the post, render and lint paths share `LintOptions`, so all take "." now.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: a real `grid job post` against a real project.md (the jobcmd test goes through `LintOptions`, the function the post path calls).
