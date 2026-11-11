# C-ff-plan-jobs-anchor-forms

Base: 06047047 (main plus lane merges). Base check: skills/plan-jobs/SKILL.md read whole; line 49 showed `plan.md#step-1` and `plan.md#verify` and never stated the checklist form or the `## Verify` heading. internal/plan/items.go:168-203 (Anchor) and :31 (itemLine) as the brief says.

## Proof table

| Row | Result |
|---|---|
| 1. `- [ ] 1` and `## Verify` in the skill | HOLDS: `git grep -n -e "- \[ \] 1" -e "## Verify" -- skills/plan-jobs/SKILL.md` exit 0, lines 41, 45, 47. Base had none. |
| 2. New test, shaped plan resolves; `### Step 1:` does not | HOLDS: `go test ./internal/plan/ -count=1 -run TestFfAnchorForms -v` PASS for TestFfAnchorFormsSkillExampleResolves and TestFfAnchorFormsHeadingStepsDoNotResolve (internal/plan/anchor_skill_forms_test.go). The first reads the example plan out of the skill itself. |
| 3. lint-before-post line | HOLDS: `git grep -n 'grid job lint' -- skills/plan-jobs/SKILL.md` lines 52 (new) and 102. |
| 4. `go test ./internal/plan/ -count=1` | HOLDS: exit 0. The runbook names no skill-check make target; Makefile has none (targets: build dist generate test lint cross golden eval-check clean). The root package test, which embeds skills, passes. |
| New tests fail on base | HOLDS: with SKILL.md stashed to base: `--- FAIL: TestFfAnchorFormsSkillExampleResolves ... skill has no 'Pointers into plan.md' part`, exit 1. |
| No existing test edited or deleted | HOLDS: one new file, one skill edit. |
| Importers of internal/plan tested | HOLDS: `go test -count=1` on root, plan, job/..., jobgraph, land, orchestrator, proof, task, cli, approvefix, hook, migrate, lease, packaging/... all ok (the importers of plan, directly or through job/task, from go list). |
| lint | HOLDS: `make lint` (go vet ./...) exit 0. |
| Windows and macOS builds | HOLDS: `make cross` exit 0 (vet and test compile for darwin/arm64 and windows/amd64). |
| No guard, gate, cap | HOLDS: skill text only. |
| RealPlugin tests | Not applicable: no *RealPlugin* test touched. |

## What changed
- skills/plan-jobs/SKILL.md: a "Pointers into plan.md" part (10 lines): step-N form, `## Verify`, heading anchors, `## Where it stands`, and lint before post.
- internal/plan/anchor_skill_forms_test.go: two tests.

Note: plan.Anchor resolves only step-N and verify, so the skill says plan.md has no slug anchors. The "slug or `{#id}`" form is stated for other files (e.g. `task:research.md#format`), as the brief asked; I did not check how those other files resolve it.

## NOT VERIFIED on a live screen
No UI. Nothing was run against a live agent; whether the text stops the pointer repair in J-1 needs an eval run.
