package settings

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// SetupAnswers records accepted answers, not proposals or instructions to run.
// The keys are the legacy setup skill's setting fields. Free text is preserved.
type SetupAnswers map[string]string

var answerKeys = []string{"account", "deployCommand", "deploys", "description", "docsFrom", "effort", "lands", "long", "model", "starts", "stops", "test"}
var answerEnums = map[string][]string{
	"model": models, "effort": efforts,
	"lands":   {"Open a pull request", "Draft pull request", "Push the branch", "Merge, don't push", "Merge, then push", "Ask each time"},
	"deploys": {"Don't deploy", "Deploy from this machine", "CI/CD deploys"},
	"long":    {"Any time", "Overnight only", "Ask first"},
}

// ParseAnswers validates the entire file before anything is written.
func ParseAnswers(data []byte) (SetupAnswers, error) {
	raw, err := object(data, answerKeys)
	if err != nil {
		return nil, err
	}
	answers := SetupAnswers{}
	for _, key := range keysOf(raw) {
		value, err := textValue(raw[key])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		if key == "model" {
			value = strings.ToLower(value)
		}
		if choices, ok := answerEnums[key]; ok {
			if err := allowed(key, value, choices); err != nil {
				return nil, err
			}
		}
		answers[key] = value
	}
	if answers["deployCommand"] != "" && answers["deploys"] != "Deploy from this machine" {
		return nil, fmt.Errorf("deployCommand requires deploys: Deploy from this machine")
	}
	return answers, nil
}

// RecordSetup replaces the accepted answers as one atomic file. It deliberately
// does not edit project.md, execute commands, send invitations or install tools.
func (s Store) RecordSetup(data []byte) (SetupAnswers, error) {
	answers, err := ParseAnswers(data)
	if err != nil {
		return nil, err
	}
	if s.Project == "" || s.Project == gridhome.Personal {
		return nil, fmt.Errorf("setup requires a registered project")
	}
	dir, err := s.directory()
	if err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(answers, "", "  ")
	if err != nil {
		return nil, err
	}
	err = s.locked(func() error {
		return atomicfile.Write(filepath.Join(dir, "setup-answers.json"), append(b, '\n'), 0o644)
	})
	return answers, err
}
