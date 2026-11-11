# Q-inputs-shown-mode

inputs.json says, per source, how the brief used it (`shown`: text, excerpt, summary or pointer), and gives `brief_bytes` and `pasted_chars`. `Text` stays in every source, so replay is unchanged.

Base: main `3ccec5cb`. The lane sits on `lane-Q-render-read-summary` (already merged into this branch).

## Proof table

| Row | Result |
| --- | --- |
| 1. Test that fails on main | HOLDS. With `internal/job/render.go` and `source.go` taken from main `3ccec5cb`: `go test -count=1 -run TestInputs ./internal/job/` exit 1, 5 tests fail. Failing line: `inputs_shown_test.go:75: the read file repo:docs/big.md has shown "" in inputs.json, want "summary" (its brief line shows only the summary)`. Test commit `92198c4b` has only the test. |
| 2. Mode per use | HOLDS with one deviation, see below. `TestInputsShownModePerUse` (`inputs_shown_test.go:85`): intent request, item and goal `text`; brain note `excerpt`; `links` repo target `summary`; personal note `pointer`; previous acceptance `text`. `TestInputsShownIsAbsentForACutNote` (`:121`): a note cut by the budget (in `not_rendered`) has no `shown` key. |
| 3. Strongest use wins | HOLDS. `TestInputsShownStrongestUseWins` (`:140`): a note that is an excerpt and in `read` is `excerpt`; a plan item that is intent and in `read` is `text`; a cut note that is read is `summary`. |
| 4. Sizes | HOLDS. `TestInputsSizes` (`:162`): `brief_bytes` 2093 = `len(Rendered.Brief)`; `pasted_chars` 318 = the summed rune count of the intent texts and the excerpt; summed `sources[].text` is 14729, so 318 < 14729 (both asserted). `TestInputsPastedCountsFencedText` (`:187`) covers a fenced (external) intent source counting as pasted. Code: `render.go` `paste`, kept-excerpt loop, `BriefBytes` set after `r.Brief`. |
| 5. Replay | HOLDS. `TestInputsReplayWithAndWithoutShown` (`:210`): `Inputs.Resolver()` on the new file renders a byte-identical brief. An inputs.json with `shown`, `brief_bytes` and `pasted_chars` removed (as main wrote it) also renders the identical brief and rebuilds the same `shown` and sizes. `resolve` clears `Source.Shown` so a snapshot's mode never carries over. |
| 6. Schema | HOLDS. `TestInputsValidateAgainstTheSchema` (`:256`): `contract.Validate("fmt-job-render", {brief, inputs})` passes for the new file and for one written by main. A `shown` outside the four values is refused. New fixture `contract/fixtures/fmt-job-render/valid/written-by-main.json` (main's `elided.json`, byte for byte). `full.json` and `elided.json` were recaptured from the real writer with `go test ./contract/formats -update-format-fixtures`; both now carry `shown` and the sizes. |
| 7. Existing tests | HOLDS. `go test -count=1 ./internal/job/ ./internal/job/jobcmd/`: exit 0, ok, ok. `go vet ./internal/job/`: exit 0. |
| Importers' tests | HOLDS. `go list` importers of `internal/job`: contract/formats, internal/cli, internal/job (+jobcmd, lint), internal/jobgraph, internal/lease, internal/orchestrator, internal/plan, internal/ports. `go test -count=1` on all of them: exit 0, all ok. |
| Lint | HOLDS. `make lint` (`go vet ./...`): exit 0. `gofmt -l internal contract` has no output after `gofmt -w`. |
| Windows, macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...`: exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...`: exit 0. |
| New tests in a new file | HOLDS. `internal/job/inputs_shown_test.go`; no existing test edited. |
| No guard, gate, cap or limit added | HOLDS. Render only records what it already wrote; nothing blocks or limits an agent. |
| `Text` not dropped | HOLDS. `git diff` of `render.go` and `source.go` removes no field. |
| Public repo hygiene | HOLDS. No personal paths, hosts or emails in the diff. Commits carry no attribution lines. |

## Deviation to check: the `read` file

Row 2 says the `read` file is `pointer`. With Q-render-read-summary merged, a `read` line shows the file's summary, so `pointer` would mislabel it. The lane sets `read` to `summary` when the line shows a summary, and to `pointer` when the line names only the path: a personal note, or a file with no summary (`readBlock`'s bare line). Both tests assert this. For the scorecard row "Pointers against pasted text" nothing changes: `summary` and `pointer` both count as not pasted. If the owner wants `read` to always be `pointer`, it is a one-line change in the `read` loop of `render.go`.

## What changed

- `internal/job/source.go`: `Source.Shown` (`json:"shown,omitempty"`).
- `internal/job/render.go`: `Inputs.BriefBytes`, `Inputs.PastedChars`, the `Shown*` constants and rank. `Render` records each use where the block is written: `paste` (intent sources and previous acceptance are `text`), the kept brain candidates (`summary` or `pointer` for a personal note, plus `excerpt`), relation links, and `read` lines. Cut candidates get no mode.
- `contract/schemas/fmt-job-render.json`: optional `shown` enum, optional `brief_bytes` and `pasted_chars` (not in `required`).
- `contract/formats.md`: the `shown` and sizes rows and the new fixture link.
- `contract/fixtures/fmt-job-render/valid/`: `full.json` and `elided.json` recaptured; `written-by-main.json` added.
- `testdata/golden/job-inputs.golden`: updated by hand to the new inputs.json shape (`UPDATE_GOLDEN=1` is not on the shell allow list). `TestBriefGolden` passes on it. This is data of the format, not an edit to the test.
- `internal/job/inputs_shown_test.go`: the new tests.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not checked: that the app's Worker context rows (eval scorecard "Job size" and "Pointers against pasted text") read `brief_bytes` and `pasted_chars`; no reader of those fields exists in this repo yet.
