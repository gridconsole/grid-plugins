# O-render-brain-recall

`grid job render` now searches the brain for the Job's title and intent words and passes the hits to `job.Render` as `RenderOptions.Recall`.

Base: main `330256ce`.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. New test fails on main through the `grid job render` entry | HOLDS | With `Recall: w.recall(j)` taken out of command.go (main's call), `go test -count=1 -run 'Recall\|Superseded\|NoBrain\|SearchError' ./internal/job/jobcmd/` exits 1: `render_recall_test.go:98: 0 sources with pointer brain:project/deploy, want 1:` (TestRenderRecallsANoteThatMatchesTheTitle); also FAIL TestSupersededNoteIsNeverRecalled `:175 the current note was not recalled`, TestPersonalJobRecallsFromTheHomeBrainOnly `:232 the home note was not recalled`, TestRecallRenderIsByteIdentical `:301 the renders hold no recall`. |
| 2. After the change: `brain_tokens > 0`, one source `brain:project/deploy` with summary = description, brief holds the block with a why | HOLDS | `TestRenderRecallsANoteThatMatchesTheTitle` (render_recall_test.go:93) passes. It asserts brain_tokens > 0, exactly 1 source, summary "How we deploy the web app to Netlify", one block `- brain:project/deploy · uses · <desc> · recalled: matches the Job's title and intent words [tier]` and its Excerpt. Each `Link` has `Rel: "uses"` (the value the plan's `keep` and `links` use) and a non-empty `Why` (recall.go:42; `TestRecalledLinksCarryARelAndAWhy`). |
| 3a. A note already named in `links` is not added twice | HOLDS | `TestRecalledNoteAlsoNamedInLinksIsRenderedOnce`: one source, one block, the planner's own why wins (rank 2 over 4). It also passes on main, since main never recalls; it guards the dedupe. |
| 3b. A superseded note is never recalled | HOLDS | `TestSupersededNoteIsNeverRecalled`: a note file with `superseded_by` matching the title is absent from inputs.json; the current note is present. |
| 3c. Personal searches the home brain only | HOLDS | `TestPersonalJobRecallsFromTheHomeBrainOnly`: a Personal task (made with `task.Store.New`, project "") recalls `brain:home/export-habit` and not a matching `brain:project/export-rule`. |
| 4a. No brain (no knowledge/ folder) | HOLDS | `TestNoBrainRendersWithoutRecall`: exit 0, `brain_tokens` 0, no brain source, stderr empty. |
| 4b. Search error | HOLDS, with a limit | `TestSearchErrorRendersWithoutRecall`: with a symlink inside knowledge/, `w.recall` returns nil and prints nothing. Any fault that makes `Search` fail also makes `w.generation()` fail first (command.go:117), so the entry exits 1 with that error, as on main. The test asserts that exit and error too. I found no fixture where only Search fails, so the "render goes on" half of 4b is checked on `recall`, not on a whole render. |
| 5. Deterministic render | HOLDS | `TestRecallRenderIsByteIdentical`: two renders give byte-identical brief and inputs.json, and hold the recalled note. The generation before/after check (command.go:141-147) runs after the search; the search is read-only (`entries` only scans). |
| 6a. `go test -count=1 ./internal/job/... ./internal/brain/ ./internal/cli/` | HOLDS | exit 0: job, job/frontmatter, job/jobcmd, job/lint, brain, cli all `ok`. |
| 6b. `go vet ./internal/job/...` | HOLDS | exit 0 (also `./internal/cli/`). |
| 6c. `GOOS=windows CGO_ENABLED=0 go vet ./internal/job/...` | HOLDS | exit 0. |
| Packages that import the changed one | HOLDS | `go list` shows `internal/cli` imports `internal/job/jobcmd`; its tests pass (6a). |
| Lint, Windows and macOS builds | HOLDS | `gofmt -l internal/job/` empty after `gofmt -w` on the new test; `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard, gate, cap or hold added | HOLDS | The one number is `recallLimit = 8` (recall.go:13), the count render can show anyway, and a comment says so. render.go, the 8-block and 2,000-token bounds, and internal/brain are untouched. |
| New tests in a new file | HOLDS | internal/job/jobcmd/render_recall_test.go; no existing test edited or deleted. |
| Commit rules | HOLDS | One commit with a one-sentence message and no attribution lines. No personal paths, hosts or emails. |

## What changed

- `internal/job/jobcmd/recall.go` (new): `(*world).recall(j)` queries `brain.Store.Search` with the Job's title plus the resolved text of its intent item and goal (`plan.md#step-N`, `plan.md#verify`), limit 8. It returns `Link{To: hit.ID, Rel: "uses", Why: "recalled: matches the Job's title and intent words"}`.
- It skips a hit that `Render` could not show (summary empty or over 300 characters), so recall can never make a render fail that would pass without it.
- A Search error gives no recall and no output.
- `internal/job/jobcmd/command.go:137`: passes `Recall: w.recall(j)`.
- `internal/job/jobcmd/render_recall_test.go` (new): the eight tests named above.

I left the owner's request (`intent.owner`) out of the query on purpose. It is the whole task's prose and would match notes the Job does not need. Title, item and goal are the Job's own words.

## NOT VERIFIED on a live screen

- Nothing here changes a UI. How the recalled pointers read in a live brief and on the dev UI is left to the orchestrator's live check.
- Ranking quality on a real, large brain was not measured. The search scoring is unchanged brain code.

DONE
