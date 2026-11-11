# C-pcov3-hook-bash-snapshot

Tests for the Bash file-change history and the task snapshot in `internal/hook/history.go`. One new test file, `internal/hook/bash_snapshot_test.go`. No non-test file changed. No existing test edited or deleted.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 coverage before/after | HOLDS | Before (untouched clone, Linux): `total: (statements) 80.7%`. After: `total: (statements) 81.9%`. Command: `go test -count=1 -coverprofile=/tmp/C-pcov3-hook-bash-snapshot-hook.out ./internal/hook/ && go tool cover -func=... \| tail -1`, exit 0. Per function after: bashHistory 90.9%, snapshot 82.4%, history 84.6%. |
| 2 early returns | HOLDS | `TestBashHistoryEarlyReturnsStoreNothing` (subtests run_in_background, empty session, empty tool use, other event Stop): history.md unchanged, a later Post writes nothing (history.go:81-86). `TestBashHistoryPreAloneAndPostWithoutPreWriteNothing`: a Pre alone leaves history.md unchanged (:102-108). |
| 3 edited/written/removed | HOLDS | `TestBashHistoryEditedWrittenRemovedInNameOrder`, subtests PostToolUse and PostToolUseFailure: three lines in name order, `file edited · a-edit.md`, `file written · sub/new.md`, `file removed · z-gone.md`, each `by claude · via Bash` (:134-141, :154). Post with no Pre: the code returns at :109-112, no line, and logs nothing; asserted in `TestBashHistoryPreAloneAndPostWithoutPreWriteNothing`. |
| 4 eleven changes | HOLDS | `TestBashHistoryMoreThanTenChangesSummarised`: 10 `file written` lines and `files changed · 1 more in this command` (:143-145). Extra pin: `TestBashHistoryExactlyTenChangesHaveNoSummary`. |
| 5 snapshot skips | HOLDS | `TestBashHistorySnapshotSkipsOwnFiles` (history.md, sessions/, feed, dot file, dot folder, nested dot file, changed and added between Pre and Post: no line; a counted file written afterwards does write one). `TestSnapshotKeysSkipsAndCounts` pins the exact keys: the skip is by top-level path only, so `sub/history.md`, `sub/feed` and `sub/sessions/a.json` ARE counted (:173); a directory and a symlink are not keys (:179-188). |
| 6 limit pin | HOLDS | `TestSnapshotFileCountLimitPin`: 2000 counted files (task.md and intent.md included) give 2000 entries and nil error; one more gives `task snapshot exceeds limit` (:190-191). `TestBashHistoryLimitAtPreToolUseLogsAndStoresNothing`: 2001 files at Pre log `grid hook: history snapshot: task snapshot exceeds limit`, the Post writes no line. `TestBashHistoryLimitAtPostToolUseLogsAndWritesNothing`: Pre at 2000 files ok, Post at 2001 logs the error and writes no line (:113-115, :149-151). The 32 MB trigger is not used. |
| 7 own history.md write | HOLDS | `TestHistoryWriteOfOwnHistoryFileIsSkipped`: PostToolUse Write of the task's history.md gives no line (:72); control write of notes.md gives `file written · notes.md`. |
| 8 checks | HOLDS | `go test -count=1 ./internal/hook/` exit 0; `GOOS=windows go vet ./internal/hook/` exit 0; `GOOS=darwin go vet ./internal/hook/` exit 0; `GOOS=windows go test -c -o /dev/null ./internal/hook/` exit 0; `make lint` (go vet ./...) exit 0; `gofmt -l internal/hook` empty. |
| Packages importing hook | HOLDS | `go list` gives `internal/cli` and `internal/packaging`. `go test -count=1 ./internal/hook/ ./internal/cli/ ./internal/packaging/`: all `ok`, exit 0. |
| New tests fail on base | NOT APPLICABLE | Tests only, no code change, so there is no base failure to show. The tests pin what the code does today. |
| Windows and macOS builds | HOLDS | Vet and test-compile above. A Windows or macOS run is "compiled, needs <OS> run": `GOOS=windows go test ./internal/hook/` on a Windows host. The symlink in `TestSnapshotKeysSkipsAndCounts` is logged and skipped if the OS refuses to create one. |
| No guard, gate, cap or hold added | HOLDS | Only a new `_test.go` file. The 2000-file limit is pinned, not added. |
| No attribution lines, no personal data | HOLDS | Commit has one sentence, no trailer. |

## What changed

- `internal/hook/bash_snapshot_test.go` (new). It calls `history` directly, so an empty SessionID or an event outside the three Bash events reaches `bashHistory` unchanged. It reuses `eventSetup`, `historyOf` and `f.run`.

## Outcome

Code wins where the brief differed: a Post with no stored snapshot returns at :109-112 before walking the folder, so it writes no line and logs nothing. No bug found. No row dropped.

## NOT VERIFIED on a live screen

Nothing here is UI. A real Bash run under a live Claude session was not run (no claude in the sandbox).

DONE
