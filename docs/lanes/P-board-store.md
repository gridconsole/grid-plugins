# P-board-store (brain-board B1): proof table

Branch `lane-P-board-store`, base `91f210e5`, merged with origin/main `5d7c2242`. The code head is `3fc8238b`; this doc is the commit after it.

The lane stopped building at the 85% budget mark, as the brief says. The per-post file package and the board fold rules are built and tested. The feed facade, the `board.Board` adapter with `Claim`, the CLI and MCP changes, the hooks and the quiet-task digest are not built. The rows that depend on them are NOT DONE below.

## Acceptance rows

| # | Row | Result | Proof |
|---|---|---|---|
| 1 | V9: a fold on a migrated home sorts `(gen, seq, id)` | HOLDS | `go test -count=1 ./internal/contract/` exit 0; `TestPbStFoldSortsGenSeqID` (internal/contract/pb_st_fold_test.go:77), where each pair sorts the other way under base's `(gen, fence, worker, seq)` and three input orders fold the same |
| 2 | V9: a control counts only from the home of its `gen`, and a non-home control is a request | HOLDS | `TestPbStFoldControlCountsOnlyFromHome` (pb_st_fold_test.go:122): an assign from machine B and a person's assign on the home machine both fold `counted` and move nothing; the cli assign from home A moves the Job. Rule: internal/contract/fold.go `authority` and `boardController`, following docs/research/brain-board.md:103 |
| 3 | V9: after a `task-move`, a refold still counts the old home's controls for the old gen (F10) | HOLDS | `TestPbStFoldTaskMoveKeepsOldGen` (pb_st_fold_test.go:153): a gen-0 assign from A counts after the move to B, A's gen-1 assign is a request, B's gen-1 assign counts, and three input orders give the same result |
| 4 | V9: the fold reports the current home from the posts, and a task.md whose `home:` names another machine does not change it (decision point 4) | NOT DONE | Half proven: `TestPbStFoldReportsHomeFromPosts` (pb_st_fold_test.go:188) shows `Loop.Home` is A, then B after a team `task-move`, and that a guest's move changes nothing. `FoldBoard` takes posts only, so it cannot read task.md. But the task.md half needs the feed facade that reads a task folder, and that is not built, so it has no test |
| 5 | V9: a duplicate claim folds `ignored`; `Claim` twice from two processes gives one post and one `ErrClaimed` | NOT DONE | Fold half: `TestPbStFoldDuplicateClaimIgnored` (pb_st_fold_test.go:211); the second started folds `ignored`, "duplicate claim", in every order, and different firings both count. The `Claim` half (the `board.Board` per-post adapter under the board lock, plus the two-process test) is not built |
| 6 | V9: `legacy` verdicts are honoured, and a JSONL home folds exactly as on base | HOLDS | `TestPbStFoldLegacyVerdicts` (pb_st_fold_test.go:238): legacy counted keeps its authority off-home and without the side match, stale stays stale, ignored is folded again. `TestPbStJSONLFoldsAsOnBase` (internal/contract/pb_st_jsonl_fold_test.go): seven JSONL scenarios match internal/contract/testdata/pb_st_jsonl_fold.golden.json, which base `91f210e5`'s `FoldLoop` wrote (`go -C <base worktree> test ... -pbst.jsonl.update`, exit 0). Regression row: it passes on base by design (gate note 5) |
| 7 | V15: the header round-trips byte-exactly (exit code `2.0`, a 64 KB body) | HOLDS | `go test -count=1 ./internal/board/...` exit 0; `TestPbStHeaderRoundTripsByteExactly` and `TestPbStHeaderMatchesQ8Example` (internal/board/postfile/pb_st_test.go) |
| 8 | V15: 30 concurrent goroutines give unique names, published by link | HOLDS | `TestPbStThirtyGoroutinesUniqueNamesByLink` (pb_st_test.go), with `TestPbStExistingNameTakesNextSeq` for EEXIST |
| 9 | V15: the body is the Q8 rendering: a golden per post type, path-qualified links, the same bytes twice, an unknown `re` as the id | HOLDS | `TestPbStRenderGoldenPerType` (12 goldens in internal/board/postfile/testdata/render/), `TestPbStRenderMatchesQ8Example`, `TestPbStRenderUnknownReIsTheId` |
| 10 | V15: deleting or editing the markdown body leaves the fold result unchanged | NOT DONE | The reader decodes only the header (`postfile.Decode`), and `postfile.Dir.List` keeps the first-sight header, but no test folds a board through the feed facade before and after a body edit, because the facade is not built |
| 11 | V15: an F4 renumber rewrites the links inside the renumbered posts and every unaccepted post that links to them | HOLDS | `TestPbStRenumberRewritesLinks` (pb_st_test.go) |
| 12 | V15: F2, F3, F5 and F6 tests | HOLDS | F2 `TestPbStPosterRemovesOldTemps`; F3 `TestPbStUnterminatedHeaderIsReadAgain`; F5 `TestPbStFoldSameSeqTiesByID` (pb_st_fold_test.go:107, the fold tie by id; the display `(seq, id)` sort belongs to the unbuilt facade); F6 `TestPbStEditedAndDeletedPostsAreFindings`. F12 as well: `TestPbStNoHardLinksFallsBackToExclusiveCreate` |
| 13 | V15: a fold benchmark at 6,000 posts | HOLDS (measurement) | `go test -count=1 -run '^$' -bench 'BenchmarkPbStFold' -benchmem ./internal/contract/` exit 0 on an AMD Ryzen 5 3600: `BenchmarkPbStFoldBoard6000-12 1 4371107133 ns/op 2665126928 B/op 34839852 allocs/op`; the same posts as JSONL: `BenchmarkPbStFoldJSONL6000-12 1 1945050932 ns/op 1172016256 B/op 15273517 allocs/op` |
| 14 | V15: the CLI JSON shape of Q3 is pinned by a golden | NOT DONE | internal/cli/feed.go is not changed and has no board mode yet |

