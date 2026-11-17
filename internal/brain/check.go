package brain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"go.yaml.in/yaml/v4"
)

// Finding codes. A finding means the text changes what runs or who may change
// it; it is a fact about the file, never a judgement of the proposal's prose.
const (
	CheckConfigTopic = "config_topic"
	CheckSkillField  = "skill_field"
	CheckSkillShell  = "skill_shell"
	CheckSkillFile   = "skill_file"
	CheckAgentField  = "agent_field"
)

// Finding names one problem. File is scope-qualified like a waiting proposal's
// path; Text is the quoted source text, and consumers must treat it as data.
type Finding struct {
	Code    string `json:"code"`
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Text    string `json:"text,omitempty"`
}

type CheckResult struct {
	Checked int `json:"checked"`
	// Adopted counts the person changes this check recorded (a vault only).
	Adopted  int       `json:"adopted,omitempty"`
	Findings []Finding `json:"findings"`
}

var (
	configProposalTopics = map[string]bool{"workflows": true, "environments": true, "machines": true, "deploy": true, "people": true, "routines": true, "agents": true}
	skillFields          = map[string]bool{"name": true, "description": true, "when_to_use": true, "metadata": true, "topics": true}
	agentFields          = map[string]bool{"name": true, "description": true, "tools": true, "model": true, "effort": true, "maxTurns": true, "metadata": true}
	runningSkillFields   = map[string]bool{"allowed-tools": true, "hooks": true, "shell": true, "context": true, "agent": true, "model": true}
)

// configTopicSteps is the step the config_topic finding names for each config
// topic: a command the plugin has, or where in the Grid app the change is made.
var configTopicSteps = map[string]string{
	"workflows":    "run grid workflow write",
	"environments": "set it with grid setup <project> --answers <file>",
	"deploy":       "set it with grid setup <project> --answers <file>",
	"agents":       "run grid agent set",
	"routines":     "run grid routine add",
	"people":       "people join through the Grid app; grid person show lists them",
	"machines":     "machines join from the Grid app's Devices page",
}

// CheckProposal checks one proposal, whatever its status. It writes nothing.
func (s Store) CheckProposal(level, name string) (CheckResult, error) {
	if !ValidName(name) {
		return CheckResult{}, fmt.Errorf("proposal id must be a plain name")
	}
	root, err := s.scope(level, false)
	if err != nil {
		return CheckResult{}, err
	}
	defer root.Close()
	res := CheckResult{Findings: []Finding{}}
	if err := checkProposal(root, level, name, &res); err != nil {
		return CheckResult{}, err
	}
	return res, nil
}

// Check covers every proposal with status proposed and every skill and agent
// entry at the store's levels. Unreadable or malformed files are errors, as in
// Waiting, so a gate never treats what it could not read as clean.
func (s Store) Check() (CheckResult, error) {
	res := CheckResult{Findings: []Finding{}}
	if s.Vault() {
		if err := s.vaultCheck(&res); err != nil {
			return CheckResult{}, err
		}
	}
	for _, level := range s.levels() {
		root, err := s.scope(level, false)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return CheckResult{}, err
		}
		err = s.checkScope(root, level, &res)
		root.Close()
		if err != nil {
			return CheckResult{}, err
		}
	}
	return res, nil
}

func (s Store) checkScope(root *os.Root, level string, res *CheckResult) error {
	if err := safefile.NoSymlinks(root, "brain-proposals"); err != nil {
		return err
	}
	entries, err := fs.ReadDir(root.FS(), "brain-proposals")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, e := range entries {
		id := strings.TrimSuffix(e.Name(), ".md")
		if id == e.Name() {
			continue
		}
		if !ValidName(id) {
			return fmt.Errorf("proposal %s: name is not a plain name", e.Name())
		}
		n, _, err := readProposalBytes(root, level, id)
		if err != nil {
			return fmt.Errorf("proposal %s: %w", e.Name(), err)
		}
		if n.Status != "proposed" {
			continue
		}
		if err := checkProposal(root, level, id, res); err != nil {
			return fmt.Errorf("proposal %s: %w", e.Name(), err)
		}
	}
	if err := checkSkills(root, level, res); err != nil {
		return err
	}
	names, err := s.ConfigNames(level, "agents")
	if err != nil {
		return err
	}
	for _, name := range names {
		rel := "knowledge/agents/" + name + ".md"
		b, err := safefile.Read(root, filepath.FromSlash(rel))
		if err != nil {
			return fmt.Errorf("agent %s: %w", name, err)
		}
		res.Checked++
		keys, err := frontKeys(b)
		if err != nil {
			return fmt.Errorf("agent %s: %w", name, err)
		}
		for _, k := range keys {
			if !agentFields[k.name] {
				res.add(CheckAgentField, scoped(level, rel), k.line, "agent front matter key is outside name, description, tools, model, effort, maxTurns, metadata", k.name)
			}
		}
	}
	if AuditHashCheck(level) {
		return auditDrift(root, level, res)
	}
	return nil
}

func (r *CheckResult) add(code, file string, line int, msg, text string) {
	r.Findings = append(r.Findings, Finding{Code: code, File: file, Line: line, Message: msg, Text: text})
}

func scoped(level, rel string) string { return "brain:" + level + "/" + rel }

