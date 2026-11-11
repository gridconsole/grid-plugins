# C-pcov5-job-source-reads

Test-only lane: one new file, `internal/job/source_reads_test.go` (package `job_test`), one test per row, helper prefix `jsr`. It reuses `srWrite`, `srTask`, `srNotes` and `srNote` from `source_resolve_test.go`.

## Proof table

| Row | Result |
|---|---|
| 1. Coverage before | HOLDS. `go test -count=1 -coverprofile=.jsrcov/cov.out ./internal/job/` exit 0, `coverage: 93.9% of statements`. |
| 2. Note links (source.go:186-187) | HOLDS. `TestResolveBrainNoteCopiesItsLinksOntoTheSource` (source_reads_test.go:27): Links equal `[{brain:project/other uses because}]`. Exit 0. |
| 3. Front matter error (source.go:265-266) | HOLDS. `TestResolveTaskFileWithBrokenFrontMatterIsRefused` (:42): error starts `bad.md: frontmatter:`. Exit 0. |
| 4. Title fallback (source.go:269-271) | HOLDS. `TestResolveTaskFileSummaryFallsBackToItsTitle` (:52): Summary `The title`. Exit 0. |
| 5. Not a heading (source.go:399-400) | HOLDS. `TestResolveAnchorSkipsAHashLineThatIsNotAHeading` (:65): `h.md#nospace: anchor "nospace" not found`; `h.md#real` gives `# Real\nreal body`. Exit 0. |
| 6. taskNumber no dash (source.go:309) | HOLDS. `TestResolveJobPointerForATaskFolderWithoutADashNeedsAnIndexResolver` (:83): `job:T-07/J-1: cross-task Job requires an index resolver`. Exit 0. |
| 7. plan.md unreadable (source.go:446-447) | HOLDS. `TestLoadPlanNamesAPlanThatCannotBeRead` (:93): error starts `read ` and contains `<sep>plan.md` (last element only, so a symlink-resolved temp path passes). Exit 0. |
| 8. Dependency (job.go:128-129) | HOLDS. `TestDependencyUnmarshalJSONRefusesABrokenString` (:109): error contains `invalid escape sequence`, d stays zero. Exit 0. |
| 9. Coverage after | HOLDS. `go test -count=1 -coverprofile=.jsrcov/cov2.out ./internal/job/` exit 0, `94.7% of statements` (before 93.9%). cov2.out count 1 for: job.go:128.4,129.1; source.go:186.3,187.1; :265.4,266.1; :269.4,270.1; :307.3,308.1 and :309.2,309.11; :399.4,399.12; :446.3,447.1. |
| `go test -count=1 ./internal/job/` | HOLDS, exit 0. |
| `make lint` | HOLDS, exit 0 (runs `go vet ./...`). |
| `GOOS=windows go vet ./...` | HOLDS, exit 0. |
| macOS build | HOLDS. `GOOS=darwin go vet ./internal/job/...` exit 0. |
| Importers' tests | HOLDS, exit 0: `go test -count=1 ./contract/formats ./internal/cli ./internal/hook ./internal/job/... ./internal/jobgraph ./internal/lease ./internal/orchestrator ./internal/plan ./internal/ports`. The list is from `go list` over Imports, TestImports and XTestImports of internal/job: contract/formats, internal/cli, internal/hook, internal/job (its own xtests), internal/job/jobcmd, internal/job/lint, internal/jobgraph, internal/lease, internal/orchestrator, internal/plan, internal/ports. |
| gofmt | HOLDS. `gofmt -l internal/job` prints nothing. |
| New tests fail on base | NOT DONE, by design. This is a test-only coverage lane: the code already does what each row says, so the tests pass on base too. They are the first tests to reach those blocks (the blocks had count 0 before). |
| No guard, gate, cap or hold on agents | HOLDS. Nothing but a test file is added. |
| No change outside the new test file | HOLDS, apart from this report. No approval.json, no jobtest Approve/TryApprove. |
| Portability | HOLDS. No chmod, Mkfifo, syscall or Symlink. OS error wording is not asserted; paths are asserted by prefix and last element. |

## What changed

- Added `internal/job/source_reads_test.go` with seven tests and one helper, `jsrNamedTask`.
- Added this report.

## NOT VERIFIED on a live screen

Nothing here touches a UI.
