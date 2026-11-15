# C-s6b-brain-search-misses-note

Brief: finish C-s6 with the r8 task files readable (end review 0212Z S6). This is a work-in-progress hand-off. The budget of about 40 calls ran out.

## Evidence from /srv/gridwork/tmp/r8-T01

- `sessions/` holds no transcript, only `.hooks/*.json`. The J-1 record, `.hooks/f619643f-….json`, names its transcript under `…/.claude/projects/-…-r8-grid-home-worktrees-01M7EDXBM4NR2GFFBAFPEXWNM1-J-1-a1/`. So J-1 ran from a worktree inside the Grid home (`<grid home>/worktrees/<task ULID>-J-1-a1`). The transcript itself is outside /srv/gridwork, and reading it was refused, so J-1's flags are not known. The words come from history.md:3: `install page handbook links commands`. The hook record parses the command with no `--project` or `--task`. If either flag had been given, `hdSSameProject` would have had to match it before the hook wrote a read.
- feed `worker@…3.jsonl` J-1 result: "Not checked: the brain note command-blocks did not show in brain search."
- Cause (C-s6 candidate 1, scope; confirmed in code): `where.Resolver.implicit` (internal/where/where.go:138) resolves any folder inside the Grid home that is not a task folder to Personal. So the CLI searched the home level only, or was refused Personal. The hook takes the project from GRID_TASK, else GRID_FEED_TASK (internal/hook/events.go:165-180), which is `handbook`, so it found the note. The CLI ignored GRID_TASK when it resolved the project (internal/braincmd/command.go `Run`; internal/cli/brain.go passed GRID_TASK only for authorship).

## Change (work in progress)

- internal/cli/brain.go: passes `SessionTask` (GRID_TASK, else GRID_FEED_TASK, the order the hook uses).
- internal/braincmd/command.go: when neither `--project` nor `--task` is given, the session task picks the project if it resolves and either differs from the cwd's project or the cwd fails. The cwd's task folder is not taken from it (`TaskPath` cleared), so `propose` and `keep` keep their folders.
- internal/cli/s6_brain_session_task_test.go: `TestS6BrainSearchUsesSessionTaskProject`. It uses the real note front matter, the real words, no flags, a cwd of `<grid home>/worktrees/01M7EDXBM4NR2GFFBAFPEXWNM1-J-1-a1`, and GRID_TASK or GRID_FEED_TASK set.

Run on base (before the fix): `go test -count=1 -run TestS6BrainSearchUsesSessionTaskProject ./internal/cli/` exit 1:
```
s6_brain_session_task_test.go:43: brain search from the Job worktree with GRID_TASK: exit 1 out "" err "brain: this caller cannot use the Personal project\n"
s6_brain_session_task_test.go:43: brain search from the Job worktree with GRID_FEED_TASK: exit 1 out "" err "brain: this caller cannot use the Personal project\n"
```
Run with the fix: still exit 1, now resolved but empty:
```
brain search from the Job worktree with GRID_TASK: exit 0 out "hits: []\nquery: install page handbook links commands\n" err ""
```
The Personal refusal is gone. Why the test fixture's note (written at `<grid home>/projects/example/knowledge/conventions/command-blocks.md`) gets no hits is not found yet. It is either the fixture's place or front matter, or a second cause in search.

## Proof table

| Row | State | Evidence |
|---|---|---|
| Acceptance 1: repro command and output before and after | NOT DONE | The before run is above (exit 1, Personal refused). The after run still misses. The knowledge copy into the clone's tmp (`cp -r`) was refused, so the repro is the test fixture. |
| Acceptance 2: new test fails on main, passes on lane; `make lint` and brain tests pass | NOT DONE | The test fails on base (above) and still fails on the lane (hits: []). Lint and the brain tests were not run. |
| Acceptance 3: lane doc | HOLDS | docs/lanes/C-s6b-brain-search-misses-note.md |
| Rule: tests of changed packages and importers | NOT DONE | Not run, out of budget. |
| Rule: walk tests, merge origin/main, Windows/macOS builds | NOT DONE | Not run, out of budget. |
| Rule: no guard, gate, cap, hold or blocking hook added | HOLDS | The change only widens which project a read resolves to. |

## Open items

1. Find why the fixture note gets no hits once the project resolves. Next step: run `grid brain search --project example …` in the same test to separate the fixture from scope.
2. Then: importers' tests (`go list` for internal/braincmd and internal/cli), `make lint`, GOOS=windows and GOOS=darwin builds, `git merge origin/main`, the plugin walk tests.

## NOT VERIFIED on a live screen

Nothing; no UI.

CLOSED BY C-s6c-brain-search-misses-note: accepted 03:31 UTC
