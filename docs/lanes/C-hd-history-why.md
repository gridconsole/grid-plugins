# C-hd-history-why: a history line can say why, and `brain given` carries the link's why

Base: 91fadeff87525f5859978740e0b66ce51f375e48 (main). Lane commits, oldest first:
6726e2ce (HistoryLine why field, AppendHistoryWhy, task tests), ee711060 (render passes the
link's why, jobcmd tests, updated assertion), 75dcbec7 (two superseded TestHistoryLineVia rows
removed, see Open item 1), then this report.

Base check: HOLDS. On base, `HistoryLine` (internal/taskfile/frontmatter.go:152-162) writes
`- <time> · <event> · <detail> · by <who> · via <via>` and nothing after it, and
`AppendHistory` (internal/task/history.go:108) has no why argument.

## Open item 1 (read first): the brief contradicts an existing test

Row 1 makes the second trailing argument of `HistoryLine` the why. The existing
`TestHistoryLineVia` (internal/taskfile/taskfile_units_test.go) pinned the opposite for the
same call shape: `HistoryLine("t","ev","d","who","app","cli")` must end `· via app`, and
`HistoryLine(..., "", "cli")` must have no via and nothing after `by who`. No implementation
satisfies both, so row 3 (`./internal/taskfile/` exit 0) and row 6 (diff names only the
Touches files) cannot both hold as written. Failing run with the brief's API and without
the removal:

```
--- FAIL: TestHistoryLineVia (0.00s)
    --- FAIL: TestHistoryLineVia/only_the_first_via_is_used (0.00s)
        taskfile_units_test.go:267: got "- t · ev · d · by who · via app · why cli"; want "- t · ev · d · by who · via app"
    --- FAIL: TestHistoryLineVia/empty_first_via_hides_a_later_one (0.00s)
        taskfile_units_test.go:267: got "- t · ev · d · by who · why cli"; want "- t · ev · d · by who"
FAIL	github.com/gridconsole/grid-plugins/internal/taskfile
```

What I did: deleted those two rows (not retargeted them) in their own commit 75dcbec7. The
behaviour they pinned ("a second trailing argument is ignored") is removed by brief
C-hd-history-why row 1; the new meaning of that argument is covered by `TestHdWLine`
(internal/task/hd_history_why_test.go:32). The other six rows of `TestHistoryLineVia` are
untouched and pass. If the orchestrator does not accept the deletion, drop commit 75dcbec7:
row 6 then holds and row 3 fails with the lines above. That is the orchestrator's call, not mine.

## Proof table

| # | Row | Result | Command / place |
|---|-----|--------|-----------------|
| 1 | `TestHdWLine`: via+why, why without via, no why equals the 4-argument call | HOLDS | `go test -count=1 -run TestHdW ./internal/task/` exit 0; internal/task/hd_history_why_test.go:32 |
| 2 | `TestHdWAppend`: "a · b" + newline gives `· why a - b`, 300 runes cut to 160, same call twice adds nothing, other why appends | HOLDS | same run, exit 0; hd_history_why_test.go:61; also `TestHdWAppendHistoryWritesNoWhy` (AppendHistory writes no why field) |
| 3 | `go test -count=1 -race ./internal/task/ ./internal/taskfile/` exit 0; `TestAppendHistoryCreatesHistoryInAFreshTask` unchanged | HOLDS with 75dcbec7 only (Open item 1) | `ok  internal/task 30.971s`, `ok  internal/taskfile 1.406s`; history_create_test.go not touched |
| 4 | `TestHdGGivenWhy`: one line ending `· brain given · brain:project/deploy · by grid job render · via J-1 · why Carry this note into the Job`; a keep:-only note gets no `· why ` | HOLDS | `go test -count=1 -run TestHdG ./internal/job/jobcmd/` exit 0; hd_given_why_test.go:45 (linked), :76 (keep: only, own fixture `hdGKeepFixture` with `links: []`), :93 (`givenWhy` picks the link of the note, anchor cut) |
| 5a | `go test -count=1 -race ./internal/job/jobcmd/ ./internal/hook/` exit 0 | HOLDS | `ok  internal/job/jobcmd 15.011s`, `ok  internal/hook 11.608s`; render_brain_given_test.go:108-112 updated (deploy line ends with the why, the recalled export-format line has none); brain_show_read_test.go unchanged |
| 5b | `make lint` exit 0 | HOLDS | `make lint && echo LINT-OK` printed `go vet ./...` then `LINT-OK` |
| 5c | `GOOS=windows go vet ./internal/task/ ./internal/job/jobcmd/` exit 0 | HOLDS | run with `./internal/taskfile/` added, no output, exit 0 |
| 6 | `git diff main --stat` names only the Touches files | NOT AS WRITTEN | two extra files: internal/taskfile/taskfile_units_test.go (2 deletions, Open item 1) and this report (the runbook requires it) |

