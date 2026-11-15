package hook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// historyLockWait is how long the PostToolUse Bash history record waits for the
// session lock before it skips the record. The PreToolUse record does not wait.
const historyLockWait = 2 * time.Second

func history(o Options, in hookio.Input) {
	if o.Resolver.GridHome == "" || !o.hookEnabled("history-log", in) {
		return
	}
	if in.ToolName == "Bash" {
		bashHistory(o, in)
		return
	}
	if in.HookEventName == "SubagentStop" {
		dir, err := taskDir(o, in)
		if err != nil {
			o.log("history", err)
			return
		}
		if dir == "" {
			return
		}
		detail := ""
		if name := briefName(in, dir); name != "" {
			detail = "brief " + strings.TrimSuffix(name, ".md")
		}
		o.log("history", task.AppendHistory(dir, "subagent finished", detail, writer(in), ""))
		return
	}
	if in.HookEventName != "PostToolUse" {
		return
	}
	key := "file_path"
	event := "file edited"
	switch in.ToolName {
	case "Write":
		event = "file written"
	case "Edit", "MultiEdit":
	case "NotebookEdit":
		key = "notebook_path"
	default:
		return
	}
	path := str(in.ToolInput[key])
	if path == "" {
		return
	}
	path = absolute(in.Cwd, path)
	dir, err := task.FolderAt(o.Resolver, path)
	if err != nil {
		o.log("history", err)
		return
	}
	if dir == "" {
		return
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == "history.md" || rel == "." {
		return
	}
	o.log("history", task.AppendHistory(dir, event, filepath.ToSlash(rel), writer(in), ""))
}

// Snapshot only the resolved task. Shell text is never executed or parsed as
// paths; background commands have no meaningful completion boundary.
func bashHistory(o Options, in hookio.Input) {
	if in.SessionID == "" || in.ToolUseID == "" || in.ToolInput["run_in_background"] == true {
		return
	}
	if in.HookEventName != "PreToolUse" && in.HookEventName != "PostToolUse" && in.HookEventName != "PostToolUseFailure" {
		return
	}
	dir, err := taskDir(o, in)
	if err != nil {
		o.log("history snapshot", err)
		return
	}
	if dir == "" {
		return
	}
	store := session.Store{TaskDir: dir}
	// The walk and the hashing of the task folder take as long as the folder is
	// big, so they run before the session lock is taken: every hook of every
	// session in the task waits on that lock, and none of them waits behind
	// another session's walk. Under the lock stay the read and the write of this
	// session's snapshots, the compare and the list of lines.
	//
	// A Post walks only when its tool use has a stored snapshot, as before. The
	// stored one is read here without the lock, which is a plain read of a file
	// that is replaced whole; the read under the lock below decides, so a
	// snapshot that is gone by then records nothing. A walk that fails logs and
	// writes nothing, as it did inside the lock.
	var snap, after map[string]string
	if in.HookEventName == "PreToolUse" {
		var walkErr error
		if snap, walkErr = snapshot(dir); walkErr != nil {
			o.log("history snapshot", walkErr)
			return
		}
	} else if stored, readErr := store.Activity(in.SessionID); readErr == nil {
		if _, ok := stored.Snapshots[in.ToolUseID]; ok {
			var walkErr error
			if after, walkErr = snapshot(dir); walkErr != nil {
				o.log("history snapshot", walkErr)
				return
			}
		}
	}
	// The lines are decided under the session lock and appended after it is
	// released: .task.lock is never taken while the session lock is held.
	type historyLine struct{ event, detail string }
	var lines []historyLine
	// Every hook of every session in the task takes this lock, so it can be
	// busy for a long time. A PreToolUse runs before the tool call, so it does
	// not wait for the lock at all: when the lock is busy it skips the record and
	// the tool call is never held up for bookkeeping. A PostToolUse, after the
	// call, waits historyLockWait for it and then gives up. The guards' decision
	// depends on neither.
	update := func(change func(*session.Activity) error) error {
		if in.HookEventName == "PreToolUse" {
			return store.UpdateActivityNow(in.SessionID, change)
		}
		ctx, cancel := context.WithTimeout(context.Background(), historyLockWait)
		defer cancel()
		return store.UpdateActivityContext(ctx, in.SessionID, change)
	}
	err = update(func(a *session.Activity) error {
		lines = nil
		if in.HookEventName == "PreToolUse" {
			a.Snapshots[in.ToolUseID] = snap
			return nil
		}
		before, ok := a.Snapshots[in.ToolUseID]
		if !ok || after == nil {
			return nil
		}
		// Stable ordering makes history independent of map iteration.
		names := map[string]bool{}
		for name := range before {
			names[name] = true
		}
		for name := range after {
			names[name] = true
		}
		changed := 0
		for _, name := range sortedNames(names) {
			if before[name] == after[name] {
				continue
			}
			changed++
			if changed > 10 {
				continue
			}
			event := "file edited"
			if _, ok := before[name]; !ok {
				event = "file written"
			}
			if _, ok := after[name]; !ok {
				event = "file removed"
			}
			lines = append(lines, historyLine{event, name})
		}
		if changed > 10 {
			lines = append(lines, historyLine{"files changed", fmtCount(changed-10) + " more in this command"})
		}
		delete(a.Snapshots, in.ToolUseID)
		return nil
	})
	if errors.Is(err, fslock.ErrLocked) {
		// No snapshot was written. The PostToolUse of this tool use finds none and
		// records nothing, which is no error.
		o.log("history skipped", errors.New("session lock busy"))
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		// No activity and no snapshot were written. A PostToolUse that finds no
		// snapshot for its tool use records nothing and is no error.
		o.log("history skipped", fmt.Errorf("session lock busy after %s", historyLockWait))
		return
	}
	if err != nil {
		o.log("history snapshot", err)
		return
	}
	for _, l := range lines {
		if err := task.AppendHistory(dir, l.event, l.detail, writer(in), "Bash"); err != nil {
			o.log("history snapshot", err)
			return
		}
	}
}

// ojSnapshotForTest is nil in production. A test sets it to stop at the start of
// a walk, as the fslock test hook does for a lock wait, and to put it back when
// the test ends; no file outside tests sets it.
var ojSnapshotForTest func()

func snapshot(dir string) (map[string]string, error) {
	if ojSnapshotForTest != nil {
		ojSnapshotForTest()
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	out := map[string]string{}
	var total int64
	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "history.md" || path == "sessions" || path == "feed" || strings.HasPrefix(d.Name(), ".") && path != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		if len(out) >= 2000 || total > 32<<20 {
			return errors.New("task snapshot exceeds limit")
		}
		f, err := root.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, io.LimitReader(f, 32<<20))
		f.Close()
		if err != nil {
			return err
		}
		out[path] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	return out, err
}

func sortedNames(m map[string]bool) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func fmtCount(n int) string { return strconv.Itoa(n) }
