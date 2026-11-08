# Legacy hook recordings

Run from the checkout root:

```sh
./record.sh
./record.sh --check
make test
make lint
```

The first command replaces `golden/` only after every reference invocation and
assertion succeeds. The second regenerates all cases independently and compares
every byte and filename, including the index, without changing the committed
corpus. A mismatch exits nonzero. Python 3.9 or newer, Node with `--import`
(Node 20 or newer), Bash and the reference plugin are required. The repository's
Go checks use its existing Makefile; they do not regenerate these recordings.

The default source is the supplied read-only reference plugin. To use another
copy, pass `--source /path/to/old-plugin` or set `OLD_PLUGIN`. An alternative
`--output golden-repeat` must stay within this checkout. The index includes the
Node version and reference SHA-256 hashes so a source or runtime change is
visible during comparison. The original source is never edited or copied into
the public repository.

`fixtures/` is the fake Grid data folder. `recordings/cases.py` defines inputs,
fixture edits, expected decisions, expected exit codes and required file effects.
Each case gets a fresh copy under a temporary directory in this checkout, with a
sibling fake user home and project checkout. The project folder token in the
fixture is expanded only in the temporary copy. The task's workflow deliberately
includes a plan approval step, since the guard needs that to distinguish held
and accepted plans. The task starts waiting with a complete but unfinished brief.

Every subprocess runs the unmodified reference with Node, a JSON event on stdin,
a fixed process working directory at the fake repository, a cleared environment
populated with the values in `env.txt`, and a ten second timeout. The event's
`cwd` may instead name the task folder. There is no inherited bridge, user config,
plugin data, hostname or credential. The sample host is `example.com`; the old
feed library shortens it to `example` in its stored identity. All persistent
writes and temporary files stay under the case directory. No network server is
started or contacted by the supplied cases.

The clock preload pins `Date` and `Date.now()` to 2025-01-02T03:04:05.000Z without
editing the source. Filesystem clocks and random feed IDs remain real until
capture. No hook decisions or filesystem calls are mocked. `invoke.mjs` is only
an argument adapter for the six requested pure functions and `terminalTask`.
That module defines exports but has no stdin main: the three direct-execution
cases show its no-op behavior, while three helper cases exercise its exported
function. Pure function input is `{"args": [...]}` and output is JSON plus a
newline; the adapter's encoding is distinct from a hook's own output encoding.

A case directory contains:

- `input.json`: the exact input template, including deliberately malformed JSON
  in malformed-input cases. Fixed input dates are retained; path tokens expand at
  execution. Hook inputs are not reconstructed from their outputs.
- `env.txt`: the complete child environment, sorted, with paths normalized.
  An absent key is unset; an empty value is present but empty.
- `stdout` and `stderr`: captured bytes, with only the normalizations below.
  No newline is added. Empty files mean zero bytes.
- `setup.json`: the case setup from `cases.py` (files, dirs, links, env, `pre`,
  `after_pre`, `call`, expected exit and markers), so a replay needs `golden/`
  alone. `$APP_CONFIG` stands for the desktop config folder under the home:
  `Library/Application Support` on macOS, `.config` elsewhere.
- `exit`: the actual numeric process status followed by a newline. Denial JSON
  usually has exit 0; stop-check deliberately uses exit 2.
- `files.diff`: sorted unified content diffs for all created, removed or changed
  regular files and symlinks anywhere under the temporary case directory,
  including plugin data and the fake user home. Headers are relative to that
  directory: `demo/` is Grid home, `workspace/demo/` is the repository, and `user/`
  is the user home. Type and permission changes are recorded too. Missing final
  newlines are explicit. Empty directories and access/mtime-only changes do not
  have file-content diffs.

Fixture edits happen before the baseline snapshot. Some cases use `pre` events
to establish prior state (dispatch, attachment, a Bash snapshot, a seen recall or
a previous heartbeat); these execute the reference too and must succeed. The
Bash completion cases then simulate a tool's file edits with `after_pre`, before
the baseline. Their final diff is the completion hook's changes, including the
snapshot deletion and history lines; it does not misattribute the simulated tool
edits to the hook. All setup and ordering are in `cases.py`. Reference hooks are
observers and never execute the Bash strings in these inputs.

Normalization is deliberately narrow:

- The temporary Grid data path becomes `$GRID_HOME`; its resolved siblings
  become `$GRID_HOME/../...`. Reference and harness paths become `$OLD_PLUGIN`
  and `$RECORDER`.
- ISO timestamps, including history's minute precision, become `<TIME>`.
  Snapshot mtime and ctime numbers become `<TIME>` while file sizes remain.
- Random feed ULIDs become `<ID-1>`, `<ID-2>`, and so on in first-seen order,
  preserving equality and references within a case.

Output JSON is never parsed and reserialized. Spacing, key order, punctuation,
line endings and trailing-newline differences survive normalization. Raw file
snapshots are compared before normalizing their contents; a change confined to
normalized values is reported explicitly.

The corpus records 136 cases: all 19 runnable `.mjs` hook files in this reference
(the twentieth `.mjs` is the shared library), plus six library functions. This
corrects the count in the old map. Extra cases cover plan-guard, brief-lint,
stop-check, grid-home-allow and history-log. The map is navigation; execution of
the reference determines the result. In particular, worker hooks silently skip
posting when the feed directory does not already exist, because lock creation
precedes posting. Both that edge and successful posting are recorded.

The index lists the remaining gaps: live notification delivery and permission
responses through a server or authenticated desktop bridge, actual desktop app
integration, and remote worker orchestration. The local fallback paths of those
hooks are recorded; their successful external integrations are not claimed.
