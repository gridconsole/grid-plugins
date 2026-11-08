package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
)

func saveResult(dir string, result Result) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".result-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, result.ID+".json"))
}

// LoadResult loads a run by id, never by an arbitrary path supplied as an id.
func LoadResult(dir, id string) (Result, error) {
	var result Result
	if !identifier.MatchString(id) {
		return result, fmt.Errorf("invalid run id")
	}
	if err := readJSON(filepath.Join(dir, id+".json"), &result); err != nil {
		return result, err
	}
	if result.SchemaVersion != SchemaVersion || result.ID != id {
		return result, fmt.Errorf("unsupported result version or mismatched run id")
	}
	return result, nil
}

// WriteSummary renders the same facts for run and show. Fake runs stay labelled
// so their passing scores cannot be mistaken for model-quality measurements.
func WriteSummary(w io.Writer, results []Result) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "RUN\tTASK\tTIER\tSIZE\tAGENT\tSTATUS\tCHECKS\tGATES\tCHECK SCORE\tTIME")
	for _, r := range results {
		score := "not measured"
		if r.Summary.CheckScore != nil {
			score = fmt.Sprintf("%.2f/5", *r.Summary.CheckScore)
		}
		agent := "configured"
		if r.Fake {
			agent = "fake"
		} else if len(r.Agent) > 0 {
			agent = filepath.Base(r.Agent[0])
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d/%d\t%d/%d\t%s\t%dms\n",
			r.ID, r.Task, r.Tier, r.Size, agent, r.Status, r.Summary.Passed, len(r.Checks),
			r.Summary.GatesPassed, r.Summary.GatesTotal, score, r.DurationMS)
	}
	return tw.Flush()
}
