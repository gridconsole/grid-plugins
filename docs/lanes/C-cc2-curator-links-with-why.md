# C-cc2-curator-links-with-why

Base: 2221cb866e76912e968cb7d90bb90c5e592080ce (lane-Pint140 merge). `git show main:internal/braincmd/decide.go | grep -c '"link"'` prints 0.

## Proof table

| Row | Result |
|---|---|
| 1 `TestCcLkAcceptAddsLinks` | HOLDS. `go test -count=1 ./internal/braincmd/ -run TestCcLk -v` exit 0, `--- PASS: TestCcLkAcceptAddsLinks`. The published `projects/sample/knowledge/conventions/lesson.md` holds A and B with summary and why; the body ends with `Source: [T-01](`. On base (decide.go and task_decide.go stashed): `cc_decide_links_test.go:59: exit 2: unknown flag --link` (the interspersed pre-parse refuses it before the flag package would print `flag provided but not defined`). |
| 2 `TestCcLkRefusesBadLink` | HOLDS, 6 bad links (empty why, 5-char why, `see also`, `task:T-01` target, unknown relation, unknown field) x accept and merge-into: exit 2, stderr holds the Link.Validate text (note.go:181, :188, :192) or `unknown field`, Grid home snapshot identical, proposal still `proposed`, `keep --task T-01` exits 0. On base: `stderr "unknown flag --link\n" lacks "link why must explain the benefit in 8 to 160 characters"` (and the same for the target, relation and unknown field texts). |
| 3 `TestCcLkMergeKeepsBoth` | HOLDS, PASS. Target links, then the proposal's, then C once. On base: `cc_decide_links_test.go:135: exit 2: unknown flag --link`. |
| 4 `TestCcLkRejectIgnoresLinks` | HOLDS, PASS. Rejected note equal with `updated` removed, same set of files under the Grid home. On base: `cc_decide_links_test.go:148: exit 2: unknown flag --link`. |
| 5 contract and walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestContractCommandsInventory\|TestContractCommandsParserInventory\|TestGgUnknownFlagWalkCoversTheContract\|TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v` exit 0: `--- PASS:` for all five. `go test -count=1 ./internal/braincmd/ ./internal/brain/ ./internal/job/lint/` exit 0, all three ok. |
| 6 `make lint`, `make cross`, diff | HOLDS for lint (exit 0) and cross (exit 0); `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0; `gofmt -l internal/` empty. The diff names two files outside Touches, see "Outside Touches". |
| Importers of brain and braincmd | HOLDS. `go list` importers: braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, projectenv, routine, task, workflow. `go test -count=1` on those plus internal/golden, internal/eval, internal/packaging: all ok (cli 98s, after the golden fix). |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `./internal/job/lint/` ok. |
| Existing callers compile | HOLDS. `links ...Link` is trailing variadic after `history`; no existing test edited. |
| No guard or cap | HOLDS. The flag only adds links. |
| RealPlugin tests | Not run: the lane changes no app-facing shape and no brief row names one. |

## What changed
- `internal/braincmd/decide.go`: repeatable `--link` through `linkFlags` (JSON, unknown fields refused, `Link.Validate` at parse, so a bad link exits 2 before anything is written); usage lines carry `[--link <json>]...`.
- `internal/brain/task_decide.go`: `DecideTask(..., history, links ...Link)`; on accept and merge-into the links go after the proposal's own and the target's, an exact duplicate (same To, Rel, Summary, Why) once (`withLinks`); reject ignores them. The Source line is unchanged. Links are also validated in the library for non-reject actions.
- `internal/braincmd/command.go`: only the `decide` line of the usage const.
- `contract/commands.json`: `--link`, takes value, not required, under brain decide.
- `agents/curator.md` step 2 and `skills/brain-keeper/SKILL.md` steps 5 and 6: `--link` with summary and why, and a `decided-by` link to the curator's own Job.
- `internal/braincmd/cc_decide_links_test.go`: the four tests.

## Outside Touches (the brief says "update what it finds")
- `testdata/golden/brain-help.golden:12` and `testdata/golden/brain-decide-help.golden` pin the decide usage text. The brief's grep for `brain decide <task>` found `brain-decide-help.golden` (the brief said no golden names it); `brain-help.golden` does not match that string but `TestRemainingTaskOperandHelp/brain` failed on it until updated. Both updated to the new output.

## Open items
- `agent-templates/grid-curator.md:24` still shows the decide usage without `--link` (not in Touches; a follow-up could add it).
- A retry of the same decision with different `--link` values reuses the receipt written by the first attempt (the receipt compares action, target and note only), so the first links win. This matches how a retry already behaves.

## NOT VERIFIED on a live screen
Nothing here has a UI. No live run of the curator agent was made (no claude run in the sandbox).

DONE
