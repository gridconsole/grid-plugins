# C-fe-contract-readme-receipts

contract/README.md "## Plan approval receipts" and the error-table row for `workflow_changed` stop describing preview and ship receipts, `--gate`, `grid preview` and the workflow digest hold. Code cites are on the lane base (plugin main a9fc9c41).

## Paragraph table (old line numbers on base)

| Old lines | Paragraph | Verdict | Code |
|---|---|---|---|
| :39 | `workflow_changed` error row | GONE | `git grep -n "workflow_changed\|ErrWorkflowChanged" -- internal ':!*_test.go'` prints only the comment internal/task/hold.go:36-37; no `Deliver blocked` anywhere |
| :788 | receipts in `approval.json`, `approval-preview.json`, `approval-ship.json` | Preview and ship GONE; approval.json HOLDS as a copy | internal/task/write.go:524-526 `legacyReceipt`, :588-592 replay drops them; gates.go:96-107 writes approval.json "a copy for readers of the task folder"; keptApproval gates.go:117-121; internal/migrate/receipt.go:112,127 |
| :789-806 | `plan_hash`, `plan_digest`, `--plan-hash` tokens | HOLDS | internal/task/write.go:79; gates.go:40-47; internal/cli/task_write.go:140-149; internal/cli/plan.go:84 |
| :808-811 | `plan_hash_token` | HOLDS | internal/workflow/plan.go:8-28; gates.go:35-46; task_write.go:169-173 (exit 3) |
| :813-818 | both values must match; implicit reapproval checks previous digest | HOLDS (approval is the plan Job result) | gates.go:73-77, :157, :300 |
| :819-820 | plan receipts have no `gate` field; ship receipts carry `gate: ship` | First half HOLDS, second half GONE | gates.go:126 (`e.Gate != ""` is refused); no code writes `gate: ship` |
| :822-832 | preview receipt, preview history line, reopen invalidates three files, `grid plan migrate` treats preview receipt | GONE (reopen: HOLDS for approval.json only) | write.go:524-526, :588-592; reopen write.go:136-149; plan/migrate.go:9-12, :42-46 takes the hash through a callback |
| :834-843 | `grid approve --gate ...`; repeat approve writes nothing | `--gate` GONE; repeat approve HOLDS | testdata/golden/approve-help.golden lists `--plan-hash`, `--note`, `--json`; `git grep -n 'String("gate"' -- internal/cli` finds nothing; repeat: gates.go:86-95; exit 3 for an agent: task_write.go:177-181, :191-196 |
| :845-850 | `grid preview` | GONE | internal/cli/cli.go commands() has no preview verb |
| :852-856 | legacy `plan accepted` history line; JSON approval takes precedence | Line HOLDS; precedence rewritten (the plan Job result takes precedence, not approval.json) | internal/task/hold.go:17, :135-150 |
| :858-863 | host guard protects `task.md`; plan approval step rules | HOLDS; warn default added | guard/host.go:13, :100-105 (host-guard level, warn by default); taskfile/workflow.go:45-52; settings/catalog.go:31 levels off, warn, block |
| :865-892 | `workflow_digest`, `workflow_changed`, `Deliver blocked`, `grid task rebind` | GONE, except the folder check which HOLDS | `git grep -n "workflow_digest" -- internal ':!*_test.go'` prints only hold.go:36-37; folder check hold.go:57-74 (`ErrProjectMismatch`), :145-147 |
| :894-897 | `workflow.md` / `workflows.md` protected against Write, Edit, MultiEdit and literal Bash writes | File tools HOLD; literal Bash GONE | guard/host.go:100 ("a Bash command is never answered here"), guard/context.go:110-116 `fileTool`; symlink aliases host.go:75-95 |
| :897-904 | permission rules, `person_verbs`, the warning text | Not placed from the code; left as it is (see open items) | internal/workflow/person.go:10 `ErrPersonVerbs`; the warning text is not in code outside tests |

## Names the rewritten section still holds, each with a hit outside tests

