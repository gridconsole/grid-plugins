# Lane C-trace-skills-deliver-questions-research

Repo plugin, branch lane-C-trace-skills-deliver-questions-research. Started from lane-D-plugin-skills-b (79a9b918). origin/main merged in at cd84f424ad766a589bcc3c5ff1477ae4060d05ff (the one conflict, internal/eval/suite_test.go, resolved by taking main's id list and adding skill-deliver, skill-questions, skill-research; merge commit a63f91f3).

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. review.md must-fix 1 (skill-research setup.sh) | HOLDS | evals/tasks/skill-research/setup.sh:4 (project.md), :41-42 (`state: running`, `created:`). The second verdict's must-fix is the same defect (STATE.md:92, "not carried to the sibling file"); it is closed by rows 3a-3c. Scripted run shows `brain search`, exit 0 (row 3b). Research checks read only research.md and the code, so no scripted 7/7 (brief row 5). Review notes: topics (`grid brain topics`) and a no-curator fallback are in skills/deliver/SKILL.md step 5; GRID_MEMBER is in skills/research/SKILL.md step 6. |
| 2. new test fails on main, passes on lane | HOLDS | internal/settings/catalog_skills_lane_test.go ranges over `SkillNames()`. On a checkout of origin/main cd84f424: `go test ./internal/settings -run TestEveryCatalogSkillHasSkillFile` FAIL: "catalog skill deliver has no SKILL.md", "catalog skill questions has no SKILL.md", "catalog skill research has no SKILL.md". On the lane `go test ./internal/settings ./internal/eval ./internal/packaging` exit 0. |
| 3. fix table | HOLDS | below |
| 4. every verb run once | HOLDS | table below |
| 5. checks | HOLDS | lines below |
| 6. no `grid ask` step; curator, not person | HOLDS | skills/questions/SKILL.md:23 ("Not built: ... Use `grid question add`"); steps never call `grid ask`. skills/deliver/SKILL.md:11 ("a subagent curates proposals; you never ask the person") and step 5 "Do not ask the person." skills/questions/SKILL.md:6 asks only at person points. |
| 7. go test ./..., make lint, make cross | HOLDS | `go test ./...` printed no line other than ok / no test files; `make lint` and `make cross` printed no failure (output piped through tail, exit code not captured separately). |
| Packages importing the changed ones | HOLDS | only test files and skills/evals changed; no Go package other than internal/settings (new test file) and internal/eval (id list) changed. Neither is imported by non-test code beyond their own package tests, which passed. |
| Rule: no guard, gate, cap | HOLDS | No guard added; skill text names only refusals the verbs return. |
| Rule: public repo, no personal paths/hosts/emails | HOLDS | skills use `$GRID_HOME` paths only. tmpbin/ (local scratch, scripts and binary) is not committed. |
| Rule: no attribution lines | HOLDS | commit messages have none. |

### Row 3 fix table

| Fix | skill-deliver | skill-questions | skill-research |
|---|---|---|---|
| a. state, created, project.md | setup.sh:8, :26-27 | setup.sh:8, :22-23 | setup.sh:4, :41-42 |
| b. `grid brain search tag --task T-01` exit 0 | exit 0 (`brain search label --task T-01`) | exit 0 | exit 0 (`hits: []`). Before the fix, review.md:8-9: exit 1 "task.md needs a matching id, title, project and state" (reviewer's run 20261111T005101Z-3d163fb8d785413c; not re-run here) |
| c. operand rule | SKILL.md:15 | SKILL.md:14 | SKILL.md:16 |
| d. propose `--task`, `--link` keys | step 5 (`--task <task>`, keys `to`, `rel`, `summary`, `why`; "takes no `--task`" is gone) | names no propose | names no propose |
| e. question add with `--applies` and `--default` | names none | step 2 | names none |

`git grep -n "^status:" -- evals/tasks/skill-deliver evals/tasks/skill-questions evals/tasks/skill-research` prints nothing (checked by reading the three setup.sh files).

### Row 4: verbs run in the eval fixtures (built grid first on PATH, `eval run --agent '["sh", script]' --json`)

Operand rule measured: the folder path works for every verb. A bare `T-01` works for proof, brain (search/propose/keep/decide/show), task history, deliver; it answers exit 1 "task T-01 needs --project: this folder is in no project with a record" for plan items, question add/list/withdraw, answer, person approvers, where, job show, job result.

| Verb | Operand | Exit |
|---|---|---|
| plan items --json | T-01 / folder | 1 / 0 |
| proof add, proof index | T-01 / folder | 0 / 0 |
| question add (--applies, --default, --blocking, --item) | T-01 / folder | 1 / 0 |
| question list | T-01 / folder | 1 / 0 |
| answer | T-01 / folder | 1 / 0 |
| question withdraw | folder, answered Q | 3 "Q-1 is answered" (fixture state); on an open Q: 0 |
| person approvers --area | T-01 / folder | 1 / 0 |
| person show alex | --project notes-app | 1 "file does not exist" (no person file in the fixture; `--project` exists, see `grid person --help`) |
| task history | T-01 / folder | 0 / 0 |
| brain topics, search | T-01 / folder | 0 / 0 |
| brain show | T-01 | 1 "accepted note not found" (no note in the fixture) |
| brain propose --task | T-01 / folder | 0 / 0 (with `--link` rel `related`; rel `relates` exits 2 "unknown link relation") |
| brain keep | T-01 / folder | 0 / 0 |
| brain decide `<task>` `<id>` reject | T-01 and folder | 0 (folder; reads where propose wrote) |
| deliver --dry-run | T-01 / folder | 0 / 1 (without proof: "plan item i-1 lacks check proof") |
| where --task | T-01 / folder | 1 / 0 |
| job show --task | T-01 / folder | 1 / "task has 0 saved Jobs; specify a Job id" exit 1 (no Job in fixture) |
| job result | folder | 1 "GRID_MEMBER is required" (who the agent is; skill text names it) |
| feed post --type question | folder | 3 "$.body: missing required ref" (body shape; `grid feed types` lists the type) |
| ask | none | 2 "ask: not built yet" (skill does not use it) |

### Row 5 result lines

- `eval run skill-research --agent '["true"]'`: 2/7 failed
- `eval run skill-questions --agent '["true"]'`: 3/7 failed
- `eval run skill-deliver --agent '["true"]'`: 3/6 failed
- sh agent questions: `20261111T041627Z-886fb5449e236a2a skill-questions sh passed 7/7`
- sh agent deliver: `20261111T041628Z-5ba53116f60fd701 skill-deliver sh passed 6/6`

The two sh agents (`BIN` is the directory holding the built `grid`):

```sh
# questions
PATH=$BIN:$PATH
grid question add --task "$GRID_HOME/projects/notes-app/tasks/T-01-tag-box" --who sam --question 'How many recent tags should the box list, five or ten?' --option five --option ten --recommended five --applies 2026-12-01T09:00:00+01:00 --default five --why 'It sets the box height.'
# deliver
PATH=$BIN:$PATH
grid proof add 1 --text 'two lines counted, exit 0' --kind check --by agent --task T-01
grid proof add 2 --text 'names tag search, exit 0' --kind check --by agent --task T-01
grid proof index --task T-01
grid brain propose --name release-notes-say-tag --topic conventions --description 'when writing release notes' --summary 'Release notes say tag.' --source T-01 --body 'Release notes say tag and never label.' --task T-01
grid deliver --outcome 'Release note written.' --dry-run --task T-01
```

Orchestrator real runs after merge (needs credentials):
`go run ./cmd/grid eval run skill-deliver --agent '["claude","-p","--plugin-dir","dist/plugin"]' --env ANTHROPIC_API_KEY --timeout 15m`, and the same for skill-questions and skill-research.

## What changed

- Merge of origin/main; id list in internal/eval/suite_test.go.
- evals/tasks/skill-research/setup.sh: new task layout and project.md.
- skills/deliver, questions, research: one operand rule, `--task` on every verb, `brain propose --task` with `--link` keys to, rel, summary, why, topics from `grid brain topics`, decide fallback without a curator, GRID_MEMBER note.
- internal/settings/catalog_skills_lane_test.go: every catalog skill has a SKILL.md.

## NOT VERIFIED

- A real model run of any of the three evals, and that a model picks each skill from its description.
- `grid feed post --type question` with a valid body; the exit codes of `make lint` and `make cross` were read from their output, not captured.
- The before-fix exit 1 for skill-research was not re-run; it comes from review.md.
