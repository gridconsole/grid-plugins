package hook

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// bsRun calls the history hook directly so that an empty SessionID or an
// event name outside the three Bash events reaches bashHistory unchanged.
func bsRun(t *testing.T, f *eventFixture, in hookio.Input) string {
	t.Helper()
	var log bytes.Buffer
	o := f.o
	o.Stderr = &log
	if in.Cwd == "" {
		in.Cwd = f.dir
	}
	history(o, in)
	return log.String()
}

func bsBash(event, session, use string, extra map[string]any) hookio.Input {
	input := map[string]any{"command": "make"}
	for k, v := range extra {
		input[k] = v
	}
	return hookio.Input{HookEventName: event, SessionID: session, ToolUseID: use, ToolName: "Bash", ToolInput: input}
}

func bsWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestBashHistoryEarlyReturnsStoreNothing(t *testing.T) {
	cases := []struct {
		name    string
		session string
		use     string
		event   string
		extra   map[string]any
	}{
		{"run_in_background", "s1", "u1", "PreToolUse", map[string]any{"run_in_background": true}},
		{"empty session", "", "u1", "PreToolUse", nil},
		{"empty tool use", "s1", "", "PreToolUse", nil},
		{"other event", "s1", "u1", "Stop", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := eventSetup(t)
			base := historyOf(t, f)
			if log := bsRun(t, f, bsBash(c.event, c.session, c.use, c.extra)); log != "" {
				t.Fatalf("early return logged: %q", log)
			}
			bsWrite(t, f.dir, "changed.md", "x")
			// The same call, as a Post, writes nothing.
			post := bsBash("PostToolUse", c.session, c.use, c.extra)
			if log := bsRun(t, f, post); log != "" {
				t.Fatalf("post logged: %q", log)
			}
			// A valid Post for the same tool use finds no stored snapshot.
			if c.session != "" && c.use != "" {
				if log := bsRun(t, f, bsBash("PostToolUse", c.session, c.use, nil)); log != "" {
					t.Fatalf("valid post logged: %q", log)
				}
			}
			if got := historyOf(t, f); got != base {
				t.Fatalf("history changed: %q", got)
			}
		})
	}
}

func TestBashHistoryPreAloneAndPostWithoutPreWriteNothing(t *testing.T) {
	f := eventSetup(t)
	base := historyOf(t, f)
	if log := bsRun(t, f, bsBash("PreToolUse", "s1", "u1", nil)); log != "" {
		t.Fatalf("pre logged: %q", log)
	}
	if got := historyOf(t, f); got != base {
		t.Fatalf("pre wrote history: %q", got)
	}
	// u2 has no stored snapshot: bashHistory returns before it walks the folder.
	bsWrite(t, f.dir, "new.md", "x")
	if log := bsRun(t, f, bsBash("PostToolUse", "s1", "u2", nil)); log != "" {
		t.Fatalf("post without pre logged: %q", log)
	}
	if got := historyOf(t, f); got != base {
		t.Fatalf("post without pre wrote history: %q", got)
	}
}

func TestBashHistoryEditedWrittenRemovedInNameOrder(t *testing.T) {
	for _, post := range []string{"PostToolUse", "PostToolUseFailure"} {
		t.Run(post, func(t *testing.T) {
			f := eventSetup(t)
			bsWrite(t, f.dir, "a-edit.md", "one")
			bsWrite(t, f.dir, "z-gone.md", "bye")
			if log := bsRun(t, f, bsBash("PreToolUse", "s1", "u1", nil)); log != "" {
				t.Fatalf("pre: %q", log)
			}
			bsWrite(t, f.dir, "a-edit.md", "two")
			bsWrite(t, f.dir, "sub/new.md", "hi")
			if err := os.Remove(filepath.Join(f.dir, "z-gone.md")); err != nil {
				t.Fatal(err)
			}
			if log := bsRun(t, f, bsBash(post, "s1", "u1", nil)); log != "" {
				t.Fatalf("post: %q", log)
			}
			h := historyOf(t, f)
			want := []string{
				"file edited · a-edit.md · by claude · via Bash",
				"file written · sub/new.md · by claude · via Bash",
				"file removed · z-gone.md · by claude · via Bash",
			}
			at := -1
			for _, w := range want {
				i := strings.Index(h, w)
				if i < 0 || i < at {
					t.Fatalf("missing or out of order %q in:\n%s", w, h)
				}
				at = i
			}
			if n := strings.Count(h, "via Bash"); n != 3 {
				t.Fatalf("%d Bash lines, want 3:\n%s", n, h)
			}
			// The snapshot is deleted by the Post: a second Post writes nothing.
			bsWrite(t, f.dir, "later.md", "x")
			bsRun(t, f, bsBash(post, "s1", "u1", nil))
			if got := historyOf(t, f); got != h {
				t.Fatalf("second post wrote: %q", got)
			}
		})
	}
}

