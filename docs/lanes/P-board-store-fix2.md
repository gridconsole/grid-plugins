# P-board-store-fix2

The second half of the P-board-store split. It sits on lane-P-board-store-fix1 (the store side) and builds the readers and writers of the store: the CLI, the MCP tools, the hooks and the quiet-task digest. P-board-store's rows and Touches are unchanged. Base: `4990ebbd^` (fix1 tip, `8d1c595a`).

Commits on this lane (`git log --oneline 4990ebbd^..HEAD`):

- `4990ebbd` Read and write a task's per-post board from the CLI, MCP, hooks and the quiet-task digest.
- `bd3ad7e3` Test the board mode of the CLI, MCP, hooks and the quiet-task digest, with a golden for the CLI read shape.
- `153eb31a` Record the proof table of the board mode of the CLI, MCP, hooks and the quiet-task digest.
- Fix round 1: Move hookFeed and dirExists into events.go so no new non-test file lands in internal/hook (this file is updated in the head commit; its sha is the head in the report).

## Proof table: acceptance rows

| Row | State | Proof |
| --- | --- | --- |
| 1. Row 14 holds, with a golden that fails on lane-P-board-store-fix1 | HOLDS | Golden `internal/cli/testdata/pb_st_feed_board_read.golden.json`, held by `TestPbStFeedBoardReadGolden` (`internal/cli/pb_st_feed_board_test.go`). It holds the Q3 shape of `grid feed read --json` on a board task: `by` and `file` added, `worker` kept as an alias of `by`, no `lease`, `fence`, `end` or `gen`. The test also checks that those four keys are absent. Passes on the lane (`go test -count=1 ./internal/cli/... ...` exit 0, below). On base (the six consumer files put back to `4990ebbd^`, same test): `--- FAIL: TestPbStFeedBoardReadGolden`, `board read differs from testdata/pb_st_feed_board_read.golden.json`, got `"end": 388`, `"fence": 0`, `"file": "member@01ARZ3NDEKTSV4RRFFQ69G5FAW.0.jsonl"`, `"gen": 0`, `"lease": null`, and no `by` |
| 2. The hook paths read `board/`; a hook test shows it and fails on the base | HOLDS | `internal/hook/events.go:391-396` (`jobHasResultOn` reads through `hookFeed`, then `store.ReadOnly`), `internal/hook/pe_job_message.go:37-45` (`hookFeed`, then `board.ReadOnly`), `internal/hook/history.go:235` (`path == "board"` is skipped, as `feed` is). `hookFeed` is `internal/hook/events.go:414` (and `dirExists` at `:445`, both moved there in fix round 1): a `board/` task is read from the board, a task with only `feed/` from its JSONL Feed. Tests in `internal/hook/pb_st_board_test.go`, all pass on the lane and fail on base: `TestPbStBoardPeMsgDelivers` (`pb_st_board_test.go:62: stdout is not hook output: unexpected end of JSON input: ""`), `TestPbStBoardJobHasResult` (`pb_st_board_test.go:79: after the result: false <nil>`), `TestPbStBoardSnapshotSkipsBoard` (`pb_st_board_test.go:100: snapshot lists board/000001-task-post.md`) |
| 3. `go test -count=1 ./internal/cli/... ./internal/mcp/... ./internal/hook/...` exits 0 | HOLDS | exit 0, rerun on the fix-round head: `ok internal/cli 57.755s`, `ok internal/mcp 0.235s`, `ok internal/hook 3.846s` |
| 4. No existing test deleted; no attribution lines (grep prints 0) | HOLDS | `git diff --name-status 4990ebbd^ HEAD` lists 5 `A` test or golden files, no `D`, and no `M` on any `_test.go`. `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` printed `0` |

## Carried rows (fix1)

Every acceptance row of fix1 (rows 4, 5, 10, the race run and the rest, all HOLDS in `docs/lanes/P-board-store-fix1.md`) is carried and is for the reviewer to judge from fix1's brief on this head. I did not re-run fix1's own commands one by one. The test runs below (`internal/feed` and `internal/orchestrator` with every importer) do cover the same packages on this head. `go test -race` of fix1 | not run, not asked.

## Proof table: rules and checks

