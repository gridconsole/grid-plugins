# L37 rework completion

All four must-fixes and six listed should-fixes from the L37 review are implemented with regression tests. The original lane report has moved to [docs/lanes/L37.md](docs/lanes/L37.md), with a rework section and validation results.

Built:

- Strict Go package-summary parsing, a captured real Node TAP fixture including failing and nested tests, a real Go test printing `ok ...`, and pytest warning/rerun/skip/xfail/error coverage.
- Exported `job.WorktreePath` for flat `name@sha` resolution. `job.RunVerify` now owns worktree selection, relative overrides and `task:` proofs through `VerifyOptions`; the CLI supplies context and prints evidence.
- Bundle hard-link exclusion before selection and again on the opened file, with the existing fslock moved under Grid support so a custom output folder contains only its archive.
- Windows home redaction for JSON escaping, case variants and shell-style paths; targeted secret/email redaction that preserves hostnames, dotted filenames and slugs.
- Windows cmd.exe quoting with tests for embedded quotes and paths containing spaces.
- Advisory plan-check warnings, in text and JSON, for code Jobs whose plain Go/make test commands may not report counts.

Run acceptance checks from this folder, using the default TMPDIR:

```sh
make test
make lint
go test -race ./internal/job/... ./internal/doctor/... ./internal/cli
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

All passed on Linux at implementation commit `8361c3c`. The full suite reported 41 passing packages, zero failures and six packages without tests; its temporary-file leak check passed. Race tests reported six passing packages, zero failures and one package without tests. Both cross-builds and lint passed. Later commits contain documentation only.

Intended differences from the old plugin and reference prototypes, using reference-specification paths and line numbers:

- Structured verification executes commands and enforces reported counts instead of checking only a verify field, headings, Outcome and Verdict text (`old-plugin/hooks/lib.mjs:34`, `old-plugin/hooks/brief-lint.mjs:34`, `old-plugin/hooks/stop-check.mjs:84`; `docs/job-contract.md:41`, `docs/job-contract.md:68`, `docs/job-contract.md:124`). Go verbose/JSON counts are required to prove a positive minimum; plain Go/make commands now get a planning warning (`docs/job-contract.md:136`).
- File checks and reviewer handoff are explicit, with exit 3 for reviewer checks. Verification returns evidence without changing task state or writing a Verdict (`docs/job-contract.md:71`, `docs/job-contract.md:124`, `docs/plugin-design.md:90`). The CLI implements the prototype action without UI changes (`prototype/task.html:5144`, `prototype/task.html:5200`).
- Worktrees use the flat path and remove commit metadata as required by the review and `docs/job-contract.md:53`. This deliberately does not use the extra repository directory described in `docs/job-contract.md:70`; scalar and repository-keyed records share one resolver.
- Support bundles are new local-only functionality (`old-plugin-map.md:118`, `docs/plugin-design.md:119`, `docs/go-architecture.md:386`, `prototype/settings.html:4853`). Collection includes diagnostics, CLI/contract/app_min versions, capped logs and the newest crash, rather than task folders, Feed, `.live`, app/agent versions or a send UI. The requested support subdirectory and timestamped filename replace the prototype's root-level filename (`prototype/settings.html:4843`, `prototype/settings.html:4858`).
- The newest 20 logs and a 5 MiB tail per file replace the prototype's seven-day selection. Partial first lines, oversized single lines, symlinks, hard links and special files are omitted. Targeted local redaction removes secrets, emails and home paths while preserving useful diagnostics; no scanner service or upload is added (`prototype/settings.html:4843`, `docs/go-architecture.md:386`).

Unavailable work and validation: L34's orchestrator is not present in this checkout, so its call site could not be updated; it can adopt `job.WorktreePath` and `RunVerify` with task/Grid context. Native macOS and Windows runtime tests are unavailable on this Linux host; their builds pass. Windows bundle publication still inherits unsupported locking from the protected fslock package, which was not changed. Platforms without a link-count implementation omit logs conservatively. L0/hook integration, UI, upload and broader support collection remain outside the requested lane scope.
