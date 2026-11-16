# P-projectenv-deploy-kind: a project environment of kind deploy (deploy target)

Base: origin/main (6cc26809). `git fetch origin` then `git log HEAD..origin/main` before the last proof run: empty, so main had not moved and there was nothing to merge.

## What a deploy target holds

The design's Environments tab (`grid-design/project.html`) lists the kinds as `{"local":"Local machine","remote":"Remote machine","deploy":"Deploy target"}` (line 1273). Its deploy card (`xEnvCard`, :3643-3645) draws the same rows as every card (Where, Runs, Secrets, Read by), plus a Deploy block when `e.kind === 'deploy'`. The Deploy block has two rows: **How** (`e.how`) and **Who may deploy** (`e.who`), each "Not set" when empty. The design's three deploy records (`staging`, `production`, `github-releases`, line 1273) put where it deploys in `where` (`https://staging.vertex.example`) and what was deployed and when in `runs` (`v2.8.1-rc2, deployed Fri 23 Oct 09:20`).

Every one of those fields is already in the record (`where`, `runs`, `how`, `who`, `secrets`). The change is the kind value only: no new key, no new line in the file. `docs/needed.md` is not in this repo or its history (`git log --all -- docs/needed.md`: empty), and the plugin docs have no deploy-target text. So no field beyond the design's Deploy block was added. A separate "last deploy" field was not added, because the design carries it inside `runs`.

`cloud` stays (no feature removed). How and Who are not required for kind deploy, because the design draws "Not set".

Removal lists: searched `docs/decisions/`, `docs/old-plugin-map.md`, `docs/agents.md` (`grep -rniE deploy`), the sibling brief `prompt-builder-C-project-reads-gaps.txt` and the hand folder (`grep -rliE "deploy target|kind deploy|kind: deploy"`). No line removes deploy targets or environment kinds.

## Example record

`grid project env add <project> --id staging --name staging --kind deploy --where https://staging.example.test --runs "v2.8.1-rc2, deployed Fri 23 Oct 09:20" --test "make smoke" --how "make deploy-staging" --who ann --who bo --secret HEALTH_CHECK_KEY:missing --approval no` writes `projects/<project>/knowledge/environments/staging.md`:

    ---
    name: staging
    ---
    ## Environments
    ### staging
    name: staging
    kind: deploy
    where: https://staging.example.test
    runs: v2.8.1-rc2, deployed Fri 23 Oct 09:20
    test: make smoke
    how: make deploy-staging
    who:
      - ann
      - bo
    secrets:
      - HEALTH_CHECK_KEY (missing)
    approval: no

(Pinned byte for byte by `TestPdDeployKindRenderParse`, `internal/projectenv/pd_deploy_kind_test.go:12`.) `grid project env list <project> --json` answers it as `{"id":"staging","name":"staging","kind":"deploy","where":"...","runs":"...","how":"make deploy-staging","who":["ann","bo"],"secrets":[{"name":"HEALTH_CHECK_KEY","set":false}]}` (`TestPdProjectEnvDeployKind`, `internal/cli/pd_project_env_deploy_test.go:13`).

## Proof table

