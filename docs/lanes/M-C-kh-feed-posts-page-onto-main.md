# M-C-kh-feed-posts-page-onto-main: Feed posts page carried onto plugin main f6383e8c

## Cause
After `git merge origin/main`, `go test -count=1 -v -run TestT12PostsEveryType ./contract` failed:

```
t12_feed_posts_test.go:154: sections missing, repeated, with a wrong line or not a post type: [found]
--- FAIL: TestT12PostsEveryType (0.09s)
FAIL	github.com/gridconsole/grid-plugins/contract	0.100s
```

Main's commit e2c5ab3d ("Let a Job agent with role worker or agent author a found record", changing `contract/schemas/found.json`, which now lists `reviewer, lead, worker, agent, cli` in `x-roles`) changed the roles on purpose. The page still said `reviewer, lead, cli`.

## Fix
`contract/docs/feed-posts.md`: the `found` line is now `Version 1. Class chatter. Written by reviewer, lead, worker, agent, cli.` and the prose says a Job agent with role worker or agent may author one. No test, row, post type or error code was dropped or changed, and no existing test's expected value was changed. Commit 44a529df.

## Proof table
| Row | Result |
|---|---|
| 1. `git merge-base --is-ancestor origin/main HEAD` | HOLDS, exit 0 |
| 2. `go test -count=1 ./contract/` | HOLDS, `ok contract` (failed after the merge alone, see Cause) |
| 3a. C-kh row 1-3: `go test -count=1 -v -run TestT12Posts ./contract` | HOLDS, exit 0: PASS TestT12PostsEveryType, TestT12PostsPreface, TestT12PostsExamplesValidate, TestT12PostsNegative |
| 3b. C-kh row 4: diff touches only the two new files | HOLDS: `git diff origin/main --stat` lists only contract/docs/feed-posts.md and contract/t12_feed_posts_test.go |
| 3c. C-kh row 5: `make lint` and `make cross` | HOLDS, both exit 0 (cross: darwin/arm64 and windows/amd64 vet and test compile, all ok) |
| 4. Walk tests (plugin) | HOLDS: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| 5. `make lint`, `make cross` | HOLDS, exit 0 |
| Packages that import `contract` (go list): only `internal/contract` | HOLDS: `go test -count=1 ./contract/... ./internal/contract/ ./internal/cli/ ./internal/feed/ ./internal/job/lint/` all ok |
| New tests fail on base, pass on lane | HOLDS: the merged head failed TestT12PostsEveryType (quoted above), the lane head passes |
| No guard, gate, cap or hold added | HOLDS: docs only |
| Nothing removed, no push | HOLDS |
| No attribution lines in commits | HOLDS |
| `*RealPlugin*` tests | Not in play for this lane |
| App-only walks (`make webui-test`, grid-server) | Not applicable to the plugin repo |

## NOT VERIFIED on a live screen
Nothing; the lane is docs and a Go test.
