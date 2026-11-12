# C-rf-curator-brain-verbs

A Job agent (`GRID_ROLE=worker`) may run `grid brain changes`, `supersede` and `revert`; a guest and any other role stay refused; `write`, `rename`, `drop` and `restore` keep their rule.

Base: 103b9157 (the clone's `main`; the brief names 44c7de5b, the three checks and the line numbers it quotes are the same on 103b9157). Head: see the report line at the end.

Base `internal/brain/revert.go:20-36`:

```go
var (
	ChangesReadAllowed = func(role string) bool { return ownerOrMember(role) }
	RevertAllowed      = func(role string) bool { return ownerOrMember(role) }
)

// ErrIndexRebuild is wrapped in the error of a revert or supersede that wrote
// its change but could not rebuild index.md afterwards; the result is valid.
var ErrIndexRebuild = errors.New("rebuilding the index failed")

func ownerOrMember(role string) bool {
	switch strings.TrimSpace(role) {
	case "", "owner", "member":
		return true
	default:
		return false
	}
}
```

No verb admitted `worker` on base (changes.go:176, changes.go:259, supersede.go:60 all refused it), so the base check passes.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| Base: TestRfCurWorkerVerbs red with exit 3 on each verb | HOLDS | Run on base with only the test file (`go test -count=1 -v -run TestRfCurWorkerVerbs ./internal/braincmd/`, exit 1): `changes as worker: exit 3, stdout "{\"error\":\"person_required\",\"reason\":\"role_not_allowed\",...}", stderr "grid brain changes: caller role is not allowed to read brain changes\n", want exit 0`; `supersede as worker: exit 3, stdout "", stderr "grid brain supersede: caller role is not allowed to supersede brain entries\n", want exit 0`; `revert as worker: exit 3, stdout "", stderr "grid brain revert: caller role is not allowed to revert brain entries\n", want exit 0`; `--- FAIL: TestRfCurWorkerVerbs/changes`, `/supersede`, `/revert`. The test file was then split in two commits: this run used the first version of the file (worker and guest tests, no `CurateAllowed` reference), so it compiled on base. The final file adds three policy tests that need `CurateAllowed` and cannot compile on base. |
| 1. TestRfCurWorkerVerbs | HOLDS | `go test -count=1 -v -run 'TestRfCur' ./internal/braincmd/` exit 0: `--- PASS: TestRfCurWorkerVerbs` with `/changes` (stdout byte-equal to the local owner's, lists checks, deploy, deploy-v2), `/supersede` (old note holds `superseded_by: brain:home/deploy-v2`, log has the superseded line), `/revert` (entry bytes equal stored version 1, log has `revert to v1: put back after review`). |
| 2. Fail-closed | HOLDS | `go test -count=1 -v -run 'TestIaBrRoleRefusal\|TestIaBrReadme\|TestLoWaGuestWorker\|TestRolesDecideBothVerbs' ./internal/braincmd/` exit 0, all four PASS, unchanged (no existing test edited). `--- PASS: TestRfCurGuestStillRefused` runs changes, supersede and revert as guest, other, agent, Worker and "worker,guest" (15 subtests): each exit 3, the roleRefused line, stdout empty, the `--json` stdout is the person_required object (iaBrObject), and `svTree` of the Grid home is equal before and after. |
| 3. write and rename still refuse a worker | HOLDS | `go test -count=1 -v -run 'TestKeRnCommand\|TestKeWrCommand\|TestLoWaGuestWorker' ./internal/braincmd/` exit 0 (the worker cases at ke_write_test.go:201 and ke_rename_test.go:98 and lo_write_anyone_test.go:69), unchanged. `TestRfCurWorkerStillRefusedOnDropAndRestore` adds drop and restore as worker: exit 3, same line, brain folder unchanged. |
| 4. brain and braincmd tests | HOLDS | `go test -count=1 ./internal/brain/ ./internal/braincmd/` exit 0. |
| 4. Walk rows | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0, both PASS (not skipped); `go test -count=1 ./internal/job/lint/` exit 0. |
| 5. make lint, make cross | HOLDS | `make lint` (go vet ./...) exit 0; `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0; `gofmt -l internal/brain internal/braincmd` prints nothing. |
| 5. Diff names only Touches | HOLDS | `git diff main --stat`: contract/README.md, internal/brain/revert.go, internal/braincmd/changes.go, internal/braincmd/supersede.go, internal/braincmd/rf_curator_verbs_test.go, plus this report under docs/lanes/ (the runbook requires it). |
| Importers' tests (go list -deps) | HOLDS | Packages whose Deps hold internal/brain or internal/braincmd: cmd/grid, approvefix, braincmd, cli, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, people, ports, projectenv, question, routine, signal, task, updates, usage, workflow, lib. `go test -count=1` on all of them: every package ok except one run of `internal/env` `TestUpHealthTimeoutHonorsStopGrace` (`signals.log = "INT\n", want "INT\nTERM\n"`), a signal-timing test under the 30-package parallel load. It passes alone: `go test -count=3 -run TestUpHealthTimeoutHonorsStopGrace ./internal/env/` ok, and `go test -count=1 ./internal/env/` ok. The test does not touch roles or the brain. |
| Cross-repo check | HOLDS | `git -C /srv/gridwork/ref/app.git grep -n -E '...' main -- '*.go'` as the brief gives it: owner rigs (gh_brain_changes_rows_realplugin_test.go:156, brain_missing_test.go) and read-only `brain changes` callers (vault_more.go:219, :282, brainchanges/http.go:99, :137); none reads the worker refusal line, and the change only widens who may run the three verbs. No app brief needed. |
| Mismatch proof, RealPlugin, app walk tests | NOT APPLICABLE | No row asks for them; this is the plugin repo and no UI or `*RealPlugin*` row. |
| No guard, gate, cap or hold on agents | HOLDS | The change removes a refusal for one role and adds no check. `CurateAllowed` has the same default as `ChangesReadAllowed`, no new flag, no new refusal. |
| Remove nothing / no test deleted or edited | HOLDS | `git diff main` deletes no test and edits none; README lines 700-707 reworded in place. |

## What changed

- `internal/brain/revert.go`: new `ownerMemberOrWorker` (the empty role, owner, member, worker, trimmed); `ChangesReadAllowed` and the new replaceable `CurateAllowed` default to it; `RevertAllowed` keeps `ownerOrMember`. Comment names which policy gates which verb.
- `internal/braincmd/changes.go:259` (`revert`) and `internal/braincmd/supersede.go:60` check `brain.CurateAllowed`. `write` (ke_write.go:92), `rename` (ke_rename.go:56) and `drop`/`restore` (hy_drop.go:98) still check `brain.RevertAllowed`.
- `contract/README.md:700-711`: says `changes` checks `ChangesReadAllowed`, `revert` and `supersede` check `CurateAllowed`, all with the worker default, and that `RevertAllowed` keeps its default and gates write, rename, drop and restore. The paragraph still opens with "Both verbs check `GRID_ROLE`", which TestIaBrReadme reads. `CurateAllowed` has no host key: the brief adds no flag or key.
- `internal/braincmd/rf_curator_verbs_test.go` (new): TestRfCurWorkerVerbs, TestRfCurGuestStillRefused, TestRfCurPolicyDefaults (role table for the three policies), TestRfCurPoliciesAreSeparate (denying CurateAllowed refuses revert and supersede for the local role; denying RevertAllowed leaves them open to a worker), TestRfCurWorkerStillRefusedOnDropAndRestore.

## Open items

- README paragraph 708 onward and lines 747 and 790 still say `brain.RevertAllowed` for write, rename, drop and restore; that stays true, so I left them.
- No host key is named for `CurateAllowed`; the host (app) does not set any of the keys today (no code in this repo reads `brain_changes_read_roles` or `brain_revert_roles`).

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: a real Job in the app with `GRID_ROLE=worker` calling the built `grid` binary; the tests use the in-process `Run` with `Env.Role = "worker"`, which `internal/cli/brain.go:14` fills from `GRID_ROLE`.

Verdict: DONE
