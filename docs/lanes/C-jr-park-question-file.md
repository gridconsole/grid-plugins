# C-jr-park-question-file

A Job parked after three failures now writes its question file, so Needs you shows the question and not "question file unreadable".

Base: plugin 99c11b7a (lane-Pint105 merge of main). Head: see the report line "head sha".

## Base check

HOLDS. On base, `park` leaves no `Q-n.md`: `ReserveGate` (internal/question/reserve.go, base lines 17-29) writes only `questions/Q-n.feed` and publishes; nothing runs `project(dir, true)` after it. Proven by the new tests failing on base (pasted under row 1).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. TestJrParkWritesQuestionFile | HOLDS | `go test -count=1 -v -run 'TestJrPark\|TestParkGateReservesQuestionFileIDs' ./internal/orchestrator` exit 0: `--- PASS: TestJrParkWritesQuestionFile (1.01s)`. The test reads `questions/Q-1.md` front matter itself (not through the Feed projection) and checks id Q-1, `post` equal to the Feed question post id, job J-1, who owner, blocking, options ["Keep parked","Retry"], recommended "Keep parked", gate Q-1/person/"Keep parked", state open, the question text, and that `question.Store{TaskDir, TaskID}.List()` returns the same id and post. Also asserts `Q-1.feed` is still there. File: internal/orchestrator/jr_park_question_file_test.go:77 |
| 1b. fails on base | HOLDS | With reserve.go and events.go stashed back to base: `go test -count=1 -run TestJrPark ./internal/orchestrator` exit 1: `--- FAIL: TestJrParkWritesQuestionFile (1.19s)` / `jr_park_question_file_test.go:87: question files [], want [Q-1.md]` and `--- FAIL: TestJrParkOneFile (1.60s)` / `jr_park_question_file_test.go:126: question files [] after another tick, want [Q-1.md]` |
| 2. TestJrParkOneFile | HOLDS | `--- PASS: TestJrParkOneFile (1.76s)` in the same run: one more tick after the park leaves exactly `[Q-1.md]` and one question post on the Feed. internal/orchestrator/jr_park_question_file_test.go:122 |
| 3a. orchestrator run | HOLDS | `go test -count=1 -v -run 'TestJrPark\|TestParkGateReservesQuestionFileIDs' ./internal/orchestrator` exit 0: `--- PASS: TestParkGateReservesQuestionFileIDs (1.34s)`, `ok ... 4.138s` |
| 3b. question run | HOLDS | `go test -count=1 ./internal/question` exit 0: `ok .../internal/question 2.114s` (includes TestReserveGate: marker kept, publish error keeps the marker and no reuse, concurrent callers get distinct ids, range exhausted) |
| 4a. lint | HOLDS | `make lint` exit 0 (`go vet ./...`); `gofmt -l` on the three files prints nothing |
| 4b. cross | HOLDS | `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok` |
| 4c. diff names only Touches | HOLDS | `git diff main --stat` (and `origin/main`) for the code commits: internal/orchestrator/events.go (1 line), internal/orchestrator/jr_park_question_file_test.go (new, 131), internal/question/reserve.go (+10/-2). This report file is the one extra file the runbook asks for. |
| Rule: importers tested | HOLDS | Importers of internal/question and internal/orchestrator from `go list -f '{{.Imports}}{{.TestImports}}{{.XTestImports}}' ./...`: cli, feed (external test), migrate, orchestrator, question, task. `go test -count=1 ./internal/question ./internal/orchestrator ./internal/cli ./internal/feed ./internal/migrate ./internal/task` exit 0: question 10.0s, orchestrator 148.6s, cli 102.6s, feed 4.2s, migrate 16.0s, task 10.5s, all `ok` |
| Rule: no guard, gate, cap, hold or hook on agents | HOLDS | None added. The projection runs after the Feed post under the existing questions lock. |
| Rule: no new lock under the questions lock | HOLDS | `ReserveGate` projects only when `s.TaskID != ""` and never calls `feed.EnsureTaskID` (which takes `.task.lock`). With a TaskID, `project` takes no lock: `ReadOnly` skips the cache guard (internal/feed/cache.go:43), `save` is an atomic write. No `.orchestrator.lock` call (no PostPerson, no Jobs port) inside `ReserveGate`. internal/question/reserve.go:30-37 (guard at :33, projection at :36) |
| Rule: a caller with no TaskID behaves as today | HOLDS | `if s.TaskID == "" { return nil }` right after publish; TestReserveGate (stores built in the question tests) passes unchanged, exit 0 |
| Rule: publish error and projection error | HOLDS | A publish error returns as before with no projection (reserve.go:27-29, TestReserveGate "publish error keeps the marker" passes); a projection error is returned (reserve.go:36-37). |
| Rule: small commits, one sentence, no attribution | HOLDS | c0bf02f4 "A Job parked after three failures now writes its question file." and e449c76d "Test that a parked Job leaves one open question file matching its Feed post."; no Co-Authored-By or session line |
| Rule: new tests fail on base, pass on lane | HOLDS | Rows 1b and 1 |
| Rule: no existing test edited or deleted | HOLDS | `git diff main --stat` shows no existing test file touched |
| Rule: tests use no clock, sleep, skip, expected-fail; temp folders only | HOLDS | The new tests use the `setup` fixture (`t.TempDir()`), no sleep, no skip |
| Rule: *RealPlugin* tests | n/a | No row names one; none run |
| Rule: mismatch proof | n/a | No row asks for one |
| Rule: public repo, no personal paths, hosts or emails | HOLDS | Files hold repo-relative paths only |

## What changed

- internal/question/reserve.go: after `publish(id)` returns nil, `ReserveGate` runs `s.project(dir, true)` under the same lock when `s.TaskID` is set, so the published question gets `questions/Q-n.md` with its `post` field. The `.feed` marker stays.
- internal/orchestrator/events.go:102: `park` passes `TaskID: s.Feed.TaskID` to the question store.
- internal/orchestrator/jr_park_question_file_test.go: `TestJrParkWritesQuestionFile`, `TestJrParkOneFile` and `jrPark*` helpers.

## Notes for the backlog

- `park`'s retry-reuse path (events.go:96-100) parks a Job whose question already exists without projecting. If a crash or a projection error lands between the Feed post and the projection, that retry leaves the question file missing until a question verb or `grid question sync` runs. The brief limits events.go to the one line, so I left it.
- `project(dir, true)` rewrites every `Q-n.md` whose Feed question is admitted (it is the same repair `Sync` does), not only the new one.
- `park` builds its question store without `Now`, so the projection state uses the wall clock; the gate default is 24 hours out, so the file is `open` at park time.

## NOT VERIFIED on a live screen

- The screenshot pair at 1440x900 (`/needs-you` on app main against design page `pages/grid-design` Needs you, element `the park question card`) needs a built app and plugin-dist; I ran no app and no browser. A checker takes it after landing.
- The app side (`questionPosts` matching `post:` in `questions/*.md`) was read from the brief only; this lane did not open the app repo. The test proves the plugin writes a file whose `post` equals the Feed question post id.