## Builds items

| Item | Result |
|---|---|
| Per-post file codec, Q8 renderer, publish by link with F12 fallback, names cache, check, renumber | HOLDS: internal/board/postfile (header.go, render.go, dir.go). This is a leaf package because internal/board imports internal/feed |
| internal/contract/fold.go new rules | HOLDS: `FoldBoard`, `BoardPost`, `Loop.Home`; `FoldLoop` and `Fold` unchanged for JSONL |
| `board.Board` per-post adapter with `Claim` under the board lock | NOT DONE (budget) |
| internal/feed facade; fold.go:84-184 state.json, needs-you.json and fold.lock to the machine folder | NOT DONE (budget). internal/feed/fold.go is unchanged from base (`git diff --stat 91f210e5 HEAD -- internal/feed/fold.go` prints nothing). On base, state.json goes through `Machine.StateFile` (fold.go:86, :92), fold.lock is `feed/.cache/fold.lock` (:96-:107), and needs-you.json is `feed/needs-you.json` (:139-:142) |
| internal/mcp/tools.go, internal/cli/feed.go, Q3 records (`by`, `file`, `worker` alias) | NOT DONE (budget) |
| Hooks events.go:385, pe_job_message.go:38, history.go:235 `feed/` to `board/` | NOT DONE (budget) |
| oh_quiet_task.go digest of board names | NOT DONE (budget) |

## Rules

