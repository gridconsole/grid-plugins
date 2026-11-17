# M-P-board-store-fix2-onto-main: P-board-store-fix2 merged onto plugin main 6fa1a6f1

Branch `lane-M-P-board-store-fix2-onto-main`, from `lane-P-board-store-fix2` (head `71ea5c09`), merged with `origin/main` `6fa1a6f1`. Carries P-board-store-fix2 (and through it P-board-store-fix1 and P-board-store).

## The conflict

One file: `internal/mcp/tools.go`, in the `default:` branch of the post tool, after `s.Session(in)`.

- The lane added `if tool.Name == "feed_handover" && store.Board != nil { return ... "a per-post board has no task lease; handover is retired" }`.
- Main added the rule that an `answer` post that answers for a person takes its `by` from the session's author (`in.Body["by"] = writer.Author.ID`).
- Both stood in the same `if` and shared one closing brace. Resolution: two separate `if` blocks, the handover guard first, then main's comment and `by` block, each unchanged. Nothing from either side was dropped. No test, row or error code changed.

## Proof table

| Row | Result | Command and result |
|---|---|---|
| 1. origin/main is an ancestor; diff names only the lane's files | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0. `git diff origin/main --stat`: 15 files, all files of P-board-store-fix2 (internal/cli/feed.go, its test and golden, internal/feed/board.go and board_view.go, internal/hook/{events,history,pe_job_message}.go and its test, internal/mcp/tools.go and its test, internal/orchestrator/oh_quiet_task.go and its test, docs/lanes/P-board-store-fix2.md) plus docs/lanes/P-board-store.md, which this brief orders rewritten |
| 2. Acceptance rows of P-board-store-fix2 | HOLDS | Row 1 (golden): `TestPbStFeedBoardReadGolden` and `internal/cli/testdata/pb_st_feed_board_read.golden.json` are in the cli run below, which exits 0; the failing-on-fix1 lines are in docs/lanes/P-board-store-fix2.md row 1 and the merge did not touch the golden or the test. Row 2 (hook paths read `board/`): `internal/hook/pb_st_board_test.go` passes in the hook run below; its failing-on-base lines are in docs/lanes/P-board-store-fix2.md row 2. Row 3: `go test -count=1 ./internal/cli/... ./internal/mcp/... ./internal/hook/...` exit 0: `ok internal/cli 46.627s`, `ok internal/mcp 0.331s`, `ok internal/hook 4.615s`. Row 4: no test deleted (`git diff origin/main --stat` shows no deleted file and no changed `_test.go`, only the new `pb_st_*` ones); attribution grep prints 0 (below). The fix-round row (`hookFeed`/`dirExists` in events.go, no `board_feed.go`): `git diff origin/main --stat` lists no `board_feed.go` |
| Tests of changed packages and importers | HOLDS | Changed packages by the lane: cli, feed, hook, mcp, orchestrator. `go test -count=1 ./internal/feed/... ./internal/board/... ./internal/orchestrator/... ./internal/golden/... ./internal/packaging/... ./contract/...` exit 0 (all `ok`; orchestrator 159.288s, feed 17.099s). The wider importer list is in docs/lanes/P-board-store-fix2.md; the merge changed only `internal/mcp/tools.go` by one hunk |
| 3. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.048s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.246s`). The other walk tests in the runbook are the app repo's: not run, not asked |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`, no findings) |
| 4. `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile, every package `ok`) |
| Merge origin/main before the last proof run | HOLDS | `git fetch origin` then `git merge origin/main`; origin/main is `6fa1a6f1`, the merge is the head's parent |
| Carried NOT DONE rows rewritten, in the first commit | HOLDS | docs/lanes/P-board-store.md: rows 4, 5, 10, 14, the five "Builds items" lines and the intro sentence now read `CLOSED BY M-P-board-store-fix2-onto-main: <what decided it>`; the old text is kept after "Was NOT DONE". docs/lanes/P-board-store-fix1.md and P-board-store-fix2.md have no NOT DONE |
| New tests fail on base | HOLDS | No new test written here. The carried lane's failing-on-base lines are in docs/lanes/P-board-store-fix2.md rows 1 and 2 |
| No existing test edited or deleted | HOLDS | The merge hunk is in `internal/mcp/tools.go`, a production file |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The handover refusal on a per-post board is the carried lane's (retired verb on a board with no lease), not a new guard here; main's `by` rule refuses nothing |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0 |
| Windows and macOS builds | HOLDS | Inside `make cross`, exit 0 |
| `*RealPlugin*` and app-repo walk tests | not run, not asked | This is the plugin repo and the brief names no real-plugin row |

## What changed

- `internal/mcp/tools.go`: the one conflict resolved as above.
- `docs/lanes/P-board-store.md`: NOT DONE rows closed as above.
- This doc.

## Open items

None.

## NOT VERIFIED on a live screen

Nothing here has a screen. The CLI and MCP are seen through Go tests only.

Verdict: DONE.
