# C-fd-plan-anchor-before-stands

Base: main 06047047ac70ebf6bba163d448a121c627a363a1. The base check found no existing test that resolves a heading anchor bordering a trailing `## Where it stands` (source_resolve_test.go:418 uses a line-1 plan with `## Where it stands` mid-file and no heading anchor; approval_workflow_test.go:14 has no blank lines).

## Proof table

| Row | Result |
|---|---|
| 1 `plan.md#steps` of rev-3 shape resolves, Text has step line, no Where-it-stands prose | HOLDS: `go test -count=1 -v ./internal/job/ -run TestFdAnchor` exit 0, TestFdAnchorStepsBeforeStands PASS (plan_anchor_before_stands_test.go). On main it failed: `plan.md#steps: plan.md#steps: anchor selects lines outside the approval digest` |
| 2 `#goal` before a trailing stands; plan ending in two newlines with no stands | HOLDS: TestFdAnchorGoalBeforeStands, TestFdAnchorTrailingBlankLinesNoStands PASS. On main: `plan.md#goal: anchor selects lines outside the approval digest` and `plan.md#steps: anchor selects lines outside the approval digest`. Also a custom `{#my-scope}` id: TestFdAnchorCustomIDBeforeStands PASS (failed on main the same way) |
| 3 `#where-it-stands`, `#L14`, `#L13-L14`, `#L10-L14` still refused with "outside the approval digest" and empty Text; TestPlanOmittedAnchorsRefused unchanged | HOLDS: TestFdAnchorStandsStillRefused PASS (also passes on main); TestPlanOmittedAnchorsRefused passes unedited in `go test ./internal/job` exit 0 |
| 4 Digest and Hash unchanged | HOLDS: TestFdAnchorApprovalUnchanged compares plan.Digest `8d8cf1a9…10c4a` and plan.Hash `d902f6`, both taken from main before the change; PASS. internal/plan not touched |
| 5 diff names only source.go and the new test | HOLDS: `git diff origin/main --stat`: internal/job/source.go (+9), internal/job/plan_anchor_before_stands_test.go (+105), plus this report |
| 5 `go test ./internal/job/... ./internal/plan/...` 0 fail | HOLDS: exit 0 |
| 5 `gofmt -l internal/job` empty | HOLDS: no output |
| 5 `GOOS=windows go vet ./internal/job/` | HOLDS: exit 0 (also GOOS=darwin) |
| Rule: importers tested | Direct importers of job/plan by go list: approvefix, cli, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, migrate, orchestrator, plan, ports, proof, task. All ok except one flake, below |
| Rule: lint, Windows and macOS builds | HOLDS: `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| Rule: no guard/gate/cap on agents | HOLDS: the change relaxes a refusal; adds none |
| Rule: *RealPlugin* tests | Not needed: no row names one |
| Rule: no attribution lines, no personal paths | HOLDS: commit has one sentence and no trailer |

Flake: in the first importer run `internal/cli` TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead failed once (`approve_record_test.go:406: accepting controls []`) under parallel load; it passes alone and a full `go test -count=1 ./internal/cli` rerun passed (exit 0). It does not touch plan anchors. I did not run it on base to compare.

## What changed

internal/job/source.go, heading-anchor branch of `file`: for a non-line anchor on plan.md, trailing blank lines (whitespace-only) are dropped from the selected range before `plan.DigestCoversLines` and before `selectedText`. Line anchors (`#L<n>`) keep their exact range, so a blank line anchored by number is still refused. Lines inside `## Where it stands` are still refused because the range check is unchanged. Text is the same as before for covered sections, since selectedText already trimmed trailing newlines.

Note: the relaxation is in the shared heading branch, so it applies to any file with a heading anchor, but the digest check only runs for plan.md.

## NOT VERIFIED on a live screen

Nothing here is UI. Not rerun: the orchestrator Job J-1 pointer `plan.md#steps` against a real plan of r2.
