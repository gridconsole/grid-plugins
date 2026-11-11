# M-plugin-held-f-e-ready2

Merges `origin/lane-M-plugin-held-f-e` onto plugin main d0400870. The 11 Nov landing batch skipped it on a conflict in `internal/cli/worker.go`; this lane only makes it fit. No new behaviour, no test written or edited.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. `git merge --no-edit origin/lane-M-plugin-held-f-e`, conflicted files and how each was resolved | HOLDS | Command exit 1 with one conflict. Conflicted file: `internal/cli/worker.go` (the `orchestrator.Stream{...}` literal in `runWorkerNext`, now lines 152-166). Main's side added `Model: func(dir, role string) (string, string, error) { return workerSpawnModel(env, dir, role) }` (commits 5e7a377c "Print the role's model and effort on every spawn line..." and 9b353e3c "Route the worker's model lookup through settingsStore..."; 5e7a377c also added the `Stream.Model` field in `internal/orchestrator/stream.go`). The lane's side added `ReceiptPending: migrate.ReceiptPending` and `ReceiptMove: func(dir string, w feed.Writer) error {...}` at the same spot (E-receipts-e2e3, 8ec0c09f). Resolution: kept both. The `Model:` line stays as on main, the lane's comment and the two receipt fields follow it; no other line of the file differs from main (`git diff --stat HEAD^1 HEAD -- internal/cli/worker.go`: 8 insertions, 0 deletions). The other files merged automatically: `contract/commands.json`, `internal/cli/orchestrator.go`, `internal/orchestrator/stream.go` (the `Model` field and the two receipt fields sit side by side in the struct, `stream.go:63-73`). `go build ./...` exit 0 after the resolution. |
| 2. `git log --oneline main..HEAD` contains the merge | HOLDS | First line of the log: `d592d2bc Merge origin/lane-M-plugin-held-f-e into main, keeping the Stream Model hook and the receipt move together.` Below it the lane's own commits (`a48c4828 Add the lane proof table for the F and E held-lane landing.`, `816dd6df`, the two inner merges `a0d4e1c3` and `a5b57fab`, and the E and F lane commits). |
| 3a. `go test -count=1 ./...` | HOLDS | exit 0: every package line `ok` or `no test files`, no FAIL (`internal/cli` 74.967s, `internal/orchestrator` 63.204s, `internal/task` 6.956s, `internal/migrate` 29.718s, `internal/plan` 1.769s). |
| 3b. `make lint` | HOLDS | exit 0 (`go vet ./...`; `gofmt -l internal` also prints nothing). |
| 3c. `GOOS=windows go vet ./...` | HOLDS | exit 0, no output. |
| 3d. `GOOS=darwin go vet ./...` | HOLDS | exit 0, no output. |
| 4. If a test changed, explain it | HOLDS | My resolution changes no test file: the only file I edited is `internal/cli/worker.go`, and the tests passed unchanged on the merge. The test edits that ride in with the merge are the lane's own, already reviewed and accepted, and are explained there: `docs/lanes/M-plugin-held-f-e.md` row 6 (`internal/plan/store_items_test.go`: six `s.Migrate(x)` became `s.Migrate(x, nil)` for the lane's new signature, `internal/plan/migrate.go:12`) and `docs/lanes/E-receipts-e2e3.md:53` (the approval fixtures rewritten under verdict 12, `docs/lanes/E-verdict-12-receipts.md`). Deleted test `internal/task/plan_job_fallback_test.go` (`TestTaskWithoutPlanJobFallsBackToApprovalJSON`): removed by the owner's direction in `docs/lanes/E-verdict-12-receipts.md:66`; the behaviour is still covered by `TestTaskWithoutPlanJobIsNotApproved` (`internal/task/approve_job_result_test.go:126`). `TestApprovedRejectsAnotherGate` is covered by `TestAnApprovalFileApprovesNothing` (`internal/task/gates_test.go:137`). |
| Rule: new tests fail on base and pass on lane | HOLDS (none new) | This lane writes no test. The lane's own new tests were proved failing on their bases in its reports (`docs/lanes/M-plugin-held-f-e.md`, rule row) and pass in 3a. |
| Rule: tests of changed packages and every importer | HOLDS | The whole module ran in 3a (`go test -count=1 ./...`), which includes `internal/cli`, the only package I edited, and every package that imports it. |
| Rule: no guard, gate, cap, hold or blocking hook added | HOLDS | The resolution adds no guard: it only keeps two struct fields side by side. |
| Rule: remove nothing that is not on the removal lists | HOLDS | The resolution removes nothing (8 insertions, 0 deletions against main in `worker.go`). The removals that come with the merge are the lane's (F-cli-drop-stubs verbs, E-receipts-e2e3 approval.json fallbacks), already accepted. |
| Rule: small commits, one-sentence messages, no attribution lines, no personal paths or emails | HOLDS | One new commit, d592d2bc, one sentence, no Co-Authored-By or session line. The report names no personal path, host or address. |
| Rule: no push, no claude/curl/background server | HOLDS | Nothing pushed; none of those run. |

## What changed

- `internal/cli/worker.go`: conflict hunk only (the `Stream` literal keeps `Model` from main and `ReceiptPending`/`ReceiptMove` from the lane).
- Everything else is the already accepted lane M-plugin-held-f-e (F-cli-drop-stubs plus E-receipts-e2e3).
- This file.

## Open items

- `contract/commands.json` was not regenerated here (the lane's report row 2 says a regen is expected to show no diff; the orchestrator can run `make generate` at landing). `go test ./internal/cli` passes `TestContractCommands*`, which compare the file with the generator's bytes.
- The three notes of `docs/lanes/M-plugin-held-f-e.md` stand: the app reads approval.json at `internal/slices/gates/model.go:477` (app follow-up), the answer schema change needs the cross-repo set at landing, and the removed `gate != "plan"` refusal is an owner decision.

## NOT VERIFIED on a live screen

No UI row. Same as the lane: the dev UI approve button against a real worker, and the app's replay of the plan-digest transcript, need a live check.
