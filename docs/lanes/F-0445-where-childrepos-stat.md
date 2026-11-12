# F-0445-where-childrepos-stat

`childRepos` (internal/where/where.go) now decides a child folder is a repo by a stat of `<dir>/.git`: a folder, or a file whose first bytes are `gitdir:`. It starts no git process. Result order (sorted) and the two-level depth are unchanged.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. 200 child repos (`.git` dirs and `gitdir:` files), PATH = empty dir, found in under 1 s | HOLDS | `go test -count=1 ./internal/where/ -run TestChildReposStatNoGit -v` exit 0: `--- PASS: TestChildReposStatNoGit (0.01s)` (internal/where/childrepos_stat_test.go) |
| New test fails on base | HOLDS | with base where.go: `childrepos_stat_test.go:64: childRepos found 0 repos, want 200` / `FAIL` (exit 1) |
| 2. `go test -count=1 ./internal/where/` passes, RuleMultiRepo tests unchanged | HOLDS | `ok  github.com/gridconsole/grid-plugins/internal/where 1.218s`, exit 0; no existing test edited |
| 3. `grid hook sessionstart` from /srv/gridwork under 2 s, plus loadavg | NOT DONE (binary run) | `make build` exit 0 (binary at `dist/grid`). Running the binary, `time` and reading /proc/loadavg were refused in this session. Stand-in, not the same thing: a throwaway test (deleted, not committed) timed `childRepos("/srv/gridwork")`: `3743 repos in 27.891498ms`. The checker should run `time dist/grid hook sessionstart </dev/null` from /srv/gridwork. |
| 4. `make lint`, `make cross` | HOLDS | `make lint` (go vet) exit 0; `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0 |
| Packages importing where | HOLDS | go list importers: braincmd, cli, guard, hook, job/jobcmd, job/jobtest, land, projectmd, projects, question, settings, signal, task, updates, workflow. `go test -count=1` on all: every one `ok` (jobtest and updates have no test files), exit 0 |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| No guard, gate, cap, hold or hook added | HOLDS | diff touches only where.go (childRepos) and the new test |
| Touches only the brief's files | HOLDS | where.go, childrepos_stat_test.go, this doc |
| No attribution lines in commits | HOLDS | one commit, one-sentence message |

## What changed

- `hasGitMarker(dir)` added; `childRepos` uses it instead of `gitx.IsRepoRoot`.
- A folder with a `.git` entry that is a file not starting `gitdir:`, or an empty file, is not a repo.
- Behaviour difference to know: `IsRepoRoot` also rejected an empty `.git` folder or a dangling worktree pointer (git said so). The stat does not; the brief asks for that trade.

## NOT VERIFIED

- Row 3 wall time of the real hook from /srv/gridwork (see above).
