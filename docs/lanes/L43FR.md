# L43FR: an empty task lease is free

Base: plugin main `1871331410e7cdd76cd7cfc1c7642ef508da207f`, after fetching and checking out `lane-L43FR` from `origin/main`. This base already contains L43F and L43G. The existing module and Makefile were retained.

## Changes and scope

- `internal/lease/task.go` recognizes whitespace-only tenure files as empty. Main already restored zero-byte task files as free; both cases now pass status and first take. Nonempty content still goes through contract and lease identity validation. Empty and whitespace-only Job grants remain invalid to both readers.
- `TestEmptyTaskLeaseRecoversCLI` now checks zero bytes and whitespace, exact `task lease free` output, successful ordinary take, and the persisted first epoch `0.1`. The failed-prepare test already expected free on this base; its assertion now also requires task identity and fence zero.
- `internal/cli/feed.go` checks `contract.ValidWorker` using the resolved member and a valid placeholder ULID before calling `feed.MachineID`. The machine API validates the actual ULID. Slash, space and tab member regressions require an error and a completely empty GRID_HOME. This caller is the smallest necessary extension beyond the named lease and identity files: `MachineID` receives no member to validate. The current task acquisition path uses the read-only machine resolver in `orchestratorWriter`, and its existing invalid-member exit-2/no-lease-storage regression still passes.
- Main already routes `-gen` and `--gen` through the same Go flag parser as other command flags. The added regression verifies both separated and equals forms with generation 2 and fence 1; no routing change was needed.
- `docs/lanes/L34F.md` now describes automatic local dead-holder adoption for plain take and distinguishes explicit adoption with an expected epoch.

The existing acquisition regression and Feed test files were extended instead of introducing parallel test helpers. No module recreation, root DONE file, private locations, or attribution lines were added.

## Reproduced failures

Ran the strengthened empty-file CLI regression in a disposable source archive of `lane-L43F` at `152bd2fb384aa3be97c667587611df33266cf19f`, adding only that test. The actual failing lines were:

```text
--- FAIL: TestEmptyTaskLeaseRecoversCLI (0.22s)
    --- FAIL: TestEmptyTaskLeaseRecoversCLI/zero_bytes (0.11s)
        l43fr_regression_test.go:13: empty task status: exit=1 out="" err=grid: lease identity mismatch
        l43fr_regression_test.go:16: empty task take: 1 grid: lease identity mismatch
    --- FAIL: TestEmptyTaskLeaseRecoversCLI/whitespace (0.02s)
        l43fr_regression_test.go:13: empty task status: exit=1 out="" err=grid: EOF
        l43fr_regression_test.go:16: empty task take: 1 grid: EOF
```

Before production changes on plugin main `18713314`, zero-byte recovery and all four generation flag spellings passed. Whitespace and early member validation failed:

```text
--- FAIL: TestFeedWriterInvalidMemberLeavesHomeEmpty (0.00s)
    --- FAIL: TestFeedWriterInvalidMemberLeavesHomeEmpty/bad/member (0.00s)
        feed_test.go:172: accepted invalid member
        feed_test.go:176: invalid member wrote GRID_HOME: [- machine] <nil>
--- FAIL: TestEmptyTaskLeaseRecoversCLI (0.04s)
    --- FAIL: TestEmptyTaskLeaseRecoversCLI/whitespace (0.01s)
        task_acquisition_regression_test.go:34: empty task status: exit=1 out="" err=grid: EOF
        task_acquisition_regression_test.go:37: empty task take: 1 grid: EOF
```

Space and tab member cases failed identically. After the fixes, the targeted lease/CLI run passed, including failed-prepare recovery, both invalid Job lease readers, invalid-member exit 2, and generation spellings.

## Validation

The first `make test` exposed three inherited failures beyond this lane's lease changes:

- `TestTaskLeaseFreshHomeSequence` failed with `task new: 1 grid task new: this caller cannot use the Personal project`. Its fresh-home fixture now uses the owner role because the first task is Personal; the command sequence and all assertions are retained. This one-line change to `internal/cli/task_reachability_test.go` is needed for the required CLI race run.
- `TestPersonalChokePoint` reported `internal/job/jobcmd/trace.go uses gridhome.TasksDirs( outside the caller-policy choke point`. Trace now stamps the environment caller and uses `Resolver.TasksDirs`, with a regression covering owner, member and guest visibility and an explicit Personal filter. The source guard records this caller-stamping boundary. These small changes to trace and its test preserve the guard instead of exempting raw enumeration.
- The same guard reported `internal/workflow/write.go uses where.Resolver{ outside the caller-policy choke point`. Its workflow exception now names the actual writer's project-selection boundary rather than the reader, which no longer constructs a resolver. Workflow write authorization remains owned by the CLI; no workflow production code changed.

The first run also hit `TestTimeoutKillsDescendants`' 100 ms startup window (`child never started`). Its focused rerun passed without a source change. The fresh-home regression passed after the role correction. The first full run reported no new matching temporary entries.

All final checks passed on implementation commit `3d79e3a`:

| Command | Result |
| --- | --- |
| `make test` | Passed; no new matching temporary entries. |
| `go test -race -count=1 ./internal/lease/... ./internal/cli/... ./internal/feed/...` | Passed: lease 3.882 s, CLI 38.516 s, Feed 12.226 s. |
| `make lint` | Passed: vet and repository-wide gofmt check. |
| `GOOS=windows go build ./...` | Passed. |
| `GOOS=darwin go build ./...` | Passed. |
| `git diff --check origin/main` | Passed. |

Windows and Darwin validation was cross-compilation, not runtime execution.

Implementation commits are `39bb9c8` (empty leases and flag compatibility), `9835a93` (member validation before machine creation), and `3d79e3a` (Personal-policy integration fixes). The final documentation commit records validation and corrects L34F's adoption rule. All commit messages are one sentence without attribution lines.