func TestBashHistoryMoreThanTenChangesSummarised(t *testing.T) {
	f := eventSetup(t)
	bsRun(t, f, bsBash("PreToolUse", "s1", "u1", nil))
	for i := 1; i <= 11; i++ {
		bsWrite(t, f.dir, fmt.Sprintf("f%02d.md", i), "x")
	}
	if log := bsRun(t, f, bsBash("PostToolUse", "s1", "u1", nil)); log != "" {
		t.Fatalf("post: %q", log)
	}
	h := historyOf(t, f)
	if n := strings.Count(h, "file written · f"); n != 10 {
		t.Fatalf("%d file lines, want 10:\n%s", n, h)
	}
	if !strings.Contains(h, "files changed · 1 more in this command · by claude · via Bash") {
		t.Fatalf("no summary:\n%s", h)
	}
	if strings.Contains(h, "f11.md") {
		t.Fatalf("eleventh file listed:\n%s", h)
	}
}

func TestBashHistoryExactlyTenChangesHaveNoSummary(t *testing.T) {
	f := eventSetup(t)
	bsRun(t, f, bsBash("PreToolUse", "s1", "u1", nil))
	for i := 1; i <= 10; i++ {
		bsWrite(t, f.dir, fmt.Sprintf("f%02d.md", i), "x")
	}
	bsRun(t, f, bsBash("PostToolUse", "s1", "u1", nil))
	h := historyOf(t, f)
	if strings.Count(h, "file written · f") != 10 || strings.Contains(h, "files changed") {
		t.Fatalf("history:\n%s", h)
	}
}

func TestBashHistorySnapshotSkipsOwnFiles(t *testing.T) {
	f := eventSetup(t)
	bsWrite(t, f.dir, "sessions/old.json", "a")
	bsWrite(t, f.dir, "feed", "a")
	bsWrite(t, f.dir, ".hidden", "a")
	bsWrite(t, f.dir, ".dotdir/inner.md", "a")
	bsWrite(t, f.dir, "sub/.deep", "a")
	bsRun(t, f, bsBash("PreToolUse", "s1", "u1", nil))
	const own = "own history text\n"
	bsWrite(t, f.dir, "history.md", own)
	bsWrite(t, f.dir, "sessions/old.json", "b")
	bsWrite(t, f.dir, "sessions/new.json", "b")
	bsWrite(t, f.dir, "feed", "b")
	bsWrite(t, f.dir, ".hidden", "b")
	bsWrite(t, f.dir, ".dotdir/inner.md", "b")
	bsWrite(t, f.dir, ".dotdir/more.md", "b")
	bsWrite(t, f.dir, "sub/.deep", "b")
	bsWrite(t, f.dir, ".newhidden", "b")
	if log := bsRun(t, f, bsBash("PostToolUse", "s1", "u1", nil)); log != "" {
		t.Fatalf("post: %q", log)
	}
	if got := historyOf(t, f); got != own {
		t.Fatalf("skipped files wrote history: %q", got)
	}
	// Control: a counted file in the same shape does write a line.
	bsRun(t, f, bsBash("PreToolUse", "s1", "u2", nil))
	bsWrite(t, f.dir, "counted.md", "x")
	bsRun(t, f, bsBash("PostToolUse", "s1", "u2", nil))
	if got := historyOf(t, f); !strings.Contains(got, "file written · counted.md") {
		t.Fatalf("control: %q", got)
	}
}

