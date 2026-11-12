# C-rb2-retry-reuse-brain-only

Verdict: DONE. A retried render reuses the saved bundle only when job.yaml is byte-equal and every saved source that is not a brain note equals the new render's. A revised plan keeps today's refusal, so the contract transcript step 7 exits 1 again. Code commit `005e00c5`. The lane head is the commit of this report.

## Base and merge

- Cloned on lane-C-rb-render-retry-saved-bundle (head `5ca255cb`). `git merge origin/main` (origin/main = local main = `103b9157`) merged with no conflicts, so no file was resolved by hand.
- Prior lane report: `docs/lanes/C-rb-render-retry-saved-bundle.md` (= /srv/gridwork/tmp/hand/C-rb-render-retry-saved-bundle-report.md). /srv/gridwork/tmp/hand/review-C-rb-render-retry-saved-bundle.md is absent.

## Brain sources vs pointer/file sources (step 2)

inputs.json `sources` holds one entry for every pointer Render resolved, keyed by source (`render.go:100-119`, written at `render.go:365-374`). Each entry keeps its `pointer`, and `job.Files.Resolve` picks the store by the pointer's prefix:

- **Brain notes** have a pointer that starts with `brain:` and are read from the brain store (`source.go:99-103`, `ParseBrain` at `source.go:72-76`). They reach Render as brain candidates through `add` (`render.go:133-146`) from:
  - the Job's `decisions` and `keep` (`render.go:151-165`);
  - its `brain:` links (`render.go:167-170`);
  - one hop of depends-on and decided-by links (`render.go:182-190`);
  - the recall hits (`render.go:191-195`, built by `recall.go:23-45` from `w.store.Search`).
- **Pointer/file sources** are every other prefix: `prototype:`, `repo:`, `job:`, and `task:` or bare task files (the plan among them) (`source.go:104-133`). Render resolves them for:
  - the intent owner, item and goal (`render.go:120-131`);
  - non-brain links (`render.go:172-174`, `289-296`);
  - the previous acceptance (`render.go:297-303`);
  - `read` (`render.go:305-320`).

**Interpretation.** inputs.json does not say why a brain note was given (recalled, or named in `keep` or `links`), but the rule does not need that. `TestRbRetryBrainChangeReusesFirstRender` edits `brain:project/export-format`, which the fixture plan names in both `links` and `keep` (`testdata/job/task/plan.md:16-17`), and that test must stay green. So "brain recall" here means every `brain:` source. No format change was needed and none was made.

The top-level inputs.json fields are not compared: `generation`, `not_rendered`, `brain_tokens`, `brief_bytes`, `pasted_chars` and `disableSkillShellExecution`. They move with the brain. brief.md is not compared either, because it carries the brain text and the brain agent (`command.go:163`).

## Proof table