| Rule | Result | Proof |
|---|---|---|
| New tests fail on base | HOLDS | On base `91f210e5`: `internal/contract/pb_st_fold_test.go:17:30: undefined: BoardPost`; `internal/board/postfile/pb_st_test.go:21:51: undefined: Header`. `TestPbStJSONLFoldsAsOnBase` is the regression row and passes on base |
| No existing test edited; new tests in new files; helpers prefixed `pbSt` | HOLDS | `git diff --name-only origin/main..HEAD` lists only new `pb_st_*` test files, new testdata, internal/board/postfile/*.go and internal/contract/fold.go |
| Files inside Touches | HOLDS | Same list: new internal/board per-post files, internal/contract/fold.go, new test files and their goldens. contract/schemas is not touched |
| Base check `git grep -n '"feed"' main -- internal/hook internal/orchestrator internal/mcp internal/cli` | HOLDS (ran) | Production hits: internal/cli/cli.go:177, internal/cli/inprocess.go:89, internal/hook/events.go:385, internal/hook/history.go:235, internal/hook/pe_job_message.go:38, internal/orchestrator/oh_quiet_task.go:59, internal/orchestrator/progress.go:33. The rest are tests. None changed, so no pinning test needed to change |
| App rigs `git -C /srv/gridwork/ref/app.git grep ... '*realplugin*'` | HOLDS (ran) | 74 hits of `feed read`/`feed state` in cmd/grid-server, internal/kernel/plugincli, internal/kernel/worker, internal/slices/{feed,gates,taskview} and tests/fixtures/richtask. All run on JSONL homes; the lane changes no CLI output, so none is affected |
| Standing test set | HOLDS | `go test -count=1 ./internal/board/... ./internal/feed/... ./internal/contract/... ./internal/hook/... ./internal/orchestrator/... ./internal/mcp/... ./internal/cli/`: every package `ok` after the merge |
| Importers | HOLDS | `go list` shows 28 packages that import internal/contract (artifact, board/postfile, brain, braincmd, cli, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, runner, session, settings, signal, task, updates, usage, where, workflow); none imports postfile. Their tests: every package `ok` or no test files |
| Walk tests | HOLDS | After the merge: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| `make lint` | HOLDS | exit 0 (`go vet ./...`, no findings) |
| `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| Merge origin/main before the last proof run | HOLDS | `git merge origin/main` (91f210e5..5d7c2242), clean |
| No guard, gate, cap, hold or blocking hook | HOLDS | The board lock orders this machine's writers only; the fold refuses nothing a person writes, and a person's control is a request, as the doc says |
| Attribution | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by|claude-session|generated with"` prints 0 |
| Proof is go test and make only | HOLDS | No claude, curl, server, deploy or push |

## What changed

- internal/board/postfile, a new package:
  - header.go: the strict header codec. A decode re-encodes and must match the header bytes exactly.
  - render.go: the Q8 body rendering.
  - dir.go: publish under `board.lock` (a temp in `board/.tmp/`, then `os.Link`; EEXIST takes the next seq; with no hard links it falls back to O_EXCL plus fsync). Also the `board.seq` counter, the names cache, `List`, `Check` (edited and deleted findings) and `Renumber`.
- internal/contract/fold.go:
  - `FoldBoard([]BoardPost)` folds by `(gen, seq, id)`. Homes come from the first Job record's `home` and the team's `task-move` posts.
  - Controls count only from an orchestrator, cli or launcher role on the home of their gen. Any other control is a request.
  - An assign must raise the attempt. A Job-side post matches its assign's `to` as author@machine(by).
  - Duplicate claims fold `ignored`. Legacy counted and stale are honoured.
  - `Loop.Home` reports the current home.
  - JSONL `FoldLoop` and `Fold` are unchanged, shown by the base-written golden.

## Open items

- The facade, the `Claim` adapter, the machine-folder caches, CLI, MCP, hooks and quiet-task digest (rows 4b, 5b, 10, 14) need a follow-up lane.
- The board fold costs 2.2 times the JSONL fold at 6,000 posts (4.4 s against 1.9 s). Most of the extra is `boardData`'s marshal, map and marshal pass for each schema variant.
- The envelope's role enum has no `orchestrator` or `launcher` role. Of the doc's three authority roles, only `cli` can be written today.
- control.json `to` is still `member@ULID`, so the board side match reads author@machine(by).
- The format bump (B3) is outside Touches, so a format-2 home is still refused by internal/cli.

## NOT VERIFIED

- That a real orchestrator pass writes its controls as role `cli` on a board. On base the grep shows `Role: "cli"` with the task lease, but no board writer exists yet.
- File modes of `board.lock`, `board.seq` and `board.cache.json` in a shared machine folder under two logins.
- The final `cp` of this report to the hand-off folder, if it is refused.

NOT DONE
