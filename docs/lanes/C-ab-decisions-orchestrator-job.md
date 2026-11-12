# C-ab-decisions-orchestrator-job: a result with decisions_needed makes one orchestrator Job that asks the person, once

Base: 2221cb866e76912e968cb7d90bb90c5e592080ce (origin/main at clone). Base check, `git grep -n -i 'DecisionsNeeded\|NeedsLead' HEAD -- internal/orchestrator internal/cli`: internal/orchestrator/events.go:148 and test files only (internal/cli/kb_found_role_test.go:83,106,107,120,121, internal/cli/tf_found_body_test.go:145). No pass read the field, so the lane went ahead.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1 TestAbDnOneJobPerResult | HOLDS | Base (pass written, not called from the tick, commit d91fe076): `go test -count=1 -run 'TestAbDn' ./internal/orchestrator/` exit 1, `ab_decisions_test.go:118: after one tick: Jobs on the Feed = [J-1 J-2], want [J-1 J-2 J-3]`. Lane: `--- PASS: TestAbDnOneJobPerResult`. J-1 gets a result with two decisions and J-2 gets `"decisions_needed":[]`; one tick posts exactly J-3 (role orchestrator, discovered_from null, no `job:` pointer). Its A1 begins `Decisions needed in result record <result id> (` and holds both lines. The body equals AbDnJobBody. |
| 2 TestAbDnSecondTickPostsNothing | HOLDS | Base: exit 1, `ab_decisions_test.go:168: after the first tick: Jobs on the Feed = [J-1], want [J-1 J-2]`. Lane: `--- PASS`. A second tick and a tick from a fresh feed.Store on the same folder (abDnRestart) post nothing. A second result on J-1 posts J-3, which names the second result id and not the first. One more tick and one more restart still leave J-1..J-3. |
| 3 TestAbDnTwoTicksOneJob | HOLDS | Base: exit 1, `ab_decisions_test.go:227: round 1: orchestrator Jobs naming result 01M76WAAMNV4REQVJFKZEAK6CE = [], want [J-2] (Feed holds [J-1])`. Lane: `--- PASS: TestAbDnTwoTicksOneJob (14.09s)`. 20 fresh folders, two Stores ticking together on kbFrTickTogether's start barrier. Each round holds Jobs [J-1 J-2], job records [J-1 J-2], and one Job names the result. The log shows the lock really contended, e.g. `round 20: one Job J-2 ... a Tick found the orchestrator lock held and waited 1 time(s)`. It uses no sleep and no timeout, only WaitGroups, so no t.Deadline ctx is needed. |
| 4 TestAbDnBodyIsOneLine | HOLDS | Lane: `--- PASS`. A decision of 40 repeated phrases plus `\n\tend` gives a title with no `\n\r\t` and at most prefix+120 runes. A1 is one line, and the body passes `contract.Validate("job-record", ...)`. A Job posted as AbDnJobBody (A1 begins with abDnA1Lead) whose folded result holds decisions_needed (asserted: Results 1, DecisionsNeeded 1) posts no Job after a tick and a restart tick. On base this test passes, because nothing posts there. Mismatch proof: I removed `abDnAsks(snap, id)` from decisions_needed.go:33 and got exit 1, `ab_decisions_test.go:271: ... Jobs on the Feed = [J-1 J-2], want [J-1]`. Then `git checkout -- internal/orchestrator/decisions_needed.go`, the rerun gave `ok`, and `git status --short` was clean. |
| 5 tests | HOLDS | `go test -count=1 -run 'TestAbDn' ./internal/orchestrator/ -v` prints `--- PASS:` for all four. `go test -count=1 ./internal/orchestrator/ ./internal/cli/ ./internal/mcp/ ./internal/migrate/` gives `ok` for all four packages (exit 0). No existing test was edited. |
| 6 lint, cross, diff | HOLDS | `make lint` exit 0. `make cross >/dev/null 2>&1 && git rev-parse --short HEAD` printed the sha (exit 0; darwin/arm64 and windows/amd64 vet and test compile). `git diff origin/main --stat` lists internal/orchestrator/decisions_needed.go, tick.go and ab_decisions_test.go, plus this report, which the builder runbook requires (docs/lanes holds every lane's report on main). |
| Importers (`go list`) | HOLDS | Packages importing internal/orchestrator: internal/cli, internal/mcp, internal/migrate. All pass (row 5). |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v`: both `--- PASS`, `ok`. `go test -count=1 ./internal/job/lint/`: `ok`. |
| RealPlugin tests | n/a | The brief names none, and the new tests need no plugin binary. |
| No guard, gate, cap or hold | HOLDS | The pass only posts a Job (decisions_needed.go:30-57). It refuses, cancels, parks and counts nothing. events.go:148 is unchanged. |
| New tests fail on base | HOLDS | Rows 1 to 3 fail as pasted above. Row 4's second half only shows its effect under mutation, as noted. |
| Touches only | HOLDS | Code changes are limited to decisions_needed.go (new), tick.go (one hunk, tick.go:123-133) and ab_decisions_test.go (new). |
| Commits | HOLDS | Small one-sentence messages, no attribution lines. |
| UI | n/a | No UI. |

## What changed

- internal/orchestrator/decisions_needed.go (new) adds `abDnA1Lead`, `(Store).abDecisionsNeeded`, `abDnDecisions`, `abDnAsks`, `abDnPosted` and `AbDnJobBody`. The pass goes through each Job of the fold, except a Job whose own A1 begins with abDnA1Lead. For each result id in `j.Results` whose record body holds a non-empty `decisions_needed`, it posts one orchestrator Job, unless an orchestrator Job whose A1 begins with `abDnA1Lead + id + " ("` is already on the Feed. It reloads the snapshot after each post. The body is ReplanJobBody with discovered_from null, the title `Decisions needed on <job>: <first decision, one line, cut to 120 runes>`, and one A1 holding every decision (one line each, gqCause-cut, joined by `; `) plus the instruction to ask with `grid question ask` or a person Job and post with `grid job result`.
- internal/orchestrator/tick.go: after kbSecurityFound, the tick reloads the snapshot, calls abDecisionsNeeded, then reloads again before proposals.
- internal/orchestrator/ab_decisions_test.go (new) holds the four TestAbDn tests and their abDn helpers. It reuses kbFrTickTogether and kbFrBlocked from kb_found_race_test.go.

## Open items

- A whitespace-only decision is dropped. A result whose decisions are all empty posts no Job.
- A result whose body is a ref (not read yet) holds no decisions here and posts nothing, as kbSecurityFound treats a ref found.

## NOT VERIFIED on a live screen

Nothing. The lane has no UI.

DONE
