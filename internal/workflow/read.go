package workflow

// Answer is the workflows read response. Empty collections encode as arrays.
type Answer struct {
	Files     []File     `json:"files"`
	Templates []Template `json:"templates"`
	Errors    []Problem  `json:"errors"`
}

// Read returns the workspace file and registered project files, or just the
// workspace and selected project (name or ID). File and parse failures remain
// in Errors; home resolution and project selection failures return an error.
func Read(home, project string) (Answer, error) {
	files, problems, err := readFiles(home, project)
	if err != nil {
		return Answer{}, err
	}
	out := Answer{Files: files, Templates: []Template{}, Errors: []Problem{}}
	failed := make(map[int]error, len(problems))
	for _, p := range problems {
		failed[p.index] = p.err
	}
	for i, file := range files {
		project := ""
		if file.Project != nil {
			project = *file.Project
		}
		if err := failed[i]; err != nil {
			out.Errors = append(out.Errors, Problem{File: labelOf(project), Message: fileReason(err)})
			continue
		}
		if file.Rev == nil {
			continue
		}
		parsed := Parse(file.Text, project)
		out.Templates = append(out.Templates, parsed.Templates...)
		out.Errors = append(out.Errors, parsed.Errors...)
	}
	return out, nil
}
