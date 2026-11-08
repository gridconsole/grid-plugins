# Lane L5: Feed in Go

Built `internal/feed` and the `grid feed post`, `read`, `state`, `types`, and `handover` commands. The existing `fold` entry is an alias for `state`. The only change in `internal/cli/cli.go` is the Feed command-table registration; `internal/where`, `internal/project`, `go.mod`, and the Makefile are unchanged.

The store writes append-only, per-tenure JSONL files with process-safe `flock`, shared lease locks, per-file sequence allocation, retry IDs, torn-tail repair before every write attempt, and a single OS write per record. State posts are synced before acknowledgment. Bodies exceeding the 32 KiB line limit are stored separately and synced. Linux uses fsync; macOS uses F_FULLFSYNC with filesystem-unsupported fallback. Unsupported platforms and known network filesystems are refused.

The reader retains offsets and arrival times in `feed/.cache/reader.json`, ignores incomplete tails, counts bad lines, checks envelope/file identity, deduplicates IDs, holds missing references, and rechecks takeover offsets on every pass. The fold counts task controls in generation/fence/file order and Job writes only against their assigned tenure. It retains questions, answer candidates and explicit selections, Job state, results, checkpoints, exits, thread references, unknown records, and stale verdicts. `state` publishes `state.json` and `needs-you.json`; lead wake candidates are separate from human questions.

Nineteen embedded type registrations carry schemas, versions, classes, and author allowlists. Validation includes lease requirements, assign/ack, Job dependency kinds/reasons, guest addressees, feedback restrictions, external knowledge, result verification and must-fix rows, checkpoint fields, usage amounts, and exit reasons. Guest/external bodies are preserved as data. Read JSON includes derived `tier` and `data` fields; plain output uses the fixed data warning and escaped JSON inside a marked data fence.

## Use

The selected task must already exist under the Grid home, with a stable ULID in `task.md` frontmatter (`id`, or `ulid` alongside a display `id`). The Grid home's `VERSION` must match the existing format version. These commands use the existing task resolver.

```sh
make build
./dist/grid feed types --json
./dist/grid feed post --task T-07 --type post \
  --body '{"ref":"task","kind":"message","text":"Example feedback"}'
./dist/grid feed post --task T-07 --type knowledge --body - < note.json
./dist/grid feed read --task T-07 --type result --job J-1 --json
./dist/grid feed read --task T-07 --since 2026-11-08T00:00:00Z
./dist/grid feed state --task T-07 --json
./dist/grid feed handover --task T-07 --lease task --lease-epoch 0.1 \
  --reason 'Continuing in another session'
```

The handover example requires a matching lease already granted by the lease component. It does not take a lease. `--body` can supply `from` or `adopted` information. `--id` reuses a post ID on retry; `--after` takes comma-separated record ULIDs; `--attempt`, `--job`, `--re`, and `--wake` provide the other envelope references.

Writer identity comes from the trusted session: `GRID_MEMBER`, else Git's configured email, else the OS account; `GRID_ROLE` defaults to `member`. The persistent machine ULID is created on the first Feed write. `GRID_LEASE`, `GRID_LEASE_EPOCH` (`gen.fence`), and `GRID_WAKE` provide defaults for their flags. Role fields inside a body never change author identity. These local records are not signed, and the invoking session must control its environment.

Exit codes are 0 for success, 1 for validation/operation errors, 2 for CLI usage errors, 3 for stale leases, and 4 for disk-full failures. The contract names the last two conditions without assigning numbers; these are the Feed implementation's numbers.

Plain output uses dotted field names and array indices, one fact per line, with strings escaped. JSON carries the same names and values. The fixed data warning/fence is the intentional multiline exception for untrusted bodies.

## Tests and verification

```sh
make test
make lint
go test -race ./internal/feed ./internal/cli
go test ./internal/feed -run 'TestConcurrent(Goroutines|Processes)$' -count=1
```

