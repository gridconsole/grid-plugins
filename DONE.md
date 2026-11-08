# Lane L1 completion

Built 26 JSON Schema 2020-12 documents for all 19 current Feed post types,
the envelope, Job record, leases, heartbeat file, takeover entry and trust tier.
There are 118 JSON fixtures: 54 valid and 64 invalid, with at least two of each
per schema. Invalid filenames identify the rule being tested.

The standard-library-only `internal/contract` package provides embedded schemas,
a minimal validator, typed post and Job fold structures, `Fold(posts)`, and merge
lease expiry checks. The canonical assets are embedded once by `contract/embed.go`
and imported by the internal package. The existing module and Go 1.27 declaration
were preserved.

Table-driven tests cover every fixture and Job assignment, ack, retired claim and
heartbeat posts, explicit stale controls, results, handover, retry, usage-limit
reoffers, checkpoints, exit reasons, trust restrictions, question-answer selection,
unknown versions, dependencies, duplicate IDs, generation/fence checks, and
interleaving-independent state. Merge leases are per project, expire fifteen
minutes after grant without renewal, and may be broken once stale. Explicit
renewal is represented by the documented optional renewed_at field.

`contract/README.md` contains one type-to-schema-to-source table, source line
references, disagreements, unspecified details and chosen defaults. A small Git
discovery-ceiling fix isolates the existing project-resolution tests from ancestor
checkouts; it has its own regression test and commit.

Run from the repository root:

```sh
make test
make lint
```

Both passed. `make test` runs `go test ./...`; `make lint` runs `go vet ./...` and
the formatting check. In this sandbox the default Go cache is read-only, so the
successful checks used checkout-local cache and temporary directories:

```sh
mkdir -p .contract-cache .contract-tmp
GOCACHE="$PWD/.contract-cache" TMPDIR="$PWD/.contract-tmp" make test
GOCACHE="$PWD/.contract-cache" TMPDIR="$PWD/.contract-tmp" make lint
```

No requested schema, fixture or listed fold scenario remains blocked. The real
filesystem/concurrency and macOS durability spike fixtures were not run: this
lane implements record contracts and an in-memory fold, not a Feed file writer,
reader or worker supervisor. The caller must verify writer filenames, admission
and takeover offsets, resolve body references, and supply authorized tenures.
Liveness polling, sleep/wake detection, ready scheduling, gate timers/CI and
crash-loop scheduling require those external components. The fold deliberately
never infers stale from a clock, and a schema cannot authenticate a stamped role.
