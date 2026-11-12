# C-xs-golden-l29-rerecord: L29 golden cases re-recorded from the Go hooks or deleted

Base: f09dda3a (plugin main, `git rev-parse HEAD` before the first commit).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Base and groups | HOLDS, one premise differs | `git show HEAD:golden/known-differences.txt`: 50 cases; in scope agent-routing 2, brain-read 2, brief-lint 6, grid-env 3, grid-home-allow 3, heartbeat 2, history-log 8, lib-parseFrontmatter 2, stop-check 7, usage-log 3, worker-exit 3 = 41; host-guard 2 and plan-guard 7 left out. No case was already gone. |
| 2. `make golden` skips | HOLDS, before-count differs | Before: `make golden` -v holds 44 `known difference:` skips, not 50 (TOTAL 131: pass 58 / known 44 / fail 0 / skip 29). The six brief-lint cases never reach corpus_test.go:125: `brief-lint` left `legacyHooks` (docs/lanes/E-verdict-2-brief.md:29), so Replay skips them at Resolve with "command not implemented". After: `known difference: 9` (TOTAL 126: pass 91 / known 9 / fail 0 / skip 26): host-guard 2, plan-guard 7 (incl. bash-recall), plus brief-lint 3 kept on purpose (see "Rows closed by C-xs2-brief-lint-stop-check"). `make golden` exit 0. |
| 3. TestXsRerecord | HOLDS | `go test -count=1 -v -run TestXsRerecord ./internal/golden/` prints `--- PASS: TestXsRerecord` with subtests stop-check/mark-done (case folder) and agent-routing/session (go-expected). It stales stdout, shows the replay differs, shows a flag naming another case rewrites nothing, re-records with `-xs-rerecord=`, then replays with no differences and no skip. The test lives in a new file, so it cannot fail on an unchanged base (the flag and the file do not exist there). |
| 4. Deleted cases | HOLDS | Commit a35de0af names each: heartbeat/post, heartbeat/missing-task: docs/lanes/L29.md:63 (D2); covered by internal/hook/events_test.go:65 `TestHookHeartbeatCannotRenewOrEndOnStop` (fatal at :93). brief-lint/complete, missing-file, no-brief: docs/lanes/L4.md:58, and the hook itself was removed by docs/lanes/E-verdict-2-brief.md:9,:29; replacement is Job lint, internal/job/lint/json_test.go:26 `TestCheckJSONValidRecordIsClean`. Deletions are only under golden/heartbeat/ and those three brief-lint folders. |
| 5. grid-home-allow decisions | HOLDS | Old (Node recording) vs new (Go), per path. inside-read: Read of `$GRID_HOME/projects/demo/tasks/T-1-demo/subagents/01-build.md`: allow, allow (stdout identical; files.diff gains the home-allow counter line `{"event":"PreToolUse","tool":"Read"}`, L4.md:71). own-script: Bash `node $OLD_PLUGIN/skills/plan/scripts/plan-hash.mjs ...`: allow, no grant (stdout now empty, Claude Code's own prompt decides; tighter). permission: PermissionRequest Write of the same task-folder file: allow (addDirectories `$GRID_HOME`), allow (stdout identical; counter line `{"event":"PermissionRequest","tool":"Write"}` added). The old recording denied none of the three paths, so no old denial is allowed by the new one. The 7 other grid-home-allow cases (broad-home, glob-traversal, outside, relative, script-outside, shell-chain, symlink-escape) already passed and are untouched. |
| 6. Tests, lint, cross | HOLDS | `make lint && make cross && go test -count=1 ./internal/golden/ && go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' && go test -count=1 ./internal/job/lint/` exit 0 (darwin/arm64 and windows/amd64 vet and test compile both ran). |
| Changed packages and importers | HOLDS | Only the test package `internal/golden` changed (test files and golden/ data, no production code). Its importers use `golden.Assert` on their own testdata and are not affected by corpus cases; `make cross` compiled every package's tests for both OSes. |
| Walk tests (plugin) | HOLDS | `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` ok; `internal/job/lint` ok (in the chain above, exit 0). |
| New test fails on base | HOLDS by construction | `xs_rerecord_test.go` is new, so it does not exist on f09dda3a. |
| No guard, gate, cap, hold on agents | HOLDS | No production code changed. |
| docs/lanes proof table | HOLDS | this file |

## Rows closed by C-xs2-brief-lint-stop-check

- brief-lint/missing-field, missing-section, no-frontmatter (3 of the 41): CLOSED BY C-xs2-brief-lint-stop-check. The earlier premise was wrong: "brief-lint" is not a Go hook. It is not in `legacyHooks` (internal/golden/corpus_test.go:24-30) or the component map (:48), so `Resolve` returns false and Replay skips all brief-lint cases as "command not implemented: brief-lint" before the known-difference check at :126. The three lines are kept on purpose, not deleted, because brief-lint is on neither briefs/needed.md nor briefs/simplify.md. Their reasons now name the true cause and the covering Job lint tests: internal/job/lint/lint_test.go:82 `TestEveryLegacyFieldAndHeading` (field rules :84-96, every heading including Outcome :97-104, frontmatter :105-107) and :173 `TestMissingFieldsAndReadRanges`. brief-lint/invalid-json is not listed and also skips; left alone.
- Row 2, after count: `make golden` -v prints 126 cases: pass 91 / known difference 9 / fail 0 / skip 26 (exit 0). The 12 kept lines of golden/known-differences.txt are host-guard 2 and plan-guard 7 (the 9 "known difference" results, each with its reason in the -v output) plus brief-lint 3, which skip with "command not implemented: brief-lint" and carry their reason in the known-differences.txt line. The other skips (needs-you, permission-forward, session-*, terminal-task, brief-lint/invalid-json) are commands with no Go hook and no listed line; they are outside this lane.
- stop-check `proves` text: CLOSED BY C-xs2-brief-lint-stop-check. The 7 re-recordings are kept; each setup.json `proves` now says what the recording pins (no GRID_JOB: silent exit 0, docs/lanes/E-verdict-5-stop.md:10; mark-done and mark-blocked: no status marking, empty files.diff). The leftover `exit` and `contains_stderr` keys in those setup.json files are the Node recorder's old expectations; the Go replay reads the recorded `exit`/`stderr` files, so they were left alone.
- agent-routing/* and grid-env/* stderr: CLOSED BY C-xs2-brief-lint-stop-check. `grid hook: agent sync: open $OLD_PLUGIN/agent-templates: no such file or directory` comes from `syncAgents` (internal/hook/hook.go:166-180), which reads `$CLAUDE_PLUGIN_ROOT/agent-templates` (internal/settings/agentsync.go:23). The corpus rig sets CLAUDE_PLUGIN_ROOT to the inert `$OLD_PLUGIN` token folder, which has no agent-templates; the real plugin root has the folder (agent-templates/ at the repo root). So the rig lacks the folder and the hook does not read an old plugin path in real use; the recording is kept and the go-diff.md of agent-routing/{session,empty-object} and grid-env/{discovered,explicit,no-config} now say so (agent-routing/invalid-json has empty stderr and is unchanged).

## Proofs for this repair (head after merge of origin/main, which merged with no conflicts)

| Row | Result | Proof |
|---|---|---|
| `make golden` exits 0, 12 kept skips named | HOLDS | `make golden` exit 0: `TOTAL 126  pass 91 / known difference 9 / fail 0 / skip 26`, `--- PASS: TestGoldenCorpus`. See row 2 above for the 12. |
| TestXsRerecord | HOLDS | `go test -count=1 -v -run 'TestXsRerecord' ./internal/golden/` prints `--- PASS: TestXsRerecord`, subtests stop-check/mark-done and agent-routing/session PASS; `git status` clean of golden/ changes afterwards. |
| Deletions only under heartbeat/ and brief-lint/{complete,missing-file,no-brief} | HOLDS | `git diff origin/main --diff-filter=D --name-only -- golden/` lists only golden/heartbeat/{post,missing-task} and golden/brief-lint/{complete,missing-file,no-brief}. |
| grid-home-allow unchanged | HOLDS | No file under golden/grid-home-allow/ changed in this repair (`git status` lists none); the base report's decision table stands. |
| `go test -count=1 ./internal/golden/` | HOLDS | exit 0, `ok .../internal/golden`. |
| `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| `make cross` | HOLDS | exit 0; `cross: windows/amd64 vet`, `windows/amd64 test compile`, darwin run, every package `ok`. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| Changed packages and importers | HOLDS | Only data files under golden/ changed in this repair, no Go code; `internal/golden` test passes, and `make cross` ran every package's tests on both OSes. |
| New test fails on base | HOLDS by construction | No new test here; `xs_rerecord_test.go` is the base lane's, new there. |
| No guard, gate, cap, hold on agents | HOLDS | No production code changed. |
| Touches | HOLDS | Files changed: golden/known-differences.txt, golden/stop-check/*/setup.json (7), go-diff.md of 5 agent-routing/grid-env cases, this report. The brief's briefs/ folder is absent from this clone, so Touches of the base brief were taken from the repair brief's text. |

## What changed

- `internal/golden/xs_rerecord_test.go` (new): `-xs-rerecord=<case,case>` test flag and `TestXsRerecord`; all helpers start with `xsRr`.
- `internal/golden/corpus_test.go`: one added line calling `xsRrApply` after Replay (the hook the flag needs).
- 30 cases re-recorded: agent-routing 2, grid-env 3 (written into go-expected, next to their go-diff.md), history-log 8, brain-read 2, stop-check 7, usage-log 3, worker-exit 3, lib-parseFrontmatter 2; plus grid-home-allow 3. Their 33 lines are gone from golden/known-differences.txt. Five cases deleted (2 heartbeat, 3 brief-lint) with their lines.

## For the reviewer: what the re-recordings pin

- stop-check: the cited reason (L29.md:80, D6) describes a wording change, but what landed is docs/lanes/E-verdict-5-stop.md:10, which replaced the old task.md and brief checks with one check, "the Job's result is posted". The five cases with no GRID_JOB (empty-next, empty-summary, finished-no-outcome, reviewer, subagent-missing) now record exit 0 and empty stderr (the silent no-Job path, covered by internal/hook/stop_test.go:94 `TestStopWithNoJobStopsSilently`); mark-done and mark-blocked lose their files.diff because status marking is gone. Their setup.json `proves` text was stale ("blocks Stop with exit 2"); C-xs2-brief-lint-stop-check rewrote it to what the recording pins.
- agent-routing/* and grid-env/*: go-expected/stdout now holds the brain topic index and no longer the agent routing line (the routing line was removed by docs/lanes/E-verdict-2-brief.md:29). go-expected/stderr pins `grid hook: agent sync: open $OLD_PLUGIN/agent-templates: no such file or directory`, which is the Go hook's real output with the recorded CLAUDE_PLUGIN_ROOT pointing at the inert token folder, not a recorded bug. The go-diff.md prose of the five cases with that stderr now explains it (rig gap, not a product bug); see "Rows closed by C-xs2-brief-lint-stop-check".
- worker-exit/missing-task stderr `grid hook: worker exit: task T-999 not found in project demo` is D8 "missing tasks log and allow".
- The grid-home-allow/permission known-difference reason ("the only-writer rule denies the recorded write") was stale: only-writer is at warn by default, and Go allows through the home grant exactly as the old script did. Decision unchanged, so the re-record does not loosen anything.

## NOT VERIFIED on a live screen

Nothing here is a screen.

DONE
