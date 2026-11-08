package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/artifact"
	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/proof"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// DeliverOptions supplies the person's outcome and the files to copy. It is
// command input; existing task records remain owned by task.Read and Fold.
type DeliverOptions struct {
	Cwd, Task, Outcome, By string
	Files                  []string
	DryRun                 bool
}

// Delivery describes performed or proposed actions, not a persisted task record.
type Delivery struct {
	Task      string            `json:"task"`
	DryRun    bool              `json:"dry_run"`
	Actions   []string          `json:"actions"`
	Artifacts []artifact.Record `json:"artifacts"`
}

func (s Store) Deliver(o DeliverOptions) (Delivery, error) {
	out := Delivery{DryRun: o.DryRun, Actions: []string{}, Artifacts: []artifact.Record{}}
	if strings.TrimSpace(o.Outcome) == "" || strings.ContainsRune(o.Outcome, 0) {
		return out, errors.New("supply a nonempty Outcome with --outcome")
	}
	if strings.TrimSpace(o.By) == "" || strings.ContainsAny(o.By, "\r\n\x00") {
		return out, errors.New("by must name the author on one line")
	}
	t, err := s.Show(o.Cwd, o.Task)
	if err != nil {
		return out, err
	}
	out.Task = t.ID
	dir, err := filepath.EvalSymlinks(t.Path)
	if err != nil {
		return out, err
	}
	project := t.Project
	if project == where.Personal {
		project = ""
	}
	bs := brain.Store{GridHome: s.Resolver.GridHome, Project: project}
	ps := proof.Store{TaskDir: dir}
	res, err := s.Resolver.Resolve(where.Query{Cwd: dir})
	if err != nil {
		return out, err
	}
	artifacts := artifact.Store{TaskPath: dir, ProjectFolder: res.Folder, Now: s.Now}
	files := make([]string, len(o.Files))
	for i, file := range o.Files {
		files[i] = file
		if !filepath.IsAbs(file) {
			files[i] = filepath.Join(o.Cwd, file)
		}
	}

	// Everything in preflight is read-only, including the dry-run path.
	preflight := func() (frontmatter.Document, error) {
		path, err := localdata.Safe(dir, "task.md")
		if err != nil {
			return frontmatter.Document{}, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return frontmatter.Document{}, err
		}
		doc, err := frontmatter.Parse(string(data))
		if err != nil {
			return doc, err
		}
		for _, key := range []string{"state", "status"} {
			if doc.Data[key] == "closed" || doc.Data[key] == "cancelled" || doc.Data[key] == "stopped" {
				return doc, errors.New("task is closed or stopped; delivery cannot reopen it")
			}
		}
		waiting, err := bs.Waiting(dir)
		if err != nil {
			return doc, err
		}
		if len(waiting) > 0 {
			return doc, fmt.Errorf("Deliver blocked: %d brain proposals wait. Run: grid brain keep", len(waiting))
		}
		idx, err := ps.Build()
		if err != nil {
			return doc, err
		}
		if err = idx.Check(); err != nil {
			return doc, err
		}
		for _, file := range files {
			if err = artifacts.CheckImport(file); err != nil {
				return doc, err
			}
		}
		for _, name := range []string{"history.md", "proof/index.json"} {
			path, err := localdata.Safe(dir, filepath.FromSlash(name))
			if err != nil {
				return doc, err
			}
			if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
				return doc, fmt.Errorf("%s must be a regular file", name)
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				return doc, err
			}
		}
		return doc, nil
	}
	if _, err = preflight(); err != nil {
		return out, err
	}
	out.Actions = append(out.Actions, "Write Outcome in task.md")
	for _, file := range files {
		out.Actions = append(out.Actions, "Copy "+filepath.Base(file)+" into artifacts/")
	}
	out.Actions = append(out.Actions, "Rebuild proof/index.json", "Append delivery history", "Raise ship gate")
	if o.DryRun {
		return out, nil
	}
	unlock, err := localdata.LockExisting(dir, ".task.lock")
	if err != nil {
		return out, err
	}
	defer unlock()
	// A plan revision cannot overtake the proof check and the raised gate.
	unlockPlan, err := localdata.LockExisting(dir, ".plan.lock")
	if err != nil {
		return out, err
	}
	defer unlockPlan()
	err = bs.WithKeeperGate(dir, func() error {
		doc, err := preflight()
		if err != nil {
			return err
		}
		now := time.Now()
		if s.Now != nil {
			now = s.Now()
		}
		outcome := strings.TrimSpace(o.Outcome)
		summary := strings.TrimSpace(strings.SplitN(outcome, "\n", 2)[0])
		doc.Data["outcome"] = summary
		doc.Data["updated"] = now.UTC().Format(time.RFC3339)
		originalBody := doc.Body
		// This closure only encodes this task update, using the shared frontmatter
		// parser and YAML library rather than introducing a new frontmatter dialect.
		save := func() error {
			front, err := yaml.Dump(doc.Data, yaml.WithIndent(2), yaml.WithLineWidth(-1))
			if err != nil {
				return err
			}
			data := []byte("---\n" + string(front) + "---\n" + replaceOutcome(originalBody, outcome))
			return atomicfile.Write(filepath.Join(dir, "task.md"), data, 0644)
		}
		if err = save(); err != nil {
			return err
		}
		for _, file := range files {
			record, err := artifacts.Import(file, "")
			if err != nil {
				return err
			}
			out.Artifacts = append(out.Artifacts, record)
		}
		if len(out.Artifacts) > 0 {
			outcome += "\n\n### Deliverables\n"
			for _, r := range out.Artifacts {
				outcome += "\n- artifacts/" + r.File
			}
		}
		idx, err := ps.Rebuild()
		if err != nil {
			return err
		}
		if err = idx.Check(); err != nil {
			return err
		}
		outcome += "\n\n### Proof\n\n"
		for _, item := range idx.Items {
			outcome += fmt.Sprintf("- %s: %s proof saved in proof/%s/\n", item.ID, item.Level, item.ID)
		}
		if err = save(); err != nil {
			return err
		}
		if err = AppendHistory(dir, "delivered · "+summary+" · by "+o.By, now); err != nil {
			return err
		}
		// Waiting is a request for approval; no approval or closing is synthesized.
		doc.Data["state"] = "waiting"
		if _, ok := doc.Data["status"]; ok {
			doc.Data["status"] = "waiting"
		}
		doc.Data["step"] = "ship"
		doc.Data["gate"] = "ship"
		doc.Data["next"] = "Review the Outcome and approve ship"
		return save()
	})
	return out, err
}

// Replace only Outcome, preserving later peer sections and all earlier notes.
func replaceOutcome(body, outcome string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	start, end := -1, len(lines)
	fenced := false
	for i, line := range lines {
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if strings.TrimSpace(line) == "## Outcome" && start < 0 {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(line, "## ") {
			end = i
			break
		}
	}
	section := "## Outcome\n\n" + strings.TrimSpace(outcome) + "\n"
	if start < 0 {
		return strings.TrimRight(body, "\n") + "\n\n" + section
	}
	before := strings.TrimRight(strings.Join(lines[:start], "\n"), "\n")
	after := strings.Join(lines[end:], "\n")
	if before != "" {
		before += "\n\n"
	}
	if after != "" {
		section += "\n"
	}
	return before + section + after
}
