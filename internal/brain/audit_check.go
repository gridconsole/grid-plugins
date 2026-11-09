package brain

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// CheckAuditHash is the finding for a published entry whose bytes no longer
// match the hash the keeper logged when it published them.
const CheckAuditHash = "audit_hash"

// AuditHashCheck is the replaceable host policy (key brain_audit_hash_levels,
// default home and project) for which levels compare entry hashes with the log.
var AuditHashCheck = func(level string) bool { return level == "home" || level == "project" }

type auditKey struct{ topic, name string }

type auditTip struct {
	c  Change
	at time.Time
}

// auditDrift compares each published entry with the newest hashed audit line
// for it. The log is streamed like Changes, never read whole. Lines written
// before hashes existed, rejections, and lines that do not parse give no hash,
// so they never raise a finding; an entry that is gone is not drift.
func auditDrift(root *os.Root, level string, res *CheckResult) error {
	f, err := openLog(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	tips := map[auditKey]auditTip{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), maxLogLine)
	for sc.Scan() {
		c, at, ok := parseAudit(sc.Text())
		// A rejection publishes nothing, so its hash never stands for an entry.
		if !ok || c.Status == "rejected" || c.Hash == "" || c.Topic == "" {
			continue
		}
		k := auditKey{c.Topic, c.Name}
		// Newest as Changes orders it: by stamp, a later line when stamps tie.
		if prev, seen := tips[k]; seen && at.Before(prev.at) {
			continue
		}
		tips[k] = auditTip{c, at}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("brain log: %w", err)
	}
	keys := make([]auditKey, 0, len(tips))
	for k := range tips {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].topic != keys[j].topic {
			return keys[i].topic < keys[j].topic
		}
		return keys[i].name < keys[j].name
	})
	for _, k := range keys {
		if !ValidName(k.topic) || !ValidName(k.name) {
			continue
		}
		rel := "knowledge/" + k.topic + "/" + k.name + ".md"
		b, err := safefile.Read(root, filepath.FromSlash(rel))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("entry %s: %w", rel, err)
		}
		if now := auditHash(b); now != tips[k].c.Hash {
			res.add(CheckAuditHash, scoped(level, rel), 0,
				fmt.Sprintf("entry changed outside the keeper: v%d was %s, now %s", tips[k].c.Version, tips[k].c.Hash, now), "")
		}
	}
	return nil
}
