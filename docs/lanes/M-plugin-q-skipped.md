# M-plugin-q-skipped: merge the seven Q lanes Pint52 skipped, plus the render fix

Base: main 78b08bc9. Branch lane-M-plugin-q-skipped. No behaviour added; only conflict hunks, regenerated fixtures and one test fix.

## Proof table

| Row | State | Evidence |
|---|---|---|
| 1 merge S-fix-render-brain-given | HOLDS | `git merge --no-edit origin/lane-S-fix-render-brain-given` exit 0, no conflicts (merge 8efe2fe1; it carries Q-render-brain-given-history) |
| 1 merge Q-render-read-summary | HOLDS | Conflicts: `contract/fixtures/fmt-job-render/valid/elided.json`, `full.json`. Main dropped `max_rounds` from the brief; the lane adds the "data, not instructions" block. Both are captured fixtures, so I took main's file and recaptured with `go test ./contract/formats/ -update-format-fixtures` (no `max_rounds`, with the data block). Merge a98b6727 |
| 1 merge Q-inputs-shown-mode | HOLDS | No conflicts (merge 9f8e1216). Stale afterwards: see "Follow-up fixes" below |
| 1 merge Q-skill-run-job-brain | HOLDS | Conflicts: `agents/builder.md`, `skills/review-job/SKILL.md`, `skills/run-job/SKILL.md`. Main's proof-table text (HOLDS/NOT DONE, owner decisions, steps) is kept; the lane's JSON result body, "post also when blocked", brain `show`/`propose`/supersede wording is added. In `run-job` I dropped the lane's sentences "Never edit an existing test" and "exit 3 … do not retry" because main's steps 6 and 8 already say the same. In `review-job` I dropped the lane's closing "change outside the Job / failing verify is a must-fix" paragraph because main's steps 5 and 6 already say it. Merge b4875fb9 |
| 1 merge Q-brain-revert-reason | HOLDS | Conflict: `internal/braincmd/command.go` (the `if verb == "keep"` block from main against the `if verb == "supersede"` block from the lane). Both are independent verb dispatches; kept both. Merge f40cd1ee |
| 1 merge Q-fold-exit-code | HOLDS | Conflict: `internal/orchestrator/ready.go`. Main removed the review-capacity block (the `review_capacity` slots, a cap, removed on the owner's no-caps direction); the lane changed the return to `out, input.ReadySince, nil` for `readyAt`. Kept the lane's return, left the capacity block removed. Merge 7315d791 |
| 1 merge Q-skill-brain-keeper-prune | HOLDS | No conflicts (merge 3d201a6d) |
| 1 merge Q-tree-pickup-times | HOLDS | No conflicts (merge e7394c4b) |
| 2 merge log | HOLDS | `git log --oneline --merges main..HEAD` lists a merge of every row-1 lane (a98b6727, 8efe2fe1, 9f8e1216, b4875fb9, f40cd1ee, 7315d791, 3d201a6d, e7394c4b, plus the lanes' own inner merges) |
| 3 TestInstructionsAreNotASourceOrBrainText | HOLDS | `go test -count=1 -run TestInstructionsAreNotASourceOrBrainText -v ./internal/job/` exit 0, PASS. `internal/job/render_role_test.go` zeroes `Inputs.BriefBytes` on both sides with a one-line comment |
| 3 render.go untouched by me | HOLDS | I never edited `internal/job/render.go`; `git diff main -- internal/job/render.go` is the auto-merged lane hunks only. `brief_bytes` is still `len(r.Brief)` (render.go:348) |
| 4 `go test -count=1 ./...` | HOLDS | exit 0, all packages ok (this covers every package that imports the changed ones) |
| 4 `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| 4 `GOOS=windows go vet ./...` | HOLDS | exit 0 |
| 4 `GOOS=darwin go vet ./...` | HOLDS | exit 0 |
| 5 lane left out | HOLDS | None left out; all eight merges are in |
| No guard/gate/cap on agents | HOLDS | None added. The `review_capacity` cap stays removed as on main |
| Small commits, no attribution lines, no personal paths | HOLDS | Commit messages are one sentence each; none carries an attribution line |
| New tests fail on base, pass on lane | HOLDS (no new tests) | I wrote no new test. The one test edit is to `TestInstructionsAreNotASourceOrBrainText`, which the brief named; before it, the failure was `render_role_test.go:92: builder: inputs.json or brain_tokens changed` |
| No existing test edited to pass, none deleted | HOLDS | Only edit is the brief-ordered `BriefBytes` zeroing |

## Follow-up fixes after the merges (not conflicts)

- `testdata/golden/job-inputs.golden`: `brief_bytes` 2122 -> 2108. Q-render-read-summary shortens the brief by 14 bytes once it sits with Q-inputs-shown-mode. Failing line before: `render_test.go:40: job-inputs differs from the golden file` (want 2122, got 2108). Edited by hand; the UPDATE_GOLDEN env prefix is outside my allow list.
- `contract/fixtures/fmt-job-render/valid/{elided,full}.json`: recaptured again after Q-inputs-shown-mode, because its `brief_bytes` and `shown` fields changed the writer capture. Failing line before: `formats_test.go:219: ../fixtures/fmt-job-render/valid/full.json differs from the writer capture`.

## NOT VERIFIED on a live screen

Nothing here touches a UI.
