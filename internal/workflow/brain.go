package workflow

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// The sources ReadFor can answer from, in the order it tries them.
const (
	SourceTask         = "task"
	SourceProjectBrain = "project-brain"
	SourceHomeBrain    = "home-brain"
	SourceDefault      = "default"
)

// NoteType is the front matter `type:` that marks a brain note as a workflow.
const NoteType = "workflow"

// ReadFor answers which workflow applies to a task: its own workflow.md, else
// the project brain's workflow note, else the Grid home brain's, else the
// shipped default. Source (and Note, for a brain note) say which one answered.
// A source that cannot be read or parsed adds a line to Errors and the next
// source is tried, so a broken file never stops a task. taskDir and project
// may be empty; project is the project folder name.
func ReadFor(r where.Resolver, project, taskDir string) Answer {
	out := Answer{Files: []File{}, Templates: []Template{}, Errors: []Problem{}}
	if taskDir != "" {
		raw, err := readTaskPolicy(taskDir)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			out.Errors = append(out.Errors, readFailure("workflow.md", err))
		default:
			if out.take(SourceTask, "", "workflow.md", string(raw), project) {
				return out
			}
		}
	}
	home := r.GridHome
	if home != "" {
		home = gridhome.Real(home)
		if project != "" && r.PersonalProjectAllowed(project) && r.GuestProjectAllowed(project) {
			if out.fromBrain(home, "project", project, SourceProjectBrain, project) {
				return out
			}
		}
		// The home brain is personal; callers who cannot see it get the default.
		if !r.Caller.IsGuest() && r.PersonalOK() {
			if out.fromBrain(home, "home", "", SourceHomeBrain, project) {
				return out
			}
		}
	}
	out.Source = SourceDefault
	out.GatePostRoles = defaultGatePostRoles()
	return out
}

// take reads workflow text from any source through the one parser: gate roles
// from the front matter, templates (steps, approvers) from the workflow body.
// Front matter that does not parse rejects the source; a bad template section
// is reported and the good ones are kept.
func (a *Answer) take(source, note, label, text, project string) bool {
	roles, err := gatePostRoles([]byte(text))
	if err != nil {
		a.Errors = append(a.Errors, Problem{File: label, Message: err.Error()})
		return false
	}
	a.Source, a.Note, a.GatePostRoles = source, note, roles
	if !hasWorkflowHeading(text, project) {
		return true
	}
	parsed := Parse(text, project)
	for _, t := range parsed.Templates {
		t.File = label
		a.Templates = append(a.Templates, t)
	}
	for _, p := range parsed.Errors {
		// A note is not the workspace file, so it owes no default template.
		if strings.HasPrefix(p.Message, "the file has no default workflow") {
			continue
		}
		p.File = label
		a.Errors = append(a.Errors, p)
	}
	return true
}

// hasWorkflowHeading says whether the body after the front matter opens with
// the workflows.md heading. A note that only sets policy has no templates.
func hasWorkflowHeading(text, project string) bool {
	_, body, err := mdfile.Split([]byte(text))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line == "# Workflows" || (project != "" && line == "# "+project+" workflows")
	}
	return false
}

func readFailure(label string, err error) Problem {
	p := readProblem("", err)
	p.File = label
	return p
}

// fromBrain finds the single workflow note at one brain level and takes it.
func (a *Answer) fromBrain(home, level, folder, source, project string) bool {
	notes, err := workflowNotes(home, folder, level, &a.Errors)
	if err != nil {
		a.Errors = append(a.Errors, readFailure("brain:"+level, err))
		return false
	}
	switch len(notes) {
	case 0:
		return false
	case 1:
	default:
		a.Errors = append(a.Errors, Problem{File: "brain:" + level, Message: "more than one workflow note; keep one"})
		return false
	}
	n := notes[0]
	return a.take(source, n.name, "brain:"+level+"/"+n.name, n.text, project)
}

type workflowNote struct{ name, text string }

// workflowNotes lists accepted brain notes whose front matter says
// `type: workflow`, below knowledge/<topic>/<name>.md of one level (folder is a
// project folder name, or empty for the Grid home). That is the layout
// brain.Store scans. The note's `name:` must be its file name, as the brain
// requires. brain.Store itself parses front matter strictly and would refuse
// the policy keys a workflow note carries, so this reads the files with the
// same safefile and mdfile calls it uses and only looks at type, name and status.
//
// A note whose front matter is not valid YAML but has a `type: workflow` line
// adds a Problem (labelled with level) to problems and is skipped alone.
func workflowNotes(home, folder, level string, problems *[]Problem) ([]workflowNote, error) {
	root, err := os.OpenRoot(home)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if folder != "" {
		if !brain.ValidName(folder) {
			return nil, nil
		}
		prefix := filepath.Join("projects", folder)
		if err := safefile.NoSymlinks(root, prefix); err != nil {
			return nil, err
		}
		level, err := root.OpenRoot(prefix)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		defer level.Close()
		root = level
	}
	if err := safefile.NoSymlinks(root, "knowledge"); err != nil {
		return nil, err
	}
	topics, err := fs.ReadDir(root.FS(), "knowledge")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var found []workflowNote
	for _, topic := range topics {
		if !topic.IsDir() || !brain.ValidName(topic.Name()) {
			continue
		}
		dir := "knowledge/" + topic.Name()
		files, err := fs.ReadDir(root.FS(), dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".md" || f.Name() == "TOPIC.md" {
				continue
			}
			data, err := safefile.Read(root, dir+"/"+f.Name())
			if err != nil {
				return nil, err
			}
			stem := strings.TrimSuffix(f.Name(), ".md")
			head, ok, loadErr := noteHead(data)
			if loadErr != nil {
				// Front matter that is not YAML but says it is a workflow
				// note: say so, and skip only this file.
				if claimsWorkflow(data) {
					*problems = append(*problems, readFailure("brain:"+level+"/"+stem, loadErr))
				}
				continue
			}
			if !ok || head.Type != NoteType {
				continue
			}
			if head.Status != "" && head.Status != "accepted" {
				continue
			}
			if head.Name != stem || !brain.ValidName(stem) {
				return nil, fmt.Errorf("workflow note %s: name must match its file", f.Name())
			}
			found = append(found, workflowNote{name: stem, text: string(data)})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].name < found[j].name })
	return found, nil
}

type noteFront struct {
	Type   string `yaml:"type"`
	Name   string `yaml:"name"`
	Status string `yaml:"status"`
}

// noteHead reads only the fields that pick a workflow note. A file that is not
// a note at all (no front matter) is simply not a workflow: ok is false. Front
// matter that is present but not valid YAML comes back as the error.
func noteHead(data []byte) (head noteFront, ok bool, err error) {
	front, _, splitErr := mdfile.Split(data)
	if splitErr != nil {
		return noteFront{}, false, nil
	}
	if err := mdfile.Load(front, &head, false); err != nil {
		return noteFront{}, false, err
	}
	return head, true, nil
}

// claimsWorkflow says whether unparseable front matter still has a
// `type: workflow` line, so the note was meant as a workflow.
func claimsWorkflow(data []byte) bool {
	front, _, err := mdfile.Split(data)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(front), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "type:") {
			continue
		}
		v := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "type:")), `"'`)
		if v == NoteType {
			return true
		}
	}
	return false
}
