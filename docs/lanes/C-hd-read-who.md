# Lane C-hd-read-who

A brain read outside a Job names its session in `via`, and a read inside a Job carries the why of the Job's link to the note. Stacked on C-hd-search-hook (base 2f97c46f).

## Proof table

| Row / rule | State | Proof |
|---|---|---|
| 1. `TestHdRSessionNamed`: no GRID_JOB, `grid brain show deploy` writes one line with `· brain read · brain:project/deploy · by claude · via session session-demo`; brain_show_read_test.go:78 asserts that form | HOLDS | `go test -count=1 -race ./internal/hook/` exit 0; test at hd_read_who_test.go:77 (also a Read, and two sessions `session-a`/`session-b` of one task give two lines that differ in via); brain_show_read_test.go:78 edited to the session form and passes |
| 2. `TestHdRJobLinkWhy`: J-3 with a link `brain:project/deploy#Steps` "Keep deploys the same": Read and show give `· via J-3 · why Keep deploys the same`; a search hit on it gives the link why, not `search: deploy` | HOLDS | hd_read_who_test.go:108, same run exit 0; job.yaml is written as JSON by the test (`hdRJob`); also a link without an anchor |
| 3. A Job with no link to the note: `· via J-3` and no ` · why ` (Read and show); a search keeps `· why search: deploy` | HOLDS | `TestHdRJobWithoutLinkHasNoWhy` hd_read_who_test.go:134 (links to `brain:home/x`, `brain:project/deploy-more` and `plan.md#deploy` give no why for deploy), same run exit 0 |
| 4. job.yaml missing, not JSON, wrong-typed links, a folder, a symlink to a file outside the task, GRID_JOB `../x` and `../../x`: line written as row 3 / row 1, out and log "", code 0, nothing outside opened | HOLDS | `TestHdRUnreadableJobFileGivesNoWhy` hd_read_who_test.go:154: each case runs Read, show and search; the helpers assert out, log, code (`hdSQuiet`, `hdRRead`); the outside why "outside" never appears. The symlink case skips on Windows only. Mismatch proof below |
| 5a. `go test -count=1 -race ./internal/hook/` | HOLDS | exit 0 (`ok internal/hook 17.2s`); `TestBrainReadNamesTheJob` (brain_show_read_test.go:170) passes |
| 5b. `make lint` | HOLDS | exit 0 (runs `go vet ./...`) |
| 5c. `GOOS=windows go vet ./internal/hook/` | HOLDS | exit 0 |
| 5d. `git diff <base> --stat` names only the files in Touches | NOT DONE, one file over | It names `internal/hook/hd_search_read_test.go` too (2 lines), see Open items. The other files are `brain.go`, `brain_why.go`, `brain_show_read_test.go`, `hd_read_who_test.go`. With this report, `docs/lanes/C-hd-read-who.md` is one more |
| New tests fail on base | HOLDS | Base code with the new tests: `--- FAIL: TestHdRSessionNamed` `hd_read_who_test.go:81: show outside a Job: - … · brain read · brain:project/deploy · by claude`; `--- FAIL: TestHdRJobLinkWhy` `hd_read_who_test.go:111: read in a Job with a link: - … · by claude · via J-3`; `--- FAIL: TestHdRJobWithoutLinkHasNoWhy` `hd_read_who_test.go:150: link to the other note: - … brain:home/x · by claude · via J-3 · why search: deploy`; `--- FAIL: TestBrainShowLogsOneReadLine` `brain_show_read_test.go:79: history: - … · by claude`. `TestHdRUnreadableJobFileGivesNoWhy` passes on base because base never gives a why; its mismatch proof is below |
| Mismatch proof for row 4 | HOLDS | With `brainLinkWhy` made to `os.ReadFile(filepath.Join(taskDir,"jobs",jobID,"job.yaml"))` (no `ValidID`, no `Within`): `FAIL TestHdRUnreadableJobFileGivesNoWhy/symlink_out_of_the_task … via J-3 · why outside`, `…/Job_id_../x … via ../x · why outside`, `…/Job_id_../../x … via ../../x · why outside`. Then `git checkout -- internal/hook/brain_why.go`; `git status --short` printed nothing |
| Importers of `internal/hook` tested (`go list` imports) | HOLDS | importers: `internal/cli`, `internal/packaging` (and through them `cmd/grid`, `lib`, `internal/packaging/cmd`). `go test -count=1 -race ./internal/hook/ ./internal/cli/ ./internal/packaging/...` exit 0: hook ok, cli ok (276s), packaging ok, packaging/cmd ok |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0; `gofmt -l internal/hook` prints nothing |
| No test reads the clock or sleeps; no timing assertion | HOLDS | hd_read_who_test.go has no `time` import and no sleep |
| No guard, gate, cap or hold on agents; hook only records | HOLDS | `brainLinkWhy` (brain_why.go:28) returns "" on every failure and nothing is printed or refused; the hook still exits 0 |
| Nothing removed; no pushing | HOLDS | no test, helper or behaviour is deleted; the removed lines are the replaced assertion at brain_show_read_test.go:78, the two `hdS` constants, and the rewritten comment and `AppendHistoryWhy` call in brain.go (see Open items); nothing pushed |
| RealPlugin tests | NOT APPLICABLE | no row names one |

