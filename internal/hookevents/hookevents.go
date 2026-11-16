// Package hookevents keeps one line per rule that a hook run consulted, and reads
// the lines back. The file is <Grid home>/.hook-counts/events.jsonl, next to the
// home-allow counter the hook already keeps there. It is always under the Grid
// home, never under CLAUDE_PLUGIN_DATA: grid hook events runs outside Claude Code,
// where that variable is not set, and must find the file the hooks wrote.
package hookevents

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gridconsole/grid-plugins/internal/logshift"
)

const (
	// Dir is the folder the hook's own records live in, under the Grid home.
	Dir = ".hook-counts"
	// File is the log inside Dir.
	File = "events.jsonl"
	// Limit and Files bound the log the way grid open bounds the app log
	// (logshift): the log is shifted to events.jsonl.1 at Limit bytes and the
	// oldest copy is dropped, so Files files of about Limit bytes are the most that
	// stay: at most 6 MiB an account. At about 100 bytes a line that is some 60,000
	// lines, which is over 3 hours at 250 lines a minute. A busy lane account writes
	// about 200 lines a minute, and the old 3 x 256 KiB (about 7,500 lines) covered
	// only some 40 minutes, less than the hourly check window.
	Limit = 2 << 20
	Files = 3
)

// The outcomes. A rule that was consulted and let the event through is ok; it
// spoke (a warning, or a question the person answers) and did not refuse is
// warned; it refused is blocked; its level was off is skipped.
const (
	OK      = "ok"
	Warned  = "warned"
	Blocked = "blocked"
	Skipped = "skipped"
)

// Record is one line: the rule as grid hook levels names it, the hook event it was
// consulted for, when (UTC, RFC 3339) and the outcome. A rule that refuses or
// warns may add the task it refused for (the task folder's id, T-NN) and a short
// reason; job-lint and stop-check do. Nothing else is kept: no tool, path, prompt,
// session or project. A line written before Task and Reason existed reads with
// both empty.
type Record struct {
	Rule    string `json:"rule"`
	Event   string `json:"event"`
	At      string `json:"at"`
	Outcome string `json:"outcome"`
	Task    string `json:"task,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

var folderTask = regexp.MustCompile(`^(T-[0-9]+)(?:-.+)?$`)

// TaskOf is the task id a task folder holds: the T-NN the folder's name starts
// with (T-10-rate-limits is T-10). A folder named some other way gives its name.
func TaskOf(folder string) string {
	name := filepath.Base(folder)
	if m := folderTask.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return name
}

// Path is the log of a Grid home.
func Path(gridHome string) string { return filepath.Join(gridHome, Dir, File) }

// Append writes the records with one append. It never reports a failure and never
// panics: a record must not change what a hook does or says. It creates Dir but
// not the Grid home.
func Append(gridHome string, recs []Record) {
	defer func() { _ = recover() }()
	if gridHome == "" || len(recs) == 0 {
		return
	}
	if fi, err := os.Stat(gridHome); err != nil || !fi.IsDir() {
		return
	}
	if err := os.Mkdir(filepath.Join(gridHome, Dir), 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return
	}
	var buf []byte
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			return
		}
		buf = append(append(buf, b...), '\n')
	}
	path := Path(gridHome)
	logshift.Shift(path, Limit, Files)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return
	}
	// A write cut short (a full disk) leaves a last line with no newline; the next
	// record would join it and both would be lost. Start a fresh line in the same
	// write, so only the torn record is lost.
	if endsTorn(f) {
		buf = append([]byte{'\n'}, buf...)
	}
	_, _ = f.Write(buf)
	_ = f.Close()
}

// endsTorn reports whether the file is not empty and its last byte is not '\n'.
// A file it cannot read counts as not torn.
func endsTorn(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Size() == 0 {
		return false
	}
	var last [1]byte
	if n, _ := f.ReadAt(last[:], fi.Size()-1); n != 1 {
		return false
	}
	return last[0] != '\n'
}

// Read returns the log newest first, across the shifted copies. A missing log is
// empty. A line that does not parse is skipped.
func Read(gridHome string) ([]Record, error) {
	if gridHome == "" {
		return nil, nil
	}
	dir := filepath.Join(gridHome, Dir)
	if fi, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	} else if !fi.IsDir() {
		return nil, nil
	}
	var out []Record
	// Oldest copy first, so the slice runs oldest to newest before it is reversed.
	names := logshift.Names(Path(gridHome), Files)
	for i := len(names) - 1; i >= 0; i-- {
		recs, err := readFile(names[i])
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func readFile(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 64<<10)
	for sc.Scan() {
		var r Record
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.Rule != "" {
			out = append(out, r)
		}
	}
	return out, nil
}

// Last returns the newest record of each rule in a log that Read returned.
func Last(newestFirst []Record) map[string]Record {
	out := map[string]Record{}
	for _, r := range newestFirst {
		if _, ok := out[r.Rule]; !ok {
			out[r.Rule] = r
		}
	}
	return out
}
