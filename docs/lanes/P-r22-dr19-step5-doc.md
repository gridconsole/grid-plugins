# P-r22-dr19-step5-doc

Step 5 of "Round 20 check (F4a)" in docs/decisions/D-r19-person-cancel-holder.md now names the worker.log `exit_superseded` line, not a Feed worker-exit. Docs only, no code.

| Row | Result |
|---|---|
| 1. `git diff main -- docs/` shows only step 5 | HOLDS. `git diff main -- docs/` shows one hunk, line 217 of the decision doc (before this lane doc was added). |
| 2. store.go:143-144 is still the holder check | HOLDS. Line 143: `if value(lease.Holder) != w.Worker \|\| lease.Lease != w.Lease \|\| lease.Fence != w.Fence \|\| lease.Gen != w.Gen {`. Line 144: `return fail(ErrStaleLease)`. |
| 3. Tests that pin the document's text | HOLDS. No test pins it. `grep -rn "Its agent exits"` hits only the doc. `grep -rn D-r19-person-cancel-holder --include=*_test.go` hits comments in r18_person_job_cancel_test.go and r20_cancel_request_cli_test.go, and no test reads the file. |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0. |
| Walk test `go test -count=1 -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' ./internal/cli/` | HOLDS, exit 0. Output: `ok  	github.com/gridconsole/grid-plugins/internal/cli	0.038s`. |
| Merge of origin/main, lint, Windows and macOS builds | not run, not asked. The change is one doc line and no Go file. |

NOT VERIFIED on a live screen: nothing here needs a screen.
