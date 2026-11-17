package account

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// codexReading is the newest rate-limit reading found in a Codex folder: its
// windows as the log held them (the reset rule is applied when answering) and the
// time of the log line.
type codexReading struct {
	windows []Window
	at      time.Time
}

// codexWindowsFor fills res from the Codex session logs under the first folder
// of dirs that holds a reading. ok is false when no folder does, and res is then
// untouched. A "" folder is skipped: filepath.Join would turn it into a path
// under the working directory.
func codexWindowsFor(env Env, res AccountWindows, dirs []string) (AccountWindows, bool) {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		r, ok := newestCodexReading(filepath.Join(d, "sessions"))
		if !ok {
			continue
		}
		for _, w := range r.windows {
			res.Windows = append(res.Windows, resetWindow(env, w.Label, w.Used, w.ResetsAt))
		}
		res.AsOf, res.Source = r.at.UTC().Format(time.RFC3339), "codex"
		return res, true
	}
	return res, false
}

type codexFile struct {
	path string
	mod  time.Time
}

// codexRollouts lists <sessions>/**/rollout-*.jsonl, the most recently written
// first (a resumed session appends to its first file, so the name's date is not
// the time of the last line), ties by path, later first.
func codexRollouts(sessions string) []codexFile {
	var files []codexFile
	// WalkDir does not descend a root that is itself a link, so a sessions folder
	// that links elsewhere is resolved first.
	if real, err := filepath.EvalSymlinks(sessions); err == nil {
		sessions = real
	}
	_ = filepath.WalkDir(sessions, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files = append(files, codexFile{path, info.ModTime()})
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		if !files[i].mod.Equal(files[j].mod) {
			return files[i].mod.After(files[j].mod)
		}
		return files[i].path > files[j].path
	})
	return files
}

// newestCodexReading answers the reading with the latest line time in the
// rollout files under sessions. The files are read most recently written first,
// and the scan stops at the first file written before the best line found, since
// no line of it can be later. A file with no reading (the "premium" lines carry
// null windows) is passed over, so the answer comes from an older file.
func newestCodexReading(sessions string) (best codexReading, found bool) {
	for _, f := range codexRollouts(sessions) {
		if found && f.mod.Before(best.at) {
			break
		}
		if r, ok := lastCodexReading(f.path, f.mod); ok && (!found || r.at.After(best.at)) {
			best, found = r, true
		}
	}
	return best, found
}

// lastCodexReading reads one rollout line by line and keeps the last line that
// holds a reading. A line has no size limit (they run to tens of kilobytes). A
// line that does not parse, such as the torn last line of a file being written,
// is skipped. A line with no timestamp is dated by the file's modification time.
func lastCodexReading(path string, mod time.Time) (last codexReading, found bool) {
	f, err := os.Open(path)
	if err != nil {
		return last, false
	}
	defer f.Close()
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if ws, at, ok := parseCodexLine(line); ok {
				if at.IsZero() {
					at = mod
				}
				last, found = codexReading{ws, at}, true
			}
		}
		if err != nil { // io.EOF, or a read error: what was read stands
			return last, found
		}
	}
}

// resetWindow is one answered window. It keeps the reset rule of the Claude path
// in windowsFor: a window whose reset time has passed started over after the
// reading, so its use is 0.
func resetWindow(env Env, label string, used float64, resetsAt int64) Window {
	w := Window{Label: label, Used: used, ResetsAt: resetsAt}
	if resetsAt != 0 && env.now().Unix() >= resetsAt {
		w.Used = 0
	}
	return w
}

// codexLimit is one window of a token_count line's rate_limits: used_percent is
// 0-100, window_minutes its length and resets_at absolute Unix seconds.
type codexLimit struct {
	Used     *float64 `json:"used_percent"`
	Minutes  float64  `json:"window_minutes"`
	ResetsAt float64  `json:"resets_at"`
}

// codexLabel maps a window by its length, not by its position (primary and
// secondary are slots): up to 360 minutes is "5h", 1440 or more is "7d", and
// anything between or without a length is not read.
func codexLabel(minutes float64) string {
	switch {
	case minutes <= 0:
		return ""
	case minutes <= 360:
		return "5h"
	case minutes >= 1440:
		return "7d"
	}
	return ""
}

// parseCodexLine reads the rate_limits of one rollout line, "5h" before "7d". It
// answers ok only for a line with at least one window it can map; the lines of
// the "premium" limit, whose primary and secondary are null, are not readings.
func parseCodexLine(line []byte) (ws []Window, at time.Time, ok bool) {
	if !bytes.Contains(line, []byte(`"rate_limits"`)) || !bytes.Contains(line, []byte(`"used_percent"`)) {
		return nil, at, false
	}
	var rec struct {
		Timestamp string `json:"timestamp"`
		Payload   struct {
			RateLimits *struct {
				Primary   *codexLimit `json:"primary"`
				Secondary *codexLimit `json:"secondary"`
			} `json:"rate_limits"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &rec) != nil || rec.Payload.RateLimits == nil {
		return nil, at, false
	}
	byLabel := map[string]Window{}
	for _, l := range []*codexLimit{rec.Payload.RateLimits.Primary, rec.Payload.RateLimits.Secondary} {
		if l == nil || l.Used == nil {
			continue
		}
		label := codexLabel(l.Minutes)
		if _, taken := byLabel[label]; label == "" || taken {
			continue
		}
		byLabel[label] = Window{Label: label, Used: *l.Used, ResetsAt: int64(l.ResetsAt)}
	}
	for _, label := range []string{"5h", "7d"} {
		if w, has := byLabel[label]; has {
			ws = append(ws, w)
		}
	}
	if len(ws) == 0 {
		return nil, at, false
	}
	if t, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err == nil {
		at = t
	}
	return ws, at, true
}
