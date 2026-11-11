# M-plugin-held-f-e-receipts: stopped at call 27, no fix built

Head is lane-M-plugin-held-f-e-ready2 (c90f97d8) plus this doc. No plugin code and no testdata changed. Two things stop the brief: the shell would not run the row 1 command, and the brief's premise conflicts with the owner direction the lane was built on. Both need a decision, so I built nothing that reverses them. I read /srv/gridwork/tmp/hand/runbook-reviewer.txt and checked the lane against it.

## Proof table

| Row / rule | Status | Evidence |
|---|---|---|
| 1. Build the lane plugin | HOLDS | `go build -o dist/grid ./cmd/grid` exit 0. |
| 1. Fresh app clone at main, run `GRID_PLUGIN_BIN=<lane>/dist/grid go test -count=1 -run 'RealPluginApprove\|RealPluginDigestReplay\|RealPluginGate' ./...`, paste the failing lines | NOT DONE | The shell allow list refuses a command that starts with `GRID_PLUGIN_BIN=` (only `GOOS=` and `CGO_ENABLED=` may precede `go`). Refused twice: once as `cd app && GRID_PLUGIN_BIN=... go test ... \| grep`, once as `GRID_PLUGIN_BIN=... go -C app test ... ./internal/slices/gates`. The gates tests read only `os.Getenv("GRID_PLUGIN_BIN")` (no `-plugin-bin` flag; realplugin_gate_test.go:50, realplugin_digest_test.go:43), so no allowed command passes the binary in. I did not use `env`, `bash -c` or a wrapper script to get round the refusal. The failing lines are therefore the brief's own, not mine. |
| 1. Name the lane commit that dropped the receipt | HOLDS (by reading, not by bisect) | `git log -S"approval.json" origin/main..HEAD -- internal/task/gates.go internal/task/write.go internal/cli/task_write.go` lists 8ec0c09f ("Record an approval only as the result of the approve person Job, move an old approval.json into it, and drop the file fallbacks.") and aa4535b6 (the test fixtures that followed). 8ec0c09f is the one: `Store.Approve` stopped writing approval.json (internal/task/gates.go) and reopen stopped writing it (internal/task/write.go). I did not bisect against the app tests, because I could not run them. |
| 2. `grid approve --json` prints one JSON object with from_state, note, plan_digest, plan_hash, task, to_state, what, when, who | HOLDS for stdout (by reading, not run) | internal/cli/task_write.go:237-242: `--json` prints `task.Event` plus `folder` through `writeJSON`; the lane did not change that. What the lane dropped is the file, not the stdout. See "What the app tests read". |
| 2. Fix: the app tests pass again, keeping the held F and E behaviour | NOT DONE | Needs an owner decision. See "Why I stopped". |
| 3. Row 1 command: 0 fail, 0 skip; then `-run RealPlugin -json ./...` with no fail, no skip, counts pasted | NOT DONE | Same refusal as row 1. |
| 4. Plugin `go test -count=1 ./...` | HOLDS (unchanged head) | exit 0; every package `ok`, packages without tests print `[no test files]`. This is the lane's own state, since I changed nothing in code. |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 4. `GOOS=windows go vet ./...` | HOLDS | exit 0, no output. |
| 4. `GOOS=darwin go vet ./...` | HOLDS | exit 0, no output. |
| 4. Changes to testdata/contract/plan-digest/transcript.json explained | HOLDS (nothing changed by me) | The 104-line change is the lane's (aa4535b6 and the merges). Its `receipt_keys` went from the nine keys to `answer job note plan_digest plan_hash record when who` (the keys of a Job result), the exit of one step went 2 to 1, and the plan-hash-only steps went to `[]`. Whether to revert it depends on the decision below. |
| New tests fail on base, pass on the lane | NOT APPLICABLE | No test written: the fix is not built. |
| Existing tests not edited, none deleted | HOLDS | `git diff origin/lane-M-plugin-held-f-e-ready2 HEAD --stat` shows only this doc. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Nothing added. |
| Tests of changed packages and importers | HOLDS (nothing changed) | No package changed; the whole-module `go test` above covers every importer. |
| Small commits, one-sentence messages, no attribution lines | HOLDS | One commit, one sentence, no Co-Authored-By or Claude-Session line. |
| UI rows | NOT APPLICABLE | No UI row in the brief. |

