# P-board-store-fix1 (brain-board B1, store side): proof table

Branch `lane-P-board-store-fix1`, from `lane-P-board-store` at `6c6855c5`, merged with origin/main `a6c5cb02`. The code commits are `1aa2cdb5` (the feed facade) and `9ed6a092` (the `board.Board` per-post adapter); this doc is the commit after the merge.

This lane builds the store side of P-board-store, with that brief's rows and Touches unchanged:
- the internal/feed facade over a per-post board, with fold.lock, state.json and needs-you.json in the machine folder;
- the `board.Board` per-post adapter, whose `Claim` holds the board lock;
- rows 4, 5 and 10.

The CLI, MCP, Q3 golden, hooks and digest belong to fix2.

## Acceptance rows

| # | Row | Result | Proof |
|---|---|---|---|
| 1 | Rows 4, 5 and 10 hold, each with a test that fails on lane-P-board-store; row 5 claims from two real processes | HOLDS | See rows 4, 5 and 10 below. On `6c6855c5` with only the new test files: `internal/feed/pb_st_board_test.go:48:54: unknown field Board in struct literal of type Store`, `:48:62: undefined: BoardHome`, `:99:16: s.Home undefined (type Store has no field or method Home)`, `:236:14: s.boardDir undefined`, `FAIL internal/feed [build failed]`; `internal/board/pb_st_claim_test.go:41:28: undefined: PerPost`, `:45:3: unknown field Board in struct literal of type feed.Store`, `FAIL internal/board [build failed]` |
| 4 | V9: the fold reports the current home from the posts, and a task.md whose `home:` names another machine does not change it | HOLDS | `TestPbStBoardHomeFromPostsNotTaskMd` (internal/feed/pb_st_board_test.go:166), through the facade on a task folder. With task.md `home:` B, `Store.Home()` is A, from the Job record. Rewriting task.md to A leaves the state JSON byte-equal. After the owner's `task-move` to B at gen 1, the home is B, and task.md naming A again changes nothing. The fold half of P-board-store row 4 (`TestPbStFoldReportsHomeFromPosts`) still passes |
| 5 | V9: a duplicate claim folds `ignored`; `Claim` twice from two processes gives one post and one `ErrClaimed` | HOLDS | `TestPbStClaimTwoProcesses` (internal/board/pb_st_claim_test.go:159) re-runs the test binary as a child (`TestPbStClaimChild`, :109), so the claimers are real OS processes. It has three cases: this process then a child, a child then this process, and four children at once. Each ends with exactly one `*-J-1-ack.md` post and J-1 Running, and every loser gets `errors.Is(err, ErrClaimed)`. Also `TestPbStClaimInProcess` (:207, 8 goroutines, one winner) and `TestPbStDuplicateClaimFoldsIgnored` (:242): a second ack written past `Claim` with `Post` folds `ignored`, "duplicate claim", and J-1 stays Running |
| 10 | V15: deleting or editing the markdown body leaves the fold result unchanged, through the facade | HOLDS | `TestPbStBoardBodyEditLeavesFold` (internal/feed/pb_st_board_test.go:186) edits the bodies of three posts: a line inserted, the body replaced, and the body deleted down to the header. The fold's state JSON is unchanged on the warm names cache, on a fresh machine folder (cold cache), and after `postfile.Dir.Check`, which reports the 3 edits as findings. The state.json bytes after `SaveState` and the `Read` records are unchanged too |
| 2 | `go test -count=1 -race ./internal/feed/... ./internal/board/...` (postfile included) exits 0 | HOLDS | After the merge, see "Race run" below |
| 3 | No existing test deleted; a pinning test the facade breaks is fixed and listed; no attribution lines | HOLDS | `git diff --stat 6c6855c5 9ed6a092` (the lane's own commits, before the merge) lists 9 files, 1003 insertions and 13 deletions. No file is deleted, and the only `_test.go` files are the two new `pb_st_*` ones. The merge brings in main's own edits to two test files. No pinning test broke: board mode is opt-in through `Store.Board`, and with it nil every JSONL path is base's (feed/board.go `foldFiles` keeps base's lock, state and needs-you paths and writers). The importers' tests below are all `ok`. Attribution: `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0 |

### Race run

`go test -count=1 -race ./internal/feed/... ./internal/board/... ./internal/contract/...`, after the merge:

```
ok  	github.com/gridconsole/grid-plugins/internal/feed	157.310s
ok  	github.com/gridconsole/grid-plugins/internal/board	19.748s
ok  	github.com/gridconsole/grid-plugins/internal/board/postfile	1.783s
ok  	github.com/gridconsole/grid-plugins/internal/contract	42.054s
```

## Rules

