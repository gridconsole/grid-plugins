# S-fix-render-brain-given

Verdict: **NOT DONE**. Row 3 does not hold. Its cause is outside this lane and outside the package I may edit (see row 3).

Base: plugin main `ebd0a240d910ea8401198e80920100d854d33172` (with lane-Q-render-brain-given-history merged).

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1. Two failing lines at start | HOLDS | `go test -count=1 ./internal/job/jobcmd/` on the branch at start, exit 1: `render_brain_given_test.go:91: want exactly one brain given line, got 2 in ["- 2026-11-11T03:13 · brain given · brain:project/deploy · by grid job render · via J-1" "- 2026-11-11T03:13 · brain given · brain:project/export-format · by grid job render · via J-1"]` and `render_brain_given_test.go:222: want one brain given line, got [the same two]`. |
| 2. Why two notes: given, or only logged? | HOLDS: the second note is really given, so the tests' expectation was stale | The fixture Job is titled "Export documentation". `recall` (`internal/job/jobcmd/recall.go:23-45`, called at `internal/job/jobcmd/command.go:138`) searches the brain by the Job title and adds a hit as a rank 4 candidate, so it is pasted into the brief. It matches the fixture note `export-format` (`internal/job/jobtest/jobtest.go:72`). Recall (a09283a4, lane O-render-brain-recall) landed on main after this lane's base 3ccec5cb, which is why the lane passed alone. The writer is right: `brainGiven` (`command.go`) logs only sources that are pasted. I changed the two tests, not the writer. |
| 2a. Test changes, line by line | HOLDS | `render_brain_given_test.go` `TestRenderLogsBrainGivenOnce`: the single-line expectation becomes a list `carried = [deploy, export-format]`, with one line each in order. The re-render check expects the same count. A new check reads the rendered brief and requires both note bodies in it ("Run the deploy script from the release branch." and "Keep the id column before the title column."), so the test proves each logged note was given. `TestRenderBrainGivenAmongConcurrentHistoryAppends`: `len(given(lines)) != 1` becomes `!= 2` (deploy and the recalled export-format). No other test touched, nothing deleted. |
| 3. Scratch merge of the three Q lanes, `go test -count=1 -run TestInstructionsAreNotASourceOrBrainText` | **NOT DONE** | The test is in `internal/job/render_role_test.go:79`, not `internal/job/jobcmd`; run in `./internal/job/` it exits 1: `render_role_test.go:92: builder: inputs.json or brain_tokens changed`. It also fails on `origin/main` plus the three Q lanes without this lane, so this lane is not involved. Bisect, each on `origin/main` alone: Q-render-read-summary passes, **Q-inputs-shown-mode fails**. Cause: that lane adds `brief_bytes` to inputs.json (`internal/job/render.go:33`, set at `render.go:348` as `len(r.Brief)`). The test compares inputs.json between the `writer` role and `builder`, `reviewer`, `orchestrator` and `curator`; the role instructions (O-render-role-instructions) make the brief longer, so `brief_bytes` differs. The scratch branches were deleted. |
| 4a. `go test -count=1 ./...` | HOLDS | exit 0, every package ok, including `internal/job`, `internal/job/jobcmd` and `internal/cli` on this branch. `internal/cli` is the only importer of `jobcmd` (`go list` shows it). The row 3 failure appears only after Q-inputs-shown-mode is merged. |
| 4b. `make lint` | HOLDS | exit 0 (prints `go vet ./...`). |
| 4c. `GOOS=windows go vet ./...` | HOLDS | exit 0, no output. |
| 4d. `GOOS=darwin go vet ./...` | HOLDS | exit 0, no output. |
| Rules: small commits, no attribution, public repo | HOLDS | One commit, `980bb7df`, one sentence, no attribution lines, no paths or hosts. |
| Rules: new tests fail on base | Not applicable | No new test. The two existing tests of this lane failed at start (row 1) and pass now. Both edits follow the brief's allowance (row 2a). |
| Rules: no guard, gate, cap or hold added | HOLDS | Only the test file changed. |

## What changed

- `internal/job/jobcmd/render_brain_given_test.go`: the lane's two tests now expect one `brain given` line per note the brief carries, including the note recall adds. The first test also reads the brief to prove it.

## Open item for the orchestrator (row 3)

Fix for the `TestInstructionsAreNotASourceOrBrainText` interaction, either of these. Both are in `internal/job`, which this brief does not let me touch, so I stopped.

1. Q-inputs-shown-mode: leave `brief_bytes` out of what that role test compares. The test's own wording, "instructions are not a source or brain text", is about sources and brain text; `brief_bytes` and `pasted_chars` describe the brief. Edit `internal/job/render_role_test.go:84-93` to zero `Inputs.BriefBytes` on both sides before `JSON()`. That is O's test, so it needs the owner's agreement.
2. Or Q-inputs-shown-mode computes `brief_bytes` over the brief without the `### Instructions` section. That is new behaviour.

I recommend 1. It needs a new lane, "fix role test brief_bytes", on top of the three Q lanes.

## NOT VERIFIED on a live screen

None. No UI.

NOT DONE
