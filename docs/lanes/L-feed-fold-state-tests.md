# L-feed-fold-state-tests

Head: see the bundle (one test commit, one report commit). New file only: `internal/feed/fold_state_test.go`. `fold.go` is unchanged.

## Proof table

| Row / rule | Result |
|---|---|
| 1. posts/FoldRead admit only counted and stale, stale marked `Stale: true` | HOLDS. `TestFoldReadAdmitsOnlyCountedAndStale` (`fold_state_test.go`), calls `posts` and `FoldRead` on a real read with one counted, one held, one ignored, one stale entry. `TestPostsAdmitsUnknownEntry` covers `unknown` admitted and `bad` left out. `go test -count=1 -race ./internal/feed/` exit 0. |
| 2. SaveState writes `state.json` and `needs-you.json` that decode to the returned State and NeedsYou | HOLDS. `TestSaveStateWritesProjections`, includes an open question so NeedsYou is not empty, and checks `.cache/fold.lock` exists. exit 0. |
| 3. SaveStateLiveness: `liveness` on J-1 only, J-9 not added, returned State and `State()` carry none, fold unchanged | HOLDS. `TestSaveStateLivenessOverlaysProjectionOnly`: one `liveness` in `state.json`, J-2 has none, J-9 absent, returned State and `State()` marshal equal to the plain fold, `needs-you.json` unchanged. exit 0. |
| 4. writeProjection leaves unchanged files untouched, a new post changes `state.json` | HOLDS. `TestSaveStateLeavesUnchangedProjectionsAlone`: mtimes set to 2001, second SaveState keeps both, a new Job post rewrites `state.json` and leaves `needs-you.json` alone. exit 0. |
| 5. Eight concurrent SaveState calls | HOLDS. `TestSaveStateConcurrent`: eight goroutines released together all return J-1..J-3 and leave a decodable `state.json`. Run with `-race`, exit 0. |
| 6. `go test -count=1 -race ./internal/feed/` | HOLDS. `ok ... internal/feed 21.037s`, exit 0. |
| New tests fail when the behaviour breaks | HOLDS. The tests only cover behaviour already on base, so they cannot fail on base without a change to `fold.go`. Mutation proof, each reverted with `git checkout` (fold.go clean): (a) `writeProjection` always rewrites: `fold_state_test.go:273: state.json rewritten with no new post: mtime 2026-11-11 02:06:43...`; (b) `posts` also admits `held`: `fold_state_test.go:84: held entry reached the fold`. |
| No existing test edited or deleted | HOLDS. `git diff origin/main --stat` shows one added test file and this report. |
| No guard, gate, cap, hold or blocking hook added | HOLDS. Test file only. |
| Tests of changed package and importers | HOLDS for `./internal/feed/` (exit 0, above). Importers from `go list`: cli, hook, job/jobcmd, lease, mcp, orchestrator, question, task, usage (see the importer row at the end of this file). |
| Lint | HOLDS. `go vet ./internal/feed/` exit 0; `gofmt -l internal/feed/` prints nothing. The `make lint` form (`go vet ./...`) is left to the orchestrator at landing. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No personal paths, hosts or emails in the public repo | HOLDS. The test file holds none. |

Importer run, `go test -count=1` on the nine packages, exit 0:

```
ok  internal/cli 107.341s
ok  internal/hook 7.251s
ok  internal/job/jobcmd 5.071s
ok  internal/lease 1.473s
ok  internal/mcp 3.534s
ok  internal/orchestrator 101.175s
ok  internal/question 6.897s
ok  internal/task 7.250s
ok  internal/usage 0.031s
```

## Not tested here

`State` using `Store.Now` is already covered by `TestPersonGateDeadlineWithoutControl` (`questions_test.go:101-105`), so this lane adds no second test for it.

## Bugs found

None.

## What changed

- `internal/feed/fold_state_test.go`: five behaviour tests plus one small `posts` test, post records through the Store as `store_test.go` does.
- `docs/lanes/L-feed-fold-state-tests.md`: this table.

## NOT VERIFIED on a live screen

Nothing here reaches a screen; the tests read `state.json` and `needs-you.json` from disk only.
