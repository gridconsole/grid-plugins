# C-vd-deliver-branch: `grid deliver` records the task's branch and base in task.md

Base: 2b47f62181522cb3036a3e38e186ec6d9e779eb8 (plugin main). Lane commits: f9be5f7b (feature), the golden commit after it, and this report.

Base check: on the base, the new test `TestVdBrDeliverRecordsGitBranchAndBase` (deliveryFixture, `grid deliver --task T-01 --outcome X` with Cwd in a git repo on `grid/T-01`) finds neither key in task.md's front matter:

```
--- FAIL: TestVdBrDeliverRecordsGitBranchAndBase
    branch/base: <nil> / <nil>, want grid/T-01 / bee68baace526215b3671e96ccf8960806ee9596
--- FAIL: TestVdBrDeliverFlagsWin
    exit 2: flag provided but not defined: -branch
--- FAIL: TestVdBrSecondDeliveryKeepsOneBranchKey
    second exit 2: flag provided but not defined: -branch
FAIL	internal/task [build failed]  (unknown field Branch / Base in struct literal of type DeliverOptions)
```

(Run by checking out the base `internal/cli/deliver.go` and `internal/task/deliver.go` over the lane, then restoring them. `git grep CurrentBranch 2b47f621` finds nothing, so the gitx tests cannot compile on base.)

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | gitx: CurrentBranch `grid/T-01`, MergeBase(HEAD, main) = main's sha, detached HEAD false, non-repo false for both | HOLDS | `go test -count=1 -v -run TestVdBr ./internal/gitx/` exit 0: `--- PASS: TestVdBrCurrentBranchAndMergeBase`, `TestVdBrDetachedHead`, `TestVdBrNotARepo` (internal/gitx/vd_branch_test.go) |
| 2 | CLI: defaults from the repo, flags win, non-repo writes neither with the same stdout, second delivery keeps one `branch:` key = b3 | HOLDS | `go test -count=1 -v -run TestVdBr ./internal/cli/` exit 0: `--- PASS:` TestVdBrDeliverRecordsGitBranchAndBase, TestVdBrDeliverFlagsWin, TestVdBrDeliverOutsideRepoWritesNeither, TestVdBrSecondDeliveryKeepsOneBranchKey. Stdout asserted equal to `T-01 delivered; waiting for the person\n`. The second delivery needs `grid task status T-01 running` first (a waiting task refuses delivery on main as well, delivery_test.go:282). |
| 3 | task: DryRun with a Branch writes nothing, Actions the same | HOLDS | `go test -count=1 -v -run TestVdBr ./internal/task/` exit 0: `--- PASS: TestVdBrDryRunWritesNothing` (tree bytes equal, Actions DeepEqual), `TestVdBrDeliverWritesBranchAndBase` (3 actions, one `branch:` key replaced in place) |
| 4 | `make generate` clean; `--branch` under deliver; TestContractCommands/Doctor/Deliver/Er | HOLDS | `make generate` run twice, `git status --short` empty after the second (exit 0). `git grep -n '"--branch"' -- contract/commands.json` hits :2491 (the deliver block, new; diff adds `--base` and `--branch` only) and :4710 (an older command, unchanged). `go test -count=1 -run 'TestContractCommands\|TestDoctorGolden\|TestDeliver\|TestEr' ./internal/cli/` exit 0. |
| 5 | `go test ./internal/task/ ./internal/gitx/`; SKILL.md sentence | HOLDS | both `ok` (exit 0, in the package run below). `git grep -n -i worktree -- skills/deliver/SKILL.md` hits skills/deliver/SKILL.md:23. |
| 6 | `make lint`, `make cross`, diff names only Touches | HOLDS for lint and cross; diff has two additions, see below | `make lint` (go vet ./...) exit 0. `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0. `git diff 2b47f621 --stat`: 9 files. |
| R1 | Tests of changed packages and every package importing them | HOLDS | Importers from `go list` (direct importers of task or gitx): braincmd, cli, doctor, guard, hook, job, job/jobcmd, job/jobtest, land, migrate, projects, signal, task, updates, where. `go test -count=1` over those plus gitx and job/...: all `ok` (exit 0) after the golden fix; the first run had one red, see below. |
| R2 | Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. (The app walk tests are for app lanes.) |
| R3 | New tests fail on base, pass on lane | HOLDS | see Base check above; pass lines in rows 1-3. |
| R4 | No existing test edited, no test deleted | HOLDS | `testdata/golden/deliver-help.golden` changed (a golden file, not test code); see below. |
| R5 | No guard, gate, cap, hold or hook | HOLDS | Nothing is refused: a missing branch or base is not written and delivery goes on; no new Delivery.Actions line (internal/task/deliver.go, recordBranch). |
| R6 | *RealPlugin* tests | not run | none named by the brief. |
| R7 | Commit identity, no attribution lines, no personal paths | HOLDS | commits by the lane identity, messages one sentence, no Co-Authored-By. |

## Two changes outside the Touches list

1. `testdata/golden/deliver-help.golden`: `TestRemainingTaskOperandHelp/deliver` compares `grid deliver --help` to this golden. The brief orders the Usage line (:38) and the help text changed, so the golden has to follow, otherwise `go test ./internal/cli` is red. I wrote the golden with the exact output the test printed. No test code was edited.
2. `docs/lanes/C-vd-deliver-branch.md`: this report, which the runbook asks for.

## What changed

- `internal/gitx/vd_branch.go`: `CurrentBranch(dir)` (`git symbolic-ref --short -q HEAD`, false when detached or outside a repo) and `MergeBase(dir, a, b)` (`git merge-base`, false on any failure).
- `internal/task/deliver.go`: `DeliverOptions.Branch` and `.Base`; `recordBranch` takes a given value, else the current branch, else the merge-base of HEAD with the first of `main`, `master` that answers; `d.write` sets `branch` and `base` with setScalar only when non-empty. No line added to `Delivery.Actions`. The default is read in `write` (under the locks, only on a real delivery), so a dry run reads and writes nothing about git.
- `internal/cli/deliver.go`: `--branch`, `--base`, Usage line and one help sentence.
- `skills/deliver/SKILL.md`: one sentence (line 23).
- `contract/commands.json`: through `make generate`.

Behaviour to know: a Cwd that is a repo with no `main` or `master` branch gets `branch:` but no `base:`. On `main` itself, base is HEAD. If main exists only as a remote ref (`origin/main`), no base is written (the brief names `main`, `master`).

## NOT VERIFIED on a live screen

The Code tab "Repository and branch" line (app `#/<project>/<id>/code`, design `pages/grid-task-design/code.html`) was not shot: it needs the app server and the UI track shoots the pair after landing. The data side is proved: task.md holds `branch:` and `base:` as plain scalars, the keys `projectFolder` reads at taskcode/build.go:486.
