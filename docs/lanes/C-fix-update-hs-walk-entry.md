# C-fix-update-hs-walk-entry

Stacked on lane-M-ip-update-act-repo-write. Plugin main 99c11b7a is merged in (merge commit 8347899a); the one fix commit is 13706638.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1a. After the merge, before the entry, the walk fails with the quoted line | HOLDS | `go test -count=1 -run TestHsIpWalksEveryCommand ./internal/cli/` exit 1: `hs_inprocess_test.go:42: command "update" has no in-process answer in hsIpTable` and `hs_inprocess_test.go:47: walked 39 commands, 38 table entries` |
| 1b. After the entry, the walk passes | HOLDS | same command, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli 0.007s` |
| 2. `go test -count=1 ./internal/cli/` | HOLDS | exit 0: `ok .../internal/cli 26.718s` |
| 3. Diff holds only the file with `hsIpTable` | HOLDS | `git diff --stat 8347899a..HEAD`: `internal/cli/inprocess.go \| 3 +++` (3 lines added, nothing else) |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| 4. `make cross` | HOLDS | exit 0; darwin/arm64 and windows/amd64 vet and test-compile all `ok` |
| Importers of internal/cli tested | HOLDS | `go list` importers: `cmd/grid`, `lib`. `go test -count=1 ./cmd/grid/ ./lib/` exit 0 (`cmd/grid` has no test files; `lib` ok 26.431s) |
| No product code changed, no other entry changed, walk not weakened | HOLDS | the diff is one new map entry `"update": {}` and a comment, at internal/cli/inprocess.go:164-167. `hs_inprocess_test.go` is untouched. |
| Merge resolved without a guess | HOLDS | the merge was automatic (`ort`), no conflict |
| No push, no guard, no attribution lines | HOLDS | nothing pushed; the commit messages carry no attribution lines |

## What changed

`hsIpTable` in `internal/cli/inprocess.go` gets `"update": {}`: binary-only is not set, and there are no `subs`, which is the shape of `plan`, `person` and `migrate`. This answer follows from `runUpdate` (internal/cli/update.go:25), which has the subverbs draft, edit, discard, skip and send. All of them act on the project's update file in the Grid home. Send only marks the update sent and mails nothing. Neither `update.go` nor `internal/updates` uses a process id, a signal, a child process, the host's environment or stdin (grep for `os/exec|Getpid|Getppid|os/signal|Environ|Stdin` found nothing). So `lib.Run` may run it in a host process.

I did not use a `subs` map. The walk test accepts a plain verb, and with `subs` set an unlisted subverb would answer false, so one more table entry would be needed for each future subverb.

## NOT VERIFIED on a live screen

Nothing here touches a screen. The whole-module test and the cross-repo set are left to the orchestrator at landing.
