# L-job-source-resolve-tests: behaviour tests for resolving a Job's sources

Repo: plugin, branch `lane-L-job-source-resolve-tests`. One new file: `internal/job/source_resolve_test.go` (package `job_test`). `source.go` is not touched.

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Brain pointers: accepted note returns its text with the tier the code assigns | HOLDS | `TestResolveBrainPointerReadsAnAcceptedNote` (real `brain.Store` via jobtest): text, `Tier == contract.Agent`, State, Topic, Summary, sha256 digest, `#rollback` anchor, `#L2-L2`, `:2-2`, missing anchor. `TestResolveBrainExcerptTierIsTheStoresNotTheText`: tier is Agent for origin session/job/server/import and for a body that claims `tier: person-team`. `go test -count=1 -run TestResolve ./internal/job/` exit 0 |
| 1 | `brain:team/x` errors "expected brain:<home\|project>/<name>" | HOLDS | `TestResolveBrainPointerRefusals`: six malformed ids, and the fake store records that it was never asked (source.go:155) |
| 1 | No brain store errors "no brain store is configured" | HOLDS | same test (source.go:158) |
| 1 | Proposed note errors "not accepted" | HOLDS | same test: "note is proposed, not accepted" and "note is rejected, not accepted" (source.go:166), body marker never in the error. Also `TestResolveBrainPointerRefusesAProposalInTheRealStore` with the real store |
| 1 | `superseded_by` errors "superseded by <id>" | HOLDS | same test: "note is superseded by brain:project/new" (source.go:168-169) |
| 2 | `plan.md` on a task with none errors and `errors.Is(err, fs.ErrNotExist)` | HOLDS | `TestResolveTaskFilePointers/a_task_with_no_plan.md...`: six spellings, "this task has no plan.md" (source.go:209, :219); a repo's `plan.md` and `research.md` keep the system error. Overlaps `plan_missing_test.go` by design; the brief names the row |
| 2 | Path outside the project folders errors "outside the project folders" | HOLDS | `.../a_repo_or_prototype_file_outside_the_project_folders_is_refused`: `repo:`, named `repo:side:`, `prototype:`, with no folders and with another folder (source.go:214); same pointers read once the folder is the project's |
| 2 | A directory errors "not a regular file" | HOLDS | `.../a_directory_is_not_a_regular_file` (source.go:226). FIFO is already covered by `source_fifo_unix_test.go` |
| 2 | Symlink out of the task folder refused by `Within` (skip if no symlinks) | HOLDS | `.../a_symlink_out_of_the_task_folder_is_refused_by_Within`: "source escapes its root" for a file link and a directory link, direct and through Resolve, the outside text never appears, an inside link still reads. `t.Skipf` with the reason when `os.Symlink` fails (Windows) |
| 3 | `notes.md#L2-L3` returns exactly lines 2-3 | HOLDS | `TestResolveRangesAndAnchors/selected_lines`: `#L2-L3`, `:2-3`, `#L2-3`, `#L4`, `#L1-L5`, `#L5`, CRLF file |
| 3 | `#L9-L12` on a 5-line file errors "range outside file (1-5)" | HOLDS | `.../a_range_outside_the_file_names_the_file's_size`: also `:9-12`, `#L6`, `#L5-L6`, `#L0-L1`, `#L3-L2` (source.go:381-382) |
| 3 | Anchor heading that appears twice errors "ambiguous anchor" | HOLDS for a repeat nested in its own section; **NOT DONE for the same-level repeat: bug found, assertion left out** | `.../an_anchor_heading_that_appears_twice_inside_its_own_section_is_ambiguous` passes. See "Bug found" |
| 3 | Missing anchor errors "not found" | HOLDS | `.../a_missing_anchor_is_not_found` (source.go:413); an empty `#` reads the whole file |
| 3 | Range and anchor together error "cannot combine range and anchor" | HOLDS | `.../a_pointer_with_both_a_range_and_an_anchor_is_refused` (source.go:86) |
| 3+ | Anchor selecting lines outside the approval digest (source.go:280) | HOLDS | `TestResolvePlanAnchorMustLieInsideTheApprovalDigest`: `#where-it-stands`, `#L5-L6`, `#L6`, `:5-6`, `#L4-L6`, `./plan.md` and a symlink alias are refused; covered `#L3-L4` and `#verify` read; the same anchor on `research.md` reads |
| 3+ | `taskTier` (source.go:138) | HOLDS | `TestResolveTaskFileTiers`: owner tier for `intent.md` (`./intent.md`, `task:intent.md`), external when the owner is unknown, Agent for `plan.md`, external for `research.md` and `sub/intent.md` |
| 4 | `job:T-02/J-1` with no index resolver errors "cross-task Job requires an index resolver" | HOLDS | `TestResolveJobPointers`: also `T-7`, `X-07`, with an anchor (source.go:111; `taskNumber` source.go:299 through task folder `T-07`) |
| 4 | `job:<this task>/bad id` errors "invalid Job id" | HOLDS | same test: eight bad ids including `J-1x`, `../plan.md`, `J-1/../J-2` (source.go:116). Own-task `job:J-1`, `job:T-07/J-2.1` read with tier Agent |
| 5 | `Within`: empty root errors "source root is not configured" | HOLDS | `TestWithin` (source.go:311) |
| 5 | `Within`: `../x` errors "must stay inside their root" | HOLDS | `TestWithin`: `../x`, `a/../../x`, `..`, empty, rooted, absolute (source.go:313-314); missing file and root return `fs.ErrNotExist`; a link inside the root resolves, one out is refused |
| 6 | `go test -count=1 ./internal/job/`: 0 fail | HOLDS | `go test -count=1 ./internal/job/` -> `ok ... 4.116s`, exit 0 |
| 6 | `GOOS=windows CGO_ENABLED=0 go vet ./internal/job/` exit 0 | HOLDS | exit 0; `GOOS=darwin CGO_ENABLED=0 go vet ./internal/job/` exit 0 |

