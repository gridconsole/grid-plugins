# Lane L2 complete

Built a placeholder Grid home in `fixtures/`, a Node-backed reference recorder in
`record.sh` and `recordings/`, and 136 cases in `golden/`. Every case records stdin,
the complete environment, stdout, stderr, exit status and file changes. The
corpus covers all 19 runnable hook scripts found in the reference and all six
requested pure functions. The twentieth `.mjs` file is the shared library; the
old map's hook count is inaccurate. Extra coverage includes plan-guard (14),
brief-lint (7), stop-check (11), grid-home-allow (10) and history-log (12).

Run `./record.sh` to regenerate and `./record.sh --check` to independently replay
and compare without overwriting. Both executed successfully, with byte-identical
results across all 817 artifacts, including `golden/INDEX.md`. The recorder
asserts expected decisions, exits and file effects, as well as complete script
coverage. Artifact completeness and scans for personal paths and emojis passed.
See `recordings/README.md` for prerequisites, fixture variants, clock control,
normalization and multi-event setup. The reference source is read-only and is
never vendored. Existing Go sources, go.mod and Makefile are unchanged.

The corpus SHA-256, calculated over sorted relative file paths and contents with
a NUL delimiter after each path and content, is:

```text
c8f14a48be9b341b9f656d8d157e058b18e86556f5561e93a1953c526e98d20a
```

Repository checks use `make test` and `make lint`. The default Go cache is not
writable in this environment, so verification used temporary cache and build
folders inside this lane. `make lint` passed when run separately from test
compilation. `make test` ran but failed existing CLI and project-resolution
expectations in `internal/cli` and `internal/where`: their temporary fixture
folders inherit this checkout's ancestor `.git`, so the resolver returns a Git
project instead of folder, Personal or multi-repo. Other tested packages passed.
The resolver walks ancestors itself, so a Git discovery ceiling does not change
this result. No unrelated Go behavior or expected outputs were changed to hide
these failures. A simultaneous first lint attempt saw transient Go test build
files; the subsequent sequential lint run passed.

To reproduce the lane-contained checks, run the following from the repository
root, invoking the two make commands sequentially and checking each exit status:

```sh
verify_dir=$(mktemp -d "$PWD/.verify-XXXXXX")
trap 'rm -rf "$verify_dir"' EXIT
mkdir -p "$verify_dir/tmp"
export TMPDIR="$verify_dir/tmp"
export GOCACHE="$verify_dir/cache"
export GOMODCACHE="$verify_dir/mod"
make test
make lint
```

No script was omitted. Successful notification delivery and permission decisions
from a live server or authenticated desktop bridge could not be recorded because
those services are absent. Their disabled and missing-bridge fallback cases are
recorded. Desktop configuration discovery uses a synthetic config; actual app
integration and remote worker orchestration are not exercised. These limits are
also listed in `golden/INDEX.md`. Heartbeat and worker-exit local feed writes are
recorded, including their silent failure when the feed directory is missing.
