package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	answers, err := parseValues(data)
	if err != nil {
		return nil, err
	}
	return answers, answersRule(answers)
}

func answersRule(answers SetupAnswers) error {
	if answers["deployCommand"] != "" && answers["deploys"] != "Deploy from this machine" {
		return fmt.Errorf("deployCommand requires deploys: Deploy from this machine")
	}
	return nil
}

// parseValues checks each key and value, not the rules between keys.
func parseValues(data []byte) (SetupAnswers, error) {
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

// commandKeys are the answers that name a command Grid runs for the project. An agent's
// record holds them in setup-pending.json until a person approves them.
var commandKeys = []string{"deployCommand", "starts", "stops"}

// AgentSetup is what an agent's record did: the answers it wrote and the command keys it
// held for a person, with the hash the person approves.
type AgentSetup struct {
	Recorded, Held SetupAnswers
	Hash           string
}

// RecordAgentSetup is RecordSetup for an agent. The answers replace the recorded ones,
// except the command keys: an approved command stays as it is, and a new or changed one is
// held in setup-pending.json, which this record replaces or, with nothing held, removes.
func (s Store) RecordAgentSetup(data []byte) (AgentSetup, error) {
	answers, err := ParseAnswers(data)
	if err != nil {
		return AgentSetup{}, err
	}
	dir, err := s.setupDirectory()
	if err != nil {
		return AgentSetup{}, err
	}
	out := AgentSetup{Recorded: SetupAnswers{}, Held: SetupAnswers{}}
	err = s.locked(func() error {
		current, err := readAnswers(filepath.Join(dir, "setup-answers.json"))
		if err != nil {
			return err
		}
		next := SetupAnswers{}
		for key, value := range answers {
			if !slices.Contains(commandKeys, key) {
				next[key] = value
				out.Recorded[key] = value
			} else if current[key] != value {
				out.Held[key] = value
			}
		}
		for _, key := range commandKeys {
			if value, ok := current[key]; ok {
				next[key] = value
			}
		}
		// Taking a command away runs nothing, so a move off local deploys drops the approved one.
		if next["deploys"] != "Deploy from this machine" {
			delete(next, "deployCommand")
		}
		if err := answersRule(next); err != nil {
			return err
		}
		if err := writeAnswers(filepath.Join(dir, "setup-answers.json"), next); err != nil {
			return err
		}
		pending := filepath.Join(dir, "setup-pending.json")
		if len(out.Held) == 0 {
			if err := os.Remove(pending); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		out.Hash = answersHash(out.Held)
		return writeAnswers(pending, out.Held)
	})
	return out, err
}

// PendingSetup returns the recorded answers, the command keys that wait for a person and
// their hash; the hash is empty when nothing waits.
func (s Store) PendingSetup() (answers, pending SetupAnswers, hash string, err error) {
	dir, err := s.setupDirectory()
	if err != nil {
		return nil, nil, "", err
	}
	if answers, err = readAnswers(filepath.Join(dir, "setup-answers.json")); err != nil {
		return nil, nil, "", err
	}
	if pending, err = readAnswers(filepath.Join(dir, "setup-pending.json")); err != nil {
		return nil, nil, "", err
	}
	if len(pending) > 0 {
		hash = answersHash(pending)
	}
	return answers, pending, hash, nil
}

// ApproveSetupCommands writes the held command keys into the recorded answers when hash is
// the hash of what waits, so a person approves exactly what they read.
func (s Store) ApproveSetupCommands(hash string) (approved, answers SetupAnswers, err error) {
	dir, err := s.setupDirectory()
	if err != nil {
		return nil, nil, err
	}
	err = s.locked(func() error {
		pendingPath := filepath.Join(dir, "setup-pending.json")
		if approved, err = readAnswers(pendingPath); err != nil {
			return err
		}
		if len(approved) == 0 {
			return fmt.Errorf("no setup commands wait for approval in %s", s.Project)
		}
		if answersHash(approved) != hash {
			return fmt.Errorf("the setup commands waiting for %s are not the ones with hash %s; read them again with grid project settings show %s", s.Project, hash, s.Project)
		}
		if answers, err = readAnswers(filepath.Join(dir, "setup-answers.json")); err != nil {
			return err
		}
		for key, value := range approved {
			answers[key] = value
		}
		if err := answersRule(answers); err != nil {
			return err
		}
		if err := writeAnswers(filepath.Join(dir, "setup-answers.json"), answers); err != nil {
			return err
		}
		return os.Remove(pendingPath)
	})
	return approved, answers, err
}

func (s Store) setupDirectory() (string, error) {
	if s.Project == "" || s.Project == gridhome.Personal {
		return "", fmt.Errorf("setup requires a registered project")
	}
	return s.directory()
}

// readAnswers reads an answers or pending file; a missing file is no answers.
func readAnswers(path string) (SetupAnswers, error) {
	b, err := read(path)
	if errors.Is(err, os.ErrNotExist) {
		return SetupAnswers{}, nil
	}
	if err != nil {
		return nil, err
	}
	answers, err := parseValues(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return answers, nil
}

func writeAnswers(path string, answers SetupAnswers) error {
	b, err := json.MarshalIndent(answers, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(b, '\n'), 0o644)
}

// answersHash is the sha256 of the answers as JSON, whose keys encoding/json sorts.
func answersHash(answers SetupAnswers) string {
	b, _ := json.Marshal(answers)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