The append tests use 20 goroutines with 10 records each and four independent processes with 12 records each, all sharing one writer file. They assert record counts, unique IDs and payloads, intact lines, and contiguous per-file sequences. Other table-driven and regression tests cover registration, roles, filters, bad/unknown records, partial tails, large bodies, injected short writes, stale lease refusal, warm snapshot rejudgment, fence-zero immunity, record holds, explicit answer selection, assignment authority, result/exit validation, wake deduplication, trust escaping, CLI output/errors, and cold/warm fold equivalence.

Verified `make test` (which runs `go test ./...`), `make lint`, race tests for Feed and CLI, and builds for Linux amd64/arm64 and Darwin amd64/arm64.

This environment exposes a `.git` marker at the default temporary root, which makes the existing resolver tests classify temporary fixtures as a Git project. A plain initial `make test` reproduced those unrelated failures. The passing full-suite run used a lane-local temporary directory mounted as a clean `/tmp`, without editing the resolver or fixtures. The build cache also stayed in the lane. To reproduce that environment workaround with bubblewrap available:

```sh
mkdir -p .cache/tmp
bwrap --ro-bind / / --dev-bind /dev /dev --proc /proc \
  --bind "$PWD" "$PWD" --bind "$PWD/.cache/tmp" /tmp \
  --setenv TMPDIR /tmp --setenv GOCACHE "$PWD/.cache/go-build" make test
GOCACHE="$PWD/.cache/go-build" make lint
```

## Intended differences from the old plugin

Reference names below are relative to the supplied read-only reference tree. Line numbers refer to that snapshot.

