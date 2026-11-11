package jobcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// save publishes a complete immutable bundle by renaming a temporary directory.
// Repeating an identical render succeeds; a changed Job requires a new Job id.
func save(task string, j job.Job, r job.Rendered) (string, error) {
	path, _, err := saveReport(task, j, r)
	return path, err
}

// saveReport is save, and also says whether this call wrote the files (true) or
// found the same files already there (false).
func saveReport(task string, j job.Job, r job.Rendered) (string, bool, error) {
	if !job.ValidID(j.ID) {
		return "", false, fmt.Errorf("invalid Job id")
	}
	if err := os.MkdirAll(filepath.Join(task, "jobs"), 0755); err != nil {
		return "", false, err
	}
	parent, err := job.Within(task, "jobs")
	if err != nil {
		return "", false, err
	}
	target := filepath.Join(parent, j.ID)
	path := filepath.Join(target, "brief.md")
	inputs, err := r.Inputs.JSON()
	if err != nil {
		return "", false, err
	}
	metadata, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return "", false, err
	}
	files := map[string][]byte{"brief.md": []byte(r.Brief), "inputs.json": append(inputs, '\n'), "job.yaml": append(metadata, '\n')}
	same := func() bool {
		for _, name := range []string{"brief.md", "inputs.json", "job.yaml"} {
			p, e := job.Within(task, filepath.Join("jobs", j.ID, name))
			if e != nil {
				return false
			}
			b, e := os.ReadFile(p)
			if e != nil || !bytes.Equal(b, files[name]) {
				return false
			}
		}
		return true
	}
	if _, err := os.Lstat(target); err == nil {
		if same() {
			return path, false, nil
		}
		return "", false, fmt.Errorf("Job %s already has different saved inputs; create a new Job for revised work", j.ID)
	} else if !os.IsNotExist(err) {
		return "", false, err
	}
	tmp, err := os.MkdirTemp(parent, ".render-")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(tmp)
	for _, name := range []string{"brief.md", "inputs.json", "job.yaml"} {
		if err := os.WriteFile(filepath.Join(tmp, name), files[name], 0644); err != nil {
			return "", false, err
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		if same() {
			return path, false, nil
		}
		return "", false, err
	}
	return path, true, nil
}
