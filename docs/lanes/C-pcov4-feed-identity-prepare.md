# C-pcov4-feed-identity-prepare: proof table

Branch lane-C-pcov4-feed-identity-prepare. One new file: internal/feed/feed_identity_prepare_test.go (package feed, no build tag, no production code).

| Row | Result |
|---|---|
| 1 Coverage before/after | HOLDS. Before, untouched clone: `go test -count=1 -coverprofile=<file> ./internal/feed/` exit 0, `total: (statements) 83.4%`. After: same command exit 0, `total: (statements) 84.6%`. 84.6 > 83.4. (Profiles were written to files under /srv/gridwork/tmp/hand because a mktemp or mkdir in the call was refused by the sandbox.) |
| 2 MachineID not a ULID | HOLDS. TestIdentityPrepareMachineIDNotULID asserts the exact text "machine must contain a ULID", an empty id, and the machine file bytes unchanged. Code: identity.go:36-39. |
| 3 EnsureTaskID, task.md a directory | HOLDS. TestIdentityPrepareEnsureTaskIDDirectory asserts "task.md must be a regular file" (identity.go:140-142); `t.Skip` when runtime.GOOS is "windows". |
| 4 WithTaskID, six subtests | HOLDS. TestIdentityPrepareWithTaskID: "id not a ULID" (:172), "conflicting id" (:176-178), "same id returns the data" (:180, bytes equal), "frontmatter never closed" (via taskID :100-102, leaves at :182-184), "nonsense ulid" -> exactly "invalid task ulid" (:196-199), "CRLF without ulid" -> whole result `---\r\ntitle: x\r\nulid: <id>\r\n---\r\nbody\r\n` (:185-193). |
| 5 prepare | HOLDS. TestIdentityPreparePrepare, reached through Store.SaveState (fold.go:56 -> :60, the first caller that has no earlier schema check; Post validates the record before prepare, so TaskID "x" would fail there first). TaskID "x" -> "task id must be a ULID" (store.go:56-58); TaskDir a regular file -> "task is not a folder" (:63-65). Neither creates a feed folder (asserted by os.Stat on Dir()). prepare 73.3% -> 86.7%. |
| 6 Unreadable body | HOLDS. TestIdentityPrepareUnreadableBody: Post a knowledge record with text strings.Repeat("x", MaxLine); asserts the writer file line holds `"ref":"bodies/<X>.json"`; the retry with Input.ID = X and the same writer first returns the first record's ID and Seq (body intact); then bodies/X.json is overwritten with `{` and Post again returns exactly "body of X is unreadable" (store.go:447-450) with the writer file bytes unchanged. stored 81.8% -> 90.9%. Runs on the lane base (13b11fe1 line numbers match the brief). |
| 7 Checks | HOLDS. `go test -count=1 ./internal/feed/` exit 0 (ok, 84.6%). `GOOS=windows go vet ./...` exit 0. `GOOS=darwin go vet ./internal/feed/` exit 0. `GOOS=windows go test -c -o /dev/null ./internal/feed/` exit 0. `make lint` exit 0 (it runs `go vet ./...`). `gofmt -l internal/feed` prints nothing. |

## Rules

- Tests only, no non-test file edited, no existing test edited, nothing deleted: HOLDS (`git diff --stat origin/main..HEAD` is the new test file plus this report).
- Helper prefix: one helper, `identityPrepareErr`; tests all start `TestIdentityPrepare`: HOLDS.
- No guard, gate, cap or limit added: HOLDS (no production code).
- Importers of internal/feed (go list: cli, hook, job/jobcmd, lease, mcp, orchestrator, question, task, usage): `go test -count=1` on all nine exit 0. No production code changed, so this is a safety check only.
- New tests fail on base and pass on the lane: NOT APPLICABLE in the usual sense. The brief is tests-only for behaviour already in main, so the new tests pass on main plus the file; they would fail only if the cited refusals were removed. I did not edit production code to show a failing line, because the brief forbids it.
- No chmod, mkfifo or unix-only syscall in the new file: HOLDS (imports bytes, os, path/filepath, runtime, strings, testing).
- Windows and macOS: compiled only. A run needs a Windows host: `go test ./internal/feed/ -run IdentityPrepare` (the directory subtest skips there).

## Differences from the brief

- Row 5: the brief says to reach prepare through "the exported Store method that calls it first". Post validates the record (and so the task id) before prepare, so I used SaveState, which calls prepare first (fold.go:60).

## What changed

Added internal/feed/feed_identity_prepare_test.go (158 lines) and this report.

## NOT VERIFIED on a live screen

Nothing here is UI. Windows and macOS test runs need those hosts (compiled and vetted only).