| Rule or check | State | Proof |
| --- | --- | --- |
| Files inside Touches | HOLDS, with one file for the reviewer to judge (see Open items 8) | `git diff --name-status origin/main..HEAD`: M `internal/cli/feed.go`, `internal/mcp/tools.go`, `internal/hook/events.go`, `internal/hook/history.go`, `internal/hook/pe_job_message.go`, `internal/orchestrator/oh_quiet_task.go`, `internal/feed/board.go`; A `internal/feed/board_view.go`; A four new test files and the golden; A this lane doc. Fix round 1: `internal/hook/board_feed.go` is gone (`git rm`); `hookFeed` and `dirExists` are in `internal/hook/events.go`, so the only added file in `internal/hook` is the test file `pb_st_board_test.go` (`git diff --stat --diff-filter=A origin/main..HEAD -- internal/hook`) |
| New tests fail on base, pass on the lane | HOLDS, except two | The six failing runs are pasted under the rows above and below. `TestPbStFeedBoardHandoverRefused` and `TestPbStBoardHandoverRefused` pass on base too (there is no lease to hand over on base either, so the call fails there as well). They are regression guards, not claimed as base failing |
| No existing test edited; helpers prefixed `pbSt` in new files | HOLDS | Only new files: `internal/cli/pb_st_feed_board_test.go`, `internal/mcp/pb_st_board_test.go`, `internal/hook/pb_st_board_test.go`, `internal/orchestrator/pb_st_quiet_board_test.go`. Helpers: `pbStBoardHarness`, `pbStScrub`, `pbStCliUpdate`, `pbStBoardFixture`, `pbStHookBoard`, `pbStResult`, `pbStWriteBoardFile` |
| Quiet-task digest of board names | HOLDS | `internal/orchestrator/oh_quiet_task.go:46-48` and `:87-101` (`ohSqBoardKey`). `TestPbStQuietTaskBoardKey` passes; on base: `pb_st_quiet_board_test.go:56: a new post left the board key as it was` |
| MCP board mode | HOLDS | `TestPbStBoardPostRead` passes; on base: `pb_st_board_test.go:44: post reply lacks "by":"01ARZ3NDEKTSV4RRFFQ69G5FAW:tester"` |
| CLI board post | HOLDS | `TestPbStFeedBoardPost` passes; on base: `pb_st_feed_board_test.go:140: file "" in {"post":{... "lease":null,"gen":0,"fence":0 ...}}` |
| Tests of every changed package and every importer | HOLDS | Fix round 1 changed only `internal/hook`. Its importers by `go list` (imports, test imports, external test imports): `internal/cli`, `internal/golden`, `internal/packaging`. `internal/cli` is row 3. `go test -count=1 ./internal/golden/... ./internal/packaging/...` exit 0: `ok internal/golden 0.213s`, `ok internal/packaging 2.299s`, `ok internal/packaging/cmd 0.040s`. The list below is the list for the whole lane (first round), `ok` on that round's head; the fix round moved code inside `internal/hook` and changed nothing in the other four packages. Changed: `internal/cli`, `internal/feed`, `internal/hook`, `internal/mcp`, `internal/orchestrator`. Importers by `go list` (direct imports, test imports and external test imports of those five): `cmd/grid`, `contract`, `contract/formats`, `internal/account`, `internal/approvefix`, `internal/board`, `internal/braincmd`, `internal/cli`, `internal/feed`, `internal/golden`, `internal/hook`, `internal/job/jobcmd`, `internal/jobgraph`, `internal/judge`, `internal/land`, `internal/lease`, `internal/mcp`, `internal/migrate`, `internal/orchestrator`, `internal/packaging`, `internal/question`, `internal/runner`, `internal/settings`, `internal/task`, `internal/usage`, `lib`. Command: `go test -count=1` over the importers other than `internal/cli`, `internal/mcp` and `internal/hook` (those are row 3), exit 0. `ok` for `contract`, `contract/formats`, `internal/account`, `internal/board`, `internal/braincmd`, `internal/feed` (29.884s), `internal/golden`, `internal/job/jobcmd`, `internal/jobgraph`, `internal/judge`, `internal/land`, `internal/lease`, `internal/migrate`, `internal/orchestrator` (157.596s), `internal/packaging`, `internal/question`, `internal/settings`, `internal/task`, `internal/usage`, `lib`; `[no test files]` for `cmd/grid`, `internal/approvefix`, `internal/runner` |
| `make lint` | HOLDS | fix-round head: exit 0 (`go vet ./...`, no findings) |
| `make cross` (Windows and macOS) | HOLDS | fix-round head: exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok` |
| `git fetch origin && git merge origin/main` before the last run | HOLDS | fix-round head: `git fetch origin` exit 0 (3 new lane branches, no change to main); `git merge origin/main`: `Already up to date.` |
| Walk tests | HOLDS | fix-round head: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.043s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok internal/job/lint 0.250s`) |
| App check (what the app rigs read) | HOLDS, rigs not run | See the paste below. Every hit reads `grid feed read --json` or `grid feed state --json`. Those shapes are unchanged for a task with no `board/` folder, and no existing home has one before the migration (B3). `feed state` is not changed at all by this lane. The app rigs themselves | not run, not asked |
| `*RealPlugin*` tests | not run, not asked | The brief names no row on them |
| Commits small, one sentence, no attribution | HOLDS in form, not split | The lane's own commits (listed at the top) are one sentence each, no attribution line (grep prints 0). The production change is one commit of nine files rather than one per surface |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS, one refusal to judge | No hook changed here blocks or holds an agent: the hooks only read. The one refusal is `feed handover` on a board (`internal/cli/feed.go:113`, `internal/mcp/tools.go`), see Open items 5 |
| One command per Bash call, no `sed -i`, redirects, tee, `cd x &&` chains, no sudo | HOLDS | |
| `grid:run-job` skill read | HOLDS | Loaded once. Its `grid job show/post/result` steps need a task and a Job record; this lane has none (the project has no Grid record), so the deliverables are this doc, the bundle and the report |

