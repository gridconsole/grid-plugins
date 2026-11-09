package brain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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
// publication succeeds but a later write is interrupted.
func logAudit(root *os.Root, c Change) error {
	before, err := safefile.Read(root, logName)
	if errors.Is(err, fs.ErrNotExist) {
		before = []byte("# Brain log\n\n")
	} else if err != nil {
		return err
	}
	line := auditLine(c)
	if strings.Contains(string(before), line) {
		return nil
	}
	return atomicWrite(root, logName, append(before, []byte(line)...))
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
	b, err := safefile.Read(root, logName)
	if errors.Is(err, fs.ErrNotExist) {
		return []Change{}, nil
	}
	if err != nil {
		return nil, err
	}
	type stamped struct {
		Change
		at time.Time
	}
	var found []stamped
	for _, l := range strings.Split(string(b), "\n") {
		if c, at, ok := parseAudit(l); ok && !at.Before(since) {
			found = append(found, stamped{c, at})
		}
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
