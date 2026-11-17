# P-jobs-land-g1: close the decision-gate gaps of run-jobs and judge-land

Docs only. No code changed and nothing built. The two docs are
`docs/research/run-jobs-launcher.md` ("RJ") and `docs/research/judge-land.md`
("JL"). Line numbers are on this lane's head, after the merge of origin/main.
Both docs still end with the line `DONE` (RJ:688, JL:962).

Checked against the reviewer runbook: one acceptance row per brief step with
its file and line, the walk tests pasted with exit codes, and checks the brief
does not ask for labelled as such.

## Proof table

| Brief step | Gate item | Closed at | Proof |
|---|---|---|---|
| 1 | run-jobs V26/V27 (a), points 1-11; judge-land V26.g / V27.j shared rows | RJ:603-633 (section C: 19 points, one owner lane and one set of names each; duplicate lanes merged at RJ:629-633); RJ:28 and JL:23-29 (each doc names what it owns and points to the other) | Names fixed across both docs: deploy approval is the existing `grid approve` accepted control with an `after ... kind: done` edge, and there is no `approved` kind (RJ point 14; JL A.2). The deploy command Job is `kind: command` running `grid land T-n` (point 15). The wait Job is `kind: wait` with `every`/`timeout` and posts `waited` (point 16). Allow exactness is held by lint `job.allow-exact` plus `allow_dropped` (point 10). The denials field is `permission_denials` (point 12). The attempt-ref push is create-only, giving `ref_exists` (point 4). A secret hit gives exit reason `secret_found` plus a rework Job with `carry` (point 5). Submit is `submit {repo, ref, sha, base_sha, commits}` (point 3). Cleanup takes two steps (point 9). Fetch is narrow with prune, plus a full prune on close and hourly (point 6). Job shape is `kind:`, not `run:` (point 17). The blocked restart uses the blocked attempt's ref as base (point 13). A `git grep` of the losing names (`kind: approved`, bare `denials`, `run: command`, "exit is done with code", lane 7.x/8.x refs) finds none in either doc. |
| 1 | C points by line | point n sits at RJ:608+n: gitref :609, secrets check :610, submit :611, attempt-ref push :612, secret hit :613, fetch :614, integrate :615, land and refs :616, cleanup :617, mode and allow :618, approval records :619, denials :620, blocked and restart :621, deploy dependency :622, deploy command Job :623, `wait` Job :624, Job shape :625, mechanical attempts :626, order :627 | JL side: A.2 :576, A.4 :609, A.5 :623, A.6 :642, B.2 :699, B.3 :704, B.4 :712, B.5 :720, B.9 :791 (pointers or the shared names). The `grant: <approval record>` name is used at RJ:192, RJ:485 and JL:630. |
| 1 | owner of the `command` and `wait` Job kinds (point 17) | JL:901 (JL lane 1 row 5, V26) | Schema and lint accept both kinds and both survive the round-trip. The `waited` schema is listed only in RJ lane 1 (RJ:643); JL lane 9 (JL:909) points to it. |
| 1 | duplicate lanes | RJ:656 (lane 14 takes JL 7), RJ:657 (lane 15 takes JL 8), JL:907-908 (stubs), JL:904 (lane 4 rows (h) fixed, (l) added), JL:909 (lane 9 owns the deploy Jobs) | Row map at RJ:629-633. Acceptance maps updated: RJ:669-671 and JL acceptance map ("RJ n (m)" references). |
| 2 | run-jobs V23 (b) | RJ:23 (row table), RJ:434 (Q8 inbox body) | The clause "the hosted web UI belongs to the app and grid-sync, not the plugin" appears in both places. |
| 3 | run-jobs lane 13 V-numbers (c) | RJ:655 | V5-V10, V18, V20-V24, V26, V27 written out. |
| 4 | judge-land V11.e | JL:903 (lane 3 rows 8-11), JL:231-237 (each test names its row) | FreshHome is row 8, ported with a stated change (the default lives in the note, not in Go). ReviewerWaitsOnBuilder is row 9 (three cases). OrchestratorJob is row 10 (two cases). DefaultNeverBeatsAWorkflow is row 11 (five cases). |
| 5 | judge-land V12.f | JL:480 (Q4 row "ported: drain", with the reason), JL:481 (the process.go output drain, not a land.sh rule), JL:906 (lane 6 row 8) | "Drain" is not in land.sh. The row says so and gives the reading (from the land.sh header :11-16, :373, :444-463) as one Abdu confirms (JL open items). The drain holds across tasks only: a land Job lands its task's whole head (B.8, JL:764), so a later land of the same task waits for the rework to be integrated. Row 8 uses three different tasks. |
| 6 | judge-land V12.c (question for Abdu) | JL:861-887 (section "Open for Abdu: V12.c", options A and B side by side, with what each changes per lane); JL:74 and the Rejected bullet marked open | Not settled here. |
| 7 | commits, report | one commit per step (3c444594, 74344a37, bdc0beb9, 0878b4cc, 9ed045c1, c57de9fb); the merge of origin/main; this file (fae31def); review fixes (c36bea62: JL lane 1 row 5, drain across tasks, one grant name, create-only wording) | Commit messages carry no attribution. |

## Checks

| Check | Result |
|---|---|
| `git fetch origin && git merge origin/main` | merged, no conflict (ort); the earlier merge of origin/lane-P-judge-land-a7 also had no conflict |
| `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli  0.047s` (rerun on c36bea62) |
| `go test -count=1 ./internal/job/lint/` | exit 0: `ok  github.com/gridconsole/grid-plugins/internal/job/lint  0.192s` (rerun on c36bea62) |
| stale-name `git grep` on c36bea62 (`kind: approved`, `run: command`, `approval post id`, `carry: a`, `only assignment`, `base, commits`, `write-once`, `grid-T-12`, lane 7.x/8.x, `green`/`red`/`timeout` as exit reasons) | no hits |
| `go vet`, full `go test ./...`, cross-builds | not run, not asked (no code changed) |
| build | not run, the brief forbids it |
| private paths, hosts, emails in the two docs and this file | none (git grep) |

## Open items

- run-jobs gate (d), V20 home source with P-brain-board: not in this brief, not closed.
- V12.c waits for Abdu (JL:861). The lane plan is written for option A.
- The drain reading (JL:480) needs Abdu's confirmation.
- The optional V12.e note (whether a test that is flaky again is deleted like one that is red again) is asked in the same question (JL "Open for Abdu").