### App check, pasted

`git -C /srv/gridwork/ref/app.git grep -n -E '"feed", ?"(read|state)"' main -- '*realplugin*'` printed 73 lines, all in `main`, each a `feed read ... --json` or `feed state ... --json` call of a `realplugin` rig. The hits as `file:line`:

```
cmd/grid-server/ (19)
  et_person_job_answer_realplugin_test.go:248   et_taskview_job_order_realplugin_test.go:172
  feed_refusal_realplugin_test.go:86            gxcd_code_tab_realplugin_test.go:57
  gxfd_feed_tab_realplugin_test.go:166          gxvj_view_jobs_realplugin_test.go:112
  gy_personjob_holds_realplugin_test.go:122     gy_personjob_member_realplugin_test.go:121
  gy_personjob_second_answer_realplugin_test.go:78   gy_personjob_two_open_realplugin_test.go:166
  hn_guest_job_answer_realplugin_test.go:27     kj_agents_verdicts_realplugin_test.go:86
  kj_code_submitted_realplugin_test.go:56,72    kj_guest_project_realplugin_test.go:100,136
  kj_job_waiting_answer_realplugin_test.go:94,106   mg_usage_report_realplugin_test.go:130
internal/kernel/plugincli/ (2)
  t1_parity_realplugin_gridlib_test.go:103      yf_not_found_code_realplugin_test.go:104
internal/kernel/worker/ (38, all *_realplugin_unix_test.go)
  d19_relay:95,168  d20_ship_person_job:160  db2_intent_realworker:221  fw_worker_reads:81,116,129
  gq_judge_person_body:83  hq_journey:181,224,239  hq_replay_approve_land:124,252,293,434
  jl_judge_person_point:57,196  jl_judge_rework:167,203  jl_worker_exit_code:94,103
  jn_review_cancelled_spawn:125  job_env_inherited:93  judge_post_request_id:53,98
  lh_found_rows:60,139  lh_needs_lead:80,131,156  r12_stopped_job_not_rerun:83,179
  replay_first_task:278,364,464,554  spawn:217,246
internal/slices/ (9)
  feed/realplugin_test.go:126,147,157
  gates/gq_person_job_answer_realplugin_test.go:237   gates/nb2_send_back_realplugin_test.go:60
  gates/nd_later_approve_realplugin_test.go:131       gates/realplugin_personal_test.go:79,80
  taskview/bd2_acceptance_rows_realplugin_test.go:67
tests/fixtures/richtask/ (5)
  kj_verdicts_realplugin_test.go:47,69,106   kj_workflow_realplugin_test.go:40
  richtask_realplugin_test.go:58
```

Two of them for the CLI shape: `internal/slices/feed/realplugin_test.go:147,157` compares two `feed read` outputs for equality, and `internal/kernel/worker/d19_relay_realplugin_unix_test.go:95` reads `feed.Records[0].Task` (kept in the Q3 shape). None of these rigs reads a task with a `board/` folder, so none sees the Q3 shape. When a home is migrated to boards (B3), the rigs that read `lease`, `fence`, `end` or `gen` from `feed read` records would see them gone, as `docs/research/brain-board.md:205` says they will.

## Fix round 1

Must-fix 1 (`internal/hook/board_feed.go` is a new non-test file outside Touches): HOLDS. `hookFeed` and `dirExists` moved, with their comments and no change of behaviour, into `internal/hook/events.go` (`:414` and `:445`, after `jobHasResultOn`), with one added import (`internal/machinefolder`); `internal/hook/board_feed.go` is deleted by `git rm`. `gofmt -l internal/hook` printed nothing; `go build ./...` exit 0. Rerun on that head: row 3 (exit 0, above), the walk tests, `make lint`, `make cross`, the importers of `internal/hook` (all pasted in the tables above). The three hook tests pass on the lane: `go test -count=1 -v -run TestPbStBoard ./internal/hook` printed `--- PASS: TestPbStBoardPeMsgDelivers`, `TestPbStBoardJobHasResult`, `TestPbStBoardSnapshotSkipsBoard`, then `ok internal/hook`. The failing-on-base runs under row 2 are unchanged: the fix round changed no test.

