# L29 delivery

Built Go handlers for heartbeat, history-log, usage-log, brain-read, brain-recall,
stop-check, worker-exit and permission-forward, plus every recorded lib helper.
Runtime events compose these handlers with the guards; component entry points
let the corpus replay each old script independently. Added observed usage folding
through `grid usage`, bounded Job verification, the SessionStart brain index and
the PreCompact summary reminder. Persistence reuses task/session writers and
fslock; no module or dependency was recreated.

Run from the checkout, leaving TMPDIR at its default:

```sh
make test
make lint
make golden
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

All five commands passed. Golden result: **136 cases, 62 pass, 55 documented
known differences, 0 fail, 19 skip**, down from 95 skips. The 35 new differences
and 20 inherited differences are individually listed in
`golden/known-differences.txt`. Tests exercise real handlers, including successful
brain reads/recall, retry deduplication, privacy, history locking, verification
failure/minimum-test caps, permissions, internal errors and liveness isolation.

Intended differences from the old plugin:

- SessionStart adds locked session evidence and the bounded brain index;
  existing context/export changes remain: `docs/lanes/L29.md:59`.
- Hooks touch existing session activity without renewing leases or posting
  heartbeat records; supervision owns feed/.live: `docs/lanes/L29.md:11`,
  `docs/lanes/L29.md:63`.
- History omits plugin-data attachment lists and uses content snapshots in
  session JSON. Scope, exclusions and limits are explicit:
  `docs/lanes/L29.md:25`, `docs/lanes/L29.md:66`.
- Brain reads recognize accepted typed notes and log portable ids, excluding
  loose untyped files: `docs/lanes/L29.md:71`.
- Recall emits pointers with summaries and reasons, never bodies; it has shorter
  messages, JSON markers and plan-content deduplication:
  `docs/lanes/L29.md:76`, `docs/lanes/L29.md:157`.
- Legacy stop diagnostics use concise names; task-owned status transitions use
  persistent locks and mode-0600 atomic replacement. New task metadata does not
  require legacy summary/next fields: `docs/lanes/L29.md:40`,
  `docs/lanes/L29.md:80`.
- Usage stores numeric observations and transcript references in session records,
  with retries replacing totals. It does not post to the Feed, copy response
  text, guess prices or implement account holds: `docs/lanes/L29.md:18`,
  `docs/lanes/L29.md:84`.
- Worker exits retain reason and attempt evidence in session records instead of
  writing unfenced legacy posts: `docs/lanes/L29.md:88`.
- Shared frontmatter helper values match, while Go sorts JSON object keys:
  `docs/lanes/L29.md:94`.
- Existing guard changes remain; Personal task files now receive the same
  only-writer protection: `docs/lanes/L29.md:55`, `docs/lanes/L29.md:98`.
- Explicit server permission forwarding has bounded responses/timeouts, no
  redirects, modified-input guard rechecks and terminal fallback. Automatic
  desktop bridge discovery is omitted: `docs/lanes/L29.md:49`,
  `docs/lanes/L29.md:112`.
- Saved Job verification enforces exit status and observed test counts, with
  bounded attempts and durable failure evidence: `docs/lanes/L29.md:144`.
- PreCompact steers summary retention; SessionStart refreshes the topic index:
  `docs/lanes/L29.md:157`.
- Internal failures log and allow; only positive stop/verification findings
  block. Legacy stop retries allow after one push: `docs/lanes/L29.md:44`.

What could not be completed:

- Publishing worker exits and exhausted verification failures requires the
  fenced Feed append writer, Job lease checks and sequence allocation, absent
  from this branch. Evidence is recorded for that integration; no substitute
  Feed record shape or unsafe writer was introduced: `docs/lanes/L29.md:105`.
- Desktop bridge discovery, legacy cross-task/dispatch inference, overlapping
  snapshot settlement, orchestration nudges, curator scheduling, and compact
  refresh of current Job/open Feed decisions remain integration work. Their
  boundaries are listed at `docs/lanes/L29.md:112`, `docs/lanes/L29.md:116` and
  `docs/lanes/L29.md:157`.
- The remaining 19 corpus cases cover other scripts, listed with counts at
  `docs/lanes/L29.md:123`. Every script requested for L29 now runs.
- Native macOS and Windows execution was unavailable in this Linux workspace;
  both requested cross-platform builds pass: `docs/lanes/L29.md:140`.
