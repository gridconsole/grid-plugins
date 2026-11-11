# C-ku-question-marker-life

A finished or refused `question ask` leaves no `Q-n.feed` marker, and a posted person Job keeps its id.

Base: 35a498dc (lane branch point). The brief named plugin main b6160963; on this base `add` is still at internal/question/question.go:146-191, reserve.go:14-63 and job.go:57-78 are as the brief reads, and job.go:69-72 carries C-jw-question-lock-order's comment. `git grep -n os.Remove -- internal/question` on the base finds only test files (question_test.go:334, 556, 653, 656), so no later lane removed the marker.
Lane commits: 62829286 (production), dd6fbfe1 (tests).

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | Finished ask leaves only `Q-n.md`; a second ask is Q-2 | HOLDS. `go test -count=1 -v -run TestKuMk ./internal/question/` exit 0, `--- PASS: TestKuMkFinishedAskLeavesNoMarker`. Test: internal/question/ku_marker_life_test.go:62. Base: `FAIL ... questions/ holds [Q-1.feed Q-1.md], want only Q-1.md` |
| 2 | Refused ask (worker, port returns "" and no error) leaves no marker; the next owner ask is Q-1 | HOLDS. `--- PASS: TestKuMkRefusedAskLeavesNoMarker` (:88). Base: `a refused ask left [Q-1.feed] in questions/` |
| 3 | Job posted, publish refused: `Q-1.feed` stays, next owner ask is Q-2 | HOLDS. `--- PASS: TestKuMkPostedJobKeepsTheId` (:128); it also reads J-7's `verify.path` = questions/Q-1.md. Base fails only on the extra final listing: `holds [Q-1.feed Q-2.feed Q-2.md], want Q-1.feed and Q-2.md` (base leaves the Q-2 marker too). Unchanged: `TestAskPostErrorLeavesNothing` and TestReserveGate "publish error keeps the marker" pass untouched (row 5 run) |
| 4 | 8 asks at once: Q-1..Q-8 once, 8 `.md`, 0 `.feed`, 8 Post calls | HOLDS. `--- PASS: TestKuMkConcurrentAsks` (:154); also `go test -count=1 -race -run 'TestKuMkConcurrentAsks\|TestConcurrentAdds' ./internal/question/` exit 0. Base: `holds 8 .md and 8 .feed files, want 8 and 0` |
| 5a | `go test -count=1 ./internal/question/ ./internal/orchestrator/ -run 'TestReserveGate\|TestAsk\|TestConcurrentAdds\|TestJwLo\|TestJr\|TestJwPr'` | HOLDS, exit 0 (question 1.359s, orchestrator 5.901s) |
| 5b | `go test -count=1 ./internal/cli -run 'TestKb3\|TestQuestion\|TestJwRp\|TestJwLo'` | HOLDS, exit 0 (3.025s). TestKb3QAskByWorkerIsRefusedNoJob is in this set |
| 6a | `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| 6b | `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64: vet and test compile, every package ok) |
| 6c | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0 |
| 6d | `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0 |
| 6e | `git diff main --stat` names only question.go and the new test file | HOLDS for the code: `internal/question/ku_marker_life_test.go 203 +`, `internal/question/question.go 27 +-`. This report, docs/lanes/C-ku-question-marker-life.md, is the one further file, as the runbook requires |

## Rules

| Rule | Result |
|------|--------|
| New tests fail on base, pass on the lane | HOLDS. The base run (production change stashed) failed 5 of 5; the failing lines are in rows 1-4 and the extra test below. The lane run passes all 5 |
| No existing test edited or deleted | HOLDS. `git diff main --stat` shows no existing test file touched |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS. No new refusal or error code. The worker refusal is the Feed's own (internal/feed/store.go:221) and is unchanged |
| Lock order unchanged: no Job-port call while questions/.lock is held | HOLDS. `postJob` still runs between the two `locked` blocks (question.go, `job, err := s.postJob`). The new removals happen inside a lock block that was already there |
| Tests of the changed package and its importers (go list) | HOLDS. `go list` importers of internal/question: internal/cli, internal/orchestrator, internal/task. `go test -count=1 ./internal/question/ ./internal/orchestrator/ ./internal/task/ ./internal/cli/` exit 0 (question 5.064s, orchestrator 169.975s, task 8.063s, cli 81.797s) |
| Windows and macOS builds | HOLDS, row 6b |
| Repo-wide walk tests, plugin set | HOLDS, rows 6c and 6d. The app set (`make webui-test`, TestFvArgvCollects, TestEveryWiredMount, slices Imports) belongs to the app repo and does not apply to this plugin lane |
| Tests named *RealPlugin* | Not named by the brief, none run |
| Public repo: no personal paths, hosts or emails | HOLDS. The diff holds none |
| No attribution lines in commits | HOLDS. Both commits are one sentence with no trailer |

## What changed

`internal/question/question.go`, `add`, second locked block only (plus the `os` import):

- After `publishQuestion` returns nil, `Q-n.feed` is removed; an error other than not-exist is returned.
- When `publishQuestion` fails and this ask posted no Job, `Q-n.feed` is removed under the same lock hold and the publish error is returned unchanged (a failed remove is ignored: the marker is then only a gap in the ids).
- When a Job was posted and publish fails, the marker stays. When `postJob` fails, the marker stays (the Job may have reached the Feed).

`internal/question/ku_marker_life_test.go`: `TestKuMkFinishedAskLeavesNoMarker`, `TestKuMkRefusedAskLeavesNoMarker`, `TestKuMkPostedJobKeepsTheId`, `TestKuMkConcurrentAsks`, and one extra, `TestKuMkCallerJobIsNotThePostedJob`.

## A choice the brief leaves open

The brief tests `rec.Job == ""` for "no Job was posted that names Q-n". But `rec.Job` starts as the caller's own `in.Job` (the `--job` flag, command.go:101), and `add` only overwrites it when the port returns an id. With `question ask --job J-3` by a worker, `rec.Job` is "J-3" though no Job names Q-n.md, so the literal test would keep the marker on exactly the refused path this lane fixes. I test a local `posted := job != ""` instead, which is what the brief's own reason says. The extra test `TestKuMkCallerJobIsNotThePostedJob` pins it; it fails on base (marker left) and passes on the lane. For every row of the brief the two conditions give the same answer.

## Open items

- Publish that fails after the Feed post took (the question file write fails) with no Job posted also removes the marker. Nothing is lost: the Feed holds the question, and the next ask's `project` (the first step under the lock, before `nextID`) writes its `Q-n.md` first, so the id is taken. Not exercised by a test, as the Feed post and the file write cannot be split in this fixture.
- A remove error on the success path is returned to the caller although the question is already published, as the brief says.

## NOT VERIFIED on a live screen

The app's Files tab (gxfl_files_tab_realplugin_test.go in the app repo) was not run; it needs the app and a built plugin. No app caller reads a `.feed` marker (brief, `git grep` on app main), and the plugin side now leaves none.

## Verdict

Every acceptance row and every rule holds; the one open choice (`posted` against `rec.Job`) is stated above.

DONE
