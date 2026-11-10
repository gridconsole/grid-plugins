# D-plugin-skills-c: the brain-keeper, browser and visual-design-review skills

Base: plugin main eb833a2d. Lane branch lane-D-plugin-skills-c.

## Proof table

| Row | State | Proof |
| --- | --- | --- |
| 1. skills/brain-keeper/SKILL.md: proposal to linked note, topic, quality bar for note, skill and agent, link-with-a-why, never asks the person | HOLDS | skills/brain-keeper/SKILL.md:10 (propose with `grid brain propose --link`), :22 (where it goes: decisions, conventions, custom topic `knowledge/<topic>/TOPIC.md`, config topics refused), :32 (keep: `grid brain keep`, `show --proposal`, `check`, `accept`, `decide`, `index`), :45 (quality bar: note, link, skill, agent), :53 (every link carries a why, 8 to 160 characters), :8 and :34 (never asks the person). Every verb is in `grid brain help` or `grid --help`. |
| 2. skills/browser/SKILL.md: read-only vs state-changing, screenshot per row, widths 390/1440/3840, NOT VERIFIED never a pass | HOLDS | skills/browser/SKILL.md:8 (steps: mode, widths, `grid proof add <item> <file> --kind screenshot`), :26 (NOT VERIFIED is never a pass). Verbs: `grid job show`, `grid job result`, `grid env up|down`, `grid proof add|index`; `grid proof --help` and `grid env --help` were run. |
| 3. skills/visual-design-review/SKILL.md ported to the new verbs (proof add for screenshots) | HOLDS | skills/visual-design-review/SKILL.md:8 (steps 1 and 4 file screenshots and measurements with `grid proof add`; step 7 feeds back with `grid brain propose`), widths 390/1440/3840, "every field names its data source" at :22. `measure.js` copied unchanged beside it. |
| 4a. One eval task per skill | HOLDS | evals/tasks/skill-brain-keeper, skill-browser, skill-visual-design-review (task.json, prompt.md, checks.json, setup.sh each). |
| 4b. Real run pasted | NOT DONE | All three real runs exited 1 before the agent ran: `agent: failed (exit 1, 783ms)`, stdout `Not logged in · Please run /login`. The runner gives the agent an empty HOME and passes only variables named with `--env`; `ANTHROPIC_API_KEY` is not set in this session. I did not point the agent at the session's own login folder, because the harness isolates credentials on purpose. Result lines below. The orchestrator runs them with an API key. |
| 4c. Checks fail when the skill is missing | HOLDS | `go run ./cmd/grid eval run --agent '["true"]'` (a no-op agent): skill-brain-keeper `failed 0/4 gates 0/1`, skill-browser `failed 1/5 gates 0/1`, skill-visual-design-review `failed 0/9 gates 0/1`. |
| 4d. Checks pass on a correct outcome | HOLDS | internal/eval/home_test.go goodOutcome entries for the three tasks; `go test ./internal/eval ./internal/evalcmd ./internal/cli` exit 0. |
| 4e. `go test ./...` and `make lint` | HOLDS | `go test ./...`: 57 packages ok, no FAIL line, exit 0. `make lint`: exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Tests of changed packages and their importers | HOLDS | Changed Go package: internal/eval (tests only). Its importers per `go list` are internal/evalcmd and internal/cli; both ran, exit 0. |
| New tests fail on base, pass on the lane | HOLDS | The first `go test ./...` on the lane's new tasks, before the registry edit, failed: `home_test.go:80: only 19 tier-b cases` and `suite_test.go:23: unexpected tasks`. The new goodOutcome entries run only with the new tasks, which base does not have. |
| No existing test edited to pass | HOLDS with a note | Two pinned registries grew because the brief adds three tasks: internal/eval/suite_test.go (`want` list gained 3 ids) and internal/eval/home_test.go (tier-b count 16 to 19). No assertion was loosened, no test deleted. |
| No guard, gate, cap or hold added on agents | HOLDS | No Go change outside tests. The skills name the existing `grid brain check` gate and say agents get no Grid-made limits (skills/brain-keeper/SKILL.md:63). |
| Topics README fixed | HOLDS | topics/README.md now says `knowledge/<topic>/TOPIC.md` in the brain, with internal/brain/topics.go:25 and :66. |
| Commits | HOLDS | One-sentence messages, no attribution lines. The attribution reminder in the session was not followed, as the owner's rule wins. |

## Real run result lines (login failure)

```
20261110T225853Z-be03baf7935505b3  skill-brain-keeper         b small claude failed 0/5 0/1 not measured 798ms
20261110T225854Z-adb4f7aebea3ba6a  skill-browser              b small claude failed 0/5 0/1 not measured 888ms
20261110T225855Z-cba366a05434bdf1  skill-visual-design-review b small claude failed 0/9 0/1 not measured 758ms
```

The first run's saved result shows `agent: failed (exit 1)`, stdout `Not logged in`. The other two ended the same way in under a second; I read the saved result for the first only. The brain-keeper run still counted 5 checks; the check list was later cut to 4 (the `no-proposal-left` check passed on an untouched fixture, so it proved nothing).

## What changed

- skills/brain-keeper/SKILL.md, skills/browser/SKILL.md, skills/visual-design-review/SKILL.md (new), skills/visual-design-review/measure.js (copied from the old plugin).
- topics/README.md: path fixed.
- evals/tasks/skill-brain-keeper, skill-browser, skill-visual-design-review.
- internal/eval/suite_test.go, internal/eval/home_test.go: registry entries and good outcomes for the three tasks.

## Notes for the reviewer

- The brain-keeper skill says a proposal is filed as written and cannot be edited (internal/brain/store.go:245, "proposal already exists"), so a weak proposal is rejected and proposed again under a new name. If an in-place rewrite verb is wanted, it does not exist yet.
- The old grid-curator agent (old-plugin/agents/grid-curator.md) is not ported here; the brief asks for the skill only. The skill is written so a subagent can follow it.
- `grid brain check` flags any skill front-matter key outside name, description, when_to_use, metadata (internal/brain/check.go:325), so brain skills must use only those.

## NOT VERIFIED on a live screen

- No skill was run by a real agent (login failure above), so whether an agent follows them is untested.
- The `measure.js` script was copied, not run, in this lane.
- The browser and visual-design-review skills were never run against a real browser.