func TestSnapshotKeysSkipsAndCounts(t *testing.T) {
	dir := t.TempDir()
	bsWrite(t, dir, "task.md", "t")
	bsWrite(t, dir, "history.md", "h")
	bsWrite(t, dir, "sessions/a.json", "s")
	bsWrite(t, dir, "feed", "f")
	bsWrite(t, dir, ".dot", "d")
	bsWrite(t, dir, ".dotdir/x.md", "d")
	bsWrite(t, dir, "sub/.dot", "d")
	bsWrite(t, dir, "sub/.dotdir/x.md", "d")
	// The skip is by exact top-level path only: nested names are counted.
	bsWrite(t, dir, "sub/history.md", "h")
	bsWrite(t, dir, "sub/feed", "f")
	bsWrite(t, dir, "sub/sessions/a.json", "s")
	if err := os.MkdirAll(filepath.Join(dir, "emptydir"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("task.md", filepath.Join(dir, "link.md")); err != nil {
		t.Logf("symlink not available: %v", err)
	}
	snap, err := snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for k := range snap {
		got = append(got, k)
	}
	want := map[string]bool{"task.md": true, "sub/history.md": true, "sub/feed": true, "sub/sessions/a.json": true}
	if len(got) != len(want) {
		t.Fatalf("keys %v", got)
	}
	for _, k := range got {
		if !want[k] {
			t.Fatalf("unexpected key %q in %v", k, got)
		}
	}
	if len(snap["task.md"]) != 64 {
		t.Fatalf("hash %q", snap["task.md"])
	}
}

func TestSnapshotFileCountLimitPin(t *testing.T) {
	dir := t.TempDir()
	bsWrite(t, dir, "task.md", "t")
	bsWrite(t, dir, "intent.md", "i")
	// task.md and intent.md count too: 1998 more make 2000.
	for i := 0; i < 1998; i++ {
		bsWrite(t, dir, fmt.Sprintf("bulk/f%04d", i), "x")
	}
	snap, err := snapshot(dir)
	if err != nil || len(snap) != 2000 {
		t.Fatalf("2000 counted files: %d entries, err %v", len(snap), err)
	}
	bsWrite(t, dir, "bulk/one-more", "x")
	snap, err = snapshot(dir)
	if err == nil || err.Error() != "task snapshot exceeds limit" {
		t.Fatalf("2001 counted files: %d entries, err %v", len(snap), err)
	}
}

// bsFill adds files under bulk/ until the folder has n counted files.
func bsFill(t *testing.T, dir string, n int) {
	t.Helper()
	snap, err := snapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := len(snap); i < n; i++ {
		bsWrite(t, dir, fmt.Sprintf("bulk/f%04d", i), "x")
	}
}

func TestBashHistoryLimitAtPreToolUseLogsAndStoresNothing(t *testing.T) {
	f := eventSetup(t)
	bsFill(t, f.dir, 2001)
	base := historyOf(t, f)
	log := bsRun(t, f, bsBash("PreToolUse", "s1", "u1", nil))
	if !strings.Contains(log, "grid hook: history snapshot: task snapshot exceeds limit") {
		t.Fatalf("pre log: %q", log)
	}
	// Folder drops under the limit; the Post has no stored snapshot.
	if err := os.RemoveAll(filepath.Join(f.dir, "bulk")); err != nil {
		t.Fatal(err)
	}
	bsWrite(t, f.dir, "new.md", "x")
	if log := bsRun(t, f, bsBash("PostToolUse", "s1", "u1", nil)); log != "" {
		t.Fatalf("post log: %q", log)
	}
	if got := historyOf(t, f); got != base {
		t.Fatalf("history written: %q", got)
	}
}

func TestBashHistoryLimitAtPostToolUseLogsAndWritesNothing(t *testing.T) {
	f := eventSetup(t)
	bsFill(t, f.dir, 2000)
	base := historyOf(t, f)
	if log := bsRun(t, f, bsBash("PreToolUse", "s1", "u1", nil)); log != "" {
		t.Fatalf("pre at 2000 files: %q", log)
	}
	bsWrite(t, f.dir, "bulk/extra", "x")
	log := bsRun(t, f, bsBash("PostToolUse", "s1", "u1", nil))
	if !strings.Contains(log, "grid hook: history snapshot: task snapshot exceeds limit") {
		t.Fatalf("post log: %q", log)
	}
	if got := historyOf(t, f); got != base {
		t.Fatalf("history written: %q", got)
	}
}

func TestHistoryWriteOfOwnHistoryFileIsSkipped(t *testing.T) {
	f := eventSetup(t)
	write := func(name string) {
		t.Helper()
		in := hookio.Input{ToolName: "Write", ToolInput: map[string]any{"file_path": filepath.Join(f.dir, name)}}
		if _, log, code := f.run(t, "PostToolUse", in); code != 0 || log != "" {
			t.Fatalf("%s: %d %q", name, code, log)
		}
	}
	write("history.md")
	if got := historyOf(t, f); got != "" {
		t.Fatalf("history.md write logged: %q", got)
	}
	// Control: any other file in the task is logged.
	write("notes.md")
	if got := historyOf(t, f); !strings.Contains(got, "file written · notes.md") {
		t.Fatalf("control: %q", got)
	}
}