## What changed

- `internal/hook/brain.go`: `recordBrainReadWhy` now takes the via from `brainReadVia` (GRID_JOB when set, as today, otherwise `session <in.SessionID>`) and, on the first read of a note in a session, asks `brainLinkWhy` for the Job link why, which wins over the search words. `by` is still `writer(in)`. `recordBrainRead` (Read and show) calls it with no why.
- `internal/hook/brain_why.go` (new): `brainReadVia`, and `brainLinkWhy(taskDir, jobID, id)`: needs `job.ValidID(jobID)`, opens `jobs/<id>/job.yaml` with `job.Within`, refuses anything that is not a regular file, decodes the JSON into `job.Job`, and returns the why of the first link whose `to` (anchor cut) equals the note id and whose why is not blank. Any failure gives "".
- The lookup runs only when the line is going to be written (first read of the note in the session), so a repeated read does not open job.yaml.
- Tests: `hd_read_who_test.go` (new); `brain_show_read_test.go:78` now asserts the session form; `hd_search_read_test.go:13-14`: the two `hdS` constants gained ` · via session session-demo`.

## Open items

1. `hd_search_read_test.go` is not in Touches, but four of its tests (`TestHdSSearchRecordsHits`, `…ShowThenSearchAddsOnlyTheOtherNote`, `…RecordsTheNotesTheCLIPrinted`, `…WhyIsTheWords`) match `· by claude` directly followed by ` · why search: …`, which is the no-via form this brief replaces for every read outside a Job. Row 5 (`go test ./internal/hook/` exit 0) cannot hold without it, so I changed the two constants `hdSDeploy` and `hdSHomeX` (lines 13-14) and nothing else. Their assertions are as strong as before. Orchestrator: accept the extra file or tell me to revert it and take row 5a as failing.
2. A GRID_JOB that is set but is not a valid Job id (`../x`) keeps the brief's "GRID_JOB when set, as today": the line says `via ../x`, not `via session …`. The app rule that only a `J-` via is a reader (C-hd-taskview-plugin-lines) would ignore such a line and not see the session. Left as the brief says.
3. If a Job has two links to one note, the first with a non-blank why is used.
4. History format change: every `brain read` line outside a Job now ends `· via session <id>` (before: nothing). This is owner question 2's default; another answer changes rows 1 and 4 only.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a real Claude session with the built plugin, so the session id the live hook receives (`in.SessionID`) is taken from the test fixture (`session-demo`) only.
