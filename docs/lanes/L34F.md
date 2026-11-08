# Lane L34F

Implemented all eight L34 follow-ups and all three review nits. The lane record is also archived in `docs/lanes/L34F.md`; `docs/lanes/L34.md` describes the inherited orchestrator.

## Built

- `grid lease take task` grants the first task tenure and posts its handover. Local adoption checks both process id and start identity under the exclusive task lease lock. Adoption and person-authorized offline takeover require the current expected epoch, so competing callers cannot replace the same tenure twice. Worker task supervision stamps mechanical handovers and controls as `cli`; Job writes retain their worker role. Unsupported task-holder roles are refused before any lease is granted. Merge lease commands retain their existing interface.
- Wakes remain pending after a pass save until a counted write carries their exact `event/n` key. Caller retries reuse the same wake and rate-limit slot. Numbers increase per Job; summaries remain pending until acknowledged. Mechanical proposal creation does not acknowledge a judgement wake. Upgrading L34 metadata honors original `/1` acknowledgements and reserves legacy delivery numbers before replay.
- A crash-loop park resumes only for the applied `Retry` answer to that park's own question. `Keep parked`, unrelated answers, ambiguous answers, chatter and file changes leave it parked. Recovery from missing metadata preserves both parks and applied retry resets.
- Task work products are scanned once per pass and content hashes are cached by path, size and modification time across CLI processes. Each Job's combined progress is computed once per pass. Unchanged Feed projections, reader cache and orchestrator metadata are not rewritten.
- Unknown liveness becomes `stale` with `host_offline` after two lease periods without a successful check and after `until`. The last successful check persists across passes. An offline host receives no new assignments; `--to` can select another host. Expired heartbeats no longer stay alive solely because a process probe succeeds. `--after-wake` also protects missing-ack assignments and preserves suppression until identity and heartbeat checks succeed.
- `ReadJob` uses `TaskStore.Read`, with one contract-validated lease decoder and `job.ValidID`. Both readers reject empty, malformed and mismatched grants.
- `grid feed post` refuses Job-side types and Job-lease writes. Workers use the guarded `grid job` commands, which reject writes after exit or closure.
- Passes and writes hold `task.json` before `.orchestrator.lock`; the complete order is documented in the orchestrator package comment. A worker waiting for the task lease leaves the pass lock free.
- Passes remove spawn requests after a Job leaves Assigned. Crash questions reserve `Q-n` identities under the question-file allocator's lock, with durable `.feed` reservation markers. Interrupted `job next --wait` returns 130.

## Running

Use the long-lived supervisor's process identity, not the short-lived lease command's process id. `SUPERVISOR_START` is its UTC RFC3339 start timestamp. Set `GRID_MEMBER` and `GRID_ROLE` before acquiring the task.

```sh
grid lease take task --task T-01 --pid "$SUPERVISOR_PID" --pid-start "$SUPERVISOR_START" --json
# Set GRID_LEASE=task and GRID_LEASE_EPOCH to the returned gen.fence.
grid job next --task T-01 --wait --json
```

Adoption adds `--adopt --expected-epoch <gen.fence>`. A person taking over an offline holder adds `--takeover --expected-epoch <gen.fence> --server-offline-since <RFC3339>`. Use `--gen` for a nonzero generation. The offline timestamp must come from the server's observation; neither a lease's age nor a sleeping laptop supplies it. The CLI trusts this explicit supervisory assertion and does not fetch it from a server.

A wake's resulting write supplies `--wake <event>/<n>` or `GRID_WAKE`. Retry an interrupted caller without changing that key. After supervision detects sleep, pass `--after-wake` to `job next` or `feed tick`.

## Validation

Run from the repository root with the default TMPDIR:

```sh
make test
make lint
go test -race -count=3 ./internal/orchestrator/... ./internal/lease/... ./internal/feed/... ./internal/cli
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

All five acceptance commands passed on Linux. `make test` reported no temporary-file leaks. The final race run completed all four package groups with three repetitions and exit status 0. Regression tests were run failing before each behavior fix. Coverage includes CLI first grant, live/unknown/remote adoption refusal, takeover evidence and role checks, two competing adoption processes, saved-but-unacknowledged wakes, legacy metadata migration, per-Job numbering, summary replay, parked-answer selection, metadata recovery, cached progress, unchanged projection timestamps, remote offline boundaries, sleep/wake suppression, matching lease decoding, the Feed CLI exit bypass, task-before-pass locking, question id reservation and subprocess Ctrl-C.

Validation exposed existing environment-test assumptions: one used the default dev ports, and the CLI test checked only the first port of a ten-port block. Both failed on occupied ports. The tests now select an available block using the existing port availability API. A 300 ms unhealthy-server timeout also expired before the helper wrote its PID under load; that scenario now uses the same 3-second startup allowance as the other server tests. No environment behavior changed.

## Intended differences from the old plugin

These are this lane's additions to the differences already listed in `docs/lanes/L34.md`. References name the read-only specification documents.

| Difference | Basis |
| --- | --- |
| The product CLI grants the task lease, records handover, and fences adoption/takeover against an expected epoch. Process identity or a person's explicit server-offline evidence replaces implicit lease stealing. | `docs/feed-contract.md:118-122,138` |
| Wakes are acknowledged by their resulting Feed writes, replay after caller failure, and use increasing per-Job numbers so rendered wake files remain distinct. Summary retries reuse their original slot. | `docs/feed-contract.md:225,228-229,251,255` |
| A parked crash loop needs its own applied `Retry`; unrelated progress and `Keep parked` do not release it. This intentionally narrows the general progress-reset sentence in the spec, as required by the L34F brief. | `docs/feed-contract.md:177,216,287` |
| Progress hashing uses one pass snapshot with a persistent size/mtime cache. Unchanged projections are not published again. Mechanical files never count as progress. | `docs/feed-contract.md:142,216,224` |
| Unknown checks have the specified offline escape, retries avoid that host, and sleep requires renewed evidence before clock expiry resumes. | `docs/feed-contract.md:212-213,218` |
| Task and Job lease views share a contract decoder and identifier validator. They do not create a second wire shape or grant protocol. | `docs/feed-contract.md:92-103,126-138` |
| Job-side writes go through guarded Job commands; generic Feed posting cannot bypass exit or closure checks. | `docs/feed-contract.md:129,185,198` |
| Passes and command writes use one task-before-pass lock order, while lease grants retain in-place writes and increasing fences. | `docs/feed-contract.md:103,118-122,129,138` |
| Spawn requests are removed after assignment ends; crash gates reserve identities shared with question files without copying independently managed question content. | `docs/feed-contract.md:183-185,188-197,225` |
| Ctrl-C returns shell status 130 from the product wait command. | `docs/feed-contract.md:223-224` |

## Limits

- There is no server liveness client in this repository. Remote takeover consumes explicit server-observed evidence supplied by a person; it does not independently authenticate or obtain that observation. Missing, recent or agent-supplied takeover authorization is refused.
- The CLI probes local processes. Remote checks remain unknown until the offline grace expires; automatic remote recovery and host discovery still belong to supervision. A caller chooses a different host with `--to`.
- Size/mtime caching intentionally does not detect a content rewrite that preserves both values. New files, deletion, changed size and changed modification time update the pass digest.
- Question reservation markers prevent identity reuse, including after a crash before publication. They are not a second question store or an answer synchronization bridge.
- macOS and Windows were cross-built from Linux. No macOS runtime host was available. Windows retains the repository's existing unsupported runtime file-lock behavior.