| Row | Result |
| --- | --- |
| A1. An environment of kind deploy is written and read back | HOLDS. `go test -count=1 -run TestPd -v ./internal/projectenv/ ./internal/cli/` exit 0: `TestPdDeployKindRenderParse` (Render gives the exact bytes above, Parse gives the entry back), `TestPdDeployKindSaveList` (Save then the stored bytes then List), `TestPdDeployKindBare` (kind deploy with no how or who), `TestPdProjectEnvDeployKind` (CLI `add --kind deploy --json`, the entry file equals `projectenv.Render`, `list --json` reads it back, `edit --kind deploy` then `--kind remote`; every answer passes `contract.Validate("cmd-project-env")`), `TestPdProjectEnvUsageNamesDeploy`. All PASS. |
| A2. Old kinds unchanged | HOLDS. `TestPdDeployOldKindsUnchanged` (`pd_deploy_kind_test.go:86`): `""`, local, remote and cloud write `kind: <k>` (or no kind line) and read back equal; `Deploy`, `deployment` and `ci` are still refused. The existing tests pass unchanged: `TestLnEnvExampleBytes` (the bytes the app pins, kind remote) and `TestLnEnvRoundTripEveryField` (kind cloud) in `projectenv_test.go`, and every `TestLnEnvCli*` in `ln_project_env_test.go`. |
| A3. Tests pass (`go test -race`) | HOLDS for the changed packages; see the note on wall-clock time. `go test -race -count=1 ./...`: 69 of 72 test packages `ok`, including `internal/projectenv` 4.0s, `contract` 12.3s, `internal/contract` 146.8s, `contract/formats`, `lib`, `internal/packaging`, `braincmd`, `golden`, `hook`, `job/lint`. Three packages hit Go's default 10-minute timeout with no assertion failure: `internal/cli`, `internal/feed`, `internal/orchestrator`. Reruns with `-timeout 45m`: `internal/feed` `ok` 1217s; `internal/cli` and `internal/orchestrator` still ran past 45 minutes under `-race` (the tests running at the alarm were `TestSettingsStoreChokePoint`, 11s in, and `TestGuCtTwoDeadJobs`, 1m14s in, so the packages are slow, not hung). The `cli` tests this lane touches, under `-race`: `go test -race -count=1 -run 'Pd\|LnEnv\|BwProjectEnv\|ProjectEnv' -v ./internal/cli/` exit 0 (every `TestLnEnvCli*` and `TestPd*` PASS, 9.3s), and `go test -race -count=1 -run 'TestBwEnvCli\|TestCt\|Contract\|Inventory' ./internal/cli/` exit 0 (79.3s). Whole packages without `-race`: `go test -count=1 -timeout 40m ./internal/cli/ ./internal/orchestrator/` gave `orchestrator` `ok` 1152.6s. In `cli`, one test failed: `TestSgRotatesAtStart` (`sg_app_log_rotate_test.go:66`, "grid-server did not write app-url within 10s"), a `grid open` startup timing test that ran while `orchestrator` loaded the box. Rerun alone, `-count=3`: 3 PASS. Then `go test -count=1 -timeout 40m ./internal/cli/` alone: `ok internal/cli 208.539s`, exit 0. `internal/orchestrator` imports neither `projectenv` nor `cli` and never reads `cmd-project-env.json`. |
| 3b. `make lint` | HOLDS. `make -C <lane> lint` exit 0; output `go vet ./...` and no finding. |
| 3c. CLI walk test (a command's flags text changed) | HOLDS. `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0: `walked 39 commands, 39 table entries`, both tests and all five subtests PASS. |
| 4. Lane doc with the command and an example record, last line DONE or NOT DONE | HOLDS. This file. |
| R1. New tests fail on base, pass on the lane | HOLDS. See Tests. |
| R2. No existing test edited or deleted | HOLDS. `git diff --stat origin/main..HEAD -- '*_test.go'` lists only the two new files. |
| R3. Packages changed and their importers (`go list`) | HOLDS. See Importers; covered by the A3 run. |
| R4. Walk test `go test -count=1 ./internal/job/lint/` | HOLDS. Exit 0, `ok internal/job/lint 4.569s`. |
| R5. Windows and macOS builds | HOLDS. `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` both exit 0, no output. `gofmt -l internal/projectenv internal/cli`: empty. |
| R6. `git fetch origin && git merge origin/main` before the last run | HOLDS. Fetch done; `git log HEAD..origin/main` empty, so there was nothing to merge. |
| R7. App guest walk tables (`make realplugin ...`), `*RealPlugin` tests | Not run, not asked. They are app-repo checks and this lane is in the plugin repo. |
| R8. No guard, gate, cap, hold or blocking hook | HOLDS. The only behaviour change is that one more value is accepted. Nothing is refused that was accepted before. |
| R9. Never: push, /srv/gmf, /root/.grid, `pkill -f`, remove a feature | HOLDS. Nothing pushed (the bundle is local). Tests use temp homes. The kinds local, remote and cloud are all kept. |
| R10. No attribution lines, no personal paths in the repo | HOLDS. The commit messages are one sentence each with no trailers. The design is cited by its repo-relative name. |
| R11. Files outside the brief's packages | One, forced by the code: `contract/schemas/cmd-project-env.json` gains `"deploy"` in both `kind` enums (`item` and `items`). It is hand-written and `lnEnvCliRun` validates every CLI answer against it. Without the change, a deploy entry's `--json` answer would fail the published schema. |

## Importers (`go list`)

Changed: `internal/projectenv`, `internal/cli` (`ln_project_env.go`), and the root `contract` package (schema file, embedded).

    cmd/grid, contract/formats, lib                imports internal/cli
    internal/cli                                   imports internal/projectenv
    internal/contract                              imports contract (the embedded schemas)
    tests of internal/account, braincmd, golden, jobgraph, land, mcp, packaging, question, settings import internal/cli
    tests of braincmd, cli, hook, job/lint, packaging, internal/contract import contract

`internal/contract` is imported by most of the module, so the whole module was run under `-race` (A3).

## Tests

The new tests were run on the base tree, with the two new test files and no production edit (`go test -count=1 -run TestPd ./internal/projectenv/ ./internal/cli/`, exit 1):

    --- FAIL: TestPdDeployKindRenderParse (0.00s)
        pd_deploy_kind_test.go:40: render kind deploy: environment kind must be local, remote or cloud
    --- FAIL: TestPdDeployKindSaveList (0.00s)
        pd_deploy_kind_test.go:57: save kind deploy: environment kind must be local, remote or cloud
    --- FAIL: TestPdDeployKindBare (0.00s)
        pd_deploy_kind_test.go:76: render bare deploy: environment kind must be local, remote or cloud
    --- FAIL: TestPdDeployOldKindsUnchanged (0.00s)
        pd_deploy_kind_test.go:104: kind "Deploy": environment kind must be local, remote or cloud
    --- FAIL: TestPdProjectEnvDeployKind (0.00s)
        pd_project_env_deploy_test.go:15: [add alpha --id staging ... --kind deploy ...]: exit 1: grid project env: environment kind must be local, remote or cloud
    --- FAIL: TestPdProjectEnvUsageNamesDeploy (0.00s)
        pd_project_env_deploy_test.go:45: usage: usage: grid project env list|add|edit|remove <project> [--id ID] [--name N] [--kind local|remote|cloud] ...

On the lane all six PASS. (`TestPdDeployOldKindsUnchanged` fails on base only on its refusal-message check, which now names deploy. Its old-kinds round trip passes on base too, which is the point.)

## What changed

- `internal/projectenv/projectenv.go:30,104,106`: `deploy` is an accepted kind; the refusal names it.
- `internal/cli/ln_project_env.go:15,85`: the usage line and the `--kind` flag help name `deploy`. These are one-line edits, so the `contract/README.md:201` line references still hold.
- `contract/schemas/cmd-project-env.json`: `deploy` is in both `kind` enums.
- Two new test files.

## Open items

- The U-r28-project-rows-5b report names two app gaps, not checked in this lane: E4, the kinds map in the app's `internal/slices/projectsettings/read.go:289` (it needs `deploy` → "Deploy target"); and E6, the Environments tab's Deploy block for kind deploy. Both are app-repo work.
- An older plugin binary refuses a `kind: deploy` entry on read. `List` stops on an unreadable entry, so `grid project env list` from an old binary fails for the whole project until it is upgraded. The same holds for any reader that validates the kind against the old set.
- Under `-race` on this VM, `internal/cli` and `internal/orchestrator` each take more than 45 minutes as whole packages, and `internal/feed` takes about 20. With the default `go test -race ./...` they time out at 10 minutes. This is not caused by this lane. The orchestrator's landing run needs a larger `-timeout` for them.
- `TestSgRotatesAtStart` can miss its 10s startup wait on a loaded box (see A3).
- The design's `state` / `since` (Online, Offline since) are not in the record (U-r28 E5). Not asked.

## NOT VERIFIED on a live screen

No UI changed in this lane. The app drawing a deploy target card from this record needs the app-side kinds map (Open items) and is not verified here.

DONE