## What the app tests read

- `TestRealPluginApprovePlanOnce` (realplugin_gate_test.go:161-162) and `TestRealPluginApproveAgentAccepted` (:198-205) run the real `grid approve`, then read **approval.json from the task folder** (`gateFiles` lists approval.json, history.md, task.md, plan.md). They do not read the `--json` stdout for the nine keys. "invalid JSON object" at :162 is `digestObject` parsing the empty string of a file that is not there.
- `TestRealPluginDigestReplay` (realplugin_digest_test.go:253-259) reads the file at `receiptPath` after each step and compares its keys with the transcript's `receipt_keys`. "receipt keys=[]" means the file is absent. Its `--json` comparison (:270-275) also expects the stdout to repeat every key the file holds.
- So "the receipt `grid approve --json` prints" is the stdout, which still has the nine keys. The nine keys the app tests want on disk are the file. The brief's one-line fix ("print the receipt as one JSON object") is already true; the tests fail on the file.

## Why I stopped

1. **The lane removed the file on purpose.** docs/lanes/E-verdict-12-receipts.md and docs/lanes/E-receipts-e2e3.md record the direction as: `grid approve` records an approval only as the result of the approve person Job; an old approval.json is read once, moved into that Job and deleted; the `receiptForGate` and `ApprovedDigest` fallbacks are gone. E-receipts-e2e3.md (the "Cross-repo resume" section) already says these five tests fail by design and that the fix is in the app: `gates/model.go:477` must read the plan Job result, and the five tests must assert the Job result. The brief says nothing in docs/needed.md or simplify.md removes the file. docs/needed.md does not exist in this clone, and simplify.md has no line naming approval.json (grep exit 2 only for the missing needed.md; no match in simplify.md). The verdict-12 direction lives in the lane docs, not in those two files. The brief and the lane docs disagree on who decided; I could not settle it from the clone.
2. **A plugin-only fix reverses the lane.** To make the tests pass the plugin has to write approval.json again on every approve, and also stop the once-migration and the worker pass (internal/migrate/receipt.go, internal/orchestrator/stream.go) from retiring a receipt that sits beside an approve Job; otherwise the next worker pass deletes the file the test reads. That undoes acceptance rows 1 and 2 of E-receipts-e2e3 and its "approval.json is deleted after it is read" default, and it brings back a second record of an approval beside the Feed. That is the owner's call, not mine.
3. **The brief forbids the other fix.** "Nothing in the app repo" rules out the change E-receipts-e2e3 names (gates/model.go and the five tests).
4. **I could not run the proof.** Rows 1 and 3 need `GRID_PLUGIN_BIN=` in the command; see the table.

## Options for the owner

- A. Keep the owner direction (no approval.json). The fix is in the app: read the plan Job result in `gates/model.go:477-498`, change the five tests to assert the Job result, and replace `receipt_keys` in the app's copy of the transcript with the lane's (`answer job note plan_digest plan_hash record when who`). The plugin lane lands with that app change in lockstep. No plugin change.
- B. Keep approval.json as a compatibility copy. A new plugin lane then: `Store.Approve` writes the nine-key file again after the Job result is recorded; reopen overwrites it; `ReceiptToJob` and the Stream pass leave a file that matches the approved Job instead of deleting it; the transcript's `receipt_keys` go back to the nine keys. I would size it at about 60 calls with the test rewrite (about 25 test files moved to the Job in the lane would need a second look). It needs an owner yes first, because it reverses the written direction.
- For either option, whoever runs rows 1 and 3 needs a shell that accepts `GRID_PLUGIN_BIN=`; the orchestrator can run them at landing.

## What changed

Only this doc and the report copy.

## NOT VERIFIED on a live screen

Nothing in this lane touches a screen. The app's approve button and task card (the model.go reads) are the live rows of the parent lane, E-receipts-e2e3.md, and stay unverified.

NOT DONE
