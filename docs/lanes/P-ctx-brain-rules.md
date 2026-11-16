# P-ctx-brain-rules: standing rules live in the brain; Jobs carry pointers

Base: main 557911e8. `git fetch origin && git merge origin/main` before the last proof run: main had not moved (head unchanged).

## Proof table

| row | state | proof |
| --- | --- | --- |
| Do 1: read plan-jobs, run-job, brain-keeper, the SessionStart hook and topics/; write the decision | HOLDS | docs/decisions/standing-rules-in-the-brain.md: the four rule kinds and their topic, how a Job points (keep, decisions, links with a why), what a worker reads, what is not changed |
| Do 2a: plan-jobs says a Job names its standing-rule notes and never pastes their text | HOLDS | skills/plan-jobs/SKILL.md, section `## Standing rules` |
| Do 2b: run-job reads only the named notes (grid brain show) and the SessionStart index, nothing else by default | HOLDS | skills/run-job/SKILL.md, `## Brain`, first bullet |
| Do 2c: grid:library answers broader brain questions with note ids | HOLDS | agents/library.md step 5 and agent-templates/grid-library.md step 5 (it already named note ids in step 4; step 5 covers "which standing rules apply": ids with one line, never bodies) |
| Do 2d: SessionStart prints the index only, never note bodies; check it and fix it if not | HOLDS, no fix needed | internal/hook/hook.go:146-152 passes only `brain.Store.Index().Text` (internal/brain/index.go:175-205: topic, description, count, last change, invalid). Pinned by internal/hook/sessionstart_index_only_test.go: `go test -count=1 -v -run TestSessionStartPrintsTheIndexNotNoteBodies ./internal/hook/` exit 0. This test passes on base too, because the hook needed no change; it is a regression pin, not a fails-on-base test |
| Do 3: a brain topic or seed for standing rules if the topics need one | HOLDS, none needed | the decision doc, section "What a standing rule is": `conventions` and `decisions` are built in (internal/brain/topics.go:25-28), and a shipped topic would add an index row to every session. skills/brain-keeper/SKILL.md `## Where it goes` says which topic each rule kind goes in. No project text in the plugin |
| Do 4a: a test that fails on base for the Job template check | HOLDS | new lint warning `job.pasted-rule` (internal/job/lint/pasted_rule.go, called at internal/job/lint/lint.go:263). Base run (lint.go change stashed): `--- FAIL: TestPastedRuleTextIsFlagged` `pasted_rule_test.go:32: want one job.pasted-rule finding, got []` and `--- FAIL: TestPastedRuleIgnoresCaseAndPunctuation` `pasted_rule_test.go:62: want one job.pasted-rule finding for a re-cased copy, got []`, exit 1. Lane: `go test -count=1 -v -run 'TestPastedRule\|TestGoldenBriefPasses' ./internal/job/lint/` exit 0 |
| Do 4b: make test | HOLDS | `make -C . test` exit 0, every package ok, temp leak check clean |
| Do 4c: make lint | HOLDS | `make -C . lint` (go vet ./...) exit 0 |
| Do 4d: make dist | HOLDS | `make -C . dist` exit 0, built darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64 |
| Rule: tests of changed packages and their importers | HOLDS | importers of internal/job/lint: internal/cli, internal/job/jobcmd; of internal/hook: internal/cli, internal/packaging (go list). All inside `make test` exit 0; also `go test -count=1 . ./internal/contract/ ./internal/cli/ ./internal/plan/ ./internal/job/... ./internal/hook/ ./internal/brain/...` exit 0 |
| Rule: Windows and macOS builds | HOLDS | `make dist` above built windows/amd64 and darwin/arm64, darwin/amd64 |
| Rule: walk tests | HOLDS | `go test -count=1 ./internal/cli -run "TestHsIpWalksEveryCommand\|TestIlGuestSiteList" -v` exit 0 (PASS both); `go test -count=1 ./internal/job/lint/` exit 0 |
| Rule: no guard, gate, cap or hold on agents | HOLDS | `job.pasted-rule` is a warning (pasted_rule.go addAt "warning"); the test checks `lint.HasErrors` is false, so the Job still posts, renders and runs |
| Rule: no existing test edited, none deleted | HOLDS | `git diff --stat origin/main` lists only new test files |
| Rule: small commits, no attribution lines | HOLDS | `git log origin/main..` |
| Rule: no personal paths, hosts or emails in the repo | HOLDS | the decision doc and this report name no host, home path or person |
| Repo-wide app checks | not run, not asked | plugin lane |

## What changed

- internal/job/lint/pasted_rule.go (new): `job.pasted-rule` warns when a Job's title, an acceptance row, a link or after why, or review_reason copies a sentence (six words or more, compared on lower-cased words) of a brain note the Job names in links, keep or decisions. Notes the Job does not name are not checked.
- internal/job/lint/lint.go: calls it beside reviewTwice.
- skills/plan-jobs/SKILL.md: `## Standing rules` (find with search or grid:library; name in keep or decisions with a links why; a removal note does not replace check 1; the lint warning; propose a missing rule first).
- skills/run-job/SKILL.md: what a worker reads from the brain (index plus the named notes and not_rendered ids; grid brain show; grid:library for anything broader; cite ids, never paste).
- skills/brain-keeper/SKILL.md: where a standing rule goes.
- agents/library.md and agent-templates/grid-library.md: step 5, standing rules answered as note ids with one line.
- docs/decisions/standing-rules-in-the-brain.md (new).
- Tests (new): internal/hook/sessionstart_index_only_test.go, internal/job/lint/pasted_rule_test.go.

## Open items

- The brief renderer pastes an excerpt of each named note (internal/job/render.go:591-597) and recall adds up to 8 notes the planner did not name (internal/job/jobcmd/recall.go:23-45, ranked last at render.go:210-214), inside the 2,000-token brain budget. Not changed here (it is a feature and outside the brief's build list); whether recall should go pointer-only once a Job names its notes is for the owner.
- `job.pasted-rule` only sees notes the Job names; a rule pasted with no link is not detected (it would need a brain-wide scan at lint time).
- grid:library's new step 5 is text; its effect on a live session is NOT VERIFIED (no live session run).

NOT VERIFIED on a live screen: nothing in this lane has a screen.

DONE
