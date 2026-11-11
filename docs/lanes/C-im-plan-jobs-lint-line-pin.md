# C-im-plan-jobs-lint-line-pin

Base: ee8a96c126781462cbe0067771db40d01911cd76 (main, origin/main).

Base check: skills/plan-jobs/SKILL.md:49 quotes `anchor "step-1" not found`; :60 reads "Run `grid job lint <file> --task <task>` and post only a Job with no errors. A post that prints "render will refuse this Job" is fixed and posted again as a new Job."; internal/plan/items.go:202 is `fmt.Errorf("anchor %q not found", anchor)`; internal/cli/orchestrator.go:471 is `tail = " (render will refuse this Job)"`. internal/plan/anchor_skill_forms_test.go reads only the markdown example after "## Pointers into plan.md". No test reads :49 or :60: grep for `plan-jobs/SKILL.md` and `render will refuse this Job` in *_test.go finds internal/job/lint/skill_example_test.go (YAML example, `grid brain search`), internal/cli/er_deliver_job_text_test.go (the "Deliver the task" line, :45), internal/cli/job_post_findings_test.go (the CLI's stderr, not the skill) and anchor_skill_forms_test.go.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. `go test ./internal/plan/ -run TestImLint -count=1 -v` exits 0 | HOLDS | exit 0; `--- PASS: TestImLintSkillQuotesTheAnchorError (0.00s)`, `--- PASS: TestImLintSkillSaysLintBeforePost (0.00s)` |
| 2. Delete SKILL.md:60: lint-line test fails, TestFfAnchorForms passes | HOLDS | `go test ./internal/plan/ -run TestImLintSkillSaysLintBeforePost -count=1 -v` exit 1: `im_lint_line_test.go:72: no paragraph of skills/plan-jobs/SKILL.md holds all of ["`grid job lint" "--task" "no errors" "render will refuse this Job"]; the closest one is missing ["no errors" "render will refuse this Job"]` / `--- FAIL: TestImLintSkillSaysLintBeforePost`. `go test ./internal/plan/ -run TestFfAnchorForms -count=1 -v` exit 0 (both PASS). `git checkout -- skills/plan-jobs/SKILL.md`; `git status --short` shows only the new test file |
| 3. items.go:202 `not found` → `missing`: anchor test fails | HOLDS | `go test ./internal/plan/ -run TestImLintSkillQuotesTheAnchorError -count=1 -v` exit 1: `im_lint_line_test.go:50: skills/plan-jobs/SKILL.md does not quote the error Anchor prints: missing `anchor "step-1" missing`` / `--- FAIL`. `git checkout -- internal/plan/items.go`; status clean |
| 4. SKILL.md:49 `anchor "step-1" not found` → `anchor step-1 not found`: anchor test fails | HOLDS | same command exit 1: `im_lint_line_test.go:50: ... missing `anchor "step-1" not found`` / `--- FAIL`. `git checkout -- skills/plan-jobs/SKILL.md`; status clean |
| 5. diff names only the test file and this table; plan+contract tests; vet | HOLDS | `git diff main --stat`: internal/plan/im_lint_line_test.go and docs/lanes/C-im-plan-jobs-lint-line-pin.md. `go test ./internal/plan/ ./internal/contract/ -count=1` exit 0. `go vet ./internal/plan/` exit 0 |
| Rule: Touches only the new test file, no production or skill change | HOLDS | internal/plan/im_lint_line_test.go (package plan; names `imLint*`, tests `TestImLint*`; skillExamplePlan not reused) |
| Rule: new tests fail on base and pass on the lane | HOLDS (as pin) | These are pin tests on text that already exists on base, so they pass on base. Their red proof is rows 2 to 4, where they fail with each word or the error text removed |
| Rule: tests of changed package and importers | HOLDS | importers from `go list`: internal/approvefix, cli, job, job/jobtest, jobgraph, land, orchestrator, proof, task. `go test` on all of them with `-count=1` exit 0 (approvefix and jobtest have no test files) |
| Rule: lint, Windows and macOS builds | HOLDS | `make lint` (go vet ./...) exit 0; `make cross` (darwin/arm64 and windows/amd64 vet plus test compile) exit 0 |
| Rule: no guard, gate, cap or hold added | HOLDS | test file only |
| Rule: no attribution lines, no personal paths | HOLDS | commit message is one sentence; the test uses only repo-relative paths |
| Rule: RealPlugin tests | n/a | no row names one |

## What changed

New internal/plan/im_lint_line_test.go:
- TestImLintSkillQuotesTheAnchorError runs `Anchor` on a plan whose step is a `### Step 1: Update install.md` heading, takes `err.Error()`, and requires that text in backticks in skills/plan-jobs/SKILL.md.
- TestImLintSkillSaysLintBeforePost splits the skill into paragraphs (runs of non-blank lines) and requires one paragraph to hold `` `grid job lint ``, `--task`, `no errors` and `render will refuse this Job` (constant `imLintRefuseTail`, copied from internal/cli/orchestrator.go:471). On failure it names the words that the closest paragraph lacks.

## NOT VERIFIED

Nothing. No UI.

## Open items

- `imLintRefuseTail` is a copy. If orchestrator.go:471 changes, job_post_findings_test.go:125 turns red, but this test does not. Whoever changes the tail must update the constant and the skill.

DONE
