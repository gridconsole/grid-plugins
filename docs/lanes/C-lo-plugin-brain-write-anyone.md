# C-lo-plugin-brain-write-anyone

`grid brain write` lets an owner or a member write a `type: workflow` note (owner answer 3, owner-answers/2026-11-11T2107Z.md). The author is written in log.md as before. Guests and workers stay refused by the verb's role rule.

Base: 73c6e36d (plugin main f6383e8c merged with lane-C-ke-brain-write 66f9c41a). Before the change, ke_write.go:121-126 read `allowWorkflow := env.Role == "" || env.Role == "owner"` followed by the refusal "write workflow notes", and :132 passed `allowWorkflow` to store.Write, which matches the brief.

| Row | Result |
|---|---|
| 1. `go test -count=1 -v -run 'TestLoWaMemberWorkflow' ./internal/braincmd/` | HOLDS, exit 0, `--- PASS: TestLoWaMemberWorkflow`. The test (lo_write_anyone_test.go:47) runs member `bo`: it creates decisions/flow v1 (stdout `created decisions/flow v1 rev <NoteRev>`), replaces it with plain text (`edited ... v2`), then replaces the plain note with workflow text (`edited ... v3`). After each write: the bytes on disk match, stderr is empty, and log.md gains one line matching `\| (created\|edited) \| flow \| person:bo \|`. On the base it fails: `lo_write_anyone_test.go:61: flow as "member" (member "bo"): exit 3, stdout "" (want "created decisions/flow v1 rev 34c991f7a3b58341\n"), stderr "grid brain write: caller role is not allowed to write workflow notes\n"` / `--- FAIL: TestLoWaMemberWorkflow` |
| 2. `TestLoWaGuestWorker` | HOLDS, exit 0, `--- PASS: TestLoWaGuestWorker`. The test (lo_write_anyone_test.go:69) covers roles guest and worker in three cases each: a new workflow note, and a replace of the member-written workflow note with workflow text and with plain text. Each case exits 3 with stderr exactly keWrRoleLine (`grid brain write: caller role is not allowed to write brain entries\n`) and empty stdout; the home is unchanged per keWrSnapshot/keWrSame; log.md gains no line. On the base it fails at setup, because the member write is refused: `lo_write_anyone_test.go:72: flow as "member" (member "bo"): exit 3, ... stderr "grid brain write: caller role is not allowed to write workflow notes\n"` / `--- FAIL: TestLoWaGuestWorker` |
| 3. `TestLoWaSessionAuthor` | HOLDS, exit 0, `--- PASS: TestLoWaSessionAuthor`. The test (lo_write_anyone_test.go:102) has a member with member `""` create a workflow note: exit 0, and the log line matches `\| created \| flow \| session:[0-9a-f]{24} \|` with no `person:`. On the base: `lo_write_anyone_test.go:105: flow as "member" (member ""): exit 3, ... "grid brain write: caller role is not allowed to write workflow notes\n"` / `--- FAIL: TestLoWaSessionAuthor` |
| 4. `go test -count=1 ./internal/braincmd/ ./internal/brain/` | HOLDS, exit 0 (`ok .../internal/braincmd 3.973s`, `ok .../internal/brain 3.299s`). `--- PASS: TestKeWrCommand` without the old :213-218. |
| 4. diff names only the four Touches files | HOLDS against the lane base: `git diff 73c6e36d --stat` lists exactly docs/decisions/D-ie-brain-write-decision.md, internal/braincmd/ke_write.go, internal/braincmd/ke_write_test.go and internal/braincmd/lo_write_anyone_test.go (4 files, +125 −33), plus this report, which the runbook requires. The brief's literal `git diff origin/lane-C-ke-brain-write --stat` also lists 286 files from main (contract/, evals/, internal/brain/ke_rename.go, ...): 66f9c41a is not on top of main f6383e8c, and the lane base merges both. Limited to `internal/braincmd internal/brain docs/decisions/D-ie-brain-write-decision.md`, the only lane files are the four above; internal/brain/ke_write.go and internal/brain/ke_write_test.go do not appear (unchanged). |
| 4. `git grep -n 'write workflow notes' -- internal/braincmd` | HOLDS, exit 1, no output (not even a test line) |
| 5. `git merge main` | `Already up to date.` (main = origin/main = f6383e8c) |
| 5. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0; `-v` shows `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList` |
| 5. `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0, `ok .../internal/job/lint` |
| 6. `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| 6. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64: vet and test compile) |
| Importers tested | `go list -f '{{.ImportPath}}: {{join .Deps " "}}' ./... \| grep internal/braincmd` → cmd/grid, internal/braincmd, internal/cli, lib (direct importer: internal/cli). `go test -count=1 ./internal/braincmd/ ./internal/brain/ ./internal/cli/ ./cmd/grid/ ./lib/` HOLDS, exit 0 (cli ok 43.6s, lib ok 10.9s, cmd/grid has no test files) |
| Walk tests (plugin) | HOLDS: the two rows 5 above, run after the last code commit |
| New tests fail on base, pass on lane | HOLDS: the base failures are pasted in rows 1-3 |
| Deleted test lines | ke_write_test.go: the four member refusals at :213-218 (with `wfPerson`) and `keWrWorkflowLine` at :130 are gone. That behaviour was removed by owner direction (owner answer 3, owner-answers/2026-11-11T2107Z.md; recorded in docs/decisions/D-ie-brain-write-decision.md (f) question 3). The member write that replaces them is covered by TestLoWaMemberWorkflow. Guests and workers on a workflow note are still covered by TestKeWrCommand (`guest workflow note`, `worker workflow note`) and TestLoWaGuestWorker. |
| No guard, gate, cap or hold added | HOLDS: one gate removed (ke_write.go old :121-126 and :137-138), none added. The verb's RevertAllowed check at ke_write.go:92 is unchanged. |
| Names | HOLDS: new helpers are loWaHome, loWaAuthor and loWaWrite; tests are TestLoWa*. The file reuses keWrNew, keWrBytes, keWrWorkflow, keWrLog, keWrSnapshot, keWrSame and keWrRoleLine, and the package compiles, so no names clash. |
| store untouched | HOLDS: internal/brain/ke_write.go, store.Write's signature and internal/brain/ke_write_test.go are unchanged. The command passes `true` at ke_write.go:128. |
| No attribution lines, no personal paths | HOLDS: commit messages are one sentence each; no host, home path or email in any file |
| *RealPlugin* tests | not applicable: no row names one |
| Skill go-conventions | not available in this sandbox's skill list; `gofmt -l internal/braincmd` prints nothing, `make lint` exit 0 |

