# P-job-from-brief: proof

Lane head: see the report; base merged: origin/main at 885dc9a6 (merge b69fc26f).

| Row | Verdict | Proof |
|---|---|---|
| 1. `grid job post --task T --from-brief b.md` posts a record that lints with 0 errors | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestJb.*Brief'` exit 0: `TestJbJobPostFromBrief/{P-land-kind,C-ak-open-home-without-task,R-land-job-type}.md` PASS; the test fails on any finding with severity error (internal/cli/jb_job_from_brief_test.go:73) |
| 2. The brief is copied into the task's inputs and intent points at its lines | HOLDS | same run; inputs/<name> is byte-equal to the brief (jb_job_from_brief_test.go:79-83); intent.owner names the `Why:` line, intent.item the `# ` title line, intent.goal the numbered rows, one line per acceptance row (jb_job_from_brief_test.go:86-106); `read` names `task:inputs/<name>:1-N` |
| 3. model, effort and base reach the spawn line; a Job without them keeps the role's values; other spawn lines byte-for-byte unchanged: test | HOLDS | `go test -count=1 -v ./internal/orchestrator -run TestJbSpawn` exit 0: `TestJbSpawnLineCarriesTheJobModelEffortAndBase` PASS (own values win, missing ones keep the role's), `TestJbSpawnLineOfAPlainJobIsUnchanged` PASS (plain line equals main's struct marshalled in main's key order); `TestJbFromBriefCarriesModelEffortBase` PASS (brief -> record) |
| 4. Golden test on 3 real briefs (one P-, one C-, one R-), scrubbed | HOLDS | internal/cli/testdata/jb_from_brief/{P-land-kind,C-ak-open-home-without-task,R-land-job-type}.md and .golden.json; compared at jb_job_from_brief_test.go:107-125; `grep -rnEi "/srv|/home|/root|@host|grid[0-9]|hetzner"` over the folder: no match |
| 5a. go test of changed packages and importers | HOLDS | changed: contract, internal/contract, internal/job, internal/orchestrator, internal/cli; `go list -f '{{.ImportPath}} {{.Imports}}' ./...` shows internal/contract and internal/job imported by nearly the whole module, so the whole module was run: `go test -count=1 -timeout 60m ./... 2>&1 | grep -E "^(FAIL|--- FAIL|panic)|_test.go:[0-9]+:"` printed nothing (grep exit 1, no match), on head b26b9d73. Separately: `ok internal/job 20.196s`, `ok internal/cli 354.330s`, `ok contract 1.337s`, `ok contract/formats 2.329s`, `ok internal/contract 9.311s`, `ok internal/orchestrator 790.943s` (needs more than the 10m default timeout, on its own) |
| 5b. contract tests | HOLDS | `ok contract`, `ok contract/formats`, `ok internal/contract` above; contract/commands.json regenerated with `make generate` for the new flag (TestContractCommandsParserInventory passes in the cli run) |
| 5c. make lint | HOLDS | `make lint` (go vet ./...) exit 0, no output |
| 5d. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList' -v`: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok internal/cli`; `go test -count=1 ./internal/job/lint/`: `ok internal/job/lint 1.438s` |
| 5e. Merge main before the proof | HOLDS | `git fetch origin && git merge origin/main` -> b69fc26f |
| Rule: new tests fail on base | HOLDS | production files reset to origin/main (`git checkout origin/main -- ...`, restored after, `git status --short` clean): `TestJbSpawnLine*`: `$.body: oneOf matched 0 branches: $.body: unknown property base`; `TestJbJobPostFromBrief`, `TestJbFromBriefCarriesModelEffortBase`: `flag provided but not defined: -from-brief` |
| Rule: no existing test edited, none deleted | HOLDS | `git diff --stat origin/main..HEAD`: only new *_test.go files |
| Rule: no guard, gate, cap, hold or blocking hook | HOLDS | none added; the budget is copied from the brief into the record as today's `budget` field |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Rule: no attribution, no private paths in committed files | HOLDS | commit messages have no trailers; testdata scrub grep above |
| make realplugin / app guest walk tables | not run, not asked (plugin lane) |
| Whole-module test and cross-repo set | run by the orchestrator at landing; whole module also run here (5a) |

## What changed

- `grid job post --task T --from-brief b.md` (internal/cli/orchestrator.go): copies the brief into `<task>/inputs/` (never overwrites; a different file of the same name gets `b-2.md`), builds the body with `job.FromBrief`, takes the next free Job id unless `--job` is given, and posts it through the normal post path (lint findings printed). `--from-brief` with `--body`, `--person` or another verb exits 2.
- internal/job/frombrief.go: brief -> job record per the brief-to-job-record convention: `Repo:` -> repo (kept only when the project lists it; else said on stderr) and base (`from <branch>`), `Model: m, e` -> model, effort, numbered rows under Acceptance (or Questions) -> A1.., `Touches:` -> write, `Never:` -> out_of_scope, `About N calls` -> budget (default 120), `Skills:` -> skills, R- lanes get role researcher, verify is file-exists on the brief's `Write docs/...md` or docs/lanes/<lane>.md.
- contract/schemas/job-record.json: optional `base`, `model`, `effort`; docs/job-contract.md rows for them.
- internal/contract (fold, types), internal/job/job.go: carry the three fields; internal/orchestrator (store, tick, stream): the spawn line takes the Job's model and effort over the role's when set, and adds `base` when set (omitempty, so other lines are unchanged).
- contract/commands.json: regenerated (forced by the new flag).

Files outside the brief's two named files: internal/contract, internal/job/job.go, internal/orchestrator, docs/job-contract.md and contract/commands.json are forced by "carried to the spawn line" and by the contract doc and inventory tests.

## NOT VERIFIED

- Nothing needs a live screen. The worker that reads the spawn line does not yet use `base` to pick its clone's branch; that is P-run-jobs.
