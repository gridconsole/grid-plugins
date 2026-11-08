package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// Totals are sums of deltas, never estimates from session transcripts.
type Totals struct {
	Dollars float64 `json:"dollars"`
	Minutes float64 `json:"minutes"`
	Posts   int     `json:"posts"`
}

type Row struct {
	Job string `json:"job"`
	Totals
}

type Summary struct {
	Jobs   []Row  `json:"jobs"`
	Totals Totals `json:"totals"`
}

func emptySummary() Summary { return Summary{Jobs: []Row{}} }

func (s *Summary) add(r Record) error {
	s.Totals.Dollars += r.Dollars
	s.Totals.Minutes += r.Minutes
	s.Totals.Posts++
	if !finite(s.Totals.Dollars) || !finite(s.Totals.Minutes) {
		return errors.New("usage totals overflow")
	}
	i := sort.Search(len(s.Jobs), func(i int) bool { return s.Jobs[i].Job >= r.Job })
	if i == len(s.Jobs) || s.Jobs[i].Job != r.Job {
		s.Jobs = append(s.Jobs, Row{})
		copy(s.Jobs[i+1:], s.Jobs[i:])
		s.Jobs[i] = Row{Job: r.Job}
	}
	s.Jobs[i].Dollars += r.Dollars
	s.Jobs[i].Minutes += r.Minutes
	s.Jobs[i].Posts++
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
	if r.V != 1 {
		return errors.New("unsupported usage version")
	}
	if at, err := time.Parse(timeFormat, r.At); err != nil || at.Format(timeFormat) != r.At {
		return errors.New("at must be UTC with millisecond precision")
	}
	return (Input{Job: r.Job, Session: r.Session, Model: r.Model, Dollars: &r.Dollars, Minutes: &r.Minutes}).validate()
}
