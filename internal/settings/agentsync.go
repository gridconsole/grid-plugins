package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
)

// AgentTemplatesDir is the plugin folder that holds the agent templates. Claude
// does not load it as agents; sync renders each template into the person's own
// agents folder, so a plugin update never replaces a rendered file.
const AgentTemplatesDir = "agent-templates"

// The states of one agent file after a sync.
const (
	SyncWritten   = "written"   // the file was missing or out of date and was rewritten
	SyncUnchanged = "unchanged" // the file already holds this template and these values
	SyncYours     = "yours"     // a file of that name without the Grid marker is the person's; it is never touched
)

// SyncOptions names the three places a sync reads and writes.
type SyncOptions struct {
	Home         string // Grid home; model and effort come from its settings.json overlay
	TemplatesDir string // the plugin's agent-templates folder
	AgentsDir    string // the person's agents folder, normally ~/.claude/agents
}

// SyncResult is one agent file and what the sync did with it.
type SyncResult struct {
	Agent string `json:"agent"`
	Path  string `json:"path"`
	State string `json:"state"`
}

// ClaudeAgentsDir locates the person's user-level agents folder, next to settings.json.
func ClaudeAgentsDir(home, configDir string) (string, error) {
	path, err := ClaudeSettingsPath(home, configDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "agents"), nil
}

var syncName = regexp.MustCompile(`^(Explore|grid-[a-z][a-z0-9-]*)$`)

// SyncAgents renders every template into AgentsDir with the overlay's model and
// effort and the Grid marker (metadata.grid.owner and the template's sha256). A
// file is written only when its rendered bytes differ, so a second run changes
// nothing. A file without the marker is the person's and is never touched.
func SyncAgents(o SyncOptions) ([]SyncResult, error) {
	entries, err := os.ReadDir(o.TemplatesDir)
	if err != nil {
		return nil, err
	}
	doc, err := Store{Home: o.Home}.Effective()
	if err != nil {
		return nil, fmt.Errorf("agent sync: %w", err)
	}
	var names []string
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e.Name(), ".md"); ok && e.Type().IsRegular() {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var out []SyncResult
	for _, name := range names {
		if !syncName.MatchString(name) {
			return out, fmt.Errorf("agent template %s.md: name must be Explore or grid-<role>", name)
		}
		raw, err := os.ReadFile(filepath.Join(o.TemplatesDir, name+".md"))
		if err != nil {
			return out, err
		}
		rendered, err := renderAgent(name, raw, doc.Agents[name])
		if err != nil {
			return out, err
		}
		res := SyncResult{Agent: name, Path: filepath.Join(o.AgentsDir, name+".md")}
		res.State, err = writeRendered(res.Path, rendered)
		if err != nil {
			return out, err
		}
		out = append(out, res)
	}
	return out, nil
}

func writeRendered(path string, rendered []byte) (string, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
	case err != nil:
		return "", err
	case !info.Mode().IsRegular():
		return SyncYours, nil
	default:
		have, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if string(have) == string(rendered) {
			return SyncUnchanged, nil
		}
		if !ownedByGrid(have) {
			return SyncYours, nil
		}
	}
	if err := atomicfile.Write(path, rendered, 0o644); err != nil {
		return "", err
	}
	return SyncWritten, nil
}

// splitFront returns the front matter lines and the body (with its leading newline) of an agent file.
func splitFront(text string) (front []string, body string, ok bool) {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return nil, "", false
	}
	if strings.HasPrefix(rest, "---\n") {
		return nil, rest[3:], true
	}
	head, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return nil, "", false
	}
	return strings.Split(head, "\n"), "\n" + body, true
}

// topKey returns the key of a top-level front matter line, or "".
func topKey(line string) string {
	if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' || line[0] == '#' {
		return ""
	}
	key, _, _ := strings.Cut(line, ":")
	return strings.TrimSpace(key)
}

func renderAgent(name string, raw []byte, ov Agent) ([]byte, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	sum := sha256.Sum256([]byte(text))
	front, body, ok := splitFront(text)
	if !ok {
		return nil, fmt.Errorf("agent template %s.md: no front matter", name)
	}
	var out []string
	var sawName, sawModel, sawEffort bool
	for _, line := range front {
		switch topKey(line) {
		case "name":
			if strings.TrimSpace(strings.TrimPrefix(line, "name:")) != name {
				return nil, fmt.Errorf("agent template %s.md: name must be %s", name, name)
			}
			sawName = true
		case "model":
			sawModel = true
			if ov.Model != "" {
				line = "model: " + ov.Model
			}
		case "effort":
			sawEffort = true
			if ov.Effort != "" {
				line = "effort: " + ov.Effort
			}
		case "metadata", "permissionMode", "hooks", "mcpServers":
			return nil, fmt.Errorf("agent template %s.md: %s is not allowed in a template", name, topKey(line))
		}
		out = append(out, line)
	}
	if !sawName {
		return nil, fmt.Errorf("agent template %s.md: no name", name)
	}
	if !sawModel && ov.Model != "" {
		out = append(out, "model: "+ov.Model)
	}
	if !sawEffort && ov.Effort != "" {
		out = append(out, "effort: "+ov.Effort)
	}
	out = append(out, "metadata:", "  grid:", "    owner: grid", "    template: "+hex.EncodeToString(sum[:]))
	return []byte("---\n" + strings.Join(out, "\n") + "\n---" + body), nil
}

var flowMarker = regexp.MustCompile(`^metadata:\s*\{.*\bowner:\s*["']?grid["']?\s*[,}]`)

// ownedByGrid reports whether an agent file carries metadata.grid.owner: grid.
func ownedByGrid(file []byte) bool {
	front, _, ok := splitFront(strings.ReplaceAll(string(file), "\r\n", "\n"))
	if !ok {
		return false
	}
	in, inGrid, gridIndent := false, false, 0
	for _, line := range front {
		if topKey(line) != "" {
			in, inGrid = topKey(line) == "metadata", false
			if in && flowMarker.MatchString(line) {
				return true
			}
			continue
		}
		if !in || strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		switch {
		case !inGrid && trimmed == "grid:":
			inGrid, gridIndent = true, indent
		case inGrid && indent <= gridIndent:
			inGrid = false
		case inGrid:
			if v, ok := strings.CutPrefix(trimmed, "owner:"); ok {
				return strings.Trim(strings.TrimSpace(v), `"'`) == "grid"
			}
		}
	}
	return false
}