| Row | Result | Proof |
|---|---|---|
| C-rb2 step 1: merge origin/main, name resolved files | HOLDS | Merge clean, no conflicts; nothing resolved by hand. |
| C-rb2 step 2: name brain vs pointer/file entries | HOLDS | Section above, with file:line. |
| C-rb2 step 3: saveReport rule | HOLDS | `save.go:47-75`: `same()` reads all three files through `job.Within` as before (a linked or missing file still fails) and requires job.yaml byte-equal. It then requires `fileSources(saved inputs.json)` == `fileSources(new inputs.json)`. `fileSources` (`save.go:103-117`) decodes `job.Inputs`, keeps the sources whose pointer does not start with `brain:`, and re-encodes them. A saved inputs.json that does not decode is not reused. Any difference keeps the refusal `Job %s already has different saved inputs; create a new Job for revised work` (`save.go:80`). The rename-race fallback (`save.go:95`) uses the same check. |
| C-rb2 step 4: TestRbRetryPlanChangeStillRefused red with the job.yaml-only compare, green with the rule | HOLDS | Red: run before save.go was edited (lane = C-rb's job.yaml-only `same()` + merge), `go test -count=1 -run 'TestRbRetryPlanChangeStillRefused' ./internal/job/jobcmd/` exit 1: `--- FAIL: TestRbRetryPlanChangeStillRefused (0.02s)` / `rb_render_retry_test.go:167: [render T-01 1 --json]: exit 0 want 1; stdout {"ok":true,"id":"J-1","path":".../projects/demo/tasks/T-01/jobs/J-1/brief.md","inputs":".../jobs/J-1/inputs.json","findings":[]}`. That run exited 0, so job.yaml was byte-equal after the plan append. Green: on `005e00c5` the same test passes (`--- PASS: TestRbRetryPlanChangeStillRefused (0.03s)`) and checks `Error == rbRetryRefusal` (the `save.go:80` message, written `:61` in the brief) and `rbRetryUnchanged` (bytes and mtimes of the three files). The test appends `"\nThe export also documents compatibility.\n"` to `T-01/plan.md`, the same edit as transcript step 5. On origin/main, where all three files are compared byte for byte, it is green by design. |
| C-rb row 1: TestRbRetryBrainChangeReusesFirstRender | HOLDS | `go test -count=1 -v -run 'TestRbRetry\|TestSaveRefuses\|TestRenderChangedSavedInputs\|TestRenderRefusesInvalidAndChangedJobs\|TestJsvSaveLinkedFile' ./internal/job/jobcmd/` exit 0: `--- PASS: TestRbRetryBrainChangeReusesFirstRender`. Red-on-base proof as in the C-rb report (unchanged test). |
| C-rb row 2: TestRbRetryNoSecondBrainGiven | HOLDS | Same run: `--- PASS: TestRbRetryNoSecondBrainGiven`. |
| C-rb row 3: TestRbRetryChangedJobStillRefused; TestJsvSaveLinkedFileIsDifferentInputs unchanged | HOLDS | Same run: `--- PASS: TestRbRetryChangedJobStillRefused`, `--- PASS: TestJsvSaveLinkedFileIsDifferentInputs` (save_verify_failures_test.go not in the diff). Also `--- PASS: TestRbRetrySaveReportReusesSavedBundle`, `TestRbRetryMissingSavedFileStillRefused`, `TestSaveRefusesAChangedRenderAndKeepsTheFirstFiles`, `TestRenderChangedSavedInputsLogsNoBrainGiven` and `TestRenderRefusesInvalidAndChangedJobs`, then `ok internal/job/jobcmd`. |
| C-rb row 4 (unchanged): `go test -count=1 ./internal/job/... ./internal/cli/` exit 0, TestPlanDigest PASS, testdata diff empty | HOLDS | `go test -count=1 ./internal/job/... ./internal/cli/` exit 0: `ok internal/job`, `ok internal/job/frontmatter`, `ok internal/job/jobcmd`, `ok internal/job/lint`, `ok internal/cli 182.786s`. `go test -count=1 -v -run 'TestPlanDigest' ./internal/cli/`: `--- PASS: TestPlanDigestRenderRecordsDigestAndStale`, `--- PASS: TestPlanDigestRoundTrip`, `--- PASS: TestPlanDigestRoundTrip/digest-and-stale/07-job_render_$task_1`, `ok internal/cli 0.985s`. `git diff main --stat -- testdata/` and `git diff origin/main --stat -- testdata/` are both empty (main = origin/main = `103b9157`). The brief's premise "the plan edit changes job.yaml" is false (step 4 red run above). Step 7 holds because the plan edit changes a non-brain source. Transcript and app `plugin-digest` untouched. |
| C-rb row 5: plugin walk rows | HOLDS | `go test -count=1 -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' ./internal/cli/ && go test -count=1 ./internal/job/lint/` exit 0: `ok internal/cli 0.574s`, `ok internal/job/lint 0.813s`. Run on `005e00c5`; the only later commit adds this file under docs/lanes. |
| C-rb row 6: `make lint`, `make cross`, diff names only Touches | HOLDS | `make lint` (`go vet ./...`) exit 0. `make cross` exit 0 (last lines `ok .../internal/worker`, `ok .../internal/workflow`, `ok .../lib`). `git diff origin/main --stat`: command_test.go, feedjob_save_test.go, rb_render_retry_test.go, render_brain_given_test.go and save.go, plus the docs/lanes reports (C-rb's and this one). This lane changes only save.go and rb_render_retry_test.go. |
| Packages that import the changed package | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep internal/job/jobcmd`: `internal/cli`, `internal/job/jobcmd`. Both are green in row 4. |
| New tests fail on base, pass on lane | HOLDS | TestRbRetryPlanChangeStillRefused: red against the job.yaml-only rule (the lane's base, C-rb), green with the rule (step 4 row). It pins a kept refusal, so it is green on origin/main too. |
| No existing test edited to pass; none deleted | HOLDS | This lane edits rb_render_retry_test.go only: the new test plus the header comment at `rb_render_retry_test.go:15-18`. No test deleted. The three edits from C-rb are unchanged. |
| No testdata/ file | HOLDS | `git diff origin/main --stat -- testdata/` is empty. |
| Walk tests (plugin) | HOLDS | Row 5. No app walk tests apply (plugin lane). |
| `*RealPlugin*` tests | not applicable | No test touched or added is named RealPlugin. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change narrows when a saved bundle is reused. It keeps the existing refusal and adds no flag, count, timeout or hook. |
| Commits: one sentence, no attribution | HOLDS | `005e00c5 Reuse the saved render only when job.yaml and every non-brain source are unchanged.` and the report commit. Neither has trailers. |
| Budget | HOLDS | About 36 tool calls of the brief's "About 40". |

## What changed

- `internal/job/jobcmd/save.go`: `same()` compares job.yaml byte for byte, then compares the non-brain sources of the saved and new inputs.json through the new `fileSources`. A brain-only change reuses the first bundle (wrote=false, so no "brain given" lines). A changed plan or any other file source refuses with today's message. The doc comments on `save` and `same` say so.
- `internal/job/jobcmd/rb_render_retry_test.go`: new `TestRbRetryPlanChangeStillRefused`; header comment updated.

## Open items

- None for the transcript: step 7 exits 1 again with "already has different saved inputs", and testdata/ is unchanged.
- Note for the reviewer: a source's `shown` field is part of the comparison. If a Job named a non-brain pointer in `keep` or `decisions`, a brain change that cut that candidate could change its `shown` and refuse. That is the safe direction (today's refusal). The fixture and the transcript name only `brain:` ids there.

## NOT VERIFIED on a live screen

None; no UI. Not run here: the whole-module test and the cross-repo set (the orchestrator runs them at landing).

DONE
