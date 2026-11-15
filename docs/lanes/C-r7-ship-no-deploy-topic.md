# C-r7-ship-no-deploy-topic

A ship answer on a project with no deploy topic records a plain outcome instead of an openat error (eval r7 F6).

Base: main d5022830. `git fetch origin && git merge origin/main`: Already up to date.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. No knowledge/deploy folder: AfterShip returns no error, one "nothing to land" line, task closed, no openat in history | HOLDS | `go test -count=1 -run TestR7 ./internal/land/` exit 0; `internal/land/r7_no_deploy_topic_test.go:63` subtest "no knowledge/deploy folder". Red on base (below). |
| 2. knowledge/deploy holds only TOPIC.md: same result | HOLDS | same test, subtests "knowledge/deploy holds only TOPIC.md" and "entry with no lands block". Red on base (below). |
| 3. Malformed deploy entry: still "land failed", task not closed | HOLDS (pin) | `TestR7ShipAnswerMalformedDeployTopicFails`, `r7_no_deploy_topic_test.go:101`. Passes on base and lane by design: it pins that a broken topic never passes. |
| 4. `grid land T-01` without a deploy topic exits non-zero, stderr "has no deploy topic", no openat | HOLDS | `TestR7GridLandNoDeployTopicSaysSo`, `r7_no_deploy_topic_test.go:118` (drives `cli.Run([]string{"land","T-01"})`; exit 1, stderr `land: project sample has no deploy topic (knowledge/deploy/<name>.md with a lands: block)`). Red on base (below). |
| 5. Existing tests unedited pass; `go test -count=1 ./internal/land/ ./internal/cli/` and `make lint` exit 0, unpiped | HOLDS | `go test -count=1 ./internal/land/ ./internal/cli/` exit 0 (land 0.862s, cli 34.754s); `make lint` exit 0. No existing test edited (`git diff origin/main --stat` lists only the three files below plus this doc). |
| Approve behaviour unchanged | HOLDS (pin) | `TestR7ApproveUnchangedWithoutDeployTopic`, `r7_no_deploy_topic_test.go:146`: missing folder is ordinary approval with no lands hash; a folder with no lands entry is still refused. Passes on base and lane. |
| Tests of changed packages and importers | HOLDS | `go list` importers of internal/land: `internal/cli`, `internal/land`. Both green above. |
| Windows build | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=windows go vet ./internal/land/` exit 0 (compiles the test files) |
| macOS build | HOLDS | `GOOS=darwin go build ./...` exit 0 |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`); `go test -count=1 ./internal/job/lint/` exit 0 |
| New tests red on base, green on lane | HOLDS | base output below; lane `go test -count=1 -run TestR7 ./internal/land/` exit 0 |
| No guard, gate, cap, hold or blocking hook added | HOLDS | the change removes a failure; no new refusal |
| No attribution lines, no personal paths | HOLDS | `git log origin/main..HEAD` |
| Whole-module test and cross-repo set | not run, not asked | run by the orchestrator at landing |
| realplugin guest walk tables | not run, not asked | app-repo check; this lane is plugin-only |

## Red on base

Production files stashed (`git stash push internal/land/land.go internal/land/ship.go`), the new test file kept, no shim:

```
--- FAIL: TestR7ShipAnswerNoDeployTopicClosesTask (0.00s)
    --- FAIL: TestR7ShipAnswerNoDeployTopicClosesTask/no_knowledge/deploy_folder (0.00s)
        r7_no_deploy_topic_test.go:77: AfterShip = ran true, deploy topic: openat projects/sample/knowledge/deploy: no such file or directory; want ran and no error; history:
    --- FAIL: TestR7ShipAnswerNoDeployTopicClosesTask/knowledge/deploy_holds_only_TOPIC.md (0.00s)
        r7_no_deploy_topic_test.go:77: AfterShip = ran true, deploy topic has no lands rule; want ran and no error; history:
    --- FAIL: TestR7ShipAnswerNoDeployTopicClosesTask/knowledge/deploy_entry_with_no_lands_block (0.00s)
        r7_no_deploy_topic_test.go:77: AfterShip = ran true, deploy topic has no lands rule; want ran and no error; history:
--- FAIL: TestR7GridLandNoDeployTopicSaysSo (0.00s)
    --- FAIL: TestR7GridLandNoDeployTopicSaysSo/no_knowledge/deploy_folder (0.00s)
        r7_no_deploy_topic_test.go:135: grid land stderr "land: deploy topic: openat projects/sample/knowledge/deploy: no such file or directory\n", want it to name "project sample has no deploy topic (knowledge/deploy/<name>.md with a lands: block)"
    --- FAIL: TestR7GridLandNoDeployTopicSaysSo/knowledge/deploy_holds_only_TOPIC.md (0.00s)
        r7_no_deploy_topic_test.go:135: grid land stderr "land: deploy topic has no lands rule\n", want it to name "project sample has no deploy topic (knowledge/deploy/<name>.md with a lands: block)"
FAIL
FAIL	github.com/gridconsole/grid-plugins/internal/land	0.016s
```

## What changed

- `internal/land/land.go`: `ErrNoDeployTopic` sentinel. `rule` returns it, with the text `project <p> has no deploy topic (knowledge/deploy/<name>.md with a lands: block)`, when knowledge/deploy is missing or holds no entry with a `lands:` block. For the missing folder the error still unwraps to the not-exist error, so `Approve` (approval.go:37) reads it exactly as before; a folder with no lands entry does not match os.ErrNotExist, so Approve still refuses it as before. Unreadable or malformed entries return their old errors.
- `internal/land/ship.go`: AfterShip, on `ErrNoDeployTopic` with no execution, appends `ship answered: <job> approved; no deploy topic for project <p>, nothing to land` and closes the task with Note `approved after <job>; no deploy topic`. History therefore holds two "ship answered" lines for the Job: markShip's once-marker (`<job> accepted; grid land runs`, unchanged, the second-call guard depends on it) and this outcome line. No other code reads either line (grep of internal/ and cmd/).
- `internal/cli`: unchanged; `landError` prints the new error text.
- `internal/land/r7_no_deploy_topic_test.go`: new, package `land_test` so row 4 can drive `cli.Run`.

## Open items

- `grid approve` on a project whose knowledge/deploy folder exists with no lands entry (row 2 shape) is refused inside `land.Store.Approve` before AfterShip runs, on base and on the lane (task_write.go:303, orchestrator.go:526); only the refusal text changed, from `deploy topic has no lands rule` to the new sentence. The worker tick path (worker.go:171) reaches AfterShip and now closes the task. Changing Approve is outside this brief; the r7 case (folder missing) is fully covered.

## NOT VERIFIED

Nothing needs a live screen; no UI change.

DONE
