package feed

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	if err := lock(f, true); err != nil {
		return "", err
	}
	defer unlock(f)
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
func TaskID(taskDir string) (string, error) {
	f, err := os.Open(filepath.Join(taskDir, "task.md"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	if !scan.Scan() || strings.TrimSpace(scan.Text()) != "---" {
		return "", fmt.Errorf("task.md needs frontmatter with a stable id")
	}
	var id string
	for scan.Scan() {
		line := scan.Text()
		if strings.TrimSpace(line) == "---" {
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
	if id == "" {
		return "", fmt.Errorf("task.md needs a stable ULID in id or ulid")
	}
	return id, nil
}

// Handover announces a task lease already granted by the lease owner. It never
// grants, transfers or takes a lease using post timestamps.
func (s Store) Handover(in Input, w Writer) (Record, error) {
	if w.Lease != "task" || w.Fence <= 0 {
		return Record{}, fmt.Errorf("handover requires an already granted task lease")
	}
	in.Type = "handover"
	if in.Body == nil {
		in.Body = map[string]any{}
	}
	return s.Post(in, w)
}
