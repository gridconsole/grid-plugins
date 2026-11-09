package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/task"
)

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
	// The lines are decided under the session lock and appended after it is
	// released: .task.lock is never taken while the session lock is held.
	type historyLine struct{ event, detail string }
	var lines []historyLine
	err = store.UpdateActivity(in.SessionID, func(a *session.Activity) error {
		lines = nil
		if in.HookEventName == "PreToolUse" {
			snap, err := snapshot(dir)
			if err == nil {
				a.Snapshots[in.ToolUseID] = snap
			}
			return err
		}
		before, ok := a.Snapshots[in.ToolUseID]
		if !ok {
			return nil
		}
		after, err := snapshot(dir)
		if err != nil {
			return err
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

func snapshot(dir string) (map[string]string, error) {
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
