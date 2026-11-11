# C-hd-search-hook: a `grid brain search` in a Bash call records the notes it showed

Base: 5a6d3516 (the tip of C-hd-history-why, already merged in). Code head: aaf3410d.
`origin/lane-C-hd-history-why` is the same tip, so `git diff origin/lane-C-hd-history-why --stat` is the lane diff.

## Gap check (row 1, on the base)

Test file committed alone as a79dd15e on base 5a6d3516, before brain.go and brain_search.go changed. `go test -count=1 -run TestHdS ./internal/hook/` exit 1:

```
--- FAIL: TestHdSSearchRecordsHits (0.06s)
    hd_search_read_test.go:49: want exactly one deploy line with the query as the why, got 0:
--- FAIL: TestHdSSearchNamesTheJob (0.04s)
    hd_search_read_test.go:61: no via and why:
--- FAIL: TestHdSSearchOneLinePerNoteAndSession (0.01s)
    hd_search_read_test.go:69: first search: 0 lines:
--- FAIL: TestHdSShowThenSearchAddsOnlyTheOtherNote (0.03s)
--- FAIL: TestHdSSearchRecordsTheNotesTheCLIPrinted (0.07s)
    "grid brain search deploy --limit 1": want 1 lines:
--- FAIL: TestHdSSearchWhyIsTheWords (0.01s)
FAIL	github.com/gridconsole/grid-plugins/internal/hook
```

No search recorded a line on the base, so the gap is real.

## Proof table

| Row / rule | Result |
|---|---|
| 1. `TestHdSSearchRecordsHits` fails on base, passes after | HOLDS. Failure pasted above (base 5a6d3516, test-only commit a79dd15e). `go test -count=1 -race -run TestHdS -v ./internal/hook/` exit 0, PASS. Note: both fixture notes (project `deploy`, home `x`) match "deploy" with the same score, so history holds two lines. Exactly one contains `· brain read · brain:project/deploy · by claude · why search: deploy`, and the other is the home note `brain:home/x` (hd_search_read_test.go:44-57). |
| 2. GRID_JOB gives `· by claude · via J-3 · why search: deploy` | HOLDS. `TestHdSSearchNamesTheJob`, PASS. |
| 3. One line per note and session; no hits writes nothing; `--limit 1` writes one | HOLDS. `TestHdSSearchOneLinePerNoteAndSession` (repeat search, `show deploy`, Read of the note file add nothing) and `TestHdSShowThenSearchAddsOnlyTheOtherNote`. `TestHdSSearchWithoutHitsWritesNothing` (`--topic nothing-here`, no-match words, other topic). `TestHdSSearchRecordsTheNotesTheCLIPrinted` (`--limit 1` is one line, `--level home` is `x` only, `--level project` is `deploy` only). All PASS. |
| 4. Not a search: nothing written, nothing printed; `&&` records | HOLDS. `TestHdSSearchIgnoredForms` holds all five named forms (`grid brain search`, `--bogus`, `echo grid brain search deploy`, `searchx`, `show --proposal deploy`) plus `--limit 0/101/many`, `--level other`, `--level home` with `--project` or `--task`, `--project another`, bad topic, unterminated quote, `mygrid`, empty. Each asserts out "", log "", code 0, history "". `grid brain search deploy && echo done` is in `TestHdSSearchRecordsTheNotesTheCLIPrinted`. PASS. |
| 5. No task resolved, or history-log off | HOLDS. `TestHdSSearchNoTaskOrHookOffWritesNothing`: GRID_TASK unset and cwd outside any task; `SetHook("history-log","off")`. Out and log empty, code 0, history empty. PASS. |
| 6a. `go test -count=1 -race ./internal/hook/` exit 0 | HOLDS. `ok internal/hook 25.120s`, exit 0 (final run on aaf3410d). |
| 6b. with brain_show_read_test.go unchanged | NOT HOLDS, by one deleted row. See "Brief contradiction" below. |
| 6c. `make lint` exit 0 | HOLDS. `go vet ./...`, exit 0. |
| 6d. `GOOS=windows go vet ./internal/hook/` exit 0 | HOLDS. Also `GOOS=windows go build ./...` exit 0 and `GOOS=darwin go build ./...` exit 0. |
| 6e. `git diff lane-C-hd-history-why --stat` names only the Touches files | NOT HOLDS, by the same one file: `brain_show_read_test.go | 1 -`. The other three are brain.go, new brain_search.go and new hd_search_read_test.go. |
| Importers' tests | HOLDS. Packages importing internal/hook (`go list`): internal/cli, internal/packaging; test-only importer: internal/golden. `go test -count=1 ./internal/cli/ ./internal/packaging/` exit 0 (`ok cli 207.353s`, `ok packaging 17.168s`); `go test -count=1 ./internal/golden/` exit 0. |
| No guard, gate, cap or hold on agents | HOLDS. The hook only records. It never refuses, warns or prints. Every parse failure and every search error returns silently (brain_search.go `brainSearch`). |
| No clock, no sleep in new tests | HOLDS. hd_search_read_test.go reads no clock and does not sleep. |
| New tests fail on base, pass on lane | HOLDS. See the gap check. |
| Existing test edited or deleted | One row deleted, named below. |
| *RealPlugin* tests | No row names one. Not run. |