## Rules

| Rule | Result | Proof |
|------|--------|-------|
| New tests fail on base, pass on the lane | HOLDS | The lane adds only tests and `source.go` is unchanged, so base has no such tests. To show they are load-bearing I mutated `source.go` (reverted with `git checkout internal/job/source.go`) and the new tests failed. Failing lines below |
| No existing test edited or deleted | HOLDS | `git diff origin/main --stat` shows one added test file and this report; nothing deleted |
| No guard, gate, cap, hold or hook added | HOLDS | tests and a report only |
| Tests of changed package and importers | HOLDS | `go test -count=1 ./internal/job/` ok. Importers (`go list` of Imports, TestImports and XTestImports naming `internal/job`): `contract/formats`, `internal/cli`, `internal/job/jobcmd`, `internal/job/lint`, `internal/jobgraph`, `internal/lease`, `internal/orchestrator`, `internal/plan`, `internal/ports`: all `ok`, exit 0 |
| Lint | HOLDS | `make lint` (go vet ./... and gofmt -l .) exit 0 |
| Windows and macOS builds | HOLDS | `make cross` (vet and test compile for darwin/arm64 and windows/amd64) exit 0 |
| Small commits, one-sentence messages, no attribution | HOLDS | `git log origin/main..HEAD` |
| No personal paths, hosts or emails | HOLDS | tests use `t.TempDir()` only |

### Failing lines with `source.go` mutated

One mutation at a time per behaviour, run together except the last:

- `n.Status != "accepted"` -> `false`: `source_resolve_test.go:157: want an error containing ["brain:project/proposed:" "note is proposed, not accepted"], got none`
- `n.SupersededBy != ""` -> `false` (alone): `source_resolve_test.go:161: want an error containing ["brain:project/old:" "note is superseded by brain:project/new"], got none`
- the range and anchor refusal -> `false`: `source_resolve_test.go:403: want an error containing ["notes.md:2-3#intro:" "cannot combine range and anchor"], got none`
- the `Within` escape check -> `false`: `source_resolve_test.go:279: Within(file.md) = "/tmp/.../secret.md", <nil>` and `source_resolve_test.go:518: a link out of the root: "/tmp/TestWithin.../out.md" <nil>`
- `isPlan && !plan.DigestCoversLines` -> `false`: `source_resolve_test.go:437: want an error containing ["plan.md#where-it-stands:" "anchor selects lines outside the approval digest"], got none`
- the project-folders check -> `false`: `source_resolve_test.go:231: want an error containing ["repo:src/a.go:" "source is outside the project folders"], got none`
- the cross-task check -> never: `source_resolve_test.go:463: want an error containing ["job:T-02/J-1:" "cross-task Job requires an index resolver"], got none`
- brain tier `Agent` -> `PersonTeam`: `source_resolve_test.go:190: origin session: tier "person-team", want "agent"` (four origins) and `:197` for the home note

## What changed

- `internal/job/source_resolve_test.go`: eight test functions in `package job_test`, all through the exported `Files.Resolve`, `Within`, with helper names prefixed `sr` so they do not collide with `render_test.go`. A fake `NoteReader` records what the store was asked, so malformed ids are shown never to reach it. One real-store test uses `jobtest.New`.
- `docs/lanes/L-job-source-resolve-tests.md`: this report.

## Bug found (NOT DONE, no assertion written)

`internal/job/source.go:396` vs `:404-406`: in `selectRange`, a heading at the same or a higher level than the matched one ends the section (`:396`) before it can be tested as a duplicate, so two sibling headings with the same slug are never reported as ambiguous. `## Notes\nfirst\n## Notes\nsecond\n` with `#notes` returns `## Notes\nfirst` and no error (probed, then the probe removed). Only a repeat nested deeper than the match (`## Plan` ... `### Plan`) reaches "ambiguous anchor". A Job pointer to a repeated sibling heading therefore silently takes the first section. The brief forbids changing `source.go`, so this is left for the owner; the test file carries a comment at the spot instead of an assertion.

## NOT VERIFIED on a live screen

Nothing here has a screen. NOT RUN: the symlink cases on a real Windows machine (they `t.Skipf` when `os.Symlink` fails, and `make cross` only compiles the Windows test binary). The whole-module test and the cross-repo set are the orchestrator's.
