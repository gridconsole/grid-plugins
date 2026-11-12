package jobcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// save publishes a complete immutable bundle by renaming a temporary directory.
// Rendering an unchanged Job again, when only the brain changed, returns the
// saved bundle and leaves it untouched; a changed Job, plan or file requires a
// new Job id.
func save(task string, j job.Job, r job.Rendered) (string, error) {
	path, _, err := saveReport(task, j, r)
	return path, err
}

// saveReport is save, and also says whether this call wrote the files (true) or
// found the Job's saved files already there (false).
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
	// same says the folder already holds a saved bundle this render may reuse: all
	// three files read through job.Within, job.yaml equals this Job's record byte
	// for byte, and every saved source that is not a brain note (the plan and the
	// other task, repo, prototype and Job files) equals this render's. Only the
	// brain may differ, so a retried attempt keeps the brain it was first given,
	// and a revised plan or file is refused.
	same := func() bool {
		saved := map[string][]byte{}
		for _, name := range []string{"brief.md", "inputs.json", "job.yaml"} {
			p, e := job.Within(task, filepath.Join("jobs", j.ID, name))
			if e != nil {
				return false
			}
			b, e := os.ReadFile(p)
			if e != nil {
				return false
			}
			saved[name] = b
		}
		if !bytes.Equal(saved["job.yaml"], files["job.yaml"]) {
			return false
		}
		old, e := fileSources(saved["inputs.json"])
		if e != nil {
			return false
		}
		now, e := fileSources(files["inputs.json"])
		return e == nil && bytes.Equal(old, now)
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

// fileSources is the JSON of the sources an inputs.json holds that are not brain
// notes: every pointer job.Files.Resolve does not read from the brain store.
func fileSources(inputs []byte) ([]byte, error) {
	var in job.Inputs
	if err := json.Unmarshal(inputs, &in); err != nil {
		return nil, err
	}
	kept := []job.Source{}
	for _, s := range in.Sources {
		if !strings.HasPrefix(s.Pointer, "brain:") {
			kept = append(kept, s)
		}
	}
	return json.Marshal(kept)
}