| Rule | Result | Proof |
|---|---|---|
| New tests fail on base, failing lines pasted | HOLDS | Row 1 |
| No existing test edited to pass; a deleted test justified | HOLDS | None edited, none deleted (row 3) |
| Files inside Touches (P-board-store's, unchanged) | HOLDS | `git diff --stat 6c6855c5 9ed6a092`: internal/feed/{board.go (new), fold.go, read.go, store.go}, the facade and fold.go:84-184; internal/board/perpost.go (new, the adapter); internal/board/postfile/dir.go (`Scan`, a `List` that does not write the names cache, for `ReadOnly`); internal/contract/fold.go (`ValidBoardPost`, the schema check the facade runs before it publishes); and the two new `pb_st_*` test files |
| Tests of the changed packages and every importer | HOLDS | `go list` of packages importing feed, board, board/postfile or contract: approvefix, artifact, board, board/postfile, brain, braincmd, cli, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, runner, session, settings, signal, task, updates, usage, where, workflow. `go test -count=1` over all of them plus contract: exit 0 on `9ed6a092`, every package `ok` (cli 259 s, orchestrator 587 s, feed 47 s, board 2.9 s), with approvefix, runner and updates `[no test files]`. The merge after it changed only test files in braincmd and orchestrator plus one doc |
| Walk tests, after the merge | HOLDS | After the merge: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` gave `ok internal/cli 0.230s`, and `go test -count=1 ./internal/job/lint/` gave `ok internal/job/lint 1.062s`. Both were also `ok` before the merge |
| `make lint` | HOLDS | exit 0 (`go vet ./...`, no findings) |
| `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok` |
| `git fetch origin && git merge origin/main` before the last proof run | HOLDS | origin/main 5d7c2242..a6c5cb02 (docs/lanes/P-endrev-0812Z-tests.md, internal/braincmd/pb_rd_walk_test.go, internal/orchestrator/tree_orphan_test.go), merged clean; walk tests and race run rerun after it |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The board lock orders this machine's writers for one publish; it is not a lease and refuses no one. `Claim` returns `ErrClaimed` only when a post with the same key is already on the board, which is the brief's row 5. A copy that arrives by sync folds `ignored`; nothing is blocked |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0 |
| Public repo: no personal paths, hosts or emails | HOLDS | The tests use `t.TempDir()` and fixed ULIDs; the doc names only repo paths |
| One command per Bash call, Edit and Write only | HOLDS | No `sed -i`, redirects, tee or `cd` chains |
| The CLI, MCP, Q3 golden, hooks and digest | not run, the brief forbids it | They belong to fix2 |
| `make test` (whole repo) | not run, not asked | `make cross` compiles every package's tests for two OSes; the importer set is run |

## What changed

- internal/feed/board.go (new): the per-post board behind `feed.Store`.
  - `BoardHome{Rel, By}`; a Store with `Board` set serves Post, Read, ReadOnly, State, SaveState, RefoldPublished, SaveStateLiveness, `Home` and `PostUnless` from `board/` in the task folder.
  - The caches (`board.lock`, `board.seq`, `board.cache.json`) and the fold files (`fold.lock`, `state.json`, `needs-you.json`) are in `<machine folder>/tasks/<task id>/`, written with the machine folder's modes.
  - Read sorts by `(seq, id)`, keeps the first copy of an id, and marks a post `unknown` for an unknown type or version, or `held` when its `re` or `after` is missing. The fold reads the header only.
  - `boardPublish` checks the post's schema, then, under the board lock: lists the posts, returns the existing record for a retry of the same id, refuses a different post with that id, returns the id of a post that `held` matches, and publishes.
- internal/feed/fold.go, read.go, store.go: dispatch to the board when `Store.Board` is set. `saveState` takes its three paths and writers from `foldFiles`, which on JSONL are base's.
- internal/board/perpost.go (new): `PerPost{Feed}`, a `board.Board`. `Claim` checks the key, then calls `PostUnless` with `key.holds`, so the look and the publish are one hold of the board lock; when a post holds the key it returns `ErrClaimed` naming that post.
- internal/board/postfile/dir.go: `Scan`, a `List` that leaves the names cache alone, for `ReadOnly`.
- internal/contract/fold.go: `ValidBoardPost`, the fold's schema check for one post.

## Open items

- fix2 must set `Store.Board` from the CLI and MCP, and bump the format (B3). Until then no production caller uses the board.
- On a board, the lease, gate and handover calls of `feed.Store` are not served. Wake times, `For` and `Check` findings are not put into state.json.
- `published()` and `writeProjection()` in internal/feed/fold.go may now be unused. `go vet` does not flag them.
- `Claim` is single-writer per machine. Across machines, two copies that meet after a sync fold the second `ignored` (row 5's fold half), as docs/research/brain-board.md says.

## NOT VERIFIED

- Nothing in this lane has a screen. No CLI or app reads a board yet, so the facade is seen only through Go tests.
- The modes of `fold.lock`, `state.json` and `needs-you.json` in a shared machine folder under two logins.
- The two-process claim on Windows and macOS. `make cross` compiles the test there; it ran only on Linux.

DONE