## Brief contradiction (owner to decide)

Row 1 needs `grid brain search deploy` on showFixture to write a line. `TestBrainShowIgnoredForms` (brain_show_read_test.go:146) pinned the same command on the same fixture as writing nothing. Row 6 asks for `go test ./internal/hook/` exit 0 with brain_show_read_test.go unchanged, and Touches does not list that file. All three cannot hold. With the file unchanged the package fails with:

```
--- FAIL: TestBrainShowIgnoredForms
    brain_show_read_test.go:166: "grid brain search deploy" wrote a line: - ... · brain read · brain:project/deploy · by claude · why search: deploy
```

I deleted that one row, in its own commit c4548a24 (the way 75dcbec7 did for C-hd-history-why). The behaviour it pinned ("a search records nothing") was removed by the owner's default in the brief (owner question 1, one `brain read` line per search hit), and `TestHdSSearchRecordsHits` covers the replacement behaviour. The other twelve rows of that test are unchanged. If the owner wants the file untouched, drop c4548a24 and the package stays red at that one row; nothing else depends on it.

## What changed

- `internal/hook/brain_search.go` (new): `parseBrainSearch` reads the first simple command with `shellWords`. It needs binary base `grid` or `grid.exe`, then `brain search <words>`. It takes the CLI's flags (`--json`, `--project`, `--level`, `--task`, `--topic`, `--limit`, with `--flag value`, `--flag=value` and `--`). It refuses what the CLI refuses: an unknown flag, no words, a limit outside 1..100, a bad level, `--level home` with `--task` or `--project`. `brainSearch` runs `Store.Search` once on the hook's `brainStore`, with the parsed words, topic, level and limit, then records each hit. `--project` or `--task` naming another project records nothing (the hook compares the project name, and resolves an id or a task through `Resolver.Resolve` only when it differs). tool_response is not read.
- `internal/hook/brain.go`: the `Bash` case also calls `brainSearch`. `recordBrainRead` now calls the new `recordBrainReadWhy(o, in, dir, id, why)`, which writes through `task.AppendHistoryWhy`. The shared `read:<id>` session key is unchanged, so one note is one line per session whichever way it was read. Via stays from GRID_JOB.
- `internal/hook/hd_search_read_test.go` (new): 10 tests, all `TestHdS*`, reusing showFixture, bash and historyOf.
- `internal/hook/brain_show_read_test.go`: one row removed (see above).

The line format is the existing `brain read` line with the existing trailing why field: `- <time> · brain read · <id> · by <who> [· via <job>] · why search: <words>`. The words are the CLI's joined positional arguments, cleaned and cut to 160 characters by `CleanHistory`.

## NOT VERIFIED on a live screen

- A real `grid brain search` in a live Claude Code Bash call. The tests drive the hook with the PostToolUse input shape the existing show tests use.
- That the app's history reader shows the `why search: ...` text. The line format is the one C-hd-history-why wrote.
- `--project <id>` and `--task <id>` forms resolve through the real Resolver in a live home. Only the project name and a task folder path are tested.
- Hook time under load (no assertion on time, as the brief says). The cost is one `entries()` scan per search, the same as `Show` in brainShow.

DONE
