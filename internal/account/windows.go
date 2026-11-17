package account

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// Reading is one rate-limit window as Claude Code's status line reports it:
// used_percentage 0-100 and resets_at in Unix seconds.
type Reading struct {
	Used     float64 `json:"used_percentage"`
	ResetsAt int64   `json:"resets_at,omitempty"`
}

// Snapshot is what `grid account note` keeps for one config folder. It holds
// use numbers only: nothing else of the status line JSON is kept.
type Snapshot struct {
	ConfigDir string   `json:"configDir"`
	FiveHour  *Reading `json:"five_hour,omitempty"`
	SevenDay  *Reading `json:"seven_day,omitempty"`
	At        string   `json:"at"`
	// Source is "stream" for numbers read from an attempt log; empty (every
	// snapshot written before the key existed) means the status line.
	Source string `json:"source,omitempty"`
}

// Window is one row of the account_windows shape.
type Window struct {
	Label    string  `json:"label"` // "5h" or "7d"
	Used     float64 `json:"used"`
	ResetsAt int64   `json:"resetsAt,omitempty"`
}

// AccountWindows is one account's entry of the account_windows shape. A source
// that is missing gives source "none" and no windows, which the app shows as
// "No data", never as 0.
type AccountWindows struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Windows []Window `json:"windows"`
	AsOf    string   `json:"asOf,omitempty"`
	Source  string   `json:"source"` // direct | statusline | stream | codex | none
	Note    string   `json:"note,omitempty"`
}

func snapshotPath(env Env, dir string) string {
	sum := sha256.Sum256([]byte(gridhome.Real(dir)))
	return filepath.Join(env.GridHome, "accounts", "windows", hex.EncodeToString(sum[:4])+".json")
}

// ConfigDir is the folder a Claude Code session runs on: CLAUDE_CONFIG_DIR, or ~/.claude.
func ConfigDir(env Env) string {
	if d := env.getenv(dirVars[Claude]); d != "" {
		return filepath.Clean(d)
	}
	return filepath.Join(env.Home, ".claude")
}

// Note reads status line JSON and keeps its rate_limits for the folder dir. A
// payload with no window (an API-key sign-in, or before the first reply) writes
// nothing, so an earlier snapshot is not overwritten by silence.
func Note(env Env, in io.Reader, dir string) error {
	_, err := NoteKept(env, in, dir)
	return err
}

// NoteKept is Note that also answers what it read: the snapshot of the payload
// (nil windows when it held none), which `note --show` prints.
func NoteKept(env Env, in io.Reader, dir string) (Snapshot, error) {
	if env.GridHome == "" {
		return Snapshot{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return Snapshot{}, err
	}
	snap, err := parsePayload(env, data, dir)
	if err != nil {
		return Snapshot{}, err
	}
	if snap.FiveHour == nil && snap.SevenDay == nil {
		return snap, nil
	}
	path := snapshotPath(env, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return snap, err
	}
	out, err := json.Marshal(snap)
	if err != nil {
		return snap, err
	}
	return snap, atomicfile.Write(path, append(out, '\n'), 0o600)
}

// parsePayload reads the rate_limits of one status line payload.
func parsePayload(env Env, data []byte, dir string) (Snapshot, error) {
	var payload struct {
		RateLimits struct {
			FiveHour *struct {
				Used     float64 `json:"used_percentage"`
				ResetsAt float64 `json:"resets_at"`
			} `json:"five_hour"`
			SevenDay *struct {
				Used     float64 `json:"used_percentage"`
				ResetsAt float64 `json:"resets_at"`
			} `json:"seven_day"`
		} `json:"rate_limits"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{ConfigDir: gridhome.Real(dir), At: env.now().UTC().Format(time.RFC3339)}
	if w := payload.RateLimits.FiveHour; w != nil {
		snap.FiveHour = &Reading{w.Used, int64(w.ResetsAt)}
	}
	if w := payload.RateLimits.SevenDay; w != nil {
		snap.SevenDay = &Reading{w.Used, int64(w.ResetsAt)}
	}
	return snap, nil
}

func readSnapshot(env Env, dir string) (Snapshot, bool) {
	if env.GridHome == "" {
		return Snapshot{}, false
	}
	data, err := os.ReadFile(snapshotPath(env, dir))
	if err != nil {
		return Snapshot{}, false
	}
	var s Snapshot
	if json.Unmarshal(data, &s) != nil || (s.FiveHour == nil && s.SevenDay == nil) {
		return Snapshot{}, false
	}
	return s, true
}

// windowsFor answers the account_windows entry for dirs, the folders that may
// hold this account's numbers, in order. The sources, best first:
//
//  1. the direct read: the provider's own command, asked for the windows (cached,
//     see direct.go), whose last good reading stays when a later read fails;
//  2. a status line or stream note, but only one made after that reading;
//  3. none, with the reason the direct read gave.
//
// The Codex rollout record is a source between 2 and 3 once it is on main; its
// place is marked below.
func windowsFor(env Env, id, kind string, dirs []string) AccountWindows {
	res := AccountWindows{ID: id, Kind: kind, Windows: []Window{}, Source: "none"}
	direct, reason := directFor(env, kind, dirs)
	var (
		note   Snapshot
		hasNot bool
	)
	if kind == Claude {
		for _, d := range dirs {
			if s, ok := readSnapshot(env, d); ok {
				note, hasNot = s, true
				break
			}
		}
	}
	switch {
	case hasNot && (!direct.good() || madeAfter(note.At, direct.At)):
		res.Windows = readingWindows(env, note.FiveHour, note.SevenDay)
		res.AsOf, res.Source = note.At, note.Source
		if res.Source == "" {
			res.Source = "statusline"
		}
	case direct.good():
		res.Windows = readingWindows(env, direct.FiveHour, direct.SevenDay)
		res.AsOf, res.Source = direct.At, "direct"
	default:
		// The Codex rollout record would be read here, only when it is on main.
		res.Note = reason
		if res.Note == "" {
			res.Note = "No use reported for this folder yet"
		}
	}
	return res
}

// readingWindows turns the two readings into the rows of the shape. A window
// whose reset time has passed started over after the reading, so it shows 0.
func readingWindows(env Env, five, seven *Reading) []Window {
	out := []Window{}
	add := func(label string, r *Reading) {
		if r == nil {
			return
		}
		w := Window{Label: label, Used: r.Used, ResetsAt: r.ResetsAt}
		if r.ResetsAt != 0 && env.now().Unix() >= r.ResetsAt {
			w.Used = 0
		}
		out = append(out, w)
	}
	add("5h", five)
	add("7d", seven)
	return out
}

// madeAfter says whether the time a is later than b. A time that cannot be read
// is the oldest there is.
func madeAfter(a, b string) bool {
	ta, errA := time.Parse(time.RFC3339, a)
	tb, errB := time.Parse(time.RFC3339, b)
	switch {
	case errA != nil:
		return false
	case errB != nil:
		return true
	}
	return ta.After(tb)
}
