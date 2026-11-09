package brain

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// Change is one audit line of knowledge/log.md. Lines written before versions
// existed have no topic, version 0 and an empty hash; so does a rejection,
// which publishes nothing.
type Change struct {
	At         string `json:"at"` // contract.TimeLayout
	Status     string `json:"status"`
	Topic      string `json:"topic"`
	Name       string `json:"name"`
	Version    int    `json:"version"`
	Hash       string `json:"hash"`
	OriginKind string `json:"origin_kind"`
	OriginID   string `json:"origin_id"`
	Reason     string `json:"reason"`
}

const logName = "knowledge/log.md"

const (
	// logTail is how much of the log's end logAudit reads to spot a retry.
	logTail = 64 << 10
	// maxLogLine bounds one line while Changes scans; the log itself has no size cap.
	maxLogLine = 1 << 20
)

// An audit line appends topic, version and hash to the original five fields,
// so those keep their positions: at, status, name, kind:id, reason, topic,
// v<n>, sha256:<16 hex>.
var (
	versionField = regexp.MustCompile(`^v(0|[1-9][0-9]*)$`)
	hashField    = regexp.MustCompile(`^(sha256:[0-9a-f]{16})?$`)
)

func auditLine(c Change) string {
	clean := func(s string) string { return strings.ReplaceAll(strings.Join(strings.Fields(s), " "), "|", "/") }
	return fmt.Sprintf("- %s | %s | %s | %s:%s | %s | %s | v%d | %s\n", c.At, c.Status, c.Name, c.OriginKind, clean(c.OriginID), clean(c.Reason), c.Topic, c.Version, c.Hash)
}

// parseAudit reads both formats and skips anything else, so a hand-edited log
// never breaks a read.
func parseAudit(line string) (Change, time.Time, bool) {
	rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "- ")
	if !ok {
		return Change{}, time.Time{}, false
	}
	f := strings.Split(rest, "|")
	if len(f) != 5 && len(f) < 8 {
		return Change{}, time.Time{}, false
	}
	for i := range f {
		f[i] = strings.TrimSpace(f[i])
	}
	at, err := time.Parse(contract.TimeLayout, f[0])
	kind, id, hasOrigin := strings.Cut(f[3], ":")
	if err != nil || !hasOrigin || f[1] == "" || f[2] == "" {
		return Change{}, time.Time{}, false
	}
	c := Change{At: f[0], Status: f[1], Name: f[2], OriginKind: kind, OriginID: id, Reason: f[4]}
	if len(f) >= 8 {
		if !versionField.MatchString(f[6]) || !hashField.MatchString(f[7]) {
			return Change{}, time.Time{}, false
		}
		c.Topic, c.Hash = f[5], f[7]
		c.Version, _ = strconv.Atoi(f[6][1:])
	}
	return c, at, true
}

// logAudit runs under the level lock. A stable line makes retries safe when
// publication succeeds but a later write is interrupted. The log grows with
// every config edit, so it is appended to and never read whole: a retry is
// spotted in the last logTail bytes only.
func logAudit(root *os.Root, c Change) error {
	if err := safefile.NoSymlinks(root, logName); err != nil {
		return err
	}
	if info, err := root.Lstat(logName); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("brain log must be a regular file")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := root.MkdirAll(filepath.Dir(logName), 0o700); err != nil {
		return err
	}
	f, err := root.OpenFile(logName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("brain log must be a regular file")
	}
	line := auditLine(c)
	size := info.Size()
	prefix := ""
	if size == 0 {
		prefix = "# Brain log\n\n"
	} else {
		start := max(0, size-logTail)
		tail := make([]byte, size-start)
		if _, err := io.ReadFull(io.NewSectionReader(f, start, size-start), tail); err != nil {
			return err
		}
		if bytes.Contains(tail, []byte(line)) {
			return nil
		}
		// An interrupted append can leave a line without its newline.
		if tail[len(tail)-1] != '\n' {
			prefix = "\n"
		}
	}
	_, err = f.WriteString(prefix + line)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil && size == 0 {
		err = atomicfile.SyncDir(filepath.Join(root.Name(), filepath.Dir(logName)))
	}
	return err
}

// openLog opens the log for reading with the checks safefile.Read makes, minus
// its size cap: the caller streams it.
func openLog(root *os.Root) (*os.File, error) {
	if err := safefile.NoSymlinks(root, logName); err != nil {
		return nil, err
	}
	info, err := root.Lstat(logName)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("brain log must be a regular file")
	}
	f, err := root.Open(logName)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("brain log must be a regular file")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// Changes reads the level's audit log, newest first. Entries at or after since
// are kept (the zero time keeps all) and limit caps the count (zero or less
// means no cap). A missing log is an empty list.
func (s Store) Changes(level string, since time.Time, limit int) ([]Change, error) {
	root, err := s.scope(level, false)
	if errors.Is(err, fs.ErrNotExist) {
		return []Change{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := openLog(root)
	if errors.Is(err, fs.ErrNotExist) {
		return []Change{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	type stamped struct {
		Change
		at time.Time
	}
	var found []stamped
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), maxLogLine)
	for sc.Scan() {
		if c, at, ok := parseAudit(sc.Text()); ok && !at.Before(since) {
			found = append(found, stamped{c, at})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("brain log: %w", err)
	}
	// The log is append-only, so a later line is newer when stamps tie.
	for i, j := 0, len(found)-1; i < j; i, j = i+1, j-1 {
		found[i], found[j] = found[j], found[i]
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].at.After(found[j].at) })
	if limit > 0 && len(found) > limit {
		found = found[:limit]
	}
	result := make([]Change, len(found))
	for i, f := range found {
		result[i] = f.Change
	}
	return result, nil
}