| Difference | Governing reference and old behavior |
| --- | --- |
| The command is `grid feed`; plain output is the default and JSON is opt-in. `handover` is now a CLI verb, and `fold` aliases `state`. | `docs/plugin-design.md:478`; the missing CLI verb is recorded in `old-plugin-map.md:53` and `old-plugin-map.md:137`; old JSON-only dispatcher: `old-plugin/bin/grid-feed:13`. |
| Use the existing Grid-home resolver and stable task ULID; never derive the record's task key from `T-NN`. Missing stable identity and incompatible format versions fail. | `docs/feed-contract.md:17`, `docs/feed-contract.md:43`; old display-ID derivation: `old-plugin/feed/lib.mjs:110`. |
| Writer files are `<member>@<machine ULID>.<fence>.jsonl`, including fence zero. They have no worker suffix or hostname-derived identity. Member defaults follow Git identity before OS identity. | `docs/feed-contract.md:21`, `docs/feed-contract.md:31`, `docs/feed-contract.md:34`; old identity/names: `old-plugin/feed/lib.mjs:93`, `old-plugin/feed/lib.mjs:105`. |
| Add lease, gen, fence, seq, attempt, after, and wake fields, keeping explicit nulls/arrays. Retire epoch. Timestamps are UTC milliseconds without the old counter suffix. Old JS envelopes/files require migration; they are not silently normalized. | `docs/feed-contract.md:38`, `docs/feed-contract.md:48`, `docs/feed-contract.md:59`; old envelope: `old-plugin/feed/lib.mjs:325`. |
| Keep author as `{member, role}` to satisfy mandatory role stamping. Machine identity is in worker, rather than another author host field. | The contract is internally inconsistent: `docs/feed-contract.md:46` calls author a string, while `docs/feed-contract.md:90` requires `author.role`. The latter wins here; old author construction: `old-plugin/feed/lib.mjs:327`. |
| Reject network filesystems before creating a Feed. Use Linux/macOS file locks and refuse unsupported platforms. | `docs/feed-contract.md:9`, `docs/feed-contract.md:11`, `docs/feed-contract.md:12`; old unlocked append: `old-plugin/feed/lib.mjs:359`. |
| Hold a shared lease lock across validation, append, and sync; hold an exclusive tenure-file lock across sequence allocation and writes. Ended tenures refuse writes even if their lease file still names the writer. | `docs/feed-contract.md:94`, `docs/feed-contract.md:124`, `docs/feed-contract.md:129`; old append: `old-plugin/feed/lib.mjs:306`. |
| Repair a torn tail before each retry, reuse the record ID, use one OS write per line, and spill bodies above 32 KiB. State records sync before acknowledgment; chatter does not. Identical retry IDs return the original record instead of the old duplicate-ID refusal. | `docs/feed-contract.md:128`, `docs/feed-contract.md:131`, `docs/feed-contract.md:132`, `docs/feed-contract.md:133`, `docs/feed-contract.md:136`; old duplicate check and append: `old-plugin/feed/lib.mjs:338`, `old-plugin/feed/lib.mjs:359`. |
| Read only complete lines. Validate filename/envelope agreement and retain bad-line counts. Persistent offset/arrival caches replace repeated raw-file parsing. Malformed complete takeover entries fail closed. | `docs/feed-contract.md:60`, `docs/feed-contract.md:111`, `docs/feed-contract.md:140`; old line parsing: `old-plugin/feed/lib.mjs:186`. |
| Remove global `(at, id)` ordering from decisions. Display merges arrival times with cold timestamp ties while preserving each writer's sequence. ID-based `since` is an exclusive cursor in that display stream; time-based `since` requires RFC3339 rather than the old loose date-prefix syntax. Neither is an authority watermark. | `docs/feed-contract.md:174`, `docs/feed-contract.md:179`; old ordering/filtering: `old-plugin/feed/lib.mjs:84`, `old-plugin/feed/lib.mjs:202`. |
| Hold missing re/after dependencies, deduplicate retry IDs, and apply takeover byte-offset snapshots to cached and new records. Fence-zero records remain unfenced. Unknown types/versions remain visible and cannot act. | `docs/feed-contract.md:33`, `docs/feed-contract.md:61`, `docs/feed-contract.md:145`, `docs/feed-contract.md:178`; old fold: `old-plugin/feed/lib.mjs:485`. |
| Control authority comes from the task lease and `(gen, fence)`, not role plus epoch or a timestamp-elected handover. Handover and transfer records announce intent without changing authority. `handover` requires an already granted task lease; it has no old transfer/accept/takeover action dispatcher. | `docs/feed-contract.md:123`, `docs/feed-contract.md:175`, `docs/feed-contract.md:265`; old handover actions: `old-plugin/feed/lib.mjs:422`. |
| Assign names job, attempt, recipient, fence, and until. Ack must cite the counted assign. Job-side state requires its granted tenure; readers never infer stale from wall time. The first tenure record is handover or ack. | `docs/feed-contract.md:123`, `docs/feed-contract.md:184`, `docs/feed-contract.md:185`, `docs/feed-contract.md:198`, `docs/feed-contract.md:212`; old fold fields: `old-plugin-map.md:71`. |
| A question keeps all answer candidates until task control explicitly selects one or closes it. Arrival order never chooses an answer. | `docs/feed-contract.md:177`; old question behavior: `old-plugin/feed/types/question.md:15`. |
| Claim and heartbeat are no longer writable types. Add ack, checkpoint, found, usage-notice, and loop-detected. Every registered type has a schema/class/author allowlist; arbitrary new writes and project Markdown overrides are refused. Unknown on-disk records are still displayed. | `docs/feed-contract.md:65`, `docs/feed-contract.md:66`, `docs/feed-contract.md:80`, `docs/feed-contract.md:82`, `docs/feed-contract.md:86`, `docs/feed-contract.md:88`, `docs/feed-contract.md:245`; old registry/overrides: `old-plugin/feed/lib.mjs:21`, `old-plugin/feed/lib.mjs:145`. |
| Guest/external input is marked data. Guests may answer only questions addressed to them, post feedback messages, or supply explicitly external knowledge; they cannot propose work. Body text cannot alter the stamped role, close a task, or escape its plain data block. | `docs/feed-contract.md:90`, `docs/job-contract.md:87`, `docs/job-contract.md:88`; old guest checks only excluded selected post kinds: `old-plugin/feed/lib.mjs:374`. |
| Result version is 2, with verification, must-fix rows, findings, and proposed Jobs; free-text next is rejected. Checkpoints are lease-bound records. Security findings and loop-detected records expose wake candidates rather than silently creating work or human questions. | `docs/feed-contract.md:80`, `docs/feed-contract.md:82`, `docs/feed-contract.md:84`, `docs/feed-contract.md:245`; `docs/job-contract.md:117`; old result schema: `old-plugin/feed/types/result.md:4`. |
| Usage notices require exactly one amount and the CLI author role. Exit reasons are enumerated; missing reason folds as crash, and usage_limit does not itself increment an attempt. The fold does not run retries. | `docs/feed-contract.md:113`, `docs/feed-contract.md:215`; old exit schema: `old-plugin/feed/types/worker-exit.md:5`. |
| Job IDs are explicit rather than auto-numbered during Feed append. Job dependency objects require their documented kind and an 8–160-character reason. Job definitions are immutable; conflicting duplicate definitions are ignored. Priority and defer_until are rejected in v1 Job bodies; rejected/cancelled controls require close_reason. Job schema/planning belongs to the Job commands. | `docs/job-contract.md:7`, `docs/job-contract.md:74`, `docs/job-contract.md:75`; old numbering: `old-plugin/feed/lib.mjs:341`; old priority fold: `old-plugin/feed/lib.mjs:535`. |
| Task-head projection requires a task tenure, and host announcements are retained without clock-derived offline decisions. Chatter does not move Jobs. A done control projects as accepted. | `docs/feed-contract.md:66`, `docs/feed-contract.md:72`, `docs/feed-contract.md:174`, `docs/feed-contract.md:212`; old head/host fold: `old-plugin/feed/lib.mjs:634`. |
| Wake writes are deduplicated by event, post verb/type, and target regardless of wake number, including concurrent writers. | `docs/feed-contract.md:255`; no equivalent in the old CLI flags at `old-plugin/bin/grid-feed:14`. |
| Remove raw Feed rsync/sync from this CLI. No SSH action is performed. | `docs/feed-contract.md:13`, `docs/feed-contract.md:298`; old sync command: `old-plugin/bin/grid-feed:41`, migration note: `old-plugin-map.md:114`. |

