package task

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

const maxBrief = 1 << 20

var briefStatus = regexp.MustCompile(`(?m)^status:[ \t]*[^\r\n]*`)

// MarkBrief uses the task writer's atomic replacement under a stable file lock.
// Only the two unfinished legacy states may advance; terminal states are kept.
func MarkBrief(dir, name, state string) error {
	if filepath.Base(name) != name || (state != "done" && state != "blocked") {
		return errors.New("invalid brief transition")
	}
	sub := filepath.Join(dir, "subagents")
	info, err := os.Lstat(sub)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("brief folder must be a real directory")
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(sub, ".status.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return err
	}
	defer fslock.Unlock(lock)
	path := filepath.Join(sub, name)
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	b, err := io.ReadAll(io.LimitReader(f, maxBrief+1))
	f.Close()
	if err != nil {
		return err
	}
	// Rewriting what was read would silently cut a longer brief.
	if len(b) > maxBrief {
		return errors.New("brief is over 1 MiB; shorten it before the hook marks it")
	}
	doc := taskfile.ParseFrontmatter(string(b))
	if doc == nil || doc.Odd {
		return errors.New("invalid brief metadata")
	}
	if doc.Data["status"] != "briefed" && doc.Data["status"] != "running" {
		return nil
	}
	at := briefStatus.FindIndex(b)
	if at == nil {
		return errors.New("brief status is missing")
	}
	next := string(b[:at[0]]) + "status: " + state + string(b[at[1]:])
	return atomicWrite(path, []byte(next))
}
