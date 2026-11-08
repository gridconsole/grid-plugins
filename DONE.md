# Lane L33: test temp hygiene

Audited all Go tests, shared test helpers, testdata and both golden harnesses.
The remaining manual Go test-helper allocation was in `testgit.TempInRepo`;
it now uses testing-owned storage while preserving fixtures inside a Git repo.
No `os.MkdirTemp`, `ioutil.TempDir` or `os.CreateTemp` calls remain in Go tests
or test helpers. The Python recorder already uses `TemporaryDirectory` and
passes a contained TMPDIR to its hook subprocesses.

Added `internal/testenv` to pass TMPDIR, TMP and TEMP under the parent test's
`t.TempDir()` to Git, hook runners and child test processes. In-process eval
and golden CLI tests use `t.Setenv` to contain operating-system temp allocations.
Started test children are killed and waited for during cleanup, including early
failure paths. A regression test proves cleanup after a child exits abruptly.

Added `make test-clean`, included in `make test`, to snapshot the default system
temp folder before and after `go test -count=1 ./...`. It rejects new entries
with every requested prefix, plus `grid-outer-` and `grid-eval-`, without deleting
existing entries or reported leaks. Tests cover files, directories, existing
entries, failed runs, scan errors and symlinked temp roots. The wrapper measured
0.13 seconds with `go version`; the two directory scans rounded to 0 milliseconds.

Run from the checkout without overriding TMPDIR:

```sh
make test
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

All commands passed. The final two consecutive test runs used the default system
temp folder with TMPDIR unset. An independent full directory comparison found
zero new entries after both runs, including entries outside the guard's prefixes.
Optional legacy differential hook checks passed against the read-only reference;
the optional 125-writer Feed stress test passed with one record per writer. Both
also passed the temp leak guard.

There are no intended product behavior differences from the old plugin in this
lane, and no production code or golden expectations changed. Test infrastructure
differences are parent-owned subprocess storage and uncached leak-checked runs
(`README.md:15`), plus temporary process TMPDIR isolation for the Go golden CLI
adapter (`internal/golden/README.md:16`). The reference recorder's existing case
isolation remains unchanged (`recordings/README.md:28`, `recordings/README.md:34`).

Native macOS and Windows execution was unavailable on this Linux worker; both
cross-compilation checks passed, and the guard's symlink-root test passed here.
The guard observes a shared folder, so unrelated concurrent tests using the same
prefixes can trigger it; run those checks separately (`README.md:18`). Historical
temp entries and the read-only reference checkout were left untouched.