| Name | Hit |
|---|---|
| `approval.json` | internal/task/write.go:519 |
| `approval-preview.json`, `approval-ship.json` (named as no longer written) | internal/guard/host.go:13; internal/task/write.go:526 |
| `plan_hash`, `plan_digest` | internal/task/write.go:79 |
| `plan_hash_token` | internal/workflow/plan.go:23 |
| `--plan-hash`, `--note`, `--json` | internal/cli/task_write.go:149-151 |
| `Changed since you approved` | internal/task/gates.go:74 |
| `lands_hash` ("lands hash") | internal/task/gates.go:73 (`LandsHash`); internal/orchestrator/person.go:147 |
| `grid plan hash --json`, `grid task show --json` | internal/cli/plan.go:84; internal/cli/task.go:169 |
| `grid plan migrate`, `--yes` | internal/cli/plan.go:41 |
| `plan accepted` history line | internal/task/hold.go:17 |
| level `block` (levels off, warn, block) | internal/settings/catalog.go:31 |
| `sign-off` (example step name only) | taskfile/workflow.go:47 rule (before the first run or work) |

The section does not call approval.json a fallback approval ("The copy is not a fallback") and does not say the host guard denies by default.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. no `--gate`, `grid preview`, `workflow_changed`, `workflow_digest`, `Deliver blocked` in README | HOLDS | `git grep -n -- "--gate\|grid preview\|workflow_changed\|workflow_digest\|Deliver blocked" -- contract/README.md` prints nothing (only `approval-preview.json` and `approval-ship.json` remain, in the sentence that says they are no longer written, README.md:795-796) |
| 2. each name has a hit; no fallback; no deny-by-default | HOLDS | table above |
| 3. `go test -count=1 -run Receipts ./internal/contract/` passes; fails with old paragraph | HOLDS | pass: `ok .../internal/contract` exit 0. On base README: 3 tests FAIL (`still names "--gate"`, `"grid preview"`, `"workflow_changed"`, `"workflow_digest"`, `"Deliver blocked"` ...) exit 1. With the old `grid approve --gate plan|preview|ship` line pasted back: `FAIL: TestReceiptsSectionDropsGoneBehaviour ... still names "--gate", which no code has` exit 1; restored |
| 4. `go test -count=1 ./internal/contract/`; `make lint` | HOLDS | `ok` exit 0; `make lint` (go vet ./...) exit 0 |
| 5. `git diff main --stat` | HOLDS | contract/README.md, internal/contract/fe_readme_receipts_test.go, docs/lanes/C-fe-contract-readme-receipts.md |
| Tests of packages that import | HOLDS | Only a `_test.go` file was added to internal/contract; no non-test code changed, so its importers (artifact, brain, braincmd, cli, feed, guard, hook, intent, job and others) cannot change. The other test that reads contract/README.md, `go test -count=1 ./internal/where/`, `ok` exit 0 |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New tests fail on base | HOLDS | see row 3 |
| No existing test edited or deleted | HOLDS | diff touches no existing test |
| No guard, gate, cap or hold added | HOLDS | docs and one test only |
| Public repo hygiene | HOLDS | no personal paths, hosts or emails; no attribution line in commits |
| `*RealPlugin*` tests | n/a | the brief names none |

## What changed

- contract/README.md: the section opens with "the plan Job's accepted result is the only approval", says `approval.json` is a copy and no fallback, and that preview and ship receipts are no longer written or read, are dropped from an old journal, and keep their names in the host guard's file list, which warns by default and denies only at level `block`. The preview receipt, `--gate`, `grid preview` and `workflow_digest` paragraphs are gone; the repeat-approve, reopen, `grid plan migrate` and legacy-history paragraphs are rewritten to what the code does. The folder-project check stays. Row :39 removed. The "literal Bash writes" claim is dropped (the host guard reads no shell text).
- internal/contract/fe_readme_receipts_test.go: three tests.

## Open items

- The sentence "Publication, the guard, and locked approval readers require both values to match" names "publication"; no publish code in internal/ reads a plan hash, so I could not place that word. Left as it is.
- The `person_verbs` sentences at the end of the old :894-905 paragraph (the warning text `grid: warning: invalid person_verbs; ignoring ...`) are not in non-test code under that wording. Left as they are; a later lane should check them.
- internal/task/hold.go:36-37 still has a comment naming `ErrWorkflowChanged`, which no longer exists. Code, not touched here.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen; it is a contract document and a test.
