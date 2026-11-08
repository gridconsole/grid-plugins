# Lane L37

Implemented `grid job verify J-N [--task T-NN] [--dir path] [--json]` and `grid doctor --bundle [--out path]` on the existing CLI, Job, task, Grid home, version, lock and atomic-file packages. No module or dependency was added.

Verification loads `jobs/J-N/job.yaml` when present, otherwise the existing `job.LoadPlan`. `job.RunVerify(ctx, job, dir)` is the reusable entry point for the later L0 code check; `Verify` was already the name of the configuration type. The result includes pass, command exit code, nullable test count, the starting HEAD commit, a one-line failure reason, and bounded captured output. A minimum-count failure leaves the command's actual exit code intact while the CLI exits 1. Reviewer checks exit 3 and require a reviewer rather than claiming a pass.

The directory override wins over a recorded worktree, which wins over the current directory. Named worktrees resolve below the task project's Grid home worktrees directory; multiple recorded worktrees require `--dir`. Both the existing `path` field and a `paths` list work for file checks. Relative paths use the selected directory; `task:` pointers select proof files in the task folder. Files must be regular files. No Job, task, Feed or verdict is modified.

Commands run through `/bin/sh` on Unix and `cmd.exe` on Windows. Cancellation kills the Unix process group; Windows uses `taskkill /T /F`. Unix cleanup also kills descendants left holding output pipes after their shell exits. Counts recognize Go verbose or JSON events, Node TAP/spec summaries, Cargo summaries and pytest summaries, including multiple package/suite summaries. Go subtests and JSON output events are not double-counted. Plain Go package summaries cannot establish a test count. Unrecognized output or oversized lines produce unknown totals and fail a positive minimum. Captured output is limited to 64 KiB while counting continues. Counts reflect reported tests, including skipped/ignored tests, excluding pytest deselections.

Bundles run the existing doctor even for an unhealthy installation. A successful collection exits 0; doctor findings are stored in `doctor.txt` and also available with `--json`. The default destination is `$GRID_HOME/support/grid-support-<UTC timestamp>.zip`. The zip and its members have mode 0600. Existing output files and symlinks are refused. Publishing uses the existing fslock and atomicfile packages.

Each zip contains `doctor.txt`, `versions.txt` (grid, contract and app_min), the newest 20 non-crash regular files recursively under `$GRID_HOME/logs`, and the newest `crash-*.txt` there. Selection uses modification time with a filename tie-break. Logs retain at most the final 5 MiB each. Symlinks and special files are excluded. Truncation discards a partial first line, including an entire oversized single-line log, so a cut token cannot leak. PEM state is tracked through the discarded prefix. Bearer tokens, long sk- keys, JWS-shaped strings, PEM blocks and the user's home path are redacted from content and entry names before zip creation. JWS matching is conservative and can remove ordinary dotted text; numeric version strings are retained. There is no upload or network client in bundle collection.

Run the required checks from the repository root, without changing TMPDIR:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

Tests cover successful and failed verification, minimum counts, unknown totals, each supported runner format, file checks, reviewer handoff, saved metadata and plan fallback, directory overrides, named worktrees, HEAD reporting, cancellation of descendants, redaction, truncation across PEM/token boundaries, archive contents and permissions, existing-output refusal, and collection without network requests. The full suite also runs the repository's temporary-file leak check.

Intentional differences from the old plugin and reference prototypes:

- The old brief lint checked the presence of a `verify` field and headings (`old-plugin/hooks/lib.mjs:34`, `old-plugin/hooks/brief-lint.mjs:34`); its stop check inspected Outcome and Verdict text (`old-plugin/hooks/stop-check.mjs:84`). This command executes structured verification and enforces reported counts, as required by `docs/job-contract.md:41`, `docs/job-contract.md:68` and `docs/job-contract.md:124`.
- Non-code checks are explicit file checks or a reviewer handoff with exit 3, following `docs/job-contract.md:71`. Verification reports evidence without marking old brief status or writing a Verdict; integrating the reusable function into L0 and stop hooks is separate work (`docs/job-contract.md:124`, `docs/plugin-design.md:90`). The prototype's `grid job verify` action is implemented as a CLI, with no prototype UI changes (`prototype/task.html:5144`, `prototype/task.html:5200`).
- Support collection is a new verb with no old counterpart (`old-plugin-map.md:118`), implementing F-8 (`docs/plugin-design.md:119`). It is local only, matching `docs/go-architecture.md:386` and `prototype/settings.html:4853`.
- This lane's requested bundle scope is deliberately narrower than `docs/go-architecture.md:386` and `prototype/settings.html:4843`: diagnostics, CLI/contract/app_min versions, capped local logs and the newest crash. It does not collect task folders, Feed, `.live`, installed app/agent versions, or provide a file-list/send UI. It uses the requested support subdirectory and timestamped filename instead of the prototype's root-level filename (`prototype/settings.html:4858`).
- The lane's newest-20 and per-file 5 MiB limits replace the prototype's last-seven-days selection (`prototype/settings.html:4843`). Partial first lines, oversized single-line logs, symlinks and special files are omitted to keep collection bounded and prevent secret leakage. Redaction uses the specified patterns rather than adding an unrelated scanner or service (`docs/go-architecture.md:386`).

Limits and unavailable validation: Linux tests and both cross-builds were run here. Native macOS and Windows runtime tests were not available. Windows compiles, but bundle writing inherits the existing fslock package's unsupported-platform result; Windows locking is outside this lane and the fslock package was not changed. Plain `go test` without verbose/JSON output cannot prove `min_tests`, so use `go test -v` or `go test -json`. L0 and hook integration, UI rendering, upload, and broader support collection were not implemented because this lane explicitly requests the reusable function and narrower local bundle only.