## Boundaries and remaining verification

- This lane does not grant/release/take over leases, run `grid job next`, evaluate timer/CI gates, perform process liveness checks, schedule retries, run loop detection, implement MCP transport, or sync with a server. Those are separate command/worker responsibilities (`docs/feed-contract.md:117`, `docs/feed-contract.md:188`, `docs/feed-contract.md:200`, `docs/feed-contract.md:222`, `docs/feed-contract.md:298`). Feed exposes the records and projections those components need; it does not invent their authority or clock decisions.
- `CheckFilesystem` is exported for the doctor integration. Doctor diagnostics for holds, unknown versions, free space, and the shared filesystem check are not wired here because this lane owns Feed, not doctor. The Feed command itself checks the filesystem and format version. The first-machine-ID behavior is limited to Feed writes, leaving global CLI initialization to its owner.
- The exact allowed-role catalog and some new body details are not fully enumerated by the contract. The embedded registrations make the choices inspectable. Full planned-Job lint and review policy are outside Feed; result and Job bodies remain available in the projection for those consumers.
- The contract's platform paragraph mentions a CRC but specifies neither a CRC envelope field nor its encoding (`docs/feed-contract.md:136` versus `docs/feed-contract.md:38`). No incompatible checksum field was invented. JSON/envelope/filename checks are implemented.
- The cache persists parsed history and offsets, but reloads its JSON snapshot and rereads the takeover log per invocation. The full 24 GB / 200 ms performance target and the 125-process crash/takeover stress fixtures were not run. Only the requested reduced concurrency tests and targeted fault fixtures are claimed (`docs/feed-contract.md:274`). A corrupt cache reports a repair instruction; remove only `feed/.cache/reader.json` to rebuild while preserving lock files. Tenure truncation is an error, not silently repaired.
- macOS builds were checked, but APFS kill/power-loss behavior and F_FULLFSYNC latency need a macOS runner (`docs/feed-contract.md:277`). Windows remains explicitly unsupported pending its locking spike. Disk-full error classification is tested; filling a real filesystem was not attempted. A failed/short append may leave an unacknowledged fragment, which the next write repairs without truncation.
- `state.json` and `needs-you.json` are each atomically replaced, not a two-file transaction. Consumers needing a single snapshot should read `state.json`, which also contains `needs_you`.
