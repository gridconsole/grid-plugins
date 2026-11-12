# C-xh3-post-refold-touches-fix1

Verdict: DONE. The two app tests that ejected C-xh3-post-refold-touches now pass unchanged against this lane's binary.

Base: lane-C-xh3-post-refold-touches, then `git merge origin/main`. The merge had no conflict, so no file was resolved by hand. Code head: d7490671 (the report commit follows it).

## Cause

The lane refolds after every post (`cli/feed.go`, `cli/orchestrator.go`, `mcp/tools.go`). `SaveState()` always writes `feed/state.json` and `feed/needs-you.json`. The app test `cmd/grid-server/gi_post_textless_kinds_realplugin_test.go` (`lines()`, `:99-118`) reads every file of the task's `feed/` folder as feed lines and wants exactly one new line per post (`:149-150`). On a fresh task the first post created both files, so the test saw 3 lines: the post, `[]` and the state object.

Before the fix, quoted from the app clone (`make realplugin ... RUN='TestRealPluginGiPostKindOwner|TestRealPluginGiPostKindMember'`):

```
realplugin: pass 0 fail 2 skip 0
fail .../cmd/grid-server TestRealPluginGiPostKindOwner: ...
fail .../cmd/grid-server TestRealPluginGiPostKindMember: ...
realplugin: no test passed
```

## Fix (in the lane's own change)

`feed.Store.RefoldPublished()` (`internal/feed/fold.go`) is the refold the three post paths now call. It keeps `SaveState`'s liveness and refreshes only what a reader could trust: `state.json` if the file is already there, `needs-you.json` if the file is there or the fold has a question to list. A task that has published nothing and has no open question gets no new file. Nothing becomes stale: the app reads `needs-you.json` and, when it is absent, folds through the runner (`internal/slices/gates/model.go:395-420` in the app). `SaveState()` and `SaveStateLiveness()` are unchanged, so `grid feed state`, `question add` and the tick still write both files.

The lane's behaviour stays: an answer, Job or result that opens or closes a question shows without a tick (`TestXhJobPostRefolds`, `TestXhFeedPostOpensQuestionAtOnce` pass unedited), and `refold: ` still prints on a fold error (`TestXhFeedPostRefoldError`).

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | App: `make realplugin GRID_PLUGIN_BIN=/srv/gridwork/build/C-xh3-post-refold-touches-fix1/dist/grid RUN='TestRealPluginGiPostKindOwner\|TestRealPluginGiPostKindMember' PKGS=./...` | HOLDS | Last line: `realplugin: pass 2 fail 0 skip 0`, exit 0. Binary: `dist/grid` built by `make build` after the fix. The wider `RUN='TestRealPlugin'` run: `realplugin: pass 329 fail 0 skip 0`. |
| 2 | `go test ./... -count=1` in the plugin clone | HOLDS | No non-ok line, exit 0. Importers of feed, cli and mcp are inside `./...`. |
| 3 | `make lint` | HOLDS | `go vet ./...`, exit 0. |
| 3 | `make cross` | HOLDS | darwin/arm64 and windows/amd64 vet and test compile, no error line, exit 0. |
| 4 | `git diff origin/main --stat` names only lane files plus step 4 files | HOLDS | Lane files: `cli/feed.go`, `cli/orchestrator.go`, `feed/fold.go`, `mcp/tools.go`, the lane's tests and docs. Step 4 adds only `internal/cli/xh_post_lazy_test.go` and the three call sites and `fold.go` edit above, all files the lane already touched. |
| R | New test fails on base, passes on the lane | HOLDS | `TestXhPostMakesNoProjectionFile`. On the lane head without the fix: `xh_post_lazy_test.go:28: post 0 made feed/state.json on a task with nothing published and no question`. With the fix: PASS. `TestXhPostRefreshesPublishedFiles` passes on both: it pins that a published `state.json` is still refreshed by a post. |
| R | No existing test edited or deleted; app clone not edited | HOLDS | `git diff origin/main --stat` shows only added test files and one 4-line addition to `intent_release_test.go` from the lane. |
| R | No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. No lock is taken while Post's locks are held; `fold.lock` is taken alone, as before. |
| R | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok`, exit 0. |
| R | `go test -count=1 ./internal/job/lint/` | HOLDS | `ok`, exit 0. |
| R | Commits: one sentence, no attribution, no personal paths | HOLDS | `git log -2`. |
| R | Remove `/srv/gridwork/build/C-xh3-post-refold-touches-fix1-app` | SEE NOTE | `rm` is not on the allow list; see open items. |

## What changed

- `internal/feed/fold.go`: new `RefoldPublished()` and a private `published()`; `saveState` takes `onlyPublished`.
- `internal/cli/feed.go`, `internal/cli/orchestrator.go`, `internal/mcp/tools.go`: call `RefoldPublished()` instead of `SaveState()` after a post.
- `internal/cli/xh_post_lazy_test.go`: two new tests.

## NOT VERIFIED

- Live screen: Home, Needs you right after an answer, as in the parent lane. A task that never ran a tick and has no open question has no `needs-you.json`; the page then folds through the runner. That path is the app's existing one and is proved only by the real-plugin tests above.

DONE
