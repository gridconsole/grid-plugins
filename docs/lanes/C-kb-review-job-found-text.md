# C-kb-review-job-found-text

Head: see the bundle. Base: main (dc301fce).

The review-job skill now tells the reviewer to post each security problem and each must-fix bug as a `found` record under the review Job's lease, before the verdict, and to put the printed ids in the result's `found` list.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. TestKbRvSkillPostsFound | HOLDS | `go test -count=1 -v -run 'TestKbRv' ./internal/contract/` exit 0, `--- PASS: TestKbRvSkillPostsFound`. On base (skill edit stashed) it fails: `kb_review_found_text_test.go:47: review-job skill has no code span that starts `grid job post` and holds --type found` and `--- FAIL: TestKbRvSkillPostsFound`. |
| 2. TestKbRvFoundBeforeVerdict | HOLDS | Same command, `--- PASS: TestKbRvFoundBeforeVerdict`. Base failure: `kb_review_found_text_test.go:64: review-job skill has no `grid job post ... --type found` span`. The test reads the required list of the inline body branch from contract/schemas/found.json through `contractdata.Schemas`, no copied list. |
| 3. commands and examples | HOLDS | `go test -count=1 -v -run 'TestSkillAndAgentCommandsExist\|TestSkillExamplesValidate\|TestImBash' ./internal/contract/` exit 0: `--- PASS: TestSkillAndAgentCommandsExist`, `--- PASS: TestSkillExamplesValidate`, `--- PASS: TestImBashEveryJobAgentSkillCarriesTheLines`, no FAIL. The example JSON still has `"found": []`. |
| 4a. diff names only the two Touches files | HOLDS | `git diff main --stat`: `internal/contract/kb_review_found_text_test.go` (new) and `skills/review-job/SKILL.md` (+4). Nothing else. |
| 4b. make lint | HOLDS | `make lint` (`go vet ./...`) exit 0. |
| 4c. make cross | HOLDS | `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0. The first run was killed by the 10 minute background limit during the windows vet; the rerun with a longer timeout exited 0. |
| Importers of internal/contract | HOLDS | `go list` importers: artifact brain braincmd cli feed guard hook intent job job/jobcmd job/lint jobgraph land lease mcp migrate orchestrator question session settings signal task updates usage where workflow. The lane changes no production code, only a `_test.go` file in package contract and a skill markdown. First run (`go test -count=1` on contract plus all of those): every package ok except cli and orchestrator, which hit Go's default 10 minute test timeout while `make cross` ran in parallel (no assertion failed). Rerun alone: `go test -count=1 -timeout 30m ./internal/orchestrator ./internal/cli` exit 0 (`ok orchestrator 819.683s`, `ok cli 759.980s`). |
| Report adds a third path | NOTE | docs/lanes/C-kb-review-job-found-text.md is in the diff because the runbook requires it; row 4a counts the two Touches files. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok (exit 0); `go test -count=1 ./internal/job/lint/` ok (exit 0). The app walk tests do not apply to the plugin repo. |
| Existing tests not edited | HOLDS | `git diff main --stat` shows no existing test file. |
| No guard, gate, cap or hold added | HOLDS | skill text and a test only. |
| No attribution lines in commits | HOLDS | one commit, one-sentence message. |

## What changed

- `skills/review-job/SKILL.md`: a paragraph before "Post with `grid job result`" (one `grid job post --task <task> --job <review-job> --type found --body <json>` call per security problem and per must-fix bug, in a Bash call of its own; the body fields; why `--job` is the review Job and the reviewed Job goes in `body.job`; the printed id; a note stays a note), and a sentence after the example saying `found` holds those ids.
- `internal/contract/kb_review_found_text_test.go`: `TestKbRvSkillPostsFound`, `TestKbRvFoundBeforeVerdict` (helpers start `kbRv`).

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a real reviewer following the new text; whether `grid job post --type found` under a review Job lease is accepted is the job of C-kb-found-worker-role.