Rules:

| Rule | Result | Evidence |
|------|--------|----------|
| New tests fail on base, pass on lane | HOLDS | base source files with the new tests: `internal/task/hd_history_why_test.go:66:12: undefined: AppendHistoryWhy` and `internal/job/jobcmd/hd_given_why_test.go:107:13: undefined: givenWhy`, both `[build failed]`. Behavioural failure with the base `HistoryLine` and the lane's other files: `--- FAIL: TestHdWLine ... got "- 2026-11-11T09:00 · brain read · brain:project/deploy · by claude · via J-3", want "... · via J-3 · why search: deploy"`, `--- FAIL: TestHdWAppend`, and `--- FAIL: TestHdGGivenWhy ... want exactly one deploy line ending with the link's why, got 0 in [... via J-1" ...]`. Passing run: see rows 1, 2, 4. |
| No existing test edited to pass; deletions named | HOLDS with one deletion | render_brain_given_test.go:108-112 is the one suffix assertion the brief names (updated, not weakened: the deploy line must end with the why, the other line must end as before). Deleted: two `TestHistoryLineVia` rows, Open item 1. |
| Importers tested | HOLDS | `go list` importers of task, taskfile, jobcmd: braincmd, cli, guard, hook, job, jobcmd, jobtest, land, plan, signal, task, workflow; test-only importers migrate, orchestrator. `go test -count=1` on all of them, plus job/..., exit 0 for each. First run of `internal/land` failed once with `TestWorkflowApproval/ship/plan: exec: WaitDelay expired before I/O complete` while the other packages ran in parallel; `go test -count=1 ./internal/land/` alone: `ok 1.623s`. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./... && GOOS=darwin go build ./... && echo BUILD-OK` printed `BUILD-OK` |
| gofmt | HOLDS | `gofmt -l internal/task internal/taskfile internal/job/jobcmd` printed nothing |
| No guard, gate, cap or hold on agents | HOLDS | the only limit touched is the existing 160-rune `CleanHistory`, which the brief names; nothing blocks or refuses an agent |
| No attribution lines, no personal paths | HOLDS | three commits, one sentence each, no trailers; this file holds no absolute path, host or email |
| Test clock | HOLDS | the new tests read no clock and sleep nowhere; expected lines are built from the stamp the file holds, or compared after the stamp |
| `*RealPlugin*` tests | not applicable | no row names one; none was run |

## What changed

- internal/taskfile/frontmatter.go: `HistoryLine` writes ` · why <text>` last when its second
  trailing argument is non-empty; the first stays the via. Lines without a why are byte-identical to main.
- internal/task/history.go: new `AppendHistoryWhy(dir, event, detail, who, via, why)`, the body
  of the old `AppendHistory` with the why cleaned by `CleanHistory` (so "·" becomes "-", whitespace
  and newlines collapse, 160 runes at most) and included in the repeat compare after the time.
  `AppendHistory` keeps its signature and calls it with an empty why.
- internal/job/jobcmd/command.go: the `brain given` loop calls `AppendHistoryWhy` with
  `givenWhy(j, id)`: the why of the first Job link whose `to`, with an anchor after `#` cut,
  equals the id and whose why is not blank. A note given through `keep:` or `decisions:` with no
  link gets none.
- Tests: internal/task/hd_history_why_test.go, internal/job/jobcmd/hd_given_why_test.go,
  one assertion in render_brain_given_test.go:108-112, two rows removed from TestHistoryLineVia.

Notes for the backlog:

- `lib-historyline` in internal/hook/events.go:402 still forwards only argument 4 (the via), so the
  diagnostic helper cannot print a why. Not in Touches; C-hd-search-hook or C-hd-read-who may want it.
- In this plugin no code outside the writer parses a history line's `· by ` or `· via ` tail
  (grep over non-test code), so a trailing field breaks no plugin reader.
- Two links to one note: the first with a non-blank why wins; the others are not joined.

## NOT VERIFIED on a live screen

- No UI row. The app readers (app 45178c45 internal/slices/taskview/view_brain.go:97-98,
  internal/slices/brain/read.go:976-977) need only parts 0 to 3 of a line; I did not have the app
  repo and did not run them. That claim is the brief's, not mine.
- The run against the built plugin binary is not part of any row and was not done.
