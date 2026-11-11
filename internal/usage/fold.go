package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// Totals are sums of deltas, never estimates from session transcripts.
type Totals struct {
	Dollars float64 `json:"dollars"`
	Minutes float64 `json:"minutes"`
	Tokens  uint64  `json:"tokens"`
	Posts   int     `json:"posts"`
	// Whole micro-units are summed, so decimal amounts add exactly.
	dollarsMicro, minutesMicro int64
}

const microPerUnit = 1e6

// maxMicro keeps sums far from int64 overflow.
const maxMicro = int64(1) << 62

func toMicro(n float64) (int64, bool) {
	m := math.Round(n * microPerUnit)
	return int64(m), m >= 0 && m < float64(maxMicro/2)
}

func (t *Totals) addMicro(dollars, minutes int64) bool {
	t.dollarsMicro += dollars
	t.minutesMicro += minutes
	t.Dollars = float64(t.dollarsMicro) / microPerUnit
	t.Minutes = float64(t.minutesMicro) / microPerUnit
	t.Posts++
	return t.dollarsMicro < maxMicro && t.minutesMicro < maxMicro
}

type Row struct {
	Job string `json:"job"`
	Totals
}

type Summary struct {
	Jobs   []Row  `json:"jobs"`
	Totals Totals `json:"totals"`
	// keys are the once-only keys seen in the log; they are not part of the answer.
	keys map[string]bool
}

func emptySummary() Summary { return Summary{Jobs: []Row{}} }

func (s *Summary) add(r Record) error {
	d, okD := toMicro(r.Dollars)
	m, okM := toMicro(r.Minutes)
	if !okD || !okM {
		return errors.New("usage amount out of range")
	}
	tokens := uint64(0)
	if r.Tokens != nil {
		tokens = *r.Tokens
	}
	if tokens > math.MaxUint64-s.Totals.Tokens {
		return errors.New("usage token totals overflow")
	}
	s.Totals.Tokens += tokens
	if !s.Totals.addMicro(d, m) {
		return errors.New("usage totals overflow")
	}
	i := sort.Search(len(s.Jobs), func(i int) bool { return s.Jobs[i].Job >= r.Job })
	if i == len(s.Jobs) || s.Jobs[i].Job != r.Job {
		s.Jobs = append(s.Jobs, Row{})
		copy(s.Jobs[i+1:], s.Jobs[i:])
		s.Jobs[i] = Row{Job: r.Job}
	}
	s.Jobs[i].addMicro(d, m)
	s.Jobs[i].Tokens += tokens
	if r.Key != "" {
		if s.keys == nil {
			s.keys = map[string]bool{}
		}
		s.keys[r.Key] = true
	}
	return nil
}

func readLog(dir string) (Summary, int64, error) {
	if dir == "" {
		return emptySummary(), 0, nil
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, "usage.jsonl"), os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return emptySummary(), 0, nil
	}
	if err != nil {
		return Summary{}, 0, err
	}
	defer f.Close()
	return fold(f)
}

// fold returns the last complete byte offset so the writer can cut a torn tail.
func fold(r io.Reader) (Summary, int64, error) {
	out := emptySummary()
	var end int64
	reader := bufio.NewReader(r)
	for n := 1; ; n++ {
		line, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return out, end, nil
		}
		if err != nil {
			return out, end, err
		}
		var record Record
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		err = dec.Decode(&record)
		if err == nil {
			var extra any
			if dec.Decode(&extra) != io.EOF {
				err = errors.New("expected one JSON object")
			}
		}
		if err == nil {
			err = record.validate(line)
		}
		if err == nil {
			err = out.add(record)
		}
		if err != nil {
			return out, end, fmt.Errorf("usage.jsonl line %d: %w", n, err)
		}
		end += int64(len(line))
	}
}

func (r Record) validate(line []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(line, &fields); err != nil {
		return err
	}
	for _, name := range []string{"v", "at", "job", "session", "model", "dollars", "minutes"} {
		if len(fields[name]) == 0 || bytes.Equal(fields[name], []byte("null")) {
			return fmt.Errorf("missing or null %s", name)
		}
	}
	if raw, ok := fields["tokens"]; ok && bytes.Equal(raw, []byte("null")) {
		return errors.New("null tokens")
	}
	if r.V != 1 {
		return errors.New("unsupported usage version")
	}
	if at, err := time.Parse(contract.TimeLayout, r.At); err != nil || contract.FormatTime(at) != r.At {
		return errors.New("at must be UTC with millisecond precision")
	}
	return (Input{Job: r.Job, Session: r.Session, Model: r.Model, Dollars: &r.Dollars, Minutes: &r.Minutes}).validate()
}
