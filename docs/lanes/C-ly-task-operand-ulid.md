# C-ly-task-operand-ulid: a task's ULID resolves as a task operand

Repo: plugin. Branch lane-C-ly-task-operand-ulid. Base 73d8d828931db69889d14dea0bab469272262a25 (main).
Code head acd3fbfc55fc3e01cf5f624295dc9c98f0797181 (two code commits, 6fe6b8ce and acd3fbfc); this doc is the third and last commit.

## What changed

- `internal/cli/task_operand.go`: one more case in the switch of `resolveTaskOperand`, placed before the plain-id case. When `feed.IsID(operand)`, it walks the `dirs` the function already reads from `r.TasksDirs()` (Personal and guest-hidden projects are already dropped there), takes each child whose name matches `taskOperandName` and whose `task.md` gives that id as `id:` or `ulid:` (`feed.TaskID`), and sets `path`. No folder gives `where.ErrTaskNotFound`; two folders give "task has more than one folder in the Grid home". After the case, the existing lexical and canonical parent checks after the switch (the lines that were :78-133) run unchanged, so the result is the same `task.Task` the T-NN gives.
- Security boundary: the walk reads `task.md` only of name-matching children of accessible tasks directories, and it does not skip links. A link found there (`T-99-link` pointing at the Personal task) becomes `path` and is then refused by the existing canonical-parent check ("folder escapes its tasks directory", `errTaskOperand`), so a lookup by ULID cannot move a task into another project's root. The Personal ULID is `ErrTaskNotFound` for a member and a guest because Personal is not in `dirs`.
- `internal/cli/ly_task_operand_ulid_test.go` (new): six `TestLy` tests.
- No help text, golden, contract/commands.json, hsIpTable entry, open.go or internal/where change.

## Proof table

Acceptance rows:

| Row | Result | Proof |
|---|---|---|
| 1 | HOLDS | `TestLyTaskOperandULIDShowAndHistory` (task show --json id and folder equal the T-NN's, from h.root and h.home; history add lands in history.md) and `TestLyDeliverAndProofByULID` (proof add --task ULID --json exit 0, the text lands in the task folder; deliver --task ULID --outcome from h.home exit 0, task.md holds `## Outcome`, `Result ready` and `state: waiting`). `go test -count=1 -v -run TestLy ./internal/cli/` exit 0, both `--- PASS`. Fails on base, see base check. |
| 2 | HOLDS | `TestLyResolveTaskOperandULIDMigratedAndLegacy/migrated` and `/legacy` PASS: same ID and Path for the ULID, the T-NN and the fixture; legacy is `id: T-01` + `ulid: <ULID>` with VERSION removed, as task_shape_test.go:34 writes it. |
| 3 | HOLDS | `TestLyTaskOperandULIDNotFound` (`errors.Is(err, where.ErrTaskNotFound)` for a well-formed unused ULID, as T-99; `task show` exit 1). `TestLyTaskOperandULIDPolicyAndLink`: owner resolves both tasks, member and guest get ErrTaskNotFound for the Personal ULID and resolve the shared one; with `T-99-link` the member gets `errors.Is(err, errTaskOperand)` and the owner gets "more than one folder". `TestLyTaskOperandULIDTwoFolders`: a copy `T-05-copy/task.md` gives "task has more than one folder in the Grid home", `task show` exit 1. All PASS. One extra check beside the link: the shared task answers its ULID as its T-NN does; both fail the same way ("... T-99-link is not a directory") because `Show` (not changed) scans the folder, so the test asserts parity only. |
| 4 | HOLDS | `go test -count=1 -run 'TestLy\|TestTaskOperand\|TestRemainingTaskOperand\|TestOtherTaskVerbFolderOperands\|TestTaskCommandsMigratedAndLegacyShape\|TestOpen\|TestDeliver\|TestDelivery\|TestProofCommandTable\|TestHistoryCommand' ./internal/cli/` exit 0 (`ok ... 2.215s`). With `-v`: `--- PASS: TestLyTaskOperandULIDShowAndHistory`, `TestLyDeliverAndProofByULID`, `TestLyResolveTaskOperandULIDMigratedAndLegacy`, `TestLyTaskOperandULIDNotFound`, `TestLyTaskOperandULIDPolicyAndLink`, `TestLyTaskOperandULIDTwoFolders`; the existing `TestTaskOperandPlainOutput`, `TestTaskOperandPolicyAndExactFolder`, `TestTaskOperandHelp`, `TestOtherTaskVerbFolderOperands`, `TestTaskOperandForms`, `TestTaskOperandEscapes`, `TestRemainingTaskOperands`, `TestRemainingTaskOperandHelp`, `TestTaskCommandsMigratedAndLegacyShape` PASS, none edited. |
| 5 | HOLDS | `git diff main --stat` on the two code commits (run before this doc was added): ` internal/cli/ly_task_operand_ulid_test.go ` 229 insertions and ` internal/cli/task_operand.go ` 30 insertions, 2 files. The one further file on the head is this doc, `docs/lanes/C-ly-task-operand-ulid.md`, the proof table the builder runbook requires (every lane commits one); no other file moved. `make lint` exit 0 (`go vet ./...`). `make cross` exit 0 (darwin/arm64 and windows/amd64: vet and test compile, all packages ok). |

Rules:

| Rule | Result | Proof |
|---|---|---|
| Base check | HOLDS | Test written first and run on base 73d8d828931db69889d14dea0bab469272262a25 (`go test -count=1 -run TestLy ./internal/cli/`, exit 1): `grid task show: invalid task id "01M76ARE055FGS82TQ97H21NZB": use T-NN`; also `proof: invalid task id "01M76ARE079X3KN0C91CZ6X3CC": use T-NN`, and for the unknown ULID `invalid task id "01ARZ3NDEKTSV4RRFFQ69G5FAZ": use T-NN, want ErrTaskNotFound`. All six `TestLy` tests failed on base and pass on the lane. |
| New tests fail on base, pass on lane | HOLDS | as above. |
| No existing test edited or deleted | HOLDS | the diff names only the new test file; no test deleted. |
| No guard, gate, cap or hold on agents | HOLDS | the change makes a refusal into a success for a well-formed ULID; it adds no refusal for agents. |
| Tests of changed package and importers | HOLDS | `go list` importers of internal/cli (direct, tests included): cmd/grid, contract/formats, internal/account, internal/braincmd, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` on all of them exit 0: ok for each (cmd/grid has no test files); internal/cli 49.856s. |
| gofmt | HOLDS | `gofmt -l internal/cli/` printed nothing. |
| Windows and macOS builds | HOLDS | `make cross` exit 0, see row 5. |
| Walk tests (plugin) | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok`. `go test -count=1 ./internal/job/lint/`: `ok ... 0.063s`. |
| *RealPlugin* tests | N/A | the brief names none. |
| UI rows | N/A | the brief has none. |
| No attribution lines in commits | HOLDS | `git log --format='%H %B' origin/main..HEAD` shows one-sentence messages only. |

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. Not run here: an agent's real `grid deliver --task $GRID_TASK` under the app worker (the sandbox proof is the CLI through `Run` in the harness); the whole-module test and the cross-repo set are the orchestrator's at landing.

Verdict: DONE
