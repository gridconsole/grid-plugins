# L-brain-index-topics-tests

Behaviour tests for the brain index rows, backlinks, SessionStart text and topic resolution. Repo: plugin. One new file: `internal/brain/index_topics_test.go`. No change to `index.go` or `topics.go`.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1a rows: two valid notes + one unparseable note, newer `updated` first by name | HOLDS | `TestRowsCountsEntriesInvalidAndLastChange` (index_topics_test.go:51): decisions Entries 2, Invalid 1, LastChange = newer `updated`. `go test -count=1 ./internal/brain/` exit 0 |
| 1b rows: invalid TOPIC.md counts in `Invalid` | HOLDS, with a deviation (below) | same test: custom topic `testing` with a bad TOPIC.md reads Invalid 1, Entries 0, "Invalid topic definition"; decisions Invalid + that Invalid = 2 |
| 1c rows: config topic, one valid and one undecodable file | HOLDS | same test: `environments` Entries 1, Invalid 1 (index.go:74-93) |
| 1d rows: note in a home-only built-in topic under project is an extra row "Invalid topic level" | HOLDS | `TestRowsShowsWrongLevelFileAsExtraRow` (`machines` at project, `conventions` at home): last row Description "Invalid topic level", Invalid 1, Entries 0, no duplicate row (index.go:105-115) |
| 2a readBacklinks on a missing file | HOLDS | `TestReadBacklinksMissingAndMalformed`: Generation 0, empty non-nil Links, nil error (index.go:44-46) |
| 2b malformed backlinks.json | HOLDS | same test: `readBacklinks` and `Index` return "invalid backlinks index" (index.go:51-52, :142-146); `RebuildIndex` also errors (:219-222) and leaves the file untouched |
| 2c RebuildIndex: one Backlink per link, From/Rel/Summary/Why | HOLDS | `TestRebuildIndexWritesBacklinkPerLinkAndBumpsGeneration`: entry for `brain:project/bee` is exactly {From `brain:project/ay`, Rel `explains`, Summary = ay's note summary, Why = link's why}; a second link to a job gives its own entry; two targets, no entry for the unlinked note |
| 2d generation +1 per call | HOLDS | same test: project and home 1, then 2; backlinks.json Generation 2; index.md starts `generation: 2\nlevel: project` |
| 3a 200-char description shown as 77 bytes + "..." | HOLDS | `TestSessionStartTextCutsLongDescription`: exact row match; also 80 bytes kept whole, 81 cut; pipes become `/`, white space collapses, date cut to 10 bytes |
| 3b 40 rows at most 1,200 bytes, ends with elision banner | HOLDS | `TestSessionStartTextElidesOverflowWithinBudget`: len <= 1200, banner count equals 40 minus rows shown, text ends with it; 3 rows give no banner |
| 3c multibyte cut on a rune boundary | HOLDS | `TestSessionStartTextCutsMultibyteOnRuneBoundary` (2, 3 and 4-byte runes): `utf8.ValidString` holds, exact rune counts 38/25/19; `TestTruncateNeverSplitsARune` checks every byte limit |
| 4a BuiltinTopics present in Topics() at each level they allow | HOLDS | `TestScopeTopicsAndTopicsMergeBothLevels`: scopeTopics lists the allowed built-ins first at project and home; Topics() keeps every built-in's Levels, not Custom, no duplicates. `TestDeclaresLevelAndAllows` covers `allows` for home, project, both, none |
| 4b `level: home` offered at home only; both at both; unknown level invalid | HOLDS | same test: `homeonly` Levels [home]; `both` Levels [project home]; `weird` (level: team) Kind invalid; a home-declared topic in the project tree (`misplaced`) is invalid. `declaresLevel` table covers string, "a, b", list, non-string items, nil, number |
| 5 `go test -count=1 ./internal/brain/` | HOLDS | exit 0, 15.8s |
| New tests fail on base | HOLDS by mutation | The lane adds tests only, so base has no failing run. Mutation: `truncate` cut one byte (index.go:200), reverted with `git checkout`: `--- FAIL: TestSessionStartTextCutsMultibyteOnRuneBoundary/two-byte ... invalid UTF-8`, `--- FAIL: TestTruncateNeverSplitsARune ... truncate(2) = "a\xc3"`. Tree after revert: only the new test file |
| No existing test edited or deleted | HOLDS | `git status`: only `index_topics_test.go` and this report |
| Packages that import brain | HOLDS | `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/cli/ ./internal/hook/ ./internal/job/... ./internal/people/ ./internal/routine/ ./internal/task/ ./internal/workflow/`: all ok, exit 0 (list from `go list` imports of internal/brain: braincmd cli hook job job/jobcmd job/jobtest people routine task workflow) |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -l internal/brain/` empty |
| Windows and macOS builds | HOLDS | `make cross` exit 0 (darwin/arm64 and windows/amd64: vet and test compile of every package) |
| No guard, gate, cap or hold added | HOLDS | test file only |
| Public repo hygiene | HOLDS | no personal paths, hosts or emails; no attribution lines in the commit |

## Deviation from the brief

Row 1 reads "a topic with two valid notes, one unparseable note file and an invalid TOPIC.md reads Entries 2, Invalid 2". The code cannot give that in one topic row:
- a TOPIC.md in a built-in topic is never read (topics.go:148 skips known names; topics.go:218 skips TOPIC.md in `scan`), so it adds nothing there. The test asserts this.
- an invalid TOPIC.md only exists in a custom topic, whose row is Kind invalid (index.go:71-73). In that topic every note fails `gate` because `topicAt` fails (store.go:531), so even a valid note there counts as Invalid.

The test therefore asserts the two rows separately: decisions Entries 2, Invalid 1; custom topic with an invalid TOPIC.md Invalid 1; their invalid counts sum to 2. This is the behaviour the code has, not a bug I am reporting.

## What changed

- `internal/brain/index_topics_test.go` (new, 10 tests, 420 lines): rows counting, wrong-level extra rows, readBacklinks, RebuildIndex backlinks and generations, SessionStartText cut/budget/multibyte, truncate, declaresLevel/allows, scopeTopics and Topics merging.

## NOT VERIFIED on a live screen

Nothing here is UI. Nothing needs a live screen.

## Bugs found

None.