## What changed

- internal/braincmd/ke_write.go: the owner-only workflow check is gone (old :121-126); store.Write gets `true` (now :128); the ErrWorkflowNote case is gone (old :137-138); the exit comment at :54 reads "3 role refused (the verb)".
- internal/braincmd/ke_write_test.go: `keWrWorkflowLine` and the four member workflow refusals with `wfPerson` are gone.
- internal/braincmd/lo_write_anyone_test.go: new, with TestLoWaMemberWorkflow, TestLoWaGuestWorker and TestLoWaSessionAuthor.
- docs/decisions/D-ie-brain-write-decision.md: the role rule (:27), table rows (:33-34), steps 3 and 6 of (d) (:97, :100) and question 3 (:133) now say owner or member, citing answer 3. The exit-3 "write workflow notes" refusal row is dropped from (b). Cases 8 and 9 now expect exit 0, the note written, and an audit line naming the member. Three lines outside the brief's list also stated the owner-only rule, and I aligned them so the doc does not contradict itself: point 3 (:17), case 10 (:74, app route member → 200), and the row summary (:91).

## Open items

- The app half (decision doc case 10 / table row :34): app main has no knowledge write route yet (brief: no caller of `grid brain write` on app main). The future app brief should follow the updated case 10.

NOT VERIFIED on a live screen: nothing UI; the lane is CLI only.

DONE
