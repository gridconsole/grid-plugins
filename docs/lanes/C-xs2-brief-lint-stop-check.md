# C-xs2-brief-lint-stop-check: brief-lint and stop-check cases of the L29 re-record settled

Repair of C-xs-golden-l29-rerecord (marked STUCK at aa16344a). The full proof table is in docs/lanes/C-xs-golden-l29-rerecord.md, section "Rows closed by C-xs2-brief-lint-stop-check"; this file lists the rows of the repair brief.

| Row | Result | Proof |
|---|---|---|
| `git merge origin/main` | HOLDS | merged with no conflicts; no file needed resolving. |
| `make golden` -v skip count | HOLDS | exit 0; `TOTAL 126  pass 91 / known difference 9 / fail 0 / skip 26`. The 12 kept lines: host-guard 2 and plan-guard 7 print `known difference: <reason>`; brief-lint 3 skip with "command not implemented: brief-lint" and carry their reason in golden/known-differences.txt:2-4. |
| Three brief-lint lines kept, true reason | HOLDS | golden/known-differences.txt:2-4 name corpus_test.go:24-30 and :48 (no Go hook) and the covering Job lint tests internal/job/lint/lint_test.go:82 (`TestEveryLegacyFieldAndHeading`, headings :97-104, frontmatter :105-107) and :173 (`TestMissingFieldsAndReadRanges`). json_test.go:26 covers only a valid record, so it was not named for these three. |
| Seven stop-check `proves` rewritten | HOLDS | golden/stop-check/{empty-next,empty-summary,finished-no-outcome,reviewer,subagent-missing,mark-done,mark-blocked}/setup.json; recordings unchanged. |
| `agent sync: open $OLD_PLUGIN/agent-templates` | HOLDS, rig gap | Source: internal/hook/hook.go:166-180 reads `$CLAUDE_PLUGIN_ROOT/agent-templates` (internal/settings/agentsync.go:23). The rig points CLAUDE_PLUGIN_ROOT at the inert `$OLD_PLUGIN` folder without agent-templates; the real plugin has agent-templates/. Not a product bug; recording kept and go-diff.md of 5 cases says so. |
| grid-home-allow untouched | HOLDS | no change under golden/grid-home-allow/ in this lane. |
| `go test -count=1 -v -run 'TestXsRerecord' ./internal/golden/` | HOLDS | `--- PASS: TestXsRerecord`. |
| Deletions only heartbeat/ and brief-lint/{complete,missing-file,no-brief} | HOLDS | `git diff origin/main --diff-filter=D --name-only -- golden/` lists only those five folders. |
| `go test -count=1 ./internal/golden/`, `make lint`, `make cross` | HOLDS | all exit 0. |
| Walk tests (plugin) | HOLDS | `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` ok; `go test -count=1 ./internal/job/lint/` ok. |
| Changed packages and importers | HOLDS | data files under golden/ and docs only; `make cross` ran every package's tests on both OSes. |
| No guard, gate, cap, hold on agents | HOLDS | no production code changed. |

## What changed

golden/known-differences.txt (3 reasons), golden/stop-check/*/setup.json (7 `proves`), go-diff.md of agent-routing/{session,empty-object} and grid-env/{discovered,explicit,no-config}, docs/lanes/C-xs-golden-l29-rerecord.md.

## Open items

- The 7 stop-check setup.json files still carry `exit: 2` and `contains_stderr` keys from the Node recorder; the Go replay does not read them. Left alone (the repair brief names only `proves`).
- go-diff.md of agent-routing and grid-env still says "compact agent list" in the context sentence, while go-expected/stdout holds the brain topic index; outside this repair's brief (stderr only).

## NOT VERIFIED on a live screen

Nothing here is a screen.

DONE
