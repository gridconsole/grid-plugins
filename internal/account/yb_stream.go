package account

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// NoteStream keeps the rate-limit windows of a headless run for the folder dir.
// log is an attempt log of stream-json lines: the last rate_limit_event of each
// rateLimitType (five_hour, seven_day) is kept, any other type is ignored. The
// lines carry no time, so the snapshot is stamped with the log's modification
// time. A log with no such event writes nothing, as Note does for silence, and
// a snapshot already newer than the log is left as it is.
func NoteStream(env Env, log, dir string) error {
	if env.GridHome == "" {
		return errors.New("cannot find the Grid home; set GRID_HOME")
	}
	st, err := os.Stat(log)
	if err != nil {
		return err
	}
	f, err := os.Open(log)
	if err != nil {
		return err
	}
	defer f.Close()
	at := st.ModTime().UTC().Truncate(time.Second)
	snap := Snapshot{ConfigDir: gridhome.Real(dir), At: at.Format(time.RFC3339), Source: "stream"}
	if err := readStreamWindows(f, &snap); err != nil {
		return err
	}
	if snap.FiveHour == nil && snap.SevenDay == nil {
		return nil
	}
	if old, ok := readSnapshot(env, dir); ok {
		if t, err := time.Parse(time.RFC3339, old.At); err == nil && t.After(at) {
			return nil
		}
	}
	path := snapshotPath(env, dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	out, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(out, '\n'), 0o600)
}

// readStreamWindows reads the log line by line (a stream-json line can be very
// long, so no line size limit) and fills the windows of snap.
func readStreamWindows(r io.Reader, snap *Snapshot) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			noteStreamLine(line, snap)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func noteStreamLine(line []byte, snap *Snapshot) {
	if !bytes.Contains(line, []byte("rate_limit_event")) {
		return
	}
	var ev struct {
		Type string `json:"type"`
		Info struct {
			ResetsAt    float64  `json:"resetsAt"`
			Type        string   `json:"rateLimitType"`
			Utilization *float64 `json:"utilization"`
		} `json:"rate_limit_info"`
	}
	if json.Unmarshal(line, &ev) != nil || ev.Type != "rate_limit_event" || ev.Info.Utilization == nil {
		return
	}
	rd := &Reading{Used: math.Round(*ev.Info.Utilization*10000) / 100, ResetsAt: int64(ev.Info.ResetsAt)}
	switch ev.Info.Type {
	case "five_hour":
		snap.FiveHour = rd
	case "seven_day":
		snap.SevenDay = rd
	}
}