## What changed

- `internal/feed/board_view.go` (new): `BoardPresent(taskDir)`; `Store.Boarded(rel, login)` sets `Store.Board` only when `board/` is there; the Q3 record types `BoardRecord`, `BoardEntry`, `BoardReadResult` and `Record.BoardView()`, `Record.BoardFile()`, `Entry.BoardView()`, `ReadResult.BoardView()`. A record keeps `id`, `type`, `seq`, `attempt`, `author`, `verdict`, `reason`, `waiting`, `body`; gains `by` and `file`; has no `lease`, `fence`, `end` or `gen`; `worker` stays as an alias of `by`.
- `internal/feed/board.go`: `BoardHome.Login`. When `By` is not set, `by` is `<machine>:<login>`, the machine from the writer's `worker` (`name@machine`).
- `internal/cli/feed.go`: the store is built with `Boarded(boardRel(...), env.login())`. `read` prints the Q3 view on a board; `post` answers `{post, file, tier, data}` on a board; a Job post on a board uses `store.Post`, not `PostJobLocked`; `handover` on a board is refused with `errBoardHandover`. A task with no `board/` folder is exactly as before.
- `internal/mcp/tools.go`: the same for `feed_read`, `feed_post` and `feed_handover`. The tool list and descriptions are unchanged.
- `internal/hook/events.go` (holds `hookFeed` and `dirExists`), `pe_job_message.go`, `history.go`: the hooks read `board/` for a board task and `feed/` for the others; the Bash history snapshot skips `board/` as it skips `feed/`. `stop.go` is unchanged and still calls `jobHasResult(dir, job, attempt)`, which now wraps `jobHasResultOn`.
- `internal/orchestrator/oh_quiet_task.go`: a board task's quiet key is the digest of the sorted names of `board/*.md`. Posts are immutable, so no size or mod time is taken; `board/.tmp` is not listed.
- New tests: five files (four `pb_st_*` test files and one golden), all new.

## Open items

1. **B3, the format bump and the migration, is outside Touches.** `gridhome.FormatVersion` is still 1 and a format-2 home is still refused by `feedStore` (an existing test pins it). So board mode is chosen per task by the presence of `board/`. When B3 lands, that choice can move to the home's format.
2. **The orchestrator Stream and `Load` read a board task as JSONL.** They are outside Touches, so a board task's Jobs are not seen by the orchestrator yet (only its quiet-task key is). This belongs to P-loop-retire.
3. **No pass lock on a board.** A Job post on a board skips `PostJobLocked`, so no `feed/.orchestrator.lock` is made in the brain. The pass lock moving to the machine folder belongs to P-loop-retire.
4. **`stop.go` keeps the 3-argument `jobHasResult`** (it is outside Touches; existing tests call it). `jobHasResult` reads the process environment for the machine folder.
5. **Handover on a board is refused** (CLI and MCP): there is no task lease on a board. It is a refusal of an operation that has no meaning there, not a hook, gate or cap on an agent; the reviewer is asked to judge that reading.
6. **The post reply on a board gains a top-level `file`.** It is not in the old shape.
7. **`arrived` is empty on a board read.** A post file has no arrival time apart from its mod time.
8. **`internal/feed/board_view.go` is a new non-test file, judged inside Touches by the round-1 review.** Fix round 1 moved the other new file (`internal/hook/board_feed.go`) into `internal/hook/events.go` on the reviewer's must-fix. For `board_view.go` the round-1 review (`/srv/gridwork/tmp/hand/review-P-board-store-fix2.md`, section "How I read `internal/feed/board_view.go`") reads the Touches clause "internal/feed facade" as a package, counts it inside Touches, and leaves it for the orchestrator to overrule. If the orchestrator reads that clause file by file, `board_view.go` moves, with no change of behaviour, into `internal/feed/board.go`. I left it, as the fix round named one file.
9. **The budget.** The brief gave about 90 calls. I used more than that: the production change was one commit and the tests were written late, after the summary of the first context.

## NOT VERIFIED

- The app rigs (`*realplugin*`) were not run, and no Grid app was started against a board task.
- No migrated home (B3) exists to run the CLI against; the CLI was run on a task folder with a `board/` folder made by the tests.
- fix1's own rows and its race run were not re-run by me (carried).
- Behaviour on Windows and macOS was built and vetted (`make cross`), not run.

Verdict: DONE
