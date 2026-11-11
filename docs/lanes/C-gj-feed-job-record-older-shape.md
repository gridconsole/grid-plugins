# C-gj-feed-job-record-older-shape

Repo: plugin. Branch: lane-C-gj-feed-job-record-older-shape. Base: b8324c53a776fefe1faadb5b7094d3a4e6f7598a.
Code head (tests and fixtures): 39b7ecef; this report is committed on top of it.

## Base check

I read read.go:38-110, contract_test.go:225-281 and fold.go:110-140 on the base. No test in `internal/feed` mentions
`max_rounds` (`grep -rln max_rounds internal --include=*_test.go` lists orchestrator, job/jobcmd, job/lint, task and
contract tests only; none is in `internal/feed`). So the stop condition does not apply. For the record: the nearest other
test is `internal/task/approval_read_test.go:66`, which posts a person Job body with `max_rounds` through `Store.Post`
for an approval test; it is in another package, written by today's code, and does not read older bytes from git.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 TestGjOlderRecordCounts | HOLDS | gj_older_job_record_test.go:154. `go test -count=1 -v -run TestGjOlderRecord ./internal/feed/` exit 0, `--- PASS: TestGjOlderRecordCounts` with subtests whole_record and planned_fields_body. Verdict "counted", Reason "", Body has review "L1" and max_rounds 2, body equal to the fixture body (reflect.DeepEqual). A control subtest (body without title gives "ignored") PASSes, so "counted" is not the helper never looking. |
| 2 TestGjOlderRecordFolds | HOLDS | :198. `s.State()` has Job J-1, title "Build example feature", Status posted, role builder, kind "non-code" for the planned-fields body, fold verdict "counted"; the marshalled state has no "max_rounds" or "level_raised" key anywhere and the Job has no "review". PASS in the same run. |
| 3 TestGjOlderRecordMixedWithNew | HOLDS | :241. Old record (seq 1, fence 0 file) plus `foldJob` J-2 (today's shape, fold_state_test.go:18): both "counted", `foldJobIDs` = [J-1 J-2], both posted. PASS in the same run. |
| 4 TestGjOlderRecordFixturesKeepTheOldShape | HOLDS | :275. Both testdata files hold review "L1", max_rounds 2 and level_raised; job-basic.json is type job; README.md names e11cf80a (testdata/older-records/README.md:3). The files equal git: `git show e11cf80a^:<path> \| diff - <file>` printed nothing for both. PASS in the same run. |
| 5 home rule | HOLDS | gjOldHome (:26) t.Setenv's HOME, CLAUDE_CONFIG_DIR, CODEX_HOME and GRID_HOME to folders under one t.TempDir() and t.Fatal's when HOME is not under it; every test calls it first (gjOldStore :43 or directly at :276). It logs no value. |
| 5 no sleep in the file | HOLDS | `grep -n -i "sleep\|/home/\|/srv/\|gridwork"` on the test file and README: no match. |
| 5 named run | HOLDS | `go test -count=1 -run 'TestGjOlderRecord\|TestReaderUnknownAndBad\|TestFoldRead' ./internal/feed/` exit 0: `ok  github.com/gridconsole/grid-plugins/internal/feed	0.174s` |
| 5 make lint | HOLDS | `make lint` exit 0 (go vet ./..., gofmt -l empty). |
| 5 diff stat | HOLDS | `git diff main --stat` at 39b7ecef: gj_older_job_record_test.go, testdata/older-records/{README.md, job-basic.json, job-record-planned-fields.json}, 4 files, 456 insertions. This report (docs/lanes) is the runbook's fifth file and is added after that check. |
| Windows and macOS builds | HOLDS | `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok. |
| Packages that import the changed package | HOLDS | Only test files and testdata changed in `internal/feed`. Importers from `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...`: approvefix, cli, hook, job/jobcmd, lease, mcp, migrate, orchestrator, question, task, usage, lib. `go test -count=1` on those plus `internal/feed` exit 0: feed ok 8.058s, cli ok 73.215s, hook ok 10.769s, job/jobcmd ok 6.307s, lease ok 0.366s, mcp ok 0.776s, migrate ok 20.329s, orchestrator ok 71.517s, question ok 4.790s, task ok 12.920s, usage ok 0.095s, lib ok 6.897s, approvefix no test files. |
| New tests fail on base | NOT APPLICABLE, nothing failed | No production file changed, so the base (b8324c53) runs the same reader and fold as the lane; every row holds on it and there are no failing lines to paste. No row failed, so no fix went into read.go or fold.go. The control subtest in row 1 is the evidence that the helper path can produce a non-counted verdict. |
| No existing test edited or deleted | HOLDS | `git diff main --stat` lists no existing file; nothing deleted. |
| Older bytes taken from git | HOLDS | job-basic.json and job-record-planned-fields.json were written with `git show e11cf80a^:<path> > <file>`, not by hand. |
| Small commits, one-sentence messages, no attribution | HOLDS | 326b3c75 (fixtures), 39b7ecef (test), then this report; no Co-Authored-By or Claude-Session line (see the log check in the hand-off). |
| Public repo: no personal paths, hosts, emails | HOLDS | grep above; the fixtures hold only the example ids from git. |
| No guard, gate, cap, hold or blocking hook | HOLDS | No production code changed; nothing was added. |
| RealPlugin tests, UI rows | NOT APPLICABLE | No test of this lane is named *RealPlugin*; no UI. |
| Whole-module test, cross-repo set | NOT RUN | The orchestrator runs them at landing. |

## What changed

- `internal/feed/testdata/older-records/`: `job-basic.json` (a whole Feed record of type job, from `git show e11cf80a^:contract/fixtures/job/valid/basic.json`), `job-record-planned-fields.json` (a Job body, from `git show e11cf80a^:contract/fixtures/job-record/valid/planned-fields.json`) and `README.md` naming e11cf80a.
- `internal/feed/gj_older_job_record_test.go`: the four tests, run for both fixtures where it makes sense. The whole record keeps its envelope from git; only task, worker, seq and the author id are set to the test store's (testTask, testWriter). The planned-fields body is wrapped in that envelope as a second record with its own id (the reader drops a repeated record id, and both fixtures use Job J-1, so they are never in one store).
- Finding: the current reader and fold already accept and fold an older Job record. The schema drop of e11cf80a did not strand any old Job.

## NOT VERIFIED on a live screen

Nothing here needs one; the lane has no UI. Not run by me: the whole-module test and the cross-repo set (orchestrator, at landing).
