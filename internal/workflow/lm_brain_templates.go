package workflow

import (
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// lmSbBrainTemplates lists the templates of the brain workflow notes in scope,
// in the order they win: the project note's (folder is the project folder name
// the file reader resolved; empty reads no project note), then the home note's.
// Access is ReadFor's: the project note only for a project the caller may see,
// the home note only for a caller who may see the home brain.
//
// Each level goes through fromBrain and take, the reader ReadFor and the gate
// roles use. A note that fromBrain rejects (unreadable, front matter that
// fails, more than one workflow note at the level) adds nothing, as in
// brainGatePostRoles. A taken note adds its templates and its own problems,
// labelled brain:<level>/<name>; problems of other files at the level stay out.
//
// A note is parsed as the workspace file is (project ""), whatever its level:
// it may hold a default template, its body opens with "# Workflows", and it
// cannot copy a workspace workflow or say "only for project".
func lmSbBrainTemplates(r where.Resolver, folder string) ([]Template, []Problem) {
	templates, problems := []Template{}, []Problem{}
	if r.GridHome == "" {
		return templates, problems
	}
	home := gridhome.Real(r.GridHome)
	add := func(level, folder, source string) {
		var note Answer
		if !note.fromBrain(home, level, folder, source, "") {
			return
		}
		templates = append(templates, note.Templates...)
		label := "brain:" + level + "/" + note.Note
		for _, p := range note.Errors {
			if p.File == label {
				problems = append(problems, p)
			}
		}
	}
	if folder != "" && r.PersonalProjectAllowed(folder) && r.GuestProjectAllowed(folder) {
		add("project", folder, SourceProjectBrain)
	}
	if !r.Caller.IsGuest() && r.PersonalOK() {
		add("home", "", SourceHomeBrain)
	}
	return templates, problems
}

// lmSbNoteFolder is the project folder whose note read lists: the selected
// project's, as readFiles resolved it from a name or an ID. With no selector
// (every project listed) or a Personal selector (no project file) it is empty.
func lmSbNoteFolder(selector string, files []File) string {
	if selector == "" {
		return ""
	}
	for _, f := range files {
		if f.Scope == "project" && f.Project != nil {
			return *f.Project
		}
	}
	return ""
}

// lmSbNoteDefault is the default template of the project note, else the home
// note's, in answer order; nil when no note in scope holds one.
func lmSbNoteDefault(scope Answer) *Template {
	for _, prefix := range []string{"brain:project/", "brain:home/"} {
		for i, t := range scope.Templates {
			if t.Default && strings.HasPrefix(t.File, prefix) {
				return &scope.Templates[i]
			}
		}
	}
	return nil
}