func readProposalBytes(root *os.Root, level, name string) (Note, []byte, error) {
	b, err := safefile.Read(root, filepath.Join("brain-proposals", name+".md"))
	if err != nil {
		return Note{}, nil, err
	}
	n, err := Parse(b)
	if err != nil {
		return Note{}, nil, err
	}
	if n.Name != name || n.Level != level {
		return Note{}, nil, fmt.Errorf("proposal name or level does not match its location")
	}
	return n, b, nil
}

func checkProposal(root *os.Root, level, name string, res *CheckResult) error {
	n, b, err := readProposalBytes(root, level, name)
	if err != nil {
		return err
	}
	return ProposalCheck(level, name, n, b, res)
}

// ProposalCheck checks one proposal from the note parsed out of b, so a caller
// that holds the bytes (Accept, under the level lock) checks what it read and
// not a second read. It is replaceable so a test can make a proposal fail.
var ProposalCheck = checkProposalBytes

func checkProposalBytes(level, name string, n Note, b []byte, res *CheckResult) error {
	res.Checked++
	topic := strings.TrimSpace(n.Topic)
	if configProposalTopics[topic] {
		line := 0
		for i, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "topic:") {
				line = i + 1
				break
			}
		}
		res.add(CheckConfigTopic, scoped(level, "brain-proposals/"+name+".md"), line, "config: "+configTopicSteps[topic], topic)
	}
	return nil
}

type frontKey struct {
	name string
	line int // 1-based line in the file, not in the front matter
}

// frontKeys lists the top-level front matter keys with their file lines.
func frontKeys(data []byte) ([]frontKey, error) {
	front, _, err := mdfile.Split(data)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := mdfile.Load(front, &doc, false); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("front matter must be a mapping")
	}
	var keys []frontKey
	for i := 0; i+1 < len(m.Content); i += 2 {
		keys = append(keys, frontKey{m.Content[i].Value, m.Content[i].Line + 1})
	}
	return keys, nil
}

const skillsDir = "knowledge/skills"

func checkSkills(root *os.Root, level string, res *CheckResult) error {
	info, err := root.Lstat(skillsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		res.Checked++
		res.add(CheckSkillFile, scoped(level, skillsDir), 0, "the skills folder must be a plain folder, not a symlink or file", "")
		return nil
	}
	entries, err := fs.ReadDir(root.FS(), skillsDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		rel := skillsDir + "/" + e.Name()
		switch {
		case e.Type()&os.ModeSymlink != 0:
			res.Checked++
			res.add(CheckSkillFile, scoped(level, rel), 0, "symlinks are not allowed in a skill", "")
		case e.IsDir():
			res.Checked++
			if err := checkSkill(root, level, rel, res); err != nil {
				return fmt.Errorf("skill %s: %w", e.Name(), err)
			}
		}
	}
	return nil
}

func checkSkill(root *os.Root, level, dir string, res *CheckResult) error {
	var files []string
	var walk func(string) error
	walk = func(d string) error {
		entries, err := fs.ReadDir(root.FS(), d)
		if err != nil {
			return err
		}
		for _, e := range entries {
			p := d + "/" + e.Name()
			rel := strings.TrimPrefix(p, dir+"/")
			if e.Type()&os.ModeSymlink != 0 {
				res.add(CheckSkillFile, scoped(level, p), 0, "symlinks are not allowed in a skill", "")
				continue
			}
			if e.IsDir() {
				switch e.Name() {
				case "hooks", "bin":
					res.add(CheckSkillFile, scoped(level, p), 0, e.Name()+"/ can run code and is not allowed in a skill", "")
					continue
				}
				if err := walk(p); err != nil {
					return err
				}
				continue
			}
			info, err := e.Info()
			if err != nil {
				return err
			}
			switch {
			case info.Mode()&0o111 != 0:
				res.add(CheckSkillFile, scoped(level, p), 0, "executable files are not allowed in a skill", "")
			case rel == ".mcp.json":
				res.add(CheckSkillFile, scoped(level, p), 0, ".mcp.json adds tool servers and is not allowed in a skill", "")
			case rel == "agents/openai.yaml":
				res.add(CheckSkillFile, scoped(level, p), 0, "agents/openai.yaml is not allowed in a skill", "")
			case rel == "SKILL.md":
				files = append(files, p)
			case path.Dir(rel) == "reference" && strings.HasSuffix(rel, ".md"):
				files = append(files, p)
			default:
				res.add(CheckSkillFile, scoped(level, p), 0, "only SKILL.md and reference/*.md belong in a skill", "")
			}
		}
		return nil
	}
	if err := walk(dir); err != nil {
		return err
	}
	sort.Strings(files)
	for _, p := range files {
		b, err := safefile.Read(root, filepath.FromSlash(p))
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, "/SKILL.md") && path.Dir(p) == dir {
			keys, err := frontKeys(b)
			if err != nil {
				return err
			}
			for _, k := range keys {
				switch {
				case runningSkillFields[k.name]:
					res.add(CheckSkillField, scoped(level, p), k.line, "skill front matter key can run code or widen permissions", k.name)
				case !skillFields[k.name]:
					res.add(CheckSkillField, scoped(level, p), k.line, "skill front matter key is outside name, description, when_to_use, metadata, topics", k.name)
				}
			}
		}
		for i, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
			t := strings.TrimLeft(l, " \t")
			if strings.HasPrefix(t, "!`") || strings.HasPrefix(t, "```!") {
				res.add(CheckSkillShell, scoped(level, p), i+1, "a skill line that starts a shell command", l)
			}
		}
	}
	return nil
}
