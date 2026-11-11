# C-ga-skills-one-command-bash

Base: b87fc36458332a9e2bd680ff9e3ed5a896638900 (run-job, plan-jobs, review-job and agents/reviewer.md read whole; none says one command per Bash call).

| Row | State | Proof |
|---|---|---|
| 1. one command per Bash call, one line per file | HOLDS | `git grep -n 'one .*command per Bash call' -- skills/` shows skills/run-job/SKILL.md:55, skills/plan-jobs/SKILL.md:28, skills/review-job/SKILL.md:20. On base b87fc364 the same grep prints nothing (exit 1). |
| 2. never set dangerouslyDisableSandbox | HOLDS | run-job/SKILL.md:59, plan-jobs/SKILL.md:32, review-job/SKILL.md:24. |
| 3. Write and Edit instead: run-job and plan-jobs only | HOLDS | run-job/SKILL.md:58, plan-jobs/SKILL.md:31; none in review-job. |
| 4. verify steps unchanged | HOLDS | `git diff b87fc364 -- skills/run-job/SKILL.md skills/review-job/SKILL.md` is additions only (a Bash section before Brain in run-job, a Bash paragraph before the post line in review-job); step 7 and step 3 are untouched. The new text points at them ("step 7", "step 3"). |
| 5. `go test ./internal/contract/ ./internal/job/lint/ -run Skill -count=1` | HOLDS | exit 0, both ok. |
| Tests of changed and importing packages | HOLDS | Only skills/*.md changed. The root package embeds them and internal/job imports it: `go test . ./internal/job/ ./internal/contract/ ./internal/job/lint/ -count=1` exit 0, all ok. |
| Lint | HOLDS | `go vet ./...` exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base | N/A | No test added; the existing tests named in row 5 cover it. |
| No existing test edited or deleted | HOLDS | `git diff --stat` shows only the three skill files and this report. |
| No guard, gate, cap or hold | HOLDS | Text guidance only. |
| No personal paths, hosts or emails | HOLDS | None in the added text. |
| *RealPlugin* tests | N/A | None named. |

## What changed

Each of run-job, plan-jobs and review-job has a short Bash part (5 bullets or fewer). It says: one command per call, with no `;`, `&&`, pipe, `$(...)` or `echo $?`; `grid` and `git` are pre-approved; the Job's verify command is still run as written, in a call of its own; no `VAR=value` prefix; Read, Glob and Grep instead of ls, cat, find and grep; never set `dangerouslyDisableSandbox`. run-job and plan-jobs add Write and Edit instead of sed, cp and heredocs. No flag is in a code span on its own. There is no feed_post line.

## NOT VERIFIED

Whether agents follow the text in a live run (needs an eval round).
