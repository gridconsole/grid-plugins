package feed

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// MachineID creates the stable local machine key once, under a file lock.
func MachineID(gridHome string) (string, error) {
	if err := os.MkdirAll(gridHome, 0o700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(filepath.Join(gridHome, "machine"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return "", err
	}
	defer fslock.Unlock(f)
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	if id != "" {
		if !IsID(id) {
			return "", fmt.Errorf("machine must contain a ULID")
		}
		return id, nil
	}
	id, err = NewID()
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(id + "\n"); err != nil {
		return "", err
	}
	if err := syncFile(f); err != nil {
		return "", err
	}
	return id, nil
}

// TaskID reads only the scalar stable identifier in task.md frontmatter. Display
// numbers are never silently substituted for missing stable ids.
// ErrNoTaskID means task.md was read but holds no stable ULID, so the task has no Feed.
var ErrNoTaskID = errors.New("task.md needs a stable ULID in id or ulid")

func TaskID(taskDir string) (string, error) {
	f, err := os.Open(filepath.Join(taskDir, "task.md"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	return taskID(f)
}

func taskID(r io.Reader) (string, error) {
	scan := bufio.NewScanner(r)
	if !scan.Scan() || strings.TrimSpace(scan.Text()) != "---" {
		return "", fmt.Errorf("task.md needs frontmatter with a stable id")
	}
	var id string
	closed := false
	for scan.Scan() {
		line := scan.Text()
		if strings.TrimSpace(line) == "---" {
			closed = true
			break
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		key, v, ok := strings.Cut(line, ":")
		if !ok || (key != "id" && key != "ulid") {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), "\"'")
		if IsID(v) {
			if id != "" && id != v {
				return "", fmt.Errorf("conflicting stable task ids")
			}
			id = v
		}
	}
	if err := scan.Err(); err != nil {
		return "", err
	}
	if !closed {
		return "", fmt.Errorf("task.md frontmatter is not closed")
	}
	if id == "" {
		return "", ErrNoTaskID
	}
	return id, nil
}

// Test hooks surround lock acquisition and the protected metadata read.
// They are nil in production and tests install them before starting writers.
var beforeTaskIDLock func()
var afterTaskIDRead func()

// EnsureTaskID is for writers only. The stable task lock serializes allocation
// with task metadata writers, and the atomic replacement is durable before any
// Feed record can reference the identity. Readers must keep using TaskID.
func EnsureTaskID(taskDir string) (string, error) {
	lock, err := fslock.OpenNoFollow(filepath.Join(taskDir, ".task.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if beforeTaskIDLock != nil {
		beforeTaskIDLock()
	}
	if err := fslock.LockExclusive(lock); err != nil {
		return "", err
	}
	defer fslock.Unlock(lock)
	path := filepath.Join(taskDir, "task.md")
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("task.md must be a regular file")
	}
	// A linked task.md may have been written through the other link, and the
	// replacement below would erase that link. Refuse it under the lock, before
	// anything is read or written, like the task journal writers do.
	if err := workflow.RefuseLinked(f, "task.md"); err != nil {
		return "", err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	if afterTaskIDRead != nil {
		afterTaskIDRead()
	}
	id, err := taskID(bytes.NewReader(data))
	if !errors.Is(err, ErrNoTaskID) {
		return id, err
	}
	id, err = NewID()
	if err != nil {
		return "", err
	}
	updated, err := WithTaskID(data, id)
	if err != nil {
		return "", err
	}
	if err := atomicfile.Write(path, updated, info.Mode().Perm()); err != nil {
		return "", err
	}
	return id, nil
}

// WithTaskID adds an identity without reformatting other metadata or the body.
// Task journal recovery uses it to retain an identity allocated after a crash.
// Existing identities must agree; malformed or conflicting IDs are not repaired.
func WithTaskID(data []byte, id string) ([]byte, error) {
	if !IsID(id) {
		return nil, fmt.Errorf("task id must be a ULID")
	}
	old, err := taskID(bytes.NewReader(data))
	if err == nil {
		if old != id {
			return nil, fmt.Errorf("conflicting stable task ids")
		}
		return data, nil
	}
	if !errors.Is(err, ErrNoTaskID) {
		return nil, err
	}
	newline := "\n"
	if bytes.HasPrefix(data, []byte("---\r\n")) {
		newline = "\r\n"
	}
	lines := strings.SplitAfter(string(data), "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			lines[i] = "ulid: " + id + newline + lines[i]
			return []byte(strings.Join(lines, "")), nil
		}
		if strings.HasPrefix(lines[i], "ulid:") {
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(lines[i], "ulid:")), "\"'")
			if value != "" {
				return nil, fmt.Errorf("invalid task ulid")
			}
			lines[i] = "ulid: " + id + newline
			updated := []byte(strings.Join(lines, ""))
			if got, err := taskID(bytes.NewReader(updated)); err != nil || got != id {
				return nil, fmt.Errorf("invalid task identity fields")
			}
			return updated, nil
		}
	}
	return nil, fmt.Errorf("task.md frontmatter is not closed")
}

// Handover announces a task lease already granted by the lease owner. It never
// grants, transfers or takes a lease using post timestamps.
func (s Store) Handover(in Input, w Writer) (Record, error) {
	if w.Lease != "task" || w.Fence <= 0 {
		return Record{}, &InvalidRecordError{Err: fmt.Errorf("handover requires an already granted task lease")}
	}
	in.Type = "handover"
	if in.Body == nil {
		in.Body = map[string]any{}
	}
	return s.Post(in, w)
}
