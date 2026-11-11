# L-brain-query-tests: behaviour tests for brain reads

Repo: plugin, branch lane-L-brain-query-tests. One new file: `internal/brain/query_test.go`. `query.go` is unchanged.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Show: project first, `brain:home/deploy` exact, `brain:team/deploy` and `../x` error, unknown name "not found" | HOLDS | `TestShowResolvesProjectFirstAndExactLevel` (query_test.go:30); `go test -count=1 ./internal/brain/ -run 'TestShow\|TestSplitID\|TestWords\|TestSearch'` exit 0. Also `TestShowIgnoresProposals` and `TestSplitID`. |
| 2. words: {library, running}; {go}; "uses"->"use", "bus" stays | HOLDS | `TestWords` (query_test.go:117), same command, exit 0. It also covers "cats", "boxes"->"boxe" and the stop-word-only input. |
| 3. Search ranking: description 4 vs body 1; name or summary 2; tie keeps project before home | HOLDS | `TestSearchRanking`, `TestSearchScoresNameAndSummaryTwo`, `TestSearchTiesKeepProjectBeforeHome`, same command, exit 0. The tie test writes home first. |
| 4. Search filters: limit 0 and 101 error; topic "a/b" errors; topic filter; `superseded_by` never returned; limit 1; no body in a Hit | HOLDS | `TestSearchRefusesBadLimitAndTopic`, `TestSearchTopicSupersededLimitAndNoBody`, same command, exit 0. The superseded note is a valid accepted file that `Show` reads, so only `superseded_by` hides it. The Hit JSON is checked for the body marker and a `"body"` key, and for exactly 7 keys. |
| 5. `go test -count=1 ./internal/brain/` 0 fail | HOLDS | exit 0: `ok github.com/gridconsole/grid-plugins/internal/brain 2.512s` |
| New tests fail on base | HOLDS, with a note | The file is new; base has no test of these names. Mutation check: with `score += 4` changed to `score += 3` in query.go, 4 tests failed, for example `query_test.go:149: description 4 ahead of body 1: ... Score:3`, `query_test.go:187: scores = map[... everywhere:6 ...]; want ... 7`, `query_test.go:217: scores: ... Score:3`, `query_test.go:313: limit 1: ... Score:3`. query.go was restored with `git checkout`; `git status` shows only the new test file. |
| No existing test edited or deleted | HOLDS | `git status --short`: only `?? internal/brain/query_test.go` before this report. |
| Importers' tests | HOLDS | `go list` direct importers of internal/brain: braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, routine, task, workflow. `go test -count=1` on those (job/... included): all `ok`, exit 0. |
| Lint | HOLDS | `make lint` (go vet ./... and gofmt -l .) exit 0, no output. |
| Windows and macOS builds | HOLDS | `GOOS=windows go vet ./internal/brain/` exit 0; `GOOS=darwin go vet ./internal/brain/` exit 0. These compile the package and its tests. |
| No guard, gate, cap or hold added | HOLDS | Only a test file was added. |
| Bug found in query.go | HOLDS: no bug found, so nothing is left out | Two behaviours look odd but match the brief, so they are asserted as stated: "boxes" folds to "boxe" (query.go:90, the "es"->"e" rule), and the length test counts bytes, not characters (query.go:83, :86). Neither is asserted beyond the "boxes" case. |

## What changed

- New `internal/brain/query_test.go` with 9 tests: `TestShowResolvesProjectFirstAndExactLevel`, `TestShowIgnoresProposals`, `TestSplitID`, `TestWords`, `TestSearchRanking`, `TestSearchScoresNameAndSummaryTwo`, `TestSearchTiesKeepProjectBeforeHome`, `TestSearchRefusesBadLimitAndTopic`, `TestSearchTopicSupersededLimitAndNoBody`.
- Notes are written through Propose and Accept, except the superseded one, which is a file under `knowledge/decisions/`.

## NOT VERIFIED on a live screen

Nothing here touches a screen; there is no UI row.
