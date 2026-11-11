# M-plugin-doctor-deny-block-followups

Base: lane-C-ex-doctor-deny-block. Fix: one cause is one failing check; the deny block says it was not checked when the allow block already fails for the same settings file.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. `git merge --no-edit origin/main` first | HOLDS | Merge made by the `ort` strategy, exit 0, no conflicted file (74 files from main merged: docs, golden/brain-recall removal, hook and cli tests). |
| 2. One cause, one failing check; deny says not checked and why | HOLDS, with a flag | `internal/doctor/installation.go` `checkDenyBlock`: allow-block reason (missing file, dangling symlink, directory, invalid JSON, unreadable) gives ok `deny block` with detail `not checked: <reason>; the allow block reports it`. A readable file holding none of setup's rules (`{}`, the missing_rules case) gives `not checked: the Claude settings hold none of setup's rules; the allow block reports it`. Flag: see Open items 1. I did not stop on the stop clause, because C-ex-doctor-deny-block row 3 says only "the deny block reasons equal the allow block reasons" and does not say the deny check must fail; the sentence that does ("a settings read failure gives the same reasons and fixes as the allow block") is in "What to build", not in a row. |
| 3. TestDoctorAllowBlockFollowups passes unchanged | HOLDS | `make test "GOFLAGS=-count=1 -v -run=TestDoctorAllowBlockFollowups"` PASS: symlink, dangling_symlink, directory_target, unreadable, missing_home, invalid_JSON, missing_rules, missing_file (all `--- PASS`). `git diff` shows the file untouched. |
| 4. Lane tests still pass, no assertion dropped; a real missing or wrong deny block still fails | HOLDS | `go test -count=1 ./internal/doctor/ -run TestExDeny -v`: 10 `--- PASS`. In `TestExDenyBlockSameReasonsAsAllow` the reason equality is kept (deny detail must be `not checked: ` + allow detail + `; the allow block reports it`), and the old both-must-fail assertion is replaced by allow fails / deny does not / exactly 1 failing check (this is the intended change of row 2). Deny still fails in `TestExDenyBlockMissingRule` (one rule removed), `TestExDenyBlockAllDenyRulesRemovedFails` (new: allow rules present, all deny rules removed, exit-relevant `rep.OK` false, fix "Run grid setup"), and in the cli test `ex_deny_block_cli_test.go` (one deny rule removed, non-zero exit). |
| 4b. New tests fail on base, pass on lane | HOLDS | With the fix stashed: `--- FAIL: TestDoctorAllowBlockFollowups/dangling_symlink ... doctor_allow_followup_test.go:98: found=true failures=2 report.OK=false` (same for directory_target, unreadable, invalid_JSON, missing_rules, missing_file); `--- FAIL: TestExDenyBlockSameReasonsAsAllow/missing_file ... ex_deny_block_test.go:165: allow should fail with a reason and deny should not: allow {... OK:false ...} deny {Name:deny block OK:false Detail:Claude settings file is missing ...}`. Same lines for unreadable_file and invalid_JSON. Pass on the lane. |
| 5. `go test -count=1` of internal/cli, internal/doctor, internal/settings through make | HOLDS | `make test "GOFLAGS=-count=1"` exit 0: `ok internal/cli 71.719s`, `ok internal/doctor 8.805s`, `ok internal/settings 2.083s`; no FAIL line in the whole module. Importers (go list): settings is imported by account, cli, doctor, guard, hook; doctor only by cli; I changed only doctor, so cli is the affected importer (passes). Goldens: none regenerated, `git status` shows no golden diff (the golden fixtures are healthy or skipped cases, whose text did not change). |
| 6. `make lint` exit 0 | HOLDS | `make lint` (go vet ./...) exit 0, no output after the command echo. `gofmt -l internal/doctor` empty. |
| 6b. Why the lane's own run did not catch it | see below | The lane did catch it: its report (docs/lanes/C-ex-doctor-deny-block.md:15, :34) lists `TestDoctorAllowBlockFollowups` as a failing existing test and row 6b NOT DONE; it left the test unedited on purpose and it was landed in Pint83 with that open item. |
| Windows build | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0. |
| New tests: no real `~/.claude` | HOLDS | New tests use `exDenyEnv`/`exDenyWrite` under `t.TempDir()`. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Doctor still only reports; nothing added that blocks an agent. |
| No attribution lines in commits | HOLDS | One commit message, one sentence, no trailer. |
| Tests named *RealPlugin* | not needed | No row uses one. |

## What changed

- `internal/doctor/installation.go`: `checkDenyBlock` reads the allow status as well. Allow reason set: deny is ok with `not checked: <reason>; the allow block reports it`. No setup rule present at all (allow and deny both empty): ok with `not checked: ... none of setup's rules ...`. Otherwise as before (deny reason, missing rules, all present).
- `internal/doctor/ex_deny_block_test.go`: `TestExDenyBlockSameReasonsAsAllow` updated; two new tests, `TestExDenyBlockNoSetupRulesIsOneFailure` and `TestExDenyBlockAllDenyRulesRemovedFails`.
- Not touched: `claude_status.go` (no change needed), `doctor_allow_followup_test.go`, goldens, the C-ex cli test.

## Open items

1. Behaviour change for the owner to confirm: with an unreadable, missing or invalid settings file doctor now shows one failure (the allow block), and the deny block line is ok and says it was not checked. The brief C-ex-doctor-deny-block row 3 said "the deny block reasons equal the allow block reasons" and the "What to build" line said "same reasons and fixes"; the reason text is kept inside the deny detail but the deny line no longer carries a Fix and is not a failure. This is what M-plugin-doctor-deny-block-followups row 2 asks for. If the owner wants two failures, TestDoctorAllowBlockFollowups must change instead.
2. A file where every allow and every deny rule are missing (for example `{}`) shows a single failure; when only some allow rules are missing and the deny rules are missing too, both fail (both have fix "Run grid setup"). I chose this so a really removed deny block in a file that still has setup rules is never hidden.

## NOT VERIFIED on a live screen

Nothing here needs a live screen; the doctor text is checked through `grid doctor` in the cli harness only.
