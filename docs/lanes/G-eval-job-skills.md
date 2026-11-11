# G-eval-job-skills: NOT DONE (guard row 0 failed)

Stopped at call 8 by the brief's own guard. No eval case, test or skill file was changed.

## Proof table

| Row | Result | Evidence |
| --- | --- | --- |
| 0. D-plugin-jobskills is on main | NOT DONE | `git log --oneline main \| grep -i jobskills` printed nothing (exit 1). `grep -ril jobskills docs .claude-plugin skills evals` printed nothing. `git merge-base --is-ancestor 013fc41e origin/main` exit 1: the lane's main commit is not an ancestor of main. The commit sits only on `origin/lane-D-plugin-jobskills` and `origin/lane-M-jobskills` (`git branch -a --contains 013fc41e`). |
| 0. Skill names on main | HOLDS | `ls skills` lists `plan-jobs`, `review-job`, `run-job`. Main has only the first version of the skills (5a1e95c3, from `git log main -- skills/run-job skills/review-job skills/plan-jobs`). |
| 1. Trigger and effect quotes | NOT DONE | Skipped. The skill text on main is the old text, and D-plugin-jobskills (013fc41e) rewrites all three SKILL.md files. Quotes taken now would be stale. |
| 2. One case per skill | NOT DONE | Skipped. Same reason as row 1, and see "What I found" below. |
| 3. Suite test pins the ids | NOT DONE | Skipped, no change made. |
| 4. make eval-check, make lint | NOT DONE | Not run, because nothing changed. |
| Rules: no guards or gates, no attribution, no test edits | HOLDS | Nothing was built. This note is the only change. |

## What I found

- Commit 013fc41e, "Rewrite the three Job skills with the 10 Nov rules ... and add three evals", is on `origin/lane-D-plugin-jobskills` and was merged into `lane-M-jobskills`. It is not on main.
- That commit already adds three eval cases, `evals/tasks/job-plan`, `evals/tasks/job-review` and `evals/tasks/job-run`. It also adds a line to the sample suite test ("List the three new Job eval tasks in the sample suite test", 6e1171c5). So the work this brief asks for overlaps with that lane. If I built cases now, they would either duplicate or conflict with those three ids once that lane lands.
- The brief's "Why" (no eval case names run-job, review-job or plan-jobs on main) is true today. It is also true that D-plugin-jobskills closes the gap when it lands.

## What changed

Only this file.

## Split or next step

Land D-plugin-jobskills (or M-jobskills) on main first. Then either:

1. Drop this lane, because D-plugin-jobskills already carries the three evals and the suite pin.
2. Re-brief this lane to review and strengthen `job-plan`, `job-review` and `job-run` (the checks, the prompt wording that avoids naming the skill, and the trace checks the way task-skill does them).

## NOT VERIFIED on a live screen

Nothing here touches a screen. No eval was run (no claude, no login).
