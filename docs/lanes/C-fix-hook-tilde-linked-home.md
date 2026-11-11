# C-fix-hook-tilde-linked-home

Base: plugin cd84f424 (lane-Pint58 merge; main 3a3d99ea is an ancestor of the line). Why lines read again on this base: `internal/where/where.go:106` still has `gridHome := gridhome.Real(r.GridHome)`; `internal/hook/hook.go:177,180,219,242,246,248` still call `gridhome.Tilde(..., Resolver.Home)`; `Tilde` matched only the literal prefix. Nothing moved; the cause shows.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Job lines in a linked root, fail on main | HOLDS | `internal/hook/sessionstart_linked_home_test.go:61` `TestSessionStartLinkedHomeJobLinesUseTilde`. On base (fix stashed): `go test ./internal/gridhome/ ./internal/hook/ -run 'Linked\|TildeUnder' -count=1` exit 1, `sessionstart_linked_home_test.go:68: missing "Your brief: ~/data/projects/example/tasks/T-07-example/jobs/J-2/brief.md (or: grid job show T-07-example J-2)." in: ... Grid task: T-07-example in project example, at /tmp/TestSessionStartLinkedHomeJobLinesUseTilde3556210588/001/real/data/projects/example/tasks/T-07-example.` and `Your brief: /tmp/.../001/real/data/projects/...`. With the fix: same command, exit 0, both pass. |
| 2. Two linked roots, same context; symlink tests skip on failure | HOLDS | `sessionstart_linked_home_test.go:73` `TestSessionStartLinkedHomeContextDoesNotDependOnTheRoot`. On base: `sessionstart_linked_home_test.go:76: two linked roots gave different contexts` (`.../001/real/data/...` against `.../003/real/data/...`). With fix: exit 0. Every `os.Symlink` here (`sessionstart_linked_home_test.go:24`, `tilde_linked_home_test.go:19`) skips with `t.Skipf("cannot make a symlink here: %v", err)`. |
| 3. tilde_linked_home_test.go cases | HOLDS | `internal/gridhome/tilde_linked_home_test.go:25-42` has real/x -> ~/x, real -> ~, link/x -> ~/x, other/x unchanged, empty home unchanged, `real+"-other/x"` unchanged, plus link itself -> ~ and real/a/b -> ~/a/b. Base: three subtests fail (resolved path under linked home, resolved home itself, deeper resolved path). Fix: exit 0. |
| 4a. Caller packages | HOLDS | `go test ./internal/gridhome/ ./internal/hook/ ./internal/agent/ ./internal/cli/ ./internal/job/jobcmd/ ./internal/doctor/ -count=1` exit 0, run as part of the importer set below (all `ok`). |
| 4b. Importers of `gridhome` (go list) | HOLDS | Direct importers from `go list` Imports: account agent braincmd cli doctor gitx guard hook job/jobcmd land localdata migrate ports project projects routine settings signal task where worker workflow. `go test` of those plus gridhome, `-count=1`: exit 0, 23 packages `ok`. Packages that reach gridhome only through those are left to the orchestrator's whole-module run. |
| 4c. `TMPDIR=<symlinked dir> go test ./internal/hook/ -run SessionStart` | NOT DONE | This shell refuses `ln`/`mkdir` and reads outside the clone, so I could not make a symlinked temp dir. The Linux link reproduction in rows 1-3 (root is a symlink to a real folder in another temp dir, the same shape as /var -> /private/var) covers the same behaviour; the unchanged `go test ./internal/hook/` passes with the fix (exit 0). The orchestrator can run the TMPDIR line. |
| 5a. `make lint` | HOLDS | `make lint` (runs `go vet ./...`) exit 0; `gofmt -l internal/gridhome internal/hook` prints nothing. |
| 5b. darwin/windows vet | HOLDS | `GOOS=darwin go vet ./internal/gridhome/ ./internal/hook/` exit 0; `GOOS=windows go vet ./internal/gridhome/ ./internal/hook/` exit 0. Also `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0. |
| Rule: new tests fail on base, pass on lane | HOLDS | Rows 1-3 lines above. |
| Rule: no existing test edited or deleted | HOLDS | `git diff --stat origin/main..` touches only gridhome.go and two new test files. `hook_test.go` and `sessionstart_job_test.go` untouched. |
| Rule: no guard, gate, cap on agents | HOLDS | Only `Tilde` changed; it spells a path. |
| Rule: hook.go untouched, one function changed | HOLDS | `internal/gridhome/gridhome.go` `Tilde` plus a small helper `tildeUnder`. |
| Rule: commit message, no attribution | HOLDS | One commit f0db1040, one sentence, no attribution lines. |

## What changed

- `internal/gridhome/gridhome.go`: `Tilde` tries the literal home first (same cost as before); only when that fails and `Real(home)` differs from home does it try the resolved home. `path == home` gives `~` for both spellings. A path under neither stays as it is. New helper `tildeUnder` holds the prefix match. No caller changed, so hook.go, cli, agent, doctor, projects, task and jobcmd gain the fix.
- New: `internal/gridhome/tilde_linked_home_test.go`, `internal/hook/sessionstart_linked_home_test.go` (own fixture, linked root).

## NOT VERIFIED on a live screen / host

- macOS host run: for the orchestrator: `go test ./internal/hook/ -run 'TestSessionStart' -count=1 -v`.
- Row 4c TMPDIR line, see above.
- Windows: the link tests skip when `os.Symlink` fails; not run on Windows.
